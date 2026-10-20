package charges

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/gateway"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// unapplied makes a charge the bill cannot take: the table pays cash while
// the guest is on the gateway's page, and then the gateway says paid.
func (w *world) unapplied(t *testing.T) Charge {
	t.Helper()
	ctx := context.Background()
	c := w.start(t)
	w.must(t)(w.orders.TenderPayment(ctx, cashier, w.ticket.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(16800)}))
	w.gw.Pay(gatewayID(1), gateway.UPI)
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Unapplied {
		t.Fatalf("charge = %+v, %v; want it unapplied", got, err)
	}
	return got
}

func (w *world) floorVersion(t *testing.T) int64 {
	t.Helper()
	v, err := w.d.FloorVersion(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestAnUnappliedChargeTellsTheTerminals: money on no bill moves the
// floor's version and nudges the hub, as a guest's request does. A charge
// that is simply paid, or fails, does not.
func TestAnUnappliedChargeTellsTheTerminals(t *testing.T) {
	w := newWorld(t)
	var nudges atomic.Int64
	w.s.onChange = func(int64) { nudges.Add(1) }
	ctx := context.Background()

	before := w.floorVersion(t)
	c := w.unapplied(t)
	if w.floorVersion(t) != before+1 || nudges.Load() != 1 {
		t.Errorf("the floor moved %d and the terminals were told %d times, want 1 and 1", w.floorVersion(t)-before, nudges.Load())
	}

	list, err := w.s.Waiting(ctx, 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("waiting = %+v, %v", list, err)
	}
	got := list[0]
	if got.PublicID != c.PublicID || got.TableID != 4 || got.TableName != "4" || got.OrderPublicID != w.ticket.PublicID ||
		got.Gateway != "test" || got.GatewayID != gatewayID(1) || got.Reference != "pay_"+gatewayID(1) || got.Method != "upi" ||
		got.Amount != inr(16800) || got.Problem == "" || !got.At.Equal(now) {
		t.Errorf("unapplied charge = %+v", got)
	}

	// Settled again, it tells nobody again.
	if _, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || nudges.Load() != 1 {
		t.Errorf("settled again: %v, told %d times", err, nudges.Load())
	}
}

func TestAPaidOrFailedChargeTellsNobody(t *testing.T) {
	for _, pay := range []bool{true, false} {
		w := newWorld(t)
		var nudges atomic.Int64
		w.s.onChange = func(int64) { nudges.Add(1) }
		before := w.floorVersion(t)
		c := w.start(t)
		if pay {
			w.gw.Pay(gatewayID(1), gateway.Card)
		} else {
			w.gw.Fail(gatewayID(1))
		}
		if _, err := w.s.Settle(context.Background(), 1, c.PublicID); err != nil {
			t.Fatal(err)
		}
		if w.floorVersion(t) != before || nudges.Load() != 0 {
			t.Errorf("paid=%v: the floor moved %d, told %d times", pay, w.floorVersion(t)-before, nudges.Load())
		}
		if list, _ := w.s.Waiting(context.Background(), 1); len(list) != 0 {
			t.Errorf("paid=%v: %d charges waiting for a person", pay, len(list))
		}
	}
}

// TestResolving: a person says what they did. It is kept with who and
// when, it leaves the list, the terminals are told, and the bill is as it
// was: resolving moves no money.
func TestResolving(t *testing.T) {
	for _, how := range Resolutions() {
		t.Run(string(how), func(t *testing.T) {
			w := newWorld(t)
			ctx := context.Background()
			c := w.unapplied(t)
			var nudges atomic.Int64
			w.s.onChange = func(int64) { nudges.Add(1) }
			version := w.floorVersion(t)
			bill := w.state(t)

			if err := w.s.Resolve(ctx, 1, 7, c.PublicID, how); err != nil {
				t.Fatal(err)
			}
			if list, _ := w.s.Waiting(ctx, 1); len(list) != 0 {
				t.Errorf("still waiting: %+v", list)
			}
			if w.floorVersion(t) != version+1 || nudges.Load() != 1 {
				t.Error("resolving did not tell the terminals")
			}
			var resolution string
			var by int64
			var status string
			if err := w.d.Reader().QueryRow(`SELECT resolution, resolved_by, status FROM gateway_charges WHERE public_id = ?`, c.PublicID).
				Scan(&resolution, &by, &status); err != nil {
				t.Fatal(err)
			}
			// It is still unapplied: that is what happened to it.
			if resolution != string(how) || by != 7 || status != string(Unapplied) {
				t.Errorf("kept as %s by %d with status %s", resolution, by, status)
			}
			after := w.state(t)
			if after.LastSeq != bill.LastSeq || len(after.Tenders) != len(bill.Tenders) {
				t.Error("resolving a charge changed the bill")
			}

			// Said twice, the second is refused, and tells nobody.
			if err := w.s.Resolve(ctx, 1, 7, c.PublicID, how); !errors.Is(err, ErrNotUnapplied) {
				t.Errorf("resolved twice: %v", err)
			}
			if nudges.Load() != 1 {
				t.Error("the second resolve told the terminals")
			}
		})
	}
}

func TestResolveIsRefused(t *testing.T) {
	tests := []struct {
		name string
		do   func(t *testing.T, w *world) error
		want error
	}{
		{"without saying how", func(t *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 1, 7, w.unapplied(t).PublicID, "")
		}, ErrResolution},
		{"in a way there is not", func(t *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 1, 7, w.unapplied(t).PublicID, "forgiven")
		}, ErrResolution},
		{"by nobody", func(t *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 1, 0, w.unapplied(t).PublicID, Refunded)
		}, ErrResolution},
		{"for a charge that is open", func(t *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 1, 7, w.start(t).PublicID, Refunded)
		}, ErrNotUnapplied},
		{"for a charge that was simply paid", func(t *testing.T, w *world) error {
			c := w.start(t)
			w.gw.Pay(gatewayID(1), gateway.UPI)
			if _, err := w.s.Settle(context.Background(), 1, c.PublicID); err != nil {
				t.Fatal(err)
			}
			return w.s.Resolve(context.Background(), 1, 7, c.PublicID, Recorded)
		}, ErrNotUnapplied},
		{"for another venue's charge", func(t *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 2, 7, w.unapplied(t).PublicID, Refunded)
		}, ErrChargeNotFound},
		{"for a charge that does not exist", func(_ *testing.T, w *world) error {
			return w.s.Resolve(context.Background(), 1, 7, "chg_NOPE0000", Refunded)
		}, ErrChargeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			if err := tt.do(t, w); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
