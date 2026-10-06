package charges

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/byjoellobo/mez/internal/gateway"
)

// teller is a gateway.Notified for tests: a notice is the gateway's when
// its header says so, and is about the charge its body names.
type teller struct{}

func (teller) Name() string { return "test" }

func (teller) Notice(header http.Header, body []byte) (string, error) {
	if header.Get("X-Test-Signed") != "yes" {
		return "", fmt.Errorf("%w: not signed", gateway.ErrRefused)
	}
	return string(body), nil
}

func signed() http.Header {
	h := http.Header{}
	h.Set("X-Test-Signed", "yes")
	return h
}

func (w *world) withNotices() { w.s.notices = teller{} }

// TestANoticeLandsAPaymentAtOnce: nobody asked and no sweep ran. The
// gateway called, and the payment is on the bill.
func TestANoticeLandsAPaymentAtOnce(t *testing.T) {
	w := newWorld(t)
	w.withNotices()
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)

	if err := w.s.Notice(ctx, "test", signed(), []byte(gatewayID(1))); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Paid {
		t.Errorf("charge is %s", got.Status)
	}
	if st := w.state(t); len(st.Tenders) != 1 {
		t.Errorf("%d tenders", len(st.Tenders))
	}
	// The same notice again, as a gateway sends one: taken, and one tender.
	if err := w.s.Notice(ctx, "test", signed(), []byte(gatewayID(1))); err != nil {
		t.Errorf("the notice again: %v", err)
	}
	if st := w.state(t); len(st.Tenders) != 1 {
		t.Errorf("%d tenders after the notice was sent twice", len(st.Tenders))
	}
}

// TestANoticeIsAHintAndNotAFact: the gateway's call says a charge moved.
// The gateway, asked, says it has not been paid. Nothing is on the bill.
func TestANoticeIsAHintAndNotAFact(t *testing.T) {
	w := newWorld(t)
	w.withNotices()
	ctx := context.Background()
	c := w.start(t)

	if err := w.s.Notice(ctx, "test", signed(), []byte(gatewayID(1))); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Errorf("a notice alone made the charge %s", got.Status)
	}
	if st := w.state(t); len(st.Tenders) != 0 {
		t.Fatal("a notice alone put money on the bill")
	}
	if w.gw.Asked != 1 {
		t.Errorf("the gateway was asked %d times, want once", w.gw.Asked)
	}
}

// TestAForgedNoticeDoesNothing: not the gateway's, so the gateway is not
// even asked.
func TestAForgedNoticeDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.withNotices()
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)

	err := w.s.Notice(ctx, "test", http.Header{}, []byte(gatewayID(1)))
	if !errors.Is(err, ErrBadNotice) {
		t.Fatalf("err = %v, want it refused", err)
	}
	if w.gw.Asked != 0 {
		t.Error("the gateway was asked on a forged notice's say-so")
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Errorf("a forged notice moved the charge to %s", got.Status)
	}
}

func TestNoticesThatAreTakenAndDoNothing(t *testing.T) {
	tests := []struct {
		name    string
		gateway string
		body    string
	}{
		{"about a charge mez never started", "test", "fake_9999"},
		{"about nothing", "test", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			w.withNotices()
			c := w.start(t)
			w.gw.Pay(gatewayID(1), gateway.UPI)
			if err := w.s.Notice(context.Background(), tt.gateway, signed(), []byte(tt.body)); err != nil {
				t.Fatalf("err = %v, want it taken", err)
			}
			if got, _ := w.s.Charge(context.Background(), 1, c.PublicID); got.Status != Open || w.gw.Asked != 0 {
				t.Errorf("the charge is %s and the gateway was asked %d times", got.Status, w.gw.Asked)
			}
		})
	}
}

// TestANoticeWithTheGatewayDownIsStillTaken: the gateway called and cannot
// now be asked. That is not the caller's fault, and the sweep asks later.
func TestANoticeWithTheGatewayDownIsStillTaken(t *testing.T) {
	w := newWorld(t)
	w.withNotices()
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)
	w.gw.Down(true)

	if err := w.s.Notice(ctx, "test", signed(), []byte(gatewayID(1))); err != nil {
		t.Fatalf("err = %v, want it taken", err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Errorf("charge is %s", got.Status)
	}
	w.gw.Down(false)
	if closed, err := w.s.Sweep(ctx, 1); err != nil || closed != 1 {
		t.Errorf("the sweep after: closed %d, %v", closed, err)
	}
}

func TestNoNoticesWithoutASecret(t *testing.T) {
	w := newWorld(t) // no Notices set
	if err := w.s.Notice(context.Background(), "test", signed(), []byte(gatewayID(1))); !errors.Is(err, ErrNoNotices) {
		t.Errorf("err = %v, want none taken", err)
	}
}

// TestANoticeAtAnotherGatewaysAddressIsNotTaken: a notice signed by the
// gateway that is set up but posted to another gateway's address passes
// the signature check and then finds no charge, which would look like a
// notice taken and done with. It is refused instead, so a misconfigured
// webhook is seen and not silent.
func TestANoticeAtAnotherGatewaysAddressIsNotTaken(t *testing.T) {
	w := newWorld(t)
	w.withNotices()
	ctx := context.Background()
	c := w.start(t)
	w.gw.Pay(gatewayID(1), gateway.UPI)

	if err := w.s.Notice(ctx, "stripe", signed(), []byte(gatewayID(1))); !errors.Is(err, ErrNoNotices) {
		t.Errorf("err = %v, want ErrNoNotices", err)
	}
	if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.Status != Open {
		t.Errorf("charge is %s, want still open: nothing was asked", got.Status)
	}
}
