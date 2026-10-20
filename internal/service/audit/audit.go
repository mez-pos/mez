// Package audit is the trail of corrections an owner reads (phase 10,
// Task 4c, ADR-117): every void, comp, discount, refund, payment void,
// reopen and tip a manager let through over the venue's ceiling (ADR-261),
// with who did it, who authorised it, why, and what it was worth.
//
// The trail is the order events, read where they are: there is no second
// copy to drift from them. What each was worth comes from its order's
// projection, so it is the figure the bill itself used.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// Kind is a kind of correction.
type Kind string

// The kinds, each one order event type. TipApproved is a payment whose tip
// a manager let through (ADR-261): the queries take a payment.tendered only
// when it names who authorised it.
const (
	Void        Kind = "void"
	Comp        Kind = "comp"
	Discount    Kind = "discount"
	Refund      Kind = "refund"
	PaymentVoid Kind = "payment_void"
	Reopen      Kind = "reopen"
	TipApproved Kind = "tip_approved"
)

var eventOf = map[Kind]order.Type{
	Void: order.TypeLineVoided, Comp: order.TypeLineComped, Discount: order.TypeDiscountApplied,
	Refund: order.TypeRefundIssued, PaymentVoid: order.TypePaymentVoided, Reopen: order.TypeOrderReopened,
	TipApproved: order.TypePaymentTendered,
}

// Kinds is every kind, for the generated vocabulary.
func Kinds() []Kind { return []Kind{Void, Comp, Discount, Refund, PaymentVoid, Reopen, TipApproved} }

func kindOf(t string) Kind {
	for k, et := range eventOf {
		if string(et) == t {
			return k
		}
	}
	return ""
}

// The refusals.
var (
	ErrStaffNotFound = errors.New("no such staff member at this venue")
	ErrBadFilter     = errors.New("the filter cannot be read")
)

// MaxPage is the most entries one page returns.
const MaxPage = 100

// MaxDays is the longest range one query may cover.
const MaxDays = 366

// Orders is what the trail needs of the order service: an order's state.
type Orders interface {
	Order(ctx context.Context, venueID, orderID int64) (svcorder.Result, error)
}

// Service reads the trail.
type Service struct {
	db     *db.DB
	orders Orders
}

// New returns a Service.
func New(d *db.DB, orders Orders) (*Service, error) {
	if d == nil || orders == nil {
		return nil, errors.New("audit: a database and the order service are required")
	}
	return &Service{db: d, orders: orders}, nil
}

// Filter is what the owner asks for. From and To are trading days,
// "2026-10-01", inclusive: a trading day runs from 04:00 to 04:00 in the
// venue's timezone, as the shift and the Z-report count it. StaffID is a
// public staff id, or empty for anyone; it matches who did it and who
// authorised it. Kind is empty for every kind.
type Filter struct {
	From    string `json:"from"`
	To      string `json:"to"`
	StaffID string `json:"staff_id,omitempty"`
	Kind    Kind   `json:"kind,omitempty"`
	Before  int64  `json:"before,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// Person is someone on an entry: their public id and name.
type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Entry is one correction.
type Entry struct {
	ID      int64     `json:"id"`
	Kind    Kind      `json:"kind"`
	At      time.Time `json:"at"`
	OrderID string    `json:"order_id"`
	By      *Person   `json:"by,omitempty"`
	// AuthorisedBy is who let it through; absent when it was not recorded
	// (a discount or refund before ADR-117).
	AuthorisedBy *Person `json:"authorised_by,omitempty"`
	Reason       string  `json:"reason,omitempty"`
	// Item is the line it was about, or empty for the whole bill.
	Item string `json:"item,omitempty"`
	Qty  int64  `json:"qty,omitempty"`
	// Amount is what it was worth; absent for a reopen. For a tip approved
	// it is the tip, not the payment.
	Amount *money.Amount `json:"amount,omitempty"`
}

// Page is a page of the trail, and how many of each kind the filter finds.
type Page struct {
	Entries []Entry `json:"entries"`
	// NextBefore is passed back as Before for the next page; 0 when none.
	NextBefore int64          `json:"next_before,omitempty"`
	Counts     map[Kind]int64 `json:"counts"`
}

// Trail reads a page of corrections.
func (s *Service) Trail(ctx context.Context, venueID int64, f Filter) (Page, error) {
	q := s.db.Queries()
	ask, err := asked(ctx, q, venueID, f)
	if err != nil {
		return Page{}, err
	}
	p := ask.params
	counts, err := q.AuditCounts(ctx, gen.AuditCountsParams{VenueID: venueID, FromAt: p.FromAt, ToAt: p.ToAt, StaffID: p.StaffID})
	if err != nil {
		return Page{}, fmt.Errorf("audit: counting the trail: %w", err)
	}

	// Every kind, none included, so a reader need not tell absent from zero.
	out := Page{Counts: make(map[Kind]int64, len(eventOf))}
	for _, k := range Kinds() {
		out.Counts[k] = 0
	}
	for _, c := range counts {
		out.Counts[kindOf(c.Type)] = c.N
	}
	out.Entries, out.NextBefore, err = s.entries(ctx, q, ask, map[int64]order.State{})
	if err != nil {
		return Page{}, err
	}
	return out, nil
}

// asking is a filter made ready to read with: the query's parameters, the
// page size, and the venue's staff to name people from.
type asking struct {
	params gen.AuditEventsParams
	limit  int
	staff  people
}

// asked checks a filter and makes it ready.
func asked(ctx context.Context, q *gen.Queries, venueID int64, f Filter) (asking, error) {
	from, to, err := rangeOf(ctx, q, venueID, f.From, f.To)
	if err != nil {
		return asking{}, err
	}
	staff, err := staffOf(ctx, q, venueID)
	if err != nil {
		return asking{}, err
	}
	staffID := int64(0)
	if f.StaffID != "" {
		p, ok := staff.byPublic[f.StaffID]
		if !ok {
			return asking{}, fmt.Errorf("%w: %s", ErrStaffNotFound, f.StaffID)
		}
		staffID = p
	}
	kind := ""
	if f.Kind != "" {
		et, ok := eventOf[f.Kind]
		if !ok {
			return asking{}, fmt.Errorf("%w: kind %q", ErrBadFilter, f.Kind)
		}
		kind = string(et)
	}
	limit := f.Limit
	if limit <= 0 || limit > MaxPage {
		limit = MaxPage
	}
	return asking{
		params: gen.AuditEventsParams{
			VenueID: venueID, FromAt: from, ToAt: to, Kind: kind, StaffID: staffID, BeforeID: f.Before, MaxRows: int64(limit + 1),
		},
		limit: limit, staff: staff,
	}, nil
}

// entries reads one page, newest first, and the cursor for the next, 0 when
// there is none. states holds the orders already projected, so a caller
// reading page after page projects each order once.
func (s *Service) entries(ctx context.Context, q *gen.Queries, ask asking, states map[int64]order.State) ([]Entry, int64, error) {
	rows, err := q.AuditEvents(ctx, ask.params)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: reading the trail: %w", err)
	}
	next := int64(0)
	if len(rows) > ask.limit {
		rows = rows[:ask.limit]
		next = rows[ask.limit-1].ID
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		st, ok := states[r.OrderID]
		if !ok {
			res, err := s.orders.Order(ctx, ask.params.VenueID, r.OrderID)
			if err != nil {
				return nil, 0, fmt.Errorf("audit: reading order %s: %w", r.OrderPublicID, err)
			}
			st = res.State
			states[r.OrderID] = st
		}
		e, err := entryOf(r, st, ask.staff)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, next, nil
}

// entryOf words one event, with what its order's projection says it was worth.
func entryOf(r gen.AuditEventsRow, st order.State, staff people) (Entry, error) {
	e := Entry{ID: r.ID, Kind: kindOf(r.Type), At: r.At.UTC(), OrderID: r.OrderPublicID, By: staff.person(r.ActorID)}
	var p struct {
		LineID       int64  `json:"line_id"`
		PaymentID    int64  `json:"payment_id"`
		ReasonCode   string `json:"reason_code"`
		Reason       string `json:"reason"`
		AuthorisedBy int64  `json:"authorised_by"`
	}
	if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
		return Entry{}, fmt.Errorf("audit: reading event %d: %w", r.ID, err)
	}
	e.AuthorisedBy = staff.person(p.AuthorisedBy)
	e.Reason = p.Reason
	if p.ReasonCode != "" {
		e.Reason = p.ReasonCode
	}
	line := func(id int64) {
		for _, l := range st.Lines {
			if l.ID == id {
				e.Item, e.Qty = l.Name, l.Qty
				gross := l.Gross
				e.Amount = &gross
			}
		}
	}

	switch e.Kind {
	case Void, Comp:
		line(p.LineID)
	case Discount:
		for _, d := range st.Discounts {
			if d.Seq == r.Seq {
				value := d.Value
				e.Amount = &value
				if d.LineID != 0 {
					line(d.LineID)
					e.Amount = &value // the discount's worth, not the line's
				}
			}
		}
	case Refund:
		for _, rf := range st.Refunds {
			if rf.Seq == r.Seq {
				amount := rf.Amount
				e.Amount = &amount
			}
		}
	case PaymentVoid:
		for _, t := range st.Tenders {
			if t.ID == p.PaymentID {
				amount := t.Amount
				e.Amount = &amount
			}
		}
	case TipApproved:
		// The payment is the event itself: its id is the event's seq.
		for _, t := range st.Tenders {
			if t.ID == r.Seq {
				tip := t.Tip
				e.Amount = &tip
			}
		}
	}
	return e, nil
}

// rangeOf turns trading days into the UTC instants they run between.
func rangeOf(ctx context.Context, q *gen.Queries, venueID int64, fromDay, toDay string) (time.Time, time.Time, error) {
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("audit: reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("audit: venue timezone %q: %w", v.Timezone, err)
	}
	dayStart, err := svcshift.DayStart(ctx, q, venueID)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("audit: %w", err)
	}
	h, m := int(dayStart/time.Hour), int(dayStart%time.Hour/time.Minute)
	start := func(day string) (time.Time, error) {
		d, err := time.Parse("2006-01-02", day)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %q is not a day like 2026-10-01", ErrBadFilter, day)
		}
		return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc), nil
	}
	from, err := start(fromDay)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	last, err := start(toDay)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to := last.AddDate(0, 0, 1)
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: the range ends before it starts", ErrBadFilter)
	}
	if to.Sub(from) > MaxDays*24*time.Hour+time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: ask for a year or less at a time", ErrBadFilter)
	}
	return from.UTC(), to.UTC(), nil
}

// people is everyone who has worked at the venue, by internal and public id.
type people struct {
	byID     map[int64]Person
	byPublic map[string]int64
}

func (p people) person(id int64) *Person {
	if id == 0 {
		return nil
	}
	if found, ok := p.byID[id]; ok {
		return &found
	}
	return &Person{Name: "Someone no longer on record"}
}

func staffOf(ctx context.Context, q *gen.Queries, venueID int64) (people, error) {
	rows, err := q.StaffForVenue(ctx, venueID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return people{}, fmt.Errorf("audit: reading staff: %w", err)
	}
	out := people{byID: map[int64]Person{}, byPublic: map[string]int64{}}
	for _, r := range rows {
		out.byID[r.ID] = Person{ID: r.PublicID, Name: r.Name}
		out.byPublic[r.PublicID] = r.ID
	}
	return out, nil
}
