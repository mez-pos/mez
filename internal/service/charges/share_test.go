package charges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/gateway"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

func (w *world) share(t *testing.T, payer string, minor int64) Charge {
	t.Helper()
	c, err := w.s.StartShare(context.Background(), 1, 4, payer, inr(minor), "Table 4", "https://cafe.example/order/t/code#bill")
	if err != nil {
		t.Fatalf("%s starting a share of %v: %v", payer, inr(minor), err)
	}
	return c
}

func (w *world) settle(t *testing.T, c Charge) Charge {
	t.Helper()
	got, err := w.s.Settle(context.Background(), 1, c.PublicID)
	if err != nil {
		t.Fatalf("settling %s: %v", c.PublicID, err)
	}
	return got
}

// TestEqualPart: a bill split evenly is the largest-remainder split of its
// total, each guest offered the next part nobody has taken, and never more
// than is left.
func TestEqualPart(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		unclaimed int64
		people    int
		want      int64
	}{
		{"the first of three carries the extra paisa", 100000, 100000, 3, 33334},
		{"the second of three", 100000, 66666, 3, 33333},
		{"the third of three", 100000, 33333, 3, 33333},
		{"two looked at once and both took the larger part: the last pays what is left", 100000, 33332, 3, 33332},
		{"part of it was paid at the counter", 100000, 60000, 3, 33333},
		{"less is left than a part", 100000, 10000, 3, 10000},
		{"one person is the whole of what is left", 100000, 60000, 1, 60000},
		{"an even split has no extra", 16800, 16800, 3, 5600},
		{"nothing is left", 16800, 0, 3, 0},
		{"more is claimed than owed", 16800, -500, 3, 0},
		{"nothing is on the bill", 0, 0, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EqualPart(inr(tt.total), inr(tt.unclaimed), tt.people)
			if err != nil || got != inr(tt.want) {
				t.Errorf("EqualPart(%v, %v, %d) = %v, %v; want %v", inr(tt.total), inr(tt.unclaimed), tt.people, got, err, inr(tt.want))
			}
		})
	}

	if _, err := EqualPart(inr(16800), inr(16800), 0); !errors.Is(err, money.ErrInvalidSplit) {
		t.Errorf("among nobody: %v", err)
	}
	if _, err := EqualPart(inr(16800), money.Amount{Minor: 16800, Currency: "USD"}, 2); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("two currencies: %v", err)
	}
}

// TestEqualPartsPaidInTurnAreTheSplit: guests paying one after another each
// pay exactly the part money.SplitEvenly gives them, and the bill is paid to
// the paisa. This is the split-payment invariant of docs/02-domain.md.
func TestEqualPartsPaidInTurnAreTheSplit(t *testing.T) {
	for _, total := range []int64{1, 2, 99, 100, 16800, 100000, 100001, 999999} {
		for people := 1; people <= 9; people++ {
			want, err := money.SplitEvenly(inr(total), people)
			if err != nil {
				t.Fatal(err)
			}
			left := total
			for i, w := range want {
				got, err := EqualPart(inr(total), inr(left), people)
				if err != nil {
					t.Fatal(err)
				}
				if got != w && left > 0 {
					t.Fatalf("%v among %d: guest %d is asked for %v, want %v", inr(total), people, i+1, got, w)
				}
				left -= got.Minor
			}
			if left != 0 {
				t.Errorf("%v among %d leaves %v", inr(total), people, inr(left))
			}
		}
	}
}

// TestSharesAddUpToTheBill: three guests each pay a third. Each is a charge
// of its own, each lands as a part payment, and the third settles and
// invoices the bill as a cashier's last tender would.
func TestSharesAddUpToTheBill(t *testing.T) {
	w := newWorld(t)
	var shares []Charge
	for i, payer := range []string{"phone-a", "phone-b", "phone-c"} {
		c := w.share(t, payer, 5600)
		if c.Replayed || c.Status != Open || c.Amount != inr(5600) || c.Payer != payer || c.OrderID != w.ticket.OrderID ||
			c.CheckoutURL != "https://pay.test/"+gatewayID(i+1) {
			t.Fatalf("%s's share = %+v", payer, c)
		}
		shares = append(shares, c)
	}
	if asked := w.gw.Started[2]; asked.Amount != inr(5600) || asked.Reference != shares[2].PublicID {
		t.Errorf("the gateway was asked for %+v", asked)
	}

	for i, c := range shares {
		w.gw.Pay(gatewayID(i+1), gateway.UPI)
		if got := w.settle(t, c); got.Status != Paid {
			t.Fatalf("share %d: %+v", i+1, got)
		}
		st := w.state(t)
		if want := inr(16800 - int64(i+1)*5600); len(st.Tenders) != i+1 || st.Totals.BalanceDue != want {
			t.Fatalf("after %d shares: %d tenders, %v owed, want %v", i+1, len(st.Tenders), st.Totals.BalanceDue, want)
		}
		if st.Invoiced != (i == 2) {
			t.Errorf("after %d shares the bill is invoiced: %v", i+1, st.Invoiced)
		}
	}
}

// TestAShareIsThePhonesOwn: two guests paying the same amount are two
// charges, where two phones asking for the whole bill are one. The same
// phone asking again is asking once.
func TestAShareIsThePhonesOwn(t *testing.T) {
	w := newWorld(t)
	a := w.share(t, "phone-a", 5600)
	b := w.share(t, "phone-b", 5600)
	if a.PublicID == b.PublicID || b.Replayed {
		t.Fatalf("two guests were given one charge: %+v and %+v", a, b)
	}
	again := w.share(t, "phone-a", 5600)
	if !again.Replayed || again.PublicID != a.PublicID || again.CheckoutURL != a.CheckoutURL || len(w.gw.Started) != 2 {
		t.Errorf("asked again: %+v, with the gateway asked %d times; want %s again and 2", again, len(w.gw.Started), a.PublicID)
	}
	// A whole-bill charge is never somebody's share of the same amount.
	rest := w.start(t)
	if rest.Replayed || rest.PublicID == a.PublicID || rest.PublicID == b.PublicID || rest.Amount != inr(5600) || rest.Payer != "" {
		t.Errorf("the rest = %+v, want a charge of its own for the 56.00 nobody is paying", rest)
	}
}

// TestSharesNeverClaimMoreThanTheBill: what others are paying is spoken for.
// A share that does not fit in the rest is refused before any gateway is
// asked, so there is no page on which to pay it.
func TestSharesNeverClaimMoreThanTheBill(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.share(t, "phone-a", 10000)

	if _, err := w.s.StartShare(ctx, 1, 4, "phone-b", inr(10000), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Fatalf("100.00 with 68.00 left: %v", err)
	}
	if len(w.gw.Started) != 1 {
		t.Fatalf("the gateway was asked for a share that did not fit: %d charges", len(w.gw.Started))
	}
	w.share(t, "phone-b", 6800)
	if _, err := w.s.StartShare(ctx, 1, 4, "phone-c", inr(1), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Errorf("a paisa with nothing left: %v", err)
	}
	// And nothing is left for a charge for the rest.
	if _, err := w.s.Start(ctx, 1, 4, "", ""); !errors.Is(err, ErrOverClaim) {
		t.Errorf("the rest, with nothing left: %v", err)
	}

	got, err := w.s.Claims(ctx, 1, 4, "phone-c")
	if err != nil || got != (Claims{Due: inr(16800), Claimed: inr(16800), Unclaimed: inr(0)}) {
		t.Errorf("claims as phone-c sees them = %+v, %v", got, err)
	}
	got, err = w.s.Claims(ctx, 1, 4, "phone-a")
	if err != nil || got != (Claims{Due: inr(16800), Claimed: inr(6800), Unclaimed: inr(10000)}) {
		t.Errorf("claims as phone-a sees them = %+v, %v", got, err)
	}
}

// TestAPhoneCanChangeItsMind: a phone's own open share does not stand in its
// own way, since the guest who backed out of a page cannot close it. Everyone
// else still sees all of it claimed.
func TestAPhoneCanChangeItsMind(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	first := w.share(t, "phone-a", 10000)
	second := w.share(t, "phone-a", 16800)
	if second.Replayed || second.PublicID == first.PublicID || second.Amount != inr(16800) {
		t.Fatalf("the second thought = %+v", second)
	}
	if _, err := w.s.StartShare(ctx, 1, 4, "phone-b", inr(100), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Errorf("another phone, with both of phone-a's pages open: %v", err)
	}

	// Paying both pages is the guest's own doing: the first lands, and the
	// second is more than the bill owes, kept for a person (ADR-220).
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.gw.Pay(gatewayID(2), gateway.UPI)
	if got := w.settle(t, first); got.Status != Paid {
		t.Fatalf("the first page: %+v", got)
	}
	if got := w.settle(t, second); got.Status != Unapplied {
		t.Fatalf("the second page: %+v", got)
	}
	if st := w.state(t); len(st.Tenders) != 1 || st.Totals.BalanceDue != inr(6800) {
		t.Errorf("%d tenders and %v owed", len(st.Tenders), st.Totals.BalanceDue)
	}
}

// TestAShareLetsGoWhenItsPageCloses: a claim lasts as long as its page can
// be paid. Failed or expired at the gateway, or paid, it is no claim.
func TestAShareLetsGoWhenItsPageCloses(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	a := w.share(t, "phone-a", 16800)
	if _, err := w.s.StartShare(ctx, 1, 4, "phone-b", inr(8400), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Fatalf("while phone-a's page is open: %v", err)
	}
	w.gw.Fail(gatewayID(1))
	if got := w.settle(t, a); got.Status != Failed {
		t.Fatalf("the failed page: %+v", got)
	}
	b := w.share(t, "phone-b", 8400)

	// Paid, a share is on the bill and not a claim on it: the other half
	// is free to take.
	w.gw.Pay(gatewayID(2), gateway.Card)
	if got := w.settle(t, b); got.Status != Paid {
		t.Fatalf("phone-b's half: %+v", got)
	}
	got, err := w.s.Claims(ctx, 1, 4, "phone-c")
	if err != nil || got != (Claims{Due: inr(8400), Claimed: inr(0), Unclaimed: inr(8400)}) {
		t.Errorf("claims after one half is paid = %+v, %v", got, err)
	}
	w.share(t, "phone-c", 8400)
}

// TestPayingTheRest: with shares open, a charge for the whole bill is for
// what nobody is paying, and is still the one charge for every phone.
func TestPayingTheRest(t *testing.T) {
	w := newWorld(t)
	a := w.share(t, "phone-a", 6800)
	rest := w.start(t)
	if rest.Amount != inr(10000) || rest.Payer != "" || rest.Replayed {
		t.Fatalf("the rest = %+v, want 100.00", rest)
	}
	if again := w.start(t); !again.Replayed || again.PublicID != rest.PublicID {
		t.Errorf("another phone paying the rest = %+v, want %s again", again, rest.PublicID)
	}
	// Once the share is paid the bill owes 100.00, which is the same charge.
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.settle(t, a)
	if again := w.start(t); !again.Replayed || again.PublicID != rest.PublicID {
		t.Errorf("after the share was paid = %+v, want %s again", again, rest.PublicID)
	}
	w.gw.Pay(gatewayID(2), gateway.Card)
	if got := w.settle(t, rest); got.Status != Paid {
		t.Fatalf("the rest: %+v", got)
	}
	if st := w.state(t); len(st.Tenders) != 2 || !st.Totals.BalanceDue.IsZero() || !st.Invoiced {
		t.Errorf("%d tenders, %v owed, invoiced %v", len(st.Tenders), st.Totals.BalanceDue, st.Invoiced)
	}
}

// TestAChargeForTheWholeBillLeavesNoShare: someone has the page for all of
// it open, so there is no part of it left to take.
func TestAChargeForTheWholeBillLeavesNoShare(t *testing.T) {
	w := newWorld(t)
	w.start(t)
	if _, err := w.s.StartShare(context.Background(), 1, 4, "phone-a", inr(5600), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Errorf("a share with the whole bill being paid: %v", err)
	}
}

// TestManyPhonesAtOnce: forty phones each ask for 50.00 of a bill of 168.00
// in the same moment. Three are given a page and thirty-seven are refused,
// so paying every page there is cannot over-pay the bill.
func TestManyPhonesAtOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	const phones = 40
	got := make([]Charge, phones)
	errs := make([]error, phones)
	var wg sync.WaitGroup
	for i := range phones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.StartShare(ctx, 1, 4, fmt.Sprintf("phone-%02d", i), inr(5000), "Table 4", "")
		}()
	}
	wg.Wait()

	var given []Charge
	for i := range phones {
		switch {
		case errs[i] == nil:
			given = append(given, got[i])
		case !errors.Is(errs[i], ErrOverClaim):
			t.Errorf("phone %d: %v", i, errs[i])
		}
	}
	if len(given) != 3 {
		t.Fatalf("%d phones were given a page for 50.00 of 168.00, want 3", len(given))
	}

	// Every page that can be paid is paid.
	for i := range w.gw.Started {
		w.gw.Pay(gatewayID(i+1), gateway.UPI)
	}
	if _, err := w.s.Sweep(ctx, 1); err != nil {
		t.Fatal(err)
	}
	st := w.state(t)
	if len(st.Tenders) != 3 || st.Totals.BalanceDue != inr(1800) {
		t.Errorf("%d tenders and %v owed, want 3 and 18.00", len(st.Tenders), st.Totals.BalanceDue)
	}
	if left, err := w.s.Waiting(ctx, 1); err != nil || len(left) != 0 {
		t.Errorf("unapplied after everyone paid: %+v, %v", left, err)
	}
}

// TestADoubleTapIsOneShare: one phone asking twice in the same moment.
func TestADoubleTapIsOneShare(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	const taps = 12
	got := make([]Charge, taps)
	errs := make([]error, taps)
	var wg sync.WaitGroup
	for i := range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.StartShare(ctx, 1, 4, "phone-a", inr(5600), "Table 4", "")
		}()
	}
	wg.Wait()
	for i := range taps {
		if errs[i] != nil {
			t.Fatalf("tap %d: %v", i, errs[i])
		}
		if got[i].PublicID != got[0].PublicID {
			t.Errorf("tap %d was given %s, and tap 0 %s", i, got[i].PublicID, got[0].PublicID)
		}
	}
	claims, err := w.s.Claims(ctx, 1, 4, "phone-b")
	if err != nil || claims.Claimed != inr(5600) {
		t.Errorf("claimed after %d taps = %v, %v; want one share of 56.00", taps, claims.Claimed, err)
	}
}

// movingOrders is the orders, with something happening to the bill just
// after it is read: the read is stale by the time it is acted on.
type movingOrders struct {
	Orders
	then func()
}

func (m *movingOrders) TableTicket(ctx context.Context, venueID, tableID int64) (svcorder.Result, bool, error) {
	res, open, err := m.Orders.TableTicket(ctx, venueID, tableID)
	if then := m.then; then != nil {
		m.then = nil
		then()
	}
	return res, open, err
}

// TestTheBillMovingUnderAShare: the table pays 100.00 in cash between the
// bill being read and the share being saved. The share is judged against the
// bill as it is, not as it was read.
func TestTheBillMovingUnderAShare(t *testing.T) {
	tests := []struct {
		name  string
		minor int64
		err   error
	}{
		{"a share that fit the bill as it was read, and not as it is", 16800, ErrOverClaim},
		{"a share that still fits", 6800, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			ctx := context.Background()
			moving := &movingOrders{Orders: w.orders, then: func() {
				w.must(t)(w.orders.TenderPayment(ctx, cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(10000)}))
			}}
			s, err := New(Options{DB: w.d, Orders: moving, Clock: fixedClock{}, Gateways: func(context.Context, int64) (gateway.Gateway, error) { return w.gw, nil }})
			if err != nil {
				t.Fatal(err)
			}
			c, err := s.StartShare(ctx, 1, 4, "phone-a", inr(tt.minor), "", "")
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			claims, cerr := s.Claims(ctx, 1, 4, "phone-b")
			if cerr != nil {
				t.Fatal(cerr)
			}
			if tt.err != nil {
				if !claims.Claimed.IsZero() {
					t.Errorf("a refused share is a claim of %v", claims.Claimed)
				}
				return
			}
			if c.Amount != inr(tt.minor) || claims != (Claims{Due: inr(6800), Claimed: inr(6800), Unclaimed: inr(0)}) {
				t.Errorf("charge %+v, claims %+v", c, claims)
			}
		})
	}
}

// TestAShareThatIsNotOne: who is paying and how much are both needed, and
// there has to be a bill.
func TestAShareThatIsNotOne(t *testing.T) {
	tests := []struct {
		name   string
		table  int64
		payer  string
		amount money.Amount
		before func(t *testing.T, w *world)
		err    error
	}{
		{name: "nobody's", table: 4, payer: "", amount: inr(5600), err: ErrBadShare},
		{name: "a name too long to be a phone's", table: 4, payer: strings.Repeat("a", 65), amount: inr(5600), err: ErrBadShare},
		{name: "of nothing", table: 4, payer: "phone-a", amount: inr(0), err: ErrBadShare},
		{name: "of less than nothing", table: 4, payer: "phone-a", amount: inr(-5600), err: ErrBadShare},
		{name: "in another currency", table: 4, payer: "phone-a", amount: money.Amount{Minor: 5600, Currency: "USD"}, err: ErrBadShare},
		{name: "of more than the bill", table: 4, payer: "phone-a", amount: inr(16801), err: ErrOverClaim},
		{name: "at a table with no ticket", table: 5, payer: "phone-a", amount: inr(5600), err: ErrNothingToPay},
		{
			name: "of a bill already paid", table: 4, payer: "phone-a", amount: inr(5600), err: ErrNothingToPay,
			before: func(t *testing.T, w *world) {
				w.must(t)(w.orders.TenderPayment(context.Background(), cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			if tt.before != nil {
				tt.before(t, w)
			}
			_, err := w.s.StartShare(context.Background(), 1, tt.table, tt.payer, tt.amount, "", "")
			if !errors.Is(err, tt.err) {
				t.Errorf("err = %v, want %v", err, tt.err)
			}
			if len(w.gw.Started) != 0 {
				t.Errorf("the gateway was asked %d times", len(w.gw.Started))
			}
		})
	}

	t.Run("at a venue with no gateway", func(t *testing.T) {
		w := newWorld(t)
		w.none = true
		if _, err := w.s.StartShare(context.Background(), 1, 4, "phone-a", inr(5600), "", ""); !errors.Is(err, ErrNoGateway) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("claims at a table with no ticket are nothing", func(t *testing.T) {
		w := newWorld(t)
		got, err := w.s.Claims(context.Background(), 1, 5, "phone-a")
		if err != nil || got != (Claims{}) {
			t.Errorf("claims = %+v, %v", got, err)
		}
	})
}
