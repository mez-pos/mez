package order

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// guest is a phone at venue 1: nobody, on no device.
func guest(key string) Meta { return Meta{VenueID: 1, IdempotencyKey: key} }

// guestMenu is what the guest tests order from: a flat white from the bar
// that needs a milk, and a brownie from the kitchen with no choices, in a
// later course.
type guestMenu struct {
	white, brownie int64
	dairy, oat     int64
}

func seedGuestMenu(t *testing.T, d *db.DB) guestMenu {
	t.Helper()
	ctx := context.Background()
	_, err := d.CreateMenu(ctx, 1, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}, {Code: "kitchen", Name: "Kitchen"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Drinks"}, {Number: 2, Name: "Desserts"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food 5%", Rate: tax.RateFromBP(500)}},
		Groups: []db.ModifierGroupSpec{{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
			{Name: "Dairy", IsDefault: true}, {Name: "Oat", Price: 4000},
		}}},
		Categories: []db.CategorySpec{{Name: "Cafe", Items: []db.ItemSpec{
			{Name: "Flat white", Price: 18000, HSN: "996331", TaxClass: "Food 5%", Station: "bar", Course: 1, Groups: []string{"Milk"}},
			{Name: "Brownie", Price: 12000, HSN: "996331", TaxClass: "Food 5%", Station: "kitchen", Course: 2},
		}}},
	}, now)
	if err != nil {
		t.Fatalf("seeding the menu: %v", err)
	}
	m, err := d.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var g guestMenu
	for _, it := range m.Items {
		switch it.Name {
		case "Flat white":
			g.white = it.ID
		case "Brownie":
			g.brownie = it.ID
		}
	}
	for _, mod := range m.Groups[0].Modifiers {
		switch mod.Name {
		case "Dairy":
			g.dairy = mod.ID
		case "Oat":
			g.oat = mod.ID
		}
	}
	return g
}

// twoOatAndABrownie is the round most tests place: 2 x (180 + 40) + 120.
func (g guestMenu) twoOatAndABrownie(table int64) PlaceRound {
	return PlaceRound{TableID: table, Picks: []menu.Pick{
		{ItemID: g.white, ModifierIDs: []int64{g.oat}, Qty: 2},
		{ItemID: g.brownie, Qty: 1},
	}}
}

func orderCount(t *testing.T, d *db.DB) int {
	t.Helper()
	rows, err := d.Queries().UnclosedOrdersForVenue(context.Background(), gen.UnclosedOrdersForVenueParams{VenueID: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

// TestARoundAtAnEmptyTableWaits: with no ticket open the round is a request
// (ADR-209). It is priced and kept, and it is no order, no event and nothing
// for the kitchen.
func TestARoundAtAnEmptyTableWaits(t *testing.T) {
	s, d, rec := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()

	round, err := party(t, s).PlaceRound(ctx, guest("k1"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundWaiting || round.OrderID != 0 || round.Replayed || round.PublicID == "" || round.TableID != 4 {
		t.Errorf("round = %+v", round)
	}
	if round.Subtotal != inr(56000) {
		t.Errorf("subtotal %v, want 560.00", round.Subtotal)
	}
	if len(round.Lines) != 2 || round.Lines[0].Name != "Flat white" || round.Lines[0].Modifiers[0].Name != "Oat" || round.Lines[1].UnitPrice != inr(12000) {
		t.Errorf("lines = %+v", round.Lines)
	}
	if n := orderCount(t, d); n != 0 {
		t.Errorf("%d orders exist: a request is not an order", n)
	}
	if rec.count() != 0 {
		t.Errorf("%d changes published for a request", rec.count())
	}
}

// TestARoundAtASeatedTableGoesStraightThrough: staff opened the ticket, so
// someone is there. The guest's lines join it and go to their stations, and
// a waiter's unsent line on the same ticket does not go with them.
func TestARoundAtASeatedTableGoesStraightThrough(t *testing.T) {
	s, d, rec := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()

	opened := open(t, s) // table 4, by the cashier
	must(t)(s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))
	before := rec.count()

	round, err := party(t, s).PlaceRound(ctx, guest("k1"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundSent || round.OrderID != opened.OrderID {
		t.Fatalf("round = %+v, want it sent on order %d", round, opened.OrderID)
	}
	if orderCount(t, d) != 1 {
		t.Error("a second order was opened: one table, one ticket")
	}
	if rec.count() != before+1 {
		t.Errorf("published %d changes, want one for the round", rec.count()-before)
	}

	res, err := s.Order(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	st := res.State
	// seq 1 opened, 2 the dosa, 3 and 4 the guest's lines, 5 and 6 tickets.
	if len(st.Lines) != 3 || st.LastSeq != 6 {
		t.Fatalf("lines %d, last seq %d", len(st.Lines), st.LastSeq)
	}
	if st.Lines[0].Sent {
		t.Error("the waiter's line went to the kitchen with the guest's round")
	}
	if !st.Lines[1].Sent || !st.Lines[2].Sent {
		t.Error("the guest's lines were not sent")
	}
	want := []order.Ticket{{Station: "bar", LineIDs: []int64{3}}, {Station: "kitchen", LineIDs: []int64{4}}}
	if len(st.Tickets) != 2 || st.Tickets[0].Station != want[0].Station || !reflect.DeepEqual(st.Tickets[0].LineIDs, want[0].LineIDs) ||
		st.Tickets[1].Station != want[1].Station || !reflect.DeepEqual(st.Tickets[1].LineIDs, want[1].LineIDs) {
		t.Errorf("tickets = %+v", st.Tickets)
	}
	// 120 + 440 + 120 = 680.00, plus 5% GST.
	if st.Totals.Total != inr(71400) {
		t.Errorf("total %v, want 714.00", st.Totals.Total)
	}

	// The guest is nobody: actor 0, device 0, and the stream says so.
	h, err := s.OrderHistory(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range h.Events[2:] {
		if e.ActorID != 0 || e.DeviceID != 0 {
			t.Errorf("%s at seq %d is by actor %d on device %d, want the guest", e.Type, e.Seq, e.ActorID, e.DeviceID)
		}
	}
}

// TestARoundIsPlacedOnce: the same key is the same round, whatever happened
// to the table since, and the kitchen gets it once.
func TestARoundIsPlacedOnce(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()
	opened := open(t, s)

	first, err := party(t, s).PlaceRound(ctx, guest("k1"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	again, err := party(t, s).PlaceRound(ctx, guest("k1"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.PublicID != first.PublicID || again.Status != RoundSent || again.OrderID != opened.OrderID ||
		again.Subtotal != first.Subtotal || !reflect.DeepEqual(again.Lines, first.Lines) {
		t.Errorf("again = %+v\nfirst = %+v", again, first)
	}
	res, err := s.Order(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 2 || len(res.State.Tickets) != 2 {
		t.Errorf("%d lines and %d tickets: the round went through twice", len(res.State.Lines), len(res.State.Tickets))
	}

	// The key with something else is not the same request.
	other := g.twoOatAndABrownie(4)
	other.Picks[1].Qty = 2
	if _, err := party(t, s).PlaceRound(ctx, guest("k1"), other); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("err = %v, want the key refused", err)
	}
}

// TestTheSameRoundTwiceAtOnce: a phone that sends twice at the same moment
// still gets one round.
func TestTheSameRoundTwiceAtOnce(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()
	opened := open(t, s)

	const phones = 6
	rounds := make([]Round, phones)
	errs := make([]error, phones)
	var wg sync.WaitGroup
	for i := range phones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rounds[i], errs[i] = party(t, s).PlaceRound(ctx, guest("same"), g.twoOatAndABrownie(4))
		}()
	}
	wg.Wait()
	for i := range phones {
		if errs[i] != nil {
			t.Fatalf("phone %d: %v", i, errs[i])
		}
		if rounds[i].PublicID != rounds[0].PublicID {
			t.Errorf("phone %d got round %s, phone 0 got %s", i, rounds[i].PublicID, rounds[0].PublicID)
		}
	}
	res, err := s.Order(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 2 {
		t.Errorf("%d lines on the ticket, want the round's 2 once", len(res.State.Lines))
	}
}

func TestARoundIsRefused(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, s *Service, d *db.DB, g guestMenu) (Meta, PlaceRound)
		want    error
	}{
		{"with no key", func(_ *testing.T, _ *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			return guest(""), g.twoOatAndABrownie(4)
		}, ErrInvalidCommand},
		{"with nothing in it", func(_ *testing.T, _ *Service, _ *db.DB, _ guestMenu) (Meta, PlaceRound) {
			return guest("k"), PlaceRound{TableID: 4}
		}, ErrInvalidCommand},
		{"with more lines than a cart holds", func(_ *testing.T, _ *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			picks := make([]menu.Pick, MaxRoundLines+1)
			for i := range picks {
				picks[i] = menu.Pick{ItemID: g.brownie, Qty: 1}
			}
			return guest("k"), PlaceRound{TableID: 4, Picks: picks}
		}, ErrInvalidCommand},
		{"for a dish not on the menu", func(_ *testing.T, _ *Service, _ *db.DB, _ guestMenu) (Meta, PlaceRound) {
			return guest("k"), PlaceRound{TableID: 4, Picks: []menu.Pick{{ItemID: 999, Qty: 1}}}
		}, menu.ErrNoSuchItem},
		{"with a required choice left out", func(_ *testing.T, _ *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			return guest("k"), PlaceRound{TableID: 4, Picks: []menu.Pick{{ItemID: g.white, Qty: 1}}}
		}, menu.ErrChoices},
		{"at a table the venue does not have", func(_ *testing.T, _ *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			return guest("k"), g.twoOatAndABrownie(99)
		}, ErrNoSuchTable},
		{"with the venue shut", func(t *testing.T, _ *Service, d *db.DB, g guestMenu) (Meta, PlaceRound) {
			ctx := context.Background()
			err := d.WriteTx(ctx, func(q *gen.Queries) error {
				_, err := q.CloseShift(ctx, gen.CloseShiftParams{ID: 1, ClosedBy: 1, ClosedAt: sql.NullTime{Time: now, Valid: true}})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			return guest("k"), g.twoOatAndABrownie(4)
		}, shift.ErrNoOpenShift},
		{"onto a ticket on hold", func(t *testing.T, s *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			must(t)(s.Hold(context.Background(), cashier(""), open(t, s).OrderID))
			return guest("k"), g.twoOatAndABrownie(4)
		}, ErrTableBusy},
		{"past the requests a table may have waiting", func(t *testing.T, s *Service, _ *db.DB, g guestMenu) (Meta, PlaceRound) {
			for i := range maxWaitingRounds {
				if _, err := party(t, s).PlaceRound(context.Background(), guest("w"+string(rune('a'+i))), g.twoOatAndABrownie(4)); err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
			}
			return guest("k"), g.twoOatAndABrownie(4)
		}, ErrTooManyWaiting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, d, _ := newService(t)
			g := seedGuestMenu(t, d)
			meta, cmd := tt.prepare(t, s, d, g)
			_, err := party(t, s).PlaceRound(context.Background(), meta, cmd)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			// A refused round leaves nothing behind to be found by its key.
			if meta.IdempotencyKey != "" {
				if _, err := d.Queries().GuestRoundByKey(context.Background(), gen.GuestRoundByKeyParams{VenueID: 1, IdempotencyKey: meta.IdempotencyKey}); !isNoRows(err) {
					t.Errorf("the refused round was kept: %v", err)
				}
			}
		})
	}
}

// TestARoundAfterTheBillClosesIsARequestAgain: the sitting is over with the
// bill, and the next guest at the table starts with a request (ADR-209).
func TestARoundAfterTheBillClosesIsARequestAgain(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()

	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
	must(t)(s.Close(ctx, cashier(""), id))

	round, err := party(t, s).PlaceRound(ctx, guest("next"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundWaiting || round.OrderID != 0 {
		t.Errorf("round = %+v, want a request", round)
	}
}

// TestARoundAtAnotherTableIsItsOwn: a ticket at table 4 says nothing about
// table 5.
func TestARoundAtAnotherTableIsItsOwn(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	open(t, s) // table 4

	round, err := party(t, s).PlaceRound(context.Background(), guest("k"), g.twoOatAndABrownie(5))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundWaiting {
		t.Errorf("round at table 5 = %+v, want a request", round)
	}
}

// TestAPaidBillRollsWhenTheGuestsOrderAgain: a bill paid in full and
// invoiced is done. A round from a phone at that table does not meet "the
// bill is invoiced": the ticket is closed and the round is the table's next
// first order, waiting for staff as a first order does (ADR-372).
func TestAPaidBillRollsWhenTheGuestsOrderAgain(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))

	round, err := party(t, s).PlaceRound(ctx, guest("k"), g.twoOatAndABrownie(4))
	if err != nil || round.Status != RoundWaiting || round.OrderID != 0 {
		t.Fatalf("the round after the bill was paid: %+v, %v", round, err)
	}
	if paid := must(t)(s.Order(ctx, 1, id)); paid.State.Status != order.StatusClosed || !paid.State.Invoiced {
		t.Fatalf("the paid bill is %s, invoiced %v; want closed", paid.State.Status, paid.State.Invoiced)
	}
	if _, open, err := s.TableTicket(ctx, 1, 4); err != nil || open {
		t.Fatalf("the table still has a ticket: %v, %v", open, err)
	}
}
