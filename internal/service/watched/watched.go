// Package watched reads the events an owner is told about (ADR-360): a
// line voided or comped, a refund, a credit note and a discount, each with
// what it was worth, who did it and who let it through. mez Cloud reads it
// over a replica of the venue's backup (ADR-362), past a cursor in the
// venue's event stream, and holds each against the owner's thresholds.
//
// What an event was worth is what the order's projection says, as the
// audit trail has it (internal/service/audit): a voided line at its gross,
// a discount at its value, a refund at its amount, a credit note at its
// total. Nothing is recomputed from prices.
package watched

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// DefaultLimit is how many of the venue's events Since reads at one asking
// when told no number, and the most it reads.
const DefaultLimit = 500

// Kind is what happened.
type Kind string

// The kinds, in the words ADR-360 uses.
const (
	Void       Kind = "void"
	Comp       Kind = "comp"
	Refund     Kind = "refund"
	CreditNote Kind = "credit_note"
	Discount   Kind = "discount"
)

// kindOf is the kind of a watched event type, and "" for one not watched.
func kindOf(t order.Type) Kind {
	switch t {
	case order.TypeLineVoided:
		return Void
	case order.TypeLineComped:
		return Comp
	case order.TypeRefundIssued:
		return Refund
	case order.TypeCreditNoteIssued:
		return CreditNote
	case order.TypeDiscountApplied:
		return Discount
	}
	return ""
}

// Person is someone on an event, as the venue's staff table names them.
// Manager says their role holds shift.manage, the permission that closes a
// day, which is what ADR-360 means by "not a cashier". A Person nobody is
// recorded as (a guest's own order, a staff id the table no longer has) is
// the zero value.
type Person struct {
	Name    string
	Manager bool
}

// Event is one thing the owner may be told about.
type Event struct {
	// ID is the event's position in the venue's stream: the cursor.
	ID   int64
	Kind Kind
	// At is when it happened, UTC.
	At time.Time
	// OrderRef is the order's public id, and Table the table it was at,
	// by name, or "" for none.
	OrderRef string
	Table    string
	// Item and Qty are the line it was about, for a void, a comp or a
	// line discount; empty for the whole bill.
	Item string
	Qty  int64
	// Amount is what it was worth, in minor units of Currency.
	Amount   int64
	Currency string
	// Reason is the reason recorded, or the reason code.
	Reason string
	// By is who did it, and AuthorisedBy who let it through, when one was
	// recorded.
	By           Person
	AuthorisedBy Person
}

// Since reads the venue's events after the one with id afterID, oldest
// first, at most limit of them, and answers the watched ones among them
// with the id of the last event read, watched or not: the cursor moves
// past every event looked at. Zero afterID is from the beginning; a limit
// of zero or less, or over DefaultLimit, is DefaultLimit. A last of 0 with
// no events is a stream read to its end.
func Since(ctx context.Context, d *db.DB, venueID, afterID, limit int64) (events []Event, last int64, err error) {
	if limit <= 0 || limit > DefaultLimit {
		limit = DefaultLimit
	}
	rows, err := d.VenueEventsAfter(ctx, venueID, afterID, int(limit))
	if err != nil {
		return nil, 0, fmt.Errorf("watched: reading the events of venue %d: %w", venueID, err)
	}
	if len(rows) == 0 {
		return nil, 0, nil
	}
	q := d.Queries()
	var staff people
	var tables map[int64]string
	var taxes order.Taxes
	states := map[int64]order.State{}
	loaded := false
	out := []Event{}
	for _, r := range rows {
		kind := kindOf(r.Event.Type)
		if kind == "" {
			continue
		}
		if !loaded {
			if staff, err = staffOf(ctx, q, venueID); err != nil {
				return nil, 0, err
			}
			if tables, err = tablesOf(ctx, q, venueID); err != nil {
				return nil, 0, err
			}
			if taxes, err = svcorder.VenueTaxes(ctx, q, venueID); err != nil {
				return nil, 0, fmt.Errorf("watched: %w", err)
			}
			loaded = true
		}
		st, ok := states[r.OrderID]
		if !ok {
			all, err := d.OrderEvents(ctx, r.OrderID, 0)
			if err != nil {
				return nil, 0, fmt.Errorf("watched: reading order %s: %w", r.OrderPublicID, err)
			}
			if st, err = order.Project(all, taxes); err != nil {
				return nil, 0, fmt.Errorf("watched: projecting order %s: %w", r.OrderPublicID, err)
			}
			states[r.OrderID] = st
		}
		e, err := eventOf(r, kind, st, staff, tables)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, rows[len(rows)-1].ID, nil
}

// eventOf words one event, with what its order's projection says it was
// worth.
func eventOf(r db.StreamEvent, kind Kind, st order.State, staff people, tables map[int64]string) (Event, error) {
	e := Event{ID: r.ID, Kind: kind, At: r.Event.At.UTC(), OrderRef: r.OrderPublicID, Table: tables[r.TableID], Currency: st.Currency}
	e.By = staff.person(r.Event.ActorID)
	raw, err := json.Marshal(r.Event.Payload)
	if err != nil {
		return Event{}, fmt.Errorf("watched: reading event %d: %w", r.ID, err)
	}
	var p struct {
		LineID       int64  `json:"line_id"`
		ReasonCode   string `json:"reason_code"`
		Reason       string `json:"reason"`
		AuthorisedBy int64  `json:"authorised_by"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Event{}, fmt.Errorf("watched: reading event %d: %w", r.ID, err)
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
				e.Amount = l.Gross.Minor
			}
		}
	}
	seq := r.Event.Seq
	switch kind {
	case Void, Comp:
		line(p.LineID)
	case Discount:
		for _, d := range st.Discounts {
			if d.Seq == seq {
				if d.LineID != 0 {
					line(d.LineID)
				}
				e.Amount = d.Value.Minor // the discount's worth, not the line's
			}
		}
	case Refund:
		for _, rf := range st.Refunds {
			if rf.Seq == seq {
				e.Amount = rf.Amount.Minor
			}
		}
	case CreditNote:
		for _, c := range st.Credits {
			if c.Seq == seq {
				e.Amount = c.Total.Minor
			}
		}
	}
	return e, nil
}

// people is everyone who has worked at the venue, by id.
type people map[int64]Person

func (p people) person(id int64) Person { return p[id] }

// staffOf reads the venue's staff with whether each one's role holds
// shift.manage.
func staffOf(ctx context.Context, q *gen.Queries, venueID int64) (people, error) {
	roles, err := q.RolesForVenue(ctx, venueID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("watched: reading roles: %w", err)
	}
	manages := map[int64]bool{}
	for _, r := range roles {
		var perms []string
		if err := json.Unmarshal([]byte(r.Permissions), &perms); err != nil {
			// A role whose permissions cannot be read grants nothing, as
			// the till reads it.
			continue
		}
		for _, p := range perms {
			if p == string(auth.PermShiftManage) {
				manages[r.ID] = true
			}
		}
	}
	rows, err := q.StaffForVenue(ctx, venueID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("watched: reading staff: %w", err)
	}
	out := people{}
	for _, s := range rows {
		out[s.ID] = Person{Name: s.Name, Manager: manages[s.RoleID]}
	}
	return out, nil
}

// tablesOf is the venue's tables by id, by name.
func tablesOf(ctx context.Context, q *gen.Queries, venueID int64) (map[int64]string, error) {
	rows, err := q.TablesForVenue(ctx, venueID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("watched: reading tables: %w", err)
	}
	out := make(map[int64]string, len(rows))
	for _, t := range rows {
		out[t.ID] = t.Name
	}
	return out, nil
}
