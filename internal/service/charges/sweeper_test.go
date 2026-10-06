package charges

import (
	"context"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/gateway"
)

func TestSweepAllAsksNobodyWithNothingOpen(t *testing.T) {
	w := newWorld(t)
	closed, err := w.s.SweepAll(context.Background())
	if err != nil || closed != 0 {
		t.Fatalf("closed %d, %v", closed, err)
	}
	if w.gw.Asked != 0 {
		t.Errorf("the gateway was asked %d times with no charge open", w.gw.Asked)
	}
}

func TestSweepAllLandsAPayment(t *testing.T) {
	w := newWorld(t)
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)
	closed, err := w.s.SweepAll(context.Background())
	if err != nil || closed != 1 {
		t.Fatalf("closed %d, %v; want 1", closed, err)
	}
	if got, _ := w.s.Charge(context.Background(), 1, c.PublicID); got.Status != Paid {
		t.Errorf("charge is %s", got.Status)
	}
}

// TestTheSweeperLandsAPaymentByItself: nothing asks. The timer does.
func TestTheSweeperLandsAPaymentByItself(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.Card)

	sweeper := w.s.StartSweeper(10 * time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := w.s.Charge(ctx, 1, c.PublicID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == Paid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the charge is still %s after five seconds of sweeping", got.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if st := w.state(t); len(st.Tenders) != 1 {
		t.Errorf("%d tenders", len(st.Tenders))
	}

	stop, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sweeper.Close(stop); err != nil {
		t.Fatalf("closing: %v", err)
	}
	// Closed twice is closed once, and it sweeps no more.
	if err := sweeper.Close(stop); err != nil {
		t.Errorf("closing twice: %v", err)
	}
	asked := w.gw.Asked
	time.Sleep(40 * time.Millisecond)
	if w.gw.Asked != asked {
		t.Error("the sweeper asked the gateway after it was closed")
	}
}

// TestTheSweeperOutlastsAGatewayThatIsDown: it keeps going, and lands the
// payment when the gateway is back.
func TestTheSweeperOutlastsAGatewayThatIsDown(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.gw.Down(true)

	sweeper := w.s.StartSweeper(10 * time.Millisecond)
	defer func() {
		stop, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = sweeper.Close(stop)
	}()
	time.Sleep(60 * time.Millisecond)
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Fatalf("with the gateway down the charge is %s", got.Status)
	}
	w.gw.Down(false)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status == Paid {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the payment never landed once the gateway was back")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
