package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
)

// EventType is a line in a shift's history.
type EventType string

// The shift's events, as shift_events stores them.
const (
	EventOpened  EventType = "opened"
	EventCashIn  EventType = EventType(shift.CashIn)
	EventCashOut EventType = EventType(shift.CashOut)
	EventClosed  EventType = "closed"
)

// EventTypes is every event type, for the generated vocabulary.
func EventTypes() []EventType {
	return []EventType{EventOpened, EventCashIn, EventCashOut, EventClosed}
}

func eventType(k shift.MovementKind) EventType { return EventType(k) }

// event is a row about to be appended, and what a retry is compared on.
type event struct {
	Type         EventType
	Amount       int64
	Reason       string
	AuthorisedBy int64
}

// venue is what the shift needs to know of the venue.
type venue struct {
	currency string
	location *time.Location
}

func venueOf(ctx context.Context, q *gen.Queries, venueID int64) (venue, error) {
	row, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return venue{}, fmt.Errorf("%w: no venue %d", ErrVenueConfig, venueID)
	}
	if err != nil {
		return venue{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return venue{}, fmt.Errorf("%w: timezone %q: %v", ErrVenueConfig, row.Timezone, err)
	}
	return venue{currency: row.Currency, location: loc}, nil
}

// isRetry reports whether the request's key already recorded an event in
// this shift. The same key for a different request is refused.
func isRetry(ctx context.Context, q *gen.Queries, shiftID int64, meta Meta, want event) (bool, error) {
	if meta.IdempotencyKey == "" {
		return false, nil
	}
	row, err := q.ShiftEventByKey(ctx, gen.ShiftEventByKeyParams{ShiftID: shiftID, IdempotencyKey: meta.IdempotencyKey})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading idempotency key: %w", err)
	}
	got := event{Type: EventType(row.Type), Amount: row.Amount, Reason: row.Reason, AuthorisedBy: row.AuthorisedBy}
	if got != want || row.ActorID != meta.ActorID {
		return false, fmt.Errorf("%w: %q", ErrIdempotencyKeyReused, meta.IdempotencyKey)
	}
	return true, nil
}

func appendEvent(ctx context.Context, q *gen.Queries, shiftID int64, meta Meta, e event, at time.Time) error {
	_, err := q.AppendShiftEvent(ctx, gen.AppendShiftEventParams{
		ShiftID: shiftID, Type: string(e.Type), Amount: e.Amount, Reason: e.Reason,
		ActorID: meta.ActorID, AuthorisedBy: e.AuthorisedBy, DeviceID: meta.DeviceID,
		IdempotencyKey: meta.IdempotencyKey, At: at,
	})
	if err != nil {
		return fmt.Errorf("recording %s: %w", e.Type, err)
	}
	return nil
}

func saveCount(ctx context.Context, q *gen.Queries, shiftID int64, c shift.Count, by int64, at time.Time) error {
	denominations := c.Denominations
	if denominations == nil {
		denominations = []shift.Denomination{}
	}
	body, err := json.Marshal(denominations)
	if err != nil {
		return fmt.Errorf("encoding the %s count: %w", c.Kind, err)
	}
	err = q.CreateDrawerCount(ctx, gen.CreateDrawerCountParams{
		ShiftID: shiftID, Kind: string(c.Kind), Denominations: string(body), Total: c.Total.Minor, CountedBy: by, CountedAt: at,
	})
	if err != nil {
		return fmt.Errorf("recording the %s count: %w", c.Kind, err)
	}
	return nil
}

// Event is a line of a shift's history, as the back office shows it.
type Event struct {
	ID           int64        `json:"id"`
	Type         EventType    `json:"type"`
	Amount       money.Amount `json:"amount"`
	Reason       string       `json:"reason"`
	ActorID      int64        `json:"actor_id"`
	AuthorisedBy int64        `json:"authorised_by"`
	DeviceID     int64        `json:"device_id"`
	At           time.Time    `json:"at"`
}

// Shift is a shift as it stands: its float, its history, and what the
// drawer should hold from the float and the cash movements alone.
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
	// Drawer is the float plus cash in less cash out. Cash sales and
	// refunds join it with Task 1b; until then it is not the expected
	// count, and nothing shows it to staff.
	Drawer money.Amount `json:"drawer"`
}

// Current is the venue's open shift, or ErrNoOpenShift.
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
	float, err := openingCount(ctx, q, row.ID, v.currency)
	if err != nil {
		return Shift{}, err
	}
	rows, err := q.ShiftEvents(ctx, row.ID)
	if err != nil {
		return Shift{}, fmt.Errorf("reading shift %d's events: %w", row.ID, err)
	}
	events := make([]Event, 0, len(rows))
	movements := []shift.Movement{}
	for _, r := range rows {
		amount, err := money.New(r.Amount, v.currency)
		if err != nil {
			return Shift{}, fmt.Errorf("shift event %d: %w", r.ID, err)
		}
		e := Event{
			ID: r.ID, Type: EventType(r.Type), Amount: amount, Reason: r.Reason,
			ActorID: r.ActorID, AuthorisedBy: r.AuthorisedBy, DeviceID: r.DeviceID, At: r.At.UTC(),
		}
		events = append(events, e)
		if e.Type == EventCashIn || e.Type == EventCashOut {
			movements = append(movements, shift.Movement{
				Kind: shift.MovementKind(e.Type), Amount: amount, Reason: e.Reason, AuthorisedBy: e.AuthorisedBy,
			})
		}
	}
	drawer, err := shift.Drawer(float, movements)
	if err != nil {
		return Shift{}, fmt.Errorf("shift %d's drawer: %w", row.ID, err)
	}
	out := Shift{
		ID: row.ID, PublicID: row.PublicID, Status: shift.Status(row.Status), TradingDay: row.TradingDay,
		OpenedBy: row.OpenedBy, OpenedAt: row.OpenedAt.UTC(), ClosedBy: row.ClosedBy,
		Float: float, Events: events, Drawer: drawer,
	}
	if row.ClosedAt.Valid {
		out.ClosedAt = row.ClosedAt.Time.UTC()
	}
	return out, nil
}

// openingCount reads a shift's float back, checking it as it was checked
// going in.
func openingCount(ctx context.Context, q *gen.Queries, shiftID int64, currency string) (shift.Count, error) {
	rows, err := q.DrawerCountsForShift(ctx, shiftID)
	if err != nil {
		return shift.Count{}, fmt.Errorf("reading shift %d's counts: %w", shiftID, err)
	}
	for _, r := range rows {
		if shift.CountKind(r.Kind) != shift.CountOpening {
			continue
		}
		c := shift.Count{Kind: shift.CountOpening, Total: money.Amount{Minor: r.Total, Currency: currency}}
		if err := json.Unmarshal([]byte(r.Denominations), &c.Denominations); err != nil {
			return shift.Count{}, fmt.Errorf("shift %d's float: %w", shiftID, err)
		}
		if err := c.Validate(currency); err != nil {
			return shift.Count{}, fmt.Errorf("shift %d's float: %w", shiftID, err)
		}
		return c, nil
	}
	return shift.Count{}, fmt.Errorf("shift %d has no opening count", shiftID)
}
