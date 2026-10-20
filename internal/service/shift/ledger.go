package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
)

// Shift is a shift as it stands: its float, its history, the cash its
// sales put through the drawer, and, once counted, the count and how it
// reconciled.
type Shift struct {
	ID         int64        `json:"id"`
	PublicID   string       `json:"public_id"`
	Status     shift.Status `json:"status"`
	TradingDay string       `json:"trading_day"`
	OpenedBy   int64        `json:"opened_by"`
	OpenedAt   time.Time    `json:"opened_at"`
	ClosedBy   int64        `json:"closed_by"`
	ClosedAt   time.Time    `json:"closed_at"` // zero while open
	Float      shift.Count  `json:"float"`
	Events     []Event      `json:"events"`
	// Sales is the cash the shift's tenders, voids and refunds put through
	// the drawer.
	Sales shift.CashSales `json:"sales"`
	// Expected is what the drawer should hold: the float, the movements
	// and the sales. Before the blind count nobody at the till may see it
	// (ADR-095); the API serves it only once Closing is set. After the
	// count it is the figure recorded with it.
	Expected money.Amount `json:"expected"`
	// Closing is the blind count and its reconciliation; nil until counted.
	Closing *Closing `json:"closing,omitempty"`
	// Reason explains the variance; nil until given.
	Reason *VarianceReason `json:"reason,omitempty"`
	// ZReport is the number of the shift's Z-report as it now stands, its
	// latest correction or the original; 0 while the shift is open.
	ZReport int64 `json:"z_report,omitempty"`
}

// Closing is the blind count at close, as recorded.
type Closing struct {
	Count          shift.Count          `json:"count"`
	Reconciliation shift.Reconciliation `json:"reconciliation"`
	CountedBy      int64                `json:"counted_by"`
	CountedAt      time.Time            `json:"counted_at"`
}

// VarianceReason is why the drawer was over or short, as first given.
type VarianceReason struct {
	Reason      string    `json:"reason"`
	ExplainedBy int64     `json:"explained_by"`
	ExplainedAt time.Time `json:"explained_at"`
}

// Counted reports whether the drawer has had its closing count.
func (s Shift) Counted() bool { return s.Closing != nil }

// ReadyToClose says why the shift cannot close yet, or nil: it must be
// open, counted, and, with a variance past the threshold, explained.
func (s Shift) ReadyToClose() error {
	switch {
	case s.Status != shift.StatusOpen:
		return fmt.Errorf("%w: %s", ErrShiftClosed, s.PublicID)
	case s.Closing == nil:
		return fmt.Errorf("%w: count the drawer before closing %s", ErrNotCounted, s.PublicID)
	case s.Closing.Reconciliation.NeedsReason() && s.Reason == nil:
		return fmt.Errorf("%w: %s is %s out", ErrVarianceUnexplained, s.PublicID, s.Closing.Reconciliation.Variance)
	}
	return nil
}

// Current is the venue's open shift, or ErrNoOpenShift. A shift counted
// but not yet closed is still the current one.
func (s *Service) Current(ctx context.Context, venueID int64) (Shift, error) {
	row, err := s.db.Queries().OpenShiftForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return Shift{}, ErrNoOpenShift
	}
	if err != nil {
		return Shift{}, fmt.Errorf("reading the open shift: %w", err)
	}
	return s.ByID(ctx, venueID, row.ID)
}

// ByID is one of the venue's shifts, open or closed.
func (s *Service) ByID(ctx context.Context, venueID, shiftID int64) (Shift, error) {
	q := s.db.Queries()
	row, err := shiftAt(ctx, q, venueID, shiftID)
	if err != nil {
		return Shift{}, err
	}
	v, err := venueOf(ctx, q, venueID)
	if err != nil {
		return Shift{}, err
	}
	return load(ctx, q, row, v.currency)
}

// load reads a shift whole, through q: the read pool, or the transaction
// about to write to it.
func load(ctx context.Context, q *gen.Queries, row gen.Shift, currency string) (Shift, error) {
	out := Shift{
		ID: row.ID, PublicID: row.PublicID, Status: shift.Status(row.Status), TradingDay: row.TradingDay,
		OpenedBy: row.OpenedBy, OpenedAt: row.OpenedAt.UTC(), ClosedBy: row.ClosedBy,
	}
	if row.ClosedAt.Valid {
		out.ClosedAt = row.ClosedAt.Time.UTC()
	}
	if err := loadCounts(ctx, q, &out, currency); err != nil {
		return Shift{}, err
	}
	movements, err := loadEvents(ctx, q, &out, currency)
	if err != nil {
		return Shift{}, err
	}
	if out.Sales, err = cashSales(ctx, q, row.ID, currency); err != nil {
		return Shift{}, err
	}
	if out.Closing != nil {
		out.Expected = out.Closing.Reconciliation.Expected
	} else if out.Expected, err = shift.Expected(out.Float, movements, out.Sales); err != nil {
		return Shift{}, fmt.Errorf("shift %d's expected drawer: %w", row.ID, err)
	}
	switch z, err := q.LatestZReportForShift(ctx, row.ID); {
	case err == nil:
		out.ZReport = z.Number
	case !errors.Is(err, sql.ErrNoRows):
		return Shift{}, fmt.Errorf("reading shift %d's Z-report: %w", row.ID, err)
	}
	reason, err := q.VarianceReason(ctx, row.ID)
	switch {
	case err == nil:
		out.Reason = &VarianceReason{Reason: reason.Reason, ExplainedBy: reason.ExplainedBy, ExplainedAt: reason.ExplainedAt.UTC()}
	case !errors.Is(err, sql.ErrNoRows):
		return Shift{}, fmt.Errorf("reading shift %d's variance reason: %w", row.ID, err)
	}
	return out, nil
}

// loadCounts reads the float and, once counted, the closing count, each
// checked as it was checked going in.
func loadCounts(ctx context.Context, q *gen.Queries, sh *Shift, currency string) error {
	rows, err := q.DrawerCountsForShift(ctx, sh.ID)
	if err != nil {
		return fmt.Errorf("reading shift %d's counts: %w", sh.ID, err)
	}
	opened := false
	for _, r := range rows {
		c := shift.Count{Kind: shift.CountKind(r.Kind), Total: money.Amount{Minor: r.Total, Currency: currency}}
		if err := json.Unmarshal([]byte(r.Denominations), &c.Denominations); err != nil {
			return fmt.Errorf("shift %d's %s count: %w", sh.ID, r.Kind, err)
		}
		if err := c.Validate(currency); err != nil {
			return fmt.Errorf("shift %d's %s count: %w", sh.ID, r.Kind, err)
		}
		switch c.Kind {
		case shift.CountOpening:
			sh.Float, opened = c, true
		case shift.CountClosing:
			rec, err := shift.Reconcile(
				money.Amount{Minor: r.Expected, Currency: currency}, c, money.Amount{Minor: r.Threshold, Currency: currency})
			if err != nil {
				return fmt.Errorf("shift %d's closing count: %w", sh.ID, err)
			}
			if rec.Variance.Minor != r.Variance {
				return fmt.Errorf("shift %d's closing count: stored variance %d, the count says %d", sh.ID, r.Variance, rec.Variance.Minor)
			}
			sh.Closing = &Closing{Count: c, Reconciliation: rec, CountedBy: r.CountedBy, CountedAt: r.CountedAt.UTC()}
		}
	}
	if !opened {
		return fmt.Errorf("shift %d has no opening count", sh.ID)
	}
	return nil
}

// loadEvents reads the shift's history, and returns its cash movements.
func loadEvents(ctx context.Context, q *gen.Queries, sh *Shift, currency string) ([]shift.Movement, error) {
	rows, err := q.ShiftEvents(ctx, sh.ID)
	if err != nil {
		return nil, fmt.Errorf("reading shift %d's events: %w", sh.ID, err)
	}
	sh.Events = make([]Event, 0, len(rows))
	movements := []shift.Movement{}
	for _, r := range rows {
		amount, err := money.New(r.Amount, currency)
		if err != nil {
			return nil, fmt.Errorf("shift event %d: %w", r.ID, err)
		}
		e := Event{
			ID: r.ID, Type: EventType(r.Type), Amount: amount, Reason: r.Reason,
			ActorID: r.ActorID, AuthorisedBy: r.AuthorisedBy, DeviceID: r.DeviceID, At: r.At.UTC(),
		}
		sh.Events = append(sh.Events, e)
		if e.Type == EventCashIn || e.Type == EventCashOut {
			movements = append(movements, shift.Movement{
				Kind: shift.MovementKind(e.Type), Amount: amount, Reason: e.Reason, AuthorisedBy: e.AuthorisedBy,
			})
		}
	}
	return movements, nil
}

// cashSales reads the orders touched in the shift and adds up their cash.
func cashSales(ctx context.Context, q *gen.Queries, shiftID int64, currency string) (shift.CashSales, error) {
	orders, err := stampedOrders(ctx, q, shiftID)
	if err != nil {
		return shift.CashSales{}, err
	}
	sales, err := shift.CashFromOrders(currency, shiftID, orders)
	if err != nil {
		return shift.CashSales{}, fmt.Errorf("shift %d's cash sales: %w", shiftID, err)
	}
	return sales, nil
}

// stampedOrders is the whole streams of the orders with any event in the
// shift, each event with the shift it was recorded in.
func stampedOrders(ctx context.Context, q *gen.Queries, shiftID int64) ([][]shift.StampedEvent, error) {
	rows, err := q.OrderEventsTouchingShift(ctx, shiftID)
	if err != nil {
		return nil, fmt.Errorf("reading shift %d's orders: %w", shiftID, err)
	}
	var orders [][]shift.StampedEvent
	last := int64(0)
	for _, r := range rows {
		p, err := order.DecodePayload(order.Type(r.Type), json.RawMessage(r.Payload))
		if err != nil {
			return nil, fmt.Errorf("order %d, seq %d: %w", r.OrderID, r.Seq, err)
		}
		if r.OrderID != last || len(orders) == 0 {
			orders = append(orders, nil)
			last = r.OrderID
		}
		e := order.Event{Seq: r.Seq, Type: order.Type(r.Type), At: r.At.UTC(), ActorID: r.ActorID, DeviceID: r.DeviceID, Payload: p}
		orders[len(orders)-1] = append(orders[len(orders)-1], shift.StampedEvent{Event: e, ShiftID: r.ShiftID})
	}
	return orders, nil
}
