package order

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// TestAPaidOrderToCollectIsSentOnATicketOfItsOwn: once paid it is a ticket
// with no table, on the web channel, to take away, with its lines sent to
// their stations and a code to call it by. It is nobody's on staff, a
// table's ticket is not touched, and sent again nothing more happens.
func TestAPaidOrderToCollectIsSentOnATicketOfItsOwn(t *testing.T) {
	w := newPrepayWorld(t, true, PayAtTable)
	ctx := context.Background()
	seated := open(t, w.s) // table 4, by the cashier
	must(t)(w.s.AddLine(ctx, cashier(""), seated.OrderID, dosa(1)))

	placed, err := w.s.PlacePickup(ctx, guest("p1"), w.g.pickup())
	if err != nil {
		t.Fatal(err)
	}
	sent, err := w.s.SendPaidRound(ctx, 1, placed.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != RoundSent || sent.OrderID == 0 || sent.OrderID == seated.OrderID || sent.TableID != 0 ||
		len(sent.PickupCode) != order.PickupCodeLength || !order.PickupCodeOK(sent.PickupCode) {
		t.Fatalf("sent = %+v", sent)
	}

	ticket := must(t)(w.s.Order(ctx, 1, sent.OrderID))
	st := ticket.State
	if st.Channel != order.ChannelWeb || st.Service != order.ServiceTakeaway || st.TableID != 0 || len(st.Tables()) != 0 {
		t.Errorf("its ticket: channel %s, service %s, table %d", st.Channel, st.Service, st.TableID)
	}
	if len(st.Lines) != 2 || st.Lines[0].Name != "Flat white" || !st.Lines[0].Sent || !st.Lines[1].Sent || len(st.Tickets) != 2 {
		t.Errorf("%d lines and %d tickets: %+v", len(st.Lines), len(st.Tickets), st.Lines)
	}
	if st.Totals.BalanceDue != inr(58800) {
		t.Errorf("it owes %v, want the 588.00 that was asked for it", st.Totals.BalanceDue)
	}
	h, err := w.s.OrderHistory(ctx, 1, sent.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range h.Events {
		if e.ActorID != 0 || e.DeviceID != 0 {
			t.Errorf("%s is by actor %d on device %d, want nobody", e.Type, e.ActorID, e.DeviceID)
		}
	}
	// The table's ticket has its dosa and nothing else.
	if table := must(t)(w.s.Order(ctx, 1, seated.OrderID)); len(table.State.Lines) != 1 {
		t.Errorf("table 4's ticket has %d lines", len(table.State.Lines))
	}

	// It is tracked as any round is, by no table.
	tracked, err := w.s.TrackRound(ctx, 1, 0, placed.PublicID)
	if err != nil || tracked.Progress != order.ProgressSent || tracked.PickupCode != sent.PickupCode || tracked.Bill != ticket.PublicID {
		t.Errorf("tracked = %+v, %v", tracked, err)
	}

	again, err := w.s.SendPaidRound(ctx, 1, placed.PublicID)
	if err != nil || again.OrderID != sent.OrderID || again.PickupCode != sent.PickupCode {
		t.Errorf("sent again = %+v, %v", again, err)
	}
	if orderCount(t, w.d) != 2 {
		t.Errorf("%d tickets, want the table's and the one order to collect", orderCount(t, w.d))
	}
	if after := must(t)(w.s.Order(ctx, 1, sent.OrderID)); len(after.State.Lines) != 2 {
		t.Errorf("sent twice, the ticket has %d lines", len(after.State.Lines))
	}
}

// pickupAlphabetForTests is the domain's own, spelt out: a test that draws a
// named code must not depend on reading the alphabet from the code under
// test.
const pickupAlphabetForTests = "23456789BCDFGHJKMNPQRSTVWXZ"

// drawing is the bytes that draw the given pickup codes, one after another.
func drawing(t *testing.T, list ...string) []byte {
	t.Helper()
	var out []byte
	for _, code := range list {
		for i := range len(code) {
			at := strings.IndexByte(pickupAlphabetForTests, code[i])
			if at < 0 {
				t.Fatalf("%q is not a pickup code", code)
			}
			out = append(out, byte(at))
		}
	}
	return out
}

type pickupWorld struct {
	d *db.DB
	g guestMenu
}

// with is a service over the world's database whose pickup codes are drawn
// from the given bytes.
func (p pickupWorld) with(t *testing.T, random []byte) *Service {
	t.Helper()
	s, err := New(Options{DB: p.d, Clock: fixedClock{}, CanCharge: true, Random: bytes.NewReader(random)})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// send places an order to collect and sends it as if it were paid.
func (p pickupWorld) send(t *testing.T, s *Service, key string) Round {
	t.Helper()
	ctx := context.Background()
	placed, err := s.PlacePickup(ctx, guest(key), p.g.pickup())
	if err != nil {
		t.Fatal(err)
	}
	sent, err := s.SendPaidRound(ctx, 1, placed.PublicID)
	if err != nil {
		t.Fatalf("sending %s: %v", key, err)
	}
	return sent
}

func newPickupWorld(t *testing.T) pickupWorld {
	t.Helper()
	_, d, _ := newService(t)
	return pickupWorld{d: d, g: seedGuestMenu(t, d)}
}

// TestNoTwoOrdersOutShareAPickupCode: a code on an order still out is drawn
// past; once that order is handed over and closed, the code is free again.
func TestNoTwoOrdersOutShareAPickupCode(t *testing.T) {
	p := newPickupWorld(t)
	ctx := context.Background()

	first := p.send(t, p.with(t, drawing(t, "B7K")), "p1")
	if first.PickupCode != "B7K" {
		t.Fatalf("the first order is %q", first.PickupCode)
	}
	// The same code is drawn twice, and drawn past both times.
	second := p.send(t, p.with(t, drawing(t, "B7K", "B7K", "M4Q")), "p2")
	if second.PickupCode != "M4Q" {
		t.Errorf("the second order is %q, want the first code that was free", second.PickupCode)
	}

	// The first is paid, handed over and closed: its code is free.
	s := p.with(t, drawing(t, "B7K"))
	must(t)(s.TenderPayment(ctx, cashier(""), first.OrderID, TenderPayment{Method: order.PaymentUPI, Amount: inr(58800)}))
	must(t)(s.Close(ctx, cashier(""), first.OrderID))
	if third := p.send(t, s, "p3"); third.PickupCode != "B7K" {
		t.Errorf("after the first was closed, the third order is %q, want B7K again", third.PickupCode)
	}
}

// TestWithEveryPickupCodeOutAPaidOrderWaits: a source that can only draw
// codes that are out. The order is not sent and stays to be sent; one ticket
// is opened for it however often it is tried; and once a code is free it
// goes on that ticket and no other.
func TestWithEveryPickupCodeOutAPaidOrderWaits(t *testing.T) {
	p := newPickupWorld(t)
	ctx := context.Background()
	// A source of zeros draws 222, and when that is out, tries it again as
	// often as it tries short codes, and then draws 2222. Put both out: the
	// first with three zeros, the second with enough to get past the short
	// ones.
	sources := map[string][]byte{
		"222":  make([]byte, order.PickupCodeLength),
		"2222": make([]byte, order.PickupCodeLength*pickupCodeDraws+order.PickupCodeLong),
	}
	for i, code := range []string{"222", "2222"} {
		if sent := p.send(t, p.with(t, sources[code]), fmt.Sprintf("z%d", i)); sent.PickupCode != code {
			t.Fatalf("putting %s out: %+v", code, sent)
		}
	}
	zeros := p.with(t, make([]byte, 8192))
	placed, err := zeros.PlacePickup(ctx, guest("p1"), p.g.pickup())
	if err != nil {
		t.Fatal(err)
	}
	before := orderCount(t, p.d)
	for range 2 {
		if _, err := zeros.SendPaidRound(ctx, 1, placed.PublicID); !errors.Is(err, ErrPickupCodesOut) {
			t.Fatalf("with every code it can draw out: %v", err)
		}
	}
	if got := orderCount(t, p.d); got != before+1 {
		t.Errorf("%d tickets were opened for an order that could not be sent, want 1", got-before)
	}
	if still, err := zeros.PayableRound(ctx, 1, 0, placed.PublicID); err != nil || still.Status != RoundUnpaid || still.PickupCode != "" {
		t.Errorf("it is %+v, %v; want it still to be sent, with no code", still, err)
	}

	sent, err := p.with(t, drawing(t, "M4Q")).SendPaidRound(ctx, 1, placed.PublicID)
	if err != nil || sent.PickupCode != "M4Q" {
		t.Fatalf("once a code is free: %+v, %v", sent, err)
	}
	if got := orderCount(t, p.d); got != before+1 {
		t.Errorf("%d tickets after it was sent, want the one opened for it", got-before)
	}
	if ticket := must(t)(zeros.Order(ctx, 1, sent.OrderID)); len(ticket.State.Lines) != 2 {
		t.Errorf("its ticket has %d lines", len(ticket.State.Lines))
	}
}
