package charges

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/gateway"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// roundLines is a dosa and a coffee: 160.00, and 168.00 with its 5%.
var roundLines = []order.LineAdded{
	{ItemID: 1, Name: "Masala dosa", Qty: 1, UnitPrice: inr(12000), Course: 1, Station: "kitchen", TaxRate: 5000, HSN: "996331"},
	{ItemID: 2, Name: "Filter coffee", Qty: 1, UnitPrice: inr(4000), Course: 1, Station: "bar", TaxRate: 5000, HSN: "996331"},
}

// hold keeps a round at a table until it is paid, asking 168.00, as the
// order service would have where guests pay first.
func (w *world) hold(t *testing.T, publicID string, tableID int64) {
	t.Helper()
	ctx := context.Background()
	lines, err := json.Marshal(roundLines)
	if err != nil {
		t.Fatal(err)
	}
	err = w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateGuestRound(ctx, gen.CreateGuestRoundParams{
			PublicID: publicID, VenueID: 1, TableID: tableID, Status: string(svcorder.RoundWaiting),
			Lines: string(lines), SubtotalMinor: 16000, Currency: "INR",
			IdempotencyKey: "key-" + publicID, Fingerprint: "fp-" + publicID, CreatedAt: now,
			Prepay: 1, DueMinor: sql.NullInt64{Int64: 16800, Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("holding a round: %v", err)
	}
}

func (w *world) startRound(t *testing.T, round string, tableID int64) Charge {
	t.Helper()
	c, err := w.s.StartRound(context.Background(), 1, tableID, round, "Table order", "https://cafe.example/order/t/code")
	if err != nil {
		t.Fatalf("starting a charge for round %s: %v", round, err)
	}
	return c
}

// ticketAt is the ticket open at a table, or false.
func (w *world) ticketAt(t *testing.T, tableID int64) (order.State, bool) {
	t.Helper()
	res, open, err := w.orders.TableTicket(context.Background(), 1, tableID)
	if err != nil {
		t.Fatal(err)
	}
	return res.State, open
}

// TestARoundPaidForIsSentAndIsOnePayment: the whole of paying first. The
// gateway is asked for what the round asks; until it says paid there is no
// ticket and nothing in the kitchen; then the round is the table's ticket,
// sent, with one tender for it, and asking again changes nothing.
func TestARoundPaidForIsSentAndIsOnePayment(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_HELD0001", 6)

	c := w.startRound(t, "rnd_HELD0001", 6)
	if c.Status != Open || c.Replayed || c.Round != "rnd_HELD0001" || c.OrderID != 0 || c.TableID != 6 || c.Amount != inr(16800) {
		t.Fatalf("charge = %+v", c)
	}
	if asked := w.gw.Started[0]; asked.Reference != c.PublicID || asked.Amount != inr(16800) {
		t.Errorf("the gateway was asked for %+v", asked)
	}
	// Asked again, by the same phone or another, it is the same charge.
	again := w.startRound(t, "rnd_HELD0001", 6)
	if !again.Replayed || again.PublicID != c.PublicID || len(w.gw.Started) != 1 {
		t.Errorf("again = %+v, and the gateway was asked %d times", again, len(w.gw.Started))
	}

	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Open {
		t.Fatalf("before paying: %+v, %v", got, err)
	}
	if _, open := w.ticketAt(t, 6); open {
		t.Fatal("an unpaid round opened a ticket")
	}

	w.gw.Pay(gatewayID(1), gateway.UPI)
	got, err = w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid || got.OrderID == 0 || got.Round != "rnd_HELD0001" {
		t.Fatalf("after paying: %+v, %v", got, err)
	}
	st, open := w.ticketAt(t, 6)
	if !open {
		t.Fatal("the paid round is on no ticket")
	}
	if st.Channel != order.ChannelQRTable || len(st.Lines) != 2 || len(st.Tickets) != 2 {
		t.Errorf("the ticket: channel %s, %d lines, %d kitchen tickets; want the guest's, 2 and 2", st.Channel, len(st.Lines), len(st.Tickets))
	}
	if len(st.Tenders) != 1 || st.Tenders[0].Amount != inr(16800) || st.Tenders[0].Method != order.PaymentUPI || st.Tenders[0].TenderedBy != 0 {
		t.Fatalf("tenders = %+v, want one of 168.00 by UPI, nobody's", st.Tenders)
	}
	if !st.Totals.BalanceDue.IsZero() {
		t.Errorf("the ticket owes %v, want nothing", st.Totals.BalanceDue)
	}

	if got, err = w.s.Settle(ctx, 1, c.PublicID); err != nil || got.Status != Paid {
		t.Fatalf("settling again: %+v, %v", got, err)
	}
	if st, _ := w.ticketAt(t, 6); len(st.Lines) != 2 || len(st.Tenders) != 1 {
		t.Errorf("settled twice: %d lines and %d tenders", len(st.Lines), len(st.Tenders))
	}
	// Paid for, there is nothing more to pay for it.
	if _, err := w.s.StartRound(ctx, 1, 6, "rnd_HELD0001", "", ""); !errors.Is(err, ErrNothingToPay) {
		t.Errorf("a charge for a round already paid: %v", err)
	}
}

// TestARoundPaidForAtASeatedTable: the payment is for the round and not for
// the table. What the waiter rang up is still owed after.
func TestARoundPaidForAtASeatedTable(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_HELD0001", 4)
	c := w.startRound(t, "rnd_HELD0001", 4)
	w.gw.Pay(gatewayID(1), gateway.Card)

	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid || got.OrderID != w.ticket.OrderID {
		t.Fatalf("after paying: %+v, %v; want it on order %d", got, err, w.ticket.OrderID)
	}
	st := w.state(t)
	if len(st.Lines) != 4 || len(st.Tenders) != 1 || st.Tenders[0].Amount != inr(16800) {
		t.Fatalf("%d lines, tenders %+v", len(st.Lines), st.Tenders)
	}
	if st.Totals.BalanceDue != inr(16800) {
		t.Errorf("the table owes %v, want the 168.00 it owed before the round", st.Totals.BalanceDue)
	}

	// A charge for the table's balance is its own, not the round's.
	bill := w.start(t)
	if bill.Round != "" || bill.PublicID == c.PublicID || bill.Amount != inr(16800) {
		t.Errorf("the bill's charge = %+v", bill)
	}
}

// TestARoundSentAndNotYetPaidForIsFinished: a settle that put the round in
// the kitchen and stopped before the tender. The next one tenders, and
// sends nothing twice.
func TestARoundSentAndNotYetPaidForIsFinished(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_HELD0001", 6)
	c := w.startRound(t, "rnd_HELD0001", 6)
	w.gw.Pay(gatewayID(1), gateway.UPI)

	if _, err := w.orders.SendPaidRound(ctx, 1, "rnd_HELD0001"); err != nil {
		t.Fatal(err)
	}
	if st, _ := w.ticketAt(t, 6); len(st.Lines) != 2 || len(st.Tenders) != 0 {
		t.Fatalf("before the settle: %d lines, %d tenders", len(st.Lines), len(st.Tenders))
	}
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid {
		t.Fatalf("settle: %+v, %v", got, err)
	}
	if st, _ := w.ticketAt(t, 6); len(st.Lines) != 2 || len(st.Tickets) != 2 || len(st.Tenders) != 1 {
		t.Errorf("after: %d lines, %d kitchen tickets, %d tenders; want 2, 2, 1", len(st.Lines), len(st.Tickets), len(st.Tenders))
	}
}

// TestSettlingARoundFromManyPlacesAtOnce: one ticket, one round of lines,
// one tender.
func TestSettlingARoundFromManyPlacesAtOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_HELD0001", 6)
	c := w.startRound(t, "rnd_HELD0001", 6)
	w.gw.Pay(gatewayID(1), gateway.UPI)

	const askers = 12
	errs := make([]error, askers)
	var wg sync.WaitGroup
	for i := range askers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = w.s.Settle(ctx, 1, c.PublicID)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("asker %d: %v", i, err)
		}
	}
	st, _ := w.ticketAt(t, 6)
	if len(st.Lines) != 2 || len(st.Tenders) != 1 {
		t.Fatalf("%d lines and %d tenders after %d settles", len(st.Lines), len(st.Tenders), askers)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Paid {
		t.Errorf("the charge is %s", got.Status)
	}
}

// TestStartingARoundsChargeFromManyPhonesAtOnce: they are given one charge.
func TestStartingARoundsChargeFromManyPhonesAtOnce(t *testing.T) {
	w := newWorld(t)
	w.hold(t, "rnd_HELD0001", 6)

	const phones = 8
	got := make([]Charge, phones)
	errs := make([]error, phones)
	var wg sync.WaitGroup
	for i := range phones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.StartRound(context.Background(), 1, 6, "rnd_HELD0001", "Table order", "https://cafe.example/")
		}()
	}
	wg.Wait()
	for i := range phones {
		if errs[i] != nil {
			t.Fatalf("phone %d: %v", i, errs[i])
		}
		if got[i].PublicID != got[0].PublicID {
			t.Errorf("phone %d was given %s, phone 0 %s", i, got[i].PublicID, got[0].PublicID)
		}
	}
	rows, err := w.d.Queries().GatewayChargesByStatus(context.Background(), gen.GatewayChargesByStatusParams{VenueID: 1, Status: string(Open), Limit: 50})
	if err != nil || len(rows) != 1 {
		t.Errorf("%d open charges, %v; want 1", len(rows), err)
	}
}

// TestARoundPaidForWithTheVenueShut: nothing is sent to a kitchen nobody is
// in, and the charge waits. If a shift opens at once it goes through; if
// the next shift is another day's, the money is for a person to deal with.
func TestARoundPaidForWithTheVenueShut(t *testing.T) {
	tests := []struct {
		name     string
		reopened time.Time
		want     Status
	}{
		{"and opened again straight away", now, Paid},
		{"and opened again the next day", now.Add(18 * time.Hour), Unapplied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			ctx := context.Background()
			w.hold(t, "rnd_HELD0001", 6)
			c := w.startRound(t, "rnd_HELD0001", 6)
			w.closeShift(t)
			w.gw.Pay(gatewayID(1), gateway.UPI)

			if _, err := w.s.Settle(ctx, 1, c.PublicID); !errors.Is(err, shift.ErrNoOpenShift) {
				t.Fatalf("with no shift: %v", err)
			}
			if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
				t.Fatalf("the charge is %s, want it open to try again", got.Status)
			}
			if _, open := w.ticketAt(t, 6); open {
				t.Fatal("a round was sent with the venue shut")
			}

			err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
				_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_NEXT0001", VenueID: 1, TradingDay: "2026-09-19", OpenedBy: 1, OpenedAt: tt.reopened})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := w.s.Settle(ctx, 1, c.PublicID)
			if err != nil || got.Status != tt.want {
				t.Fatalf("once a shift is open: %+v, %v; want %s", got, err, tt.want)
			}
			_, open := w.ticketAt(t, 6)
			if open != (tt.want == Paid) {
				t.Errorf("a ticket at the table: %v", open)
			}
			if tt.want != Unapplied {
				return
			}
			if !strings.Contains(got.Problem, "shift that has since closed") || !strings.HasSuffix(got.Problem, "not sent to the kitchen.") {
				t.Errorf("problem = %q", got.Problem)
			}
			// It is in front of staff, with no ticket to name.
			list, err := w.s.Waiting(ctx, 1)
			if err != nil || len(list) != 1 || list[0].PublicID != c.PublicID || list[0].OrderPublicID != "" || list[0].TableID != 6 {
				t.Errorf("waiting = %+v, %v", list, err)
			}
		})
	}
}

// TestARoundsChargeTheBillCannotTake: money the gateway has that does not
// match, or that the ticket will not take. Each is kept for a person, and
// says whether the food went.
func TestARoundsChargeTheBillCannotTake(t *testing.T) {
	t.Run("the gateway took another amount: nothing is sent", func(t *testing.T) {
		w := newWorld(t)
		ctx := context.Background()
		w.hold(t, "rnd_HELD0001", 6)
		c := w.startRound(t, "rnd_HELD0001", 6)
		w.gw.PayAmount(gatewayID(1), inr(16000))

		got, err := w.s.Settle(ctx, 1, c.PublicID)
		if err != nil || got.Status != Unapplied || !strings.HasSuffix(got.Problem, "not sent to the kitchen.") {
			t.Fatalf("got %+v, %v", got, err)
		}
		if _, open := w.ticketAt(t, 6); open {
			t.Error("a round was sent on a payment of the wrong amount")
		}
		// The round is still held: a second charge for it can be started.
		if again := w.startRound(t, "rnd_HELD0001", 6); again.PublicID == c.PublicID || again.Replayed {
			t.Errorf("again = %+v, want a new charge", again)
		}
	})

	t.Run("the bill owes less by the time it lands: the food went", func(t *testing.T) {
		w := newWorld(t)
		ctx := context.Background()
		w.hold(t, "rnd_HELD0001", 6)
		c := w.startRound(t, "rnd_HELD0001", 6)
		w.gw.Pay(gatewayID(1), gateway.UPI)

		// The round reaches the kitchen, and before the tender a cashier
		// takes 100.00 of it in cash.
		sent, err := w.orders.SendPaidRound(ctx, 1, "rnd_HELD0001")
		if err != nil {
			t.Fatal(err)
		}
		w.must(t)(w.orders.TenderPayment(ctx, cashier, sent.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(10000)}))

		got, err := w.s.Settle(ctx, 1, c.PublicID)
		if err != nil || got.Status != Unapplied || got.OrderID != sent.OrderID {
			t.Fatalf("got %+v, %v", got, err)
		}
		if !strings.Contains(got.Problem, "with the kitchen") {
			t.Errorf("problem = %q", got.Problem)
		}
		st, _ := w.ticketAt(t, 6)
		if len(st.Tenders) != 1 || st.Totals.BalanceDue != inr(6800) {
			t.Errorf("tenders %+v, owing %v: the charge was forced onto the bill", st.Tenders, st.Totals.BalanceDue)
		}
		list, err := w.s.Waiting(ctx, 1)
		if err != nil || len(list) != 1 || list[0].OrderPublicID == "" {
			t.Errorf("waiting = %+v, %v; want it with its ticket named", list, err)
		}
	})
}

func TestStartingARoundsChargeIsRefused(t *testing.T) {
	tests := []struct {
		name    string
		tableID int64
		round   string
		prepare func(t *testing.T, w *world)
		want    error
	}{
		{"for a round that is not there", 6, "rnd_NOTTHERE", nil, svcorder.ErrRoundNotFound},
		{"from another table", 7, "rnd_HELD0001", nil, svcorder.ErrRoundNotFound},
		{"with no gateway", 6, "rnd_HELD0001", func(_ *testing.T, w *world) { w.none = true }, ErrNoGateway},
		{"with the venue shut", 6, "rnd_HELD0001", func(t *testing.T, w *world) { w.closeShift(t) }, shift.ErrNoOpenShift},
		{"with the gateway down", 6, "rnd_HELD0001", func(_ *testing.T, w *world) { w.gw.Down(true) }, gateway.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			w.hold(t, "rnd_HELD0001", 6)
			if tt.prepare != nil {
				tt.prepare(t, w)
			}
			_, err := w.s.StartRound(context.Background(), 1, tt.tableID, tt.round, "", "")
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			rows, err := w.d.Queries().GatewayChargesByStatus(context.Background(), gen.GatewayChargesByStatusParams{VenueID: 1, Status: string(Open), Limit: 50})
			if err != nil || len(rows) != 0 {
				t.Errorf("%d charges kept, %v", len(rows), err)
			}
		})
	}
}
