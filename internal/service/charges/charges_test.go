package charges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/gateway/gatewaytest"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

var now = time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

var cashier = svcorder.Meta{VenueID: 1, ActorID: 7, DeviceID: 3}

// world is a GST cafe with ten tables and an open shift, an order service
// over it, a fake gateway, and table 4 seated with a dosa and a coffee:
// 160.00 plus 5%, so 168.00 owed.
type world struct {
	s      *Service
	orders *svcorder.Service
	d      *db.DB
	gw     *gatewaytest.Fake
	ticket svcorder.Result
	// none makes the venue have no gateway.
	none bool
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_CAFE0001", Name: "Filter Coffee House", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		for k, val := range map[string]string{svcorder.SettingTaxStateCode: "29", svcorder.SettingTaxPricing: "exclusive"} {
			if err := q.SetSetting(ctx, gen.SetSettingParams{VenueID: v.ID, Key: k, Value: val, UpdatedAt: now}); err != nil {
				return err
			}
		}
		area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: v.ID, Name: "Floor", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		for i := int64(1); i <= 10; i++ {
			if _, err := q.CreateTable(ctx, gen.CreateTableParams{
				VenueID: v.ID, AreaID: area, Name: fmt.Sprint(i), Seats: 4, Shape: "square", X: i - 1, Y: 0, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		_, err = q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_CAFE0001", VenueID: v.ID, TradingDay: "2026-09-18", OpenedBy: 1, OpenedAt: now})
		return err
	})
	if err != nil {
		t.Fatalf("setting the venue up: %v", err)
	}

	orders, err := svcorder.New(svcorder.Options{DB: d, Clock: fixedClock{}})
	if err != nil {
		t.Fatal(err)
	}
	w := &world{orders: orders, d: d, gw: gatewaytest.New()}
	w.s, err = New(Options{DB: d, Orders: orders, Clock: fixedClock{}, Gateways: func(context.Context, int64) (gateway.Gateway, error) {
		if w.none {
			return nil, ErrNoGateway
		}
		return w.gw, nil
	}})
	if err != nil {
		t.Fatal(err)
	}

	opened := w.must(t)(orders.Open(ctx, cashier, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))
	w.must(t)(orders.AddLine(ctx, cashier, opened.OrderID, svcorder.AddLine{
		ItemID: 1, Name: "Masala dosa", Qty: 1, UnitPrice: inr(12000), Station: "kitchen", TaxRate: 5000, HSN: "996331",
	}))
	w.ticket = w.must(t)(orders.AddLine(ctx, cashier, opened.OrderID, svcorder.AddLine{
		ItemID: 2, Name: "Filter coffee", Qty: 1, UnitPrice: inr(4000), Station: "bar", TaxRate: 5000, HSN: "996331",
	}))
	if due := w.ticket.State.Totals.BalanceDue; due != inr(16800) {
		t.Fatalf("the ticket owes %v, want 168.00", due)
	}
	return w
}

func (w *world) must(t *testing.T) func(svcorder.Result, error) svcorder.Result {
	return func(res svcorder.Result, err error) svcorder.Result {
		t.Helper()
		if err != nil {
			t.Fatalf("command: %v", err)
		}
		return res
	}
}

func (w *world) start(t *testing.T) Charge {
	t.Helper()
	c, err := w.s.Start(context.Background(), 1, 4, "Table 4", "https://cafe.example/order/t/code#bill")
	if err != nil {
		t.Fatalf("starting a charge: %v", err)
	}
	return c
}

func (w *world) state(t *testing.T) order.State {
	t.Helper()
	res, err := w.orders.Order(context.Background(), 1, w.ticket.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	return res.State
}

// gatewayID is the fake's id for the n'th charge it started, from 1.
func gatewayID(n int) string { return fmt.Sprintf("fake_%04d", n) }

// TestAChargePaidIsOnePaymentOnTheBill: the whole of the happy path. The
// gateway is asked for what the table owes; nothing is on the bill until
// it says paid; then there is one tender, by the method the guest used,
// with the gateway's reference, and the bill is settled and invoiced as a
// cashier's tender would have left it.
func TestAChargePaidIsOnePaymentOnTheBill(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	c := w.start(t)
	if c.Status != Open || c.Replayed || c.PublicID == "" || c.Gateway != "test" || c.Amount != inr(16800) ||
		c.CheckoutURL != "https://pay.test/"+gatewayID(1) || c.OrderID != w.ticket.OrderID || c.TableID != 4 {
		t.Fatalf("charge = %+v", c)
	}
	asked := w.gw.Started[0]
	if asked.Reference != c.PublicID || asked.Amount != inr(16800) || asked.Description != "Table 4" || asked.ReturnURL == "" {
		t.Errorf("the gateway was asked for %+v", asked)
	}

	// Asked before the guest has paid: still open, nothing on the bill.
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Open {
		t.Fatalf("before paying: %+v, %v", got, err)
	}
	if st := w.state(t); len(st.Tenders) != 0 || st.Totals.BalanceDue != inr(16800) {
		t.Fatalf("an unpaid charge is on the bill: %+v", st.Tenders)
	}

	w.gw.Pay(gatewayID(1), gateway.UPI)
	got, err = w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid || got.Problem != "" {
		t.Fatalf("after paying: %+v, %v", got, err)
	}
	st := w.state(t)
	if len(st.Tenders) != 1 {
		t.Fatalf("%d tenders, want 1", len(st.Tenders))
	}
	tender := st.Tenders[0]
	if tender.Method != order.PaymentUPI || tender.Amount != inr(16800) || tender.Reference != "pay_"+gatewayID(1) ||
		tender.TenderedBy != 0 || tender.Status != order.PaymentStatusCaptured {
		t.Errorf("tender = %+v", tender)
	}
	if !st.Totals.BalanceDue.IsZero() || !st.Invoiced {
		t.Errorf("the bill: due %v, invoiced %v; want settled and invoiced", st.Totals.BalanceDue, st.Invoiced)
	}
	// It is the guest's: nobody's, on no device.
	h, err := w.orders.OrderHistory(ctx, 1, w.ticket.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range h.Events {
		if e.Type == order.TypePaymentTendered && (e.ActorID != 0 || e.DeviceID != 0) {
			t.Errorf("the tender is by actor %d on device %d, want the guest", e.ActorID, e.DeviceID)
		}
	}
}

// TestSettlingTwiceIsOnePayment: asked again, and again, and by twenty
// things at once, the bill has one tender.
func TestSettlingTwiceIsOnePayment(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.Card)

	const askers = 20
	errs := make([]error, askers)
	got := make([]Charge, askers)
	var wg sync.WaitGroup
	for i := range askers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.Settle(ctx, 1, c.PublicID)
		}()
	}
	wg.Wait()
	for i := range askers {
		if errs[i] != nil {
			t.Fatalf("asker %d: %v", i, errs[i])
		}
		if got[i].Status != Paid {
			t.Errorf("asker %d was told %s", i, got[i].Status)
		}
	}
	if st := w.state(t); len(st.Tenders) != 1 || st.Tenders[0].Method != order.PaymentCard {
		t.Fatalf("%d tenders after %d settles, want 1 by card: %+v", len(st.Tenders), askers, st.Tenders)
	}

	// A closed charge is answered as it is, and the gateway is left alone.
	before := w.gw.Asked
	if again, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || again.Status != Paid || w.gw.Asked != before {
		t.Errorf("settling a paid charge: %+v, %v, asked the gateway %d more times", again, err, w.gw.Asked-before)
	}
}

// TestTheTenderIsMadeFromWhatWasWrittenFirst: the gateway says UPI, then on
// a second asking says card. What was written first is what is tendered.
func TestTheTenderIsMadeFromWhatWasWrittenFirst(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)

	// The first asking finds it paid by UPI and cannot tender: no shift.
	w.closeShift(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)
	if _, err := w.s.Settle(ctx, 1, c.PublicID); !errors.Is(err, shift.ErrNoOpenShift) {
		t.Fatalf("with no shift: %v", err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Fatalf("a charge the bill could not take yet is %s, want it open to try again", got.Status)
	}
	if st := w.state(t); len(st.Tenders) != 0 {
		t.Fatal("something was tendered with no shift open")
	}

	// A shift opens; the gateway now words the method differently.
	w.openShift(t, "shf_NEXT0001")
	w.gw.Pay(gatewayID(1), gateway.Card)
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid {
		t.Fatalf("once a shift is open: %+v, %v", got, err)
	}
	if st := w.state(t); len(st.Tenders) != 1 || st.Tenders[0].Method != order.PaymentUPI {
		t.Errorf("tenders = %+v, want one by UPI, as first written", st.Tenders)
	}
}

func (w *world) closeShift(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CloseShift(ctx, gen.CloseShiftParams{ID: 1, ClosedBy: 1, ClosedAt: sql.NullTime{Time: now, Valid: true}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (w *world) openShift(t *testing.T, publicID string) {
	t.Helper()
	ctx := context.Background()
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: publicID, VenueID: 1, TradingDay: "2026-09-19", OpenedBy: 1, OpenedAt: now})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestMoneyTheBillCannotTakeIsKeptForAPerson: the gateway has the money and
// the ticket will not take it. Nothing is forced onto the bill, and the
// charge is not lost: it is unapplied, with why.
func TestMoneyTheBillCannotTakeIsKeptForAPerson(t *testing.T) {
	tests := []struct {
		name    string
		before  func(t *testing.T, w *world)
		pay     func(w *world)
		problem string
		tenders int
	}{
		{
			name: "the table paid cash while the guest was on the gateway's page",
			before: func(t *testing.T, w *world) {
				w.must(t)(w.orders.TenderPayment(context.Background(), cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
			},
			pay:     func(w *world) { w.gw.Pay(gatewayID(1), gateway.UPI) },
			problem: "no longer owes this much",
			tenders: 1,
		},
		{
			name: "part of it was paid another way",
			before: func(t *testing.T, w *world) {
				w.must(t)(w.orders.TenderPayment(context.Background(), cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(10000)}))
			},
			pay:     func(w *world) { w.gw.Pay(gatewayID(1), gateway.UPI) },
			problem: "no longer owes this much",
			tenders: 1,
		},
		{
			name: "the bill was settled and closed",
			before: func(t *testing.T, w *world) {
				ctx := context.Background()
				w.must(t)(w.orders.TenderPayment(ctx, cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
				w.must(t)(w.orders.Close(ctx, cashier, w.ticket.OrderID))
			},
			pay:     func(w *world) { w.gw.Pay(gatewayID(1), gateway.Card) },
			problem: "", // closed or over-tendered: either is a true account
			tenders: 1,
		},
		{
			name:    "the gateway took less than was asked",
			before:  func(*testing.T, *world) {},
			pay:     func(w *world) { w.gw.PayAmount(gatewayID(1), inr(16000)) },
			problem: "The gateway took 160.00 INR, and the charge was for 168.00 INR.",
			tenders: 0,
		},
		{
			name:    "the gateway took another currency",
			before:  func(*testing.T, *world) {},
			pay:     func(w *world) { w.gw.PayAmount(gatewayID(1), money.Amount{Minor: 16800, Currency: "USD"}) },
			problem: "The gateway took 168.00 USD",
			tenders: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			ctx := context.Background()
			c := w.start(t)
			tt.before(t, w)
			tt.pay(w)

			got, err := w.s.Settle(ctx, 1, c.PublicID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != Unapplied || got.Problem == "" {
				t.Fatalf("charge = %+v, want it unapplied with why", got)
			}
			if tt.problem != "" && !strings.Contains(got.Problem, tt.problem) {
				t.Errorf("problem %q does not say %q", got.Problem, tt.problem)
			}
			res, err := w.orders.Order(ctx, 1, w.ticket.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.State.Tenders) != tt.tenders {
				t.Errorf("%d tenders on the bill, want %d: the gateway's money was forced on", len(res.State.Tenders), tt.tenders)
			}
			list, err := w.s.Waiting(ctx, 1)
			if err != nil || len(list) != 1 || list[0].PublicID != c.PublicID || list[0].Amount != inr(16800) ||
				list[0].TableName != "4" || list[0].Problem != got.Problem || list[0].Gateway != "test" || list[0].GatewayID != gatewayID(1) {
				t.Errorf("waiting = %+v, %v", list, err)
			}
			// Asked again, it is still unapplied: it is not retried onto the bill.
			if again, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || again.Status != Unapplied {
				t.Errorf("settled again: %+v, %v", again, err)
			}
		})
	}
}

// TestARoundAddedSinceIsStillOwed: the charge was for what was owed then.
// Paid, it goes on the bill as a part payment and the rest is still owed.
func TestARoundAddedSinceIsStillOwed(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.must(t)(w.orders.AddLine(ctx, cashier, w.ticket.OrderID, svcorder.AddLine{
		ItemID: 2, Name: "Filter coffee", Qty: 1, UnitPrice: inr(4000), Station: "bar", TaxRate: 5000, HSN: "996331",
	}))
	w.gw.Pay(gatewayID(1), gateway.UPI)

	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid {
		t.Fatalf("charge = %+v, %v", got, err)
	}
	st := w.state(t)
	if len(st.Tenders) != 1 || st.Tenders[0].Amount != inr(16800) || st.Totals.BalanceDue != inr(4200) || st.Invoiced {
		t.Errorf("tenders %+v, due %v, invoiced %v; want 168.00 paid, 42.00 owed, no invoice yet", st.Tenders, st.Totals.BalanceDue, st.Invoiced)
	}
}

func TestAFailedChargeIsClosedAndNothingIsPaid(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.gw.Fail(gatewayID(1))

	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Failed {
		t.Fatalf("charge = %+v, %v", got, err)
	}
	if st := w.state(t); len(st.Tenders) != 0 {
		t.Errorf("a failed charge is on the bill: %+v", st.Tenders)
	}
	// The guest tries again: a new charge, since the old is closed.
	again := w.start(t)
	if again.Replayed || again.PublicID == c.PublicID {
		t.Errorf("after a failure: %+v, want a new charge", again)
	}
}

// TestStartingAgainIsTheSameCharge: tapped twice, the gateway is asked
// once. A table that owes something else gets another.
func TestStartingAgainIsTheSameCharge(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	first := w.start(t)
	second := w.start(t)
	if !second.Replayed || second.PublicID != first.PublicID || second.CheckoutURL != first.CheckoutURL || len(w.gw.Started) != 1 {
		t.Fatalf("second = %+v, the gateway was asked %d times", second, len(w.gw.Started))
	}

	w.must(t)(w.orders.AddLine(ctx, cashier, w.ticket.OrderID, svcorder.AddLine{
		ItemID: 2, Name: "Filter coffee", Qty: 1, UnitPrice: inr(4000), Station: "bar", TaxRate: 5000, HSN: "996331",
	}))
	third := w.start(t)
	if third.Replayed || third.PublicID == first.PublicID || third.Amount != inr(21000) {
		t.Fatalf("after a round was added: %+v, want a new charge for 210.00", third)
	}

	// The guest pays the old page after all, and then the new one. The
	// first goes on the bill; the second is more than is left, and is kept.
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.gw.Pay(gatewayID(2), gateway.UPI)
	closed, err := w.s.Sweep(ctx, 1)
	if err != nil || closed != 2 {
		t.Fatalf("sweep closed %d, %v; want 2", closed, err)
	}
	old, _ := w.s.Charge(ctx, 1, first.PublicID)
	newer, _ := w.s.Charge(ctx, 1, third.PublicID)
	if old.Status != Paid || newer.Status != Unapplied {
		t.Errorf("the old charge is %s and the new %s, want paid and unapplied", old.Status, newer.Status)
	}
	if st := w.state(t); len(st.Tenders) != 1 || st.Totals.BalanceDue != inr(4200) {
		t.Errorf("tenders %+v, due %v", st.Tenders, st.Totals.BalanceDue)
	}
}

// TestSweepLandsAPaymentNobodyCameBackFor: the guest paid and closed the
// page. Nothing asked about that charge by name, and it is on the bill.
func TestSweepLandsAPaymentNobodyCameBackFor(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)

	if closed, err := w.s.Sweep(ctx, 1); err != nil || closed != 0 {
		t.Fatalf("with nothing paid: closed %d, %v", closed, err)
	}
	w.gw.Pay(gatewayID(1), gateway.Card)
	if closed, err := w.s.Sweep(ctx, 1); err != nil || closed != 1 {
		t.Fatalf("closed %d, %v; want 1", closed, err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Paid {
		t.Errorf("charge is %s", got.Status)
	}
	if st := w.state(t); len(st.Tenders) != 1 {
		t.Errorf("%d tenders", len(st.Tenders))
	}
	// Nothing open, nothing to ask.
	before := w.gw.Asked
	if closed, err := w.s.Sweep(ctx, 1); err != nil || closed != 0 || w.gw.Asked != before {
		t.Errorf("a sweep with nothing open: closed %d, %v, asked the gateway %d times", closed, err, w.gw.Asked-before)
	}
}

// TestAGatewayThatCannotBeReachedDecidesNothing: the charge stays open, the
// bill is untouched, and when the gateway is back the payment lands.
func TestAGatewayThatCannotBeReachedDecidesNothing(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.gw.Down(true)

	if _, err := w.s.Settle(ctx, 1, c.PublicID); !errors.Is(err, gateway.ErrUnavailable) {
		t.Fatalf("with the gateway down: %v", err)
	}
	if closed, err := w.s.Sweep(ctx, 1); closed != 0 || !errors.Is(err, gateway.ErrUnavailable) {
		t.Errorf("a sweep with the gateway down: closed %d, %v", closed, err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Fatalf("charge is %s, want it open", got.Status)
	}
	if st := w.state(t); len(st.Tenders) != 0 {
		t.Fatal("something was tendered on the gateway's silence")
	}
	if _, err := w.s.Start(ctx, 1, 5, "Table 5", ""); err == nil {
		t.Error("a charge was started at a table with no ticket")
	}

	w.gw.Down(false)
	if got, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || got.Status != Paid {
		t.Errorf("once it is back: %+v, %v", got, err)
	}
}

func TestStartIsRefused(t *testing.T) {
	tests := []struct {
		name string
		do   func(t *testing.T, w *world) error
		want error
	}{
		{"at a table with no ticket", func(_ *testing.T, w *world) error {
			_, err := w.s.Start(context.Background(), 1, 5, "Table 5", "")
			return err
		}, ErrNothingToPay},
		{"with nothing owed", func(t *testing.T, w *world) error {
			w.must(t)(w.orders.TenderPayment(context.Background(), cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
			_, err := w.s.Start(context.Background(), 1, 4, "Table 4", "")
			return err
		}, ErrNothingToPay},
		{"at a venue with no gateway", func(_ *testing.T, w *world) error {
			w.none = true
			_, err := w.s.Start(context.Background(), 1, 4, "Table 4", "")
			return err
		}, ErrNoGateway},
		{"with the gateway down", func(_ *testing.T, w *world) error {
			w.gw.Down(true)
			_, err := w.s.Start(context.Background(), 1, 4, "Table 4", "")
			return err
		}, gateway.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			if err := tt.do(t, w); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			// A refused start leaves no charge behind.
			var n int
			if err := w.d.Reader().QueryRow(`SELECT count(*) FROM gateway_charges`).Scan(&n); err != nil || n != 0 {
				t.Errorf("%d charges kept, %v", n, err)
			}
		})
	}
}

func TestAChargeIsItsVenuesOwn(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	if _, err := w.s.Settle(ctx, 2, c.PublicID); !errors.Is(err, ErrChargeNotFound) {
		t.Errorf("another venue settling it: %v", err)
	}
	if _, err := w.s.Charge(ctx, 1, "chg_NOPE0000"); !errors.Is(err, ErrChargeNotFound) {
		t.Errorf("a charge that does not exist: %v", err)
	}
}

func TestNewNeedsItsParts(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("a service was made with nothing to work with")
	}
}

func TestMethods(t *testing.T) {
	tests := []struct {
		in   gateway.Method
		want order.PaymentMethod
	}{
		{gateway.UPI, order.PaymentUPI}, {gateway.Card, order.PaymentCard}, {gateway.Other, order.PaymentOther},
		{"netbanking", order.PaymentOther}, {"", order.PaymentOther},
	}
	for _, tt := range tests {
		if got := methodOf(tt.in); got != tt.want {
			t.Errorf("methodOf(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
