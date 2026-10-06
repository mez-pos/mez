package charges

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/gateway/gatewaytest"
)

// TestStandingNamesTheGatewayAndWhenItLastTookAPayment: nothing has been
// paid until something has, and a charge only started, failed or
// unapplied is not a payment that landed.
func TestStandingNamesTheGatewayAndWhenItLastTookAPayment(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	st, err := w.s.Standing(ctx, 1)
	if err != nil || st.Gateway != "test" || st.Notices || !st.LastPaidAt.IsZero() {
		t.Fatalf("at the start: %+v, %v", st, err)
	}

	c := w.start(t)
	if st, _ = w.s.Standing(ctx, 1); !st.LastPaidAt.IsZero() {
		t.Errorf("a charge only started counts as paid: %v", st.LastPaidAt)
	}
	w.gw.Fail(gatewayID(1))
	if _, err := w.s.Settle(ctx, 1, c.PublicID); err != nil {
		t.Fatal(err)
	}
	if st, _ = w.s.Standing(ctx, 1); !st.LastPaidAt.IsZero() {
		t.Errorf("a failed charge counts as paid: %v", st.LastPaidAt)
	}

	// The gateway takes another amount: unapplied, and still not a payment
	// that landed on a bill.
	wrong := w.start(t)
	w.gw.PayAmount(gatewayID(2), inr(100))
	if got, err := w.s.Settle(ctx, 1, wrong.PublicID); err != nil || got.Status != Unapplied {
		t.Fatalf("got %+v, %v", got, err)
	}
	if st, _ = w.s.Standing(ctx, 1); !st.LastPaidAt.IsZero() {
		t.Errorf("an unapplied charge counts as paid: %v", st.LastPaidAt)
	}

	paid := w.start(t)
	w.gw.Pay(gatewayID(3), gateway.UPI)
	if got, err := w.s.Settle(ctx, 1, paid.PublicID); err != nil || got.Status != Paid {
		t.Fatalf("got %+v, %v", got, err)
	}
	st, err = w.s.Standing(ctx, 1)
	if err != nil || !st.LastPaidAt.Equal(now) {
		t.Errorf("after a payment: %+v, %v; want it at %v", st, err, now)
	}
	// Another venue's payments are not this one's.
	if other, err := w.s.Standing(ctx, 2); err != nil || !other.LastPaidAt.IsZero() {
		t.Errorf("venue 2: %+v, %v", other, err)
	}
}

func TestStandingWithNoGateway(t *testing.T) {
	w := newWorld(t)
	w.none = true
	if _, err := w.s.Standing(context.Background(), 1); !errors.Is(err, ErrNoGateway) {
		t.Errorf("err = %v", err)
	}
}

// TestStandingSaysWhetherNoticesAreTaken: only when the service was given
// a gateway that calls back.
func TestStandingSaysWhetherNoticesAreTaken(t *testing.T) {
	w := newWorld(t)
	told, err := New(Options{
		DB: w.d, Orders: w.orders, Clock: fixedClock{}, Notices: teller{},
		Gateways: func(context.Context, int64) (gateway.Gateway, error) { return gatewaytest.New(), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := told.Standing(context.Background(), 1)
	if err != nil || !st.Notices {
		t.Errorf("st = %+v, %v", st, err)
	}
}
