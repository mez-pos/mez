package relay

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// keeper carries every venue and keeps what each says of its health: the
// shape of mez-cloud's entitlements (ADR-361).
type keeper struct {
	got chan keptHealth
	// hold, when set, is waited on inside VenueHealth, so a test can make
	// the sink slow and see what reaches it.
	hold chan struct{}
	// entered says VenueHealth has begun, once per call.
	entered chan struct{}

	mu   sync.Mutex
	errs int
}

type keptHealth struct {
	venueID string
	health  wire.Health
}

func newKeeper() *keeper {
	return &keeper{got: make(chan keptHealth, 16), entered: make(chan struct{}, 16)}
}

func (k *keeper) Entitled(context.Context, string) (Entitlement, error) {
	return Entitlement{Until: NoEnd, Remote: true}, nil
}

func (k *keeper) VenueHealth(ctx context.Context, venueID string, h wire.Health) error {
	k.entered <- struct{}{}
	if k.hold != nil {
		select {
		case <-k.hold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	k.got <- keptHealth{venueID, h}
	return nil
}

func (k *keeper) next(t *testing.T) keptHealth {
	t.Helper()
	select {
	case kh := <-k.got:
		return kh
	case <-time.After(testTimeout):
		t.Fatal("the sink was not handed a summary")
		return keptHealth{}
	}
}

func (k *keeper) none(t *testing.T, for_ time.Duration) {
	t.Helper()
	select {
	case kh := <-k.got:
		t.Fatalf("the sink was handed a summary it should not have been: %+v", kh)
	case <-time.After(for_):
	}
}

func summary(at time.Time) wire.Health {
	return wire.Health{At: at, Timezone: "Asia/Kolkata", Backup: wire.BackupHealth{State: "ok"}}
}

func sendHealth(t *testing.T, ws *websocket.Conn, h wire.Health) {
	t.Helper()
	msg, err := wire.EncodeHealth(h)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageBinary, msg); err != nil {
		t.Fatalf("sending health: %v", err)
	}
}

func TestAVenuesHealthReachesTheSink(t *testing.T) {
	k := newKeeper()
	f := newFixture(t, func(o *Options) { o.Entitlements = k })
	ws, resp, err := f.dial(mustIssue(t, f.keys, "cafe"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if got := resp.Header.Get(wire.HeaderHealth); got != "1" {
		t.Fatalf("the relay did not say it reads health: %s = %q", wire.HeaderHealth, got)
	}
	f.waitFor("cafe registered", func() bool { return f.srv.agentFor("cafe") != nil })

	at := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)
	sendHealth(t, ws, summary(at))

	kh := k.next(t)
	if kh.venueID != "cafe" || !kh.health.At.Equal(at) || kh.health.Backup.State != "ok" {
		t.Errorf("the sink was handed %+v", kh)
	}
}

func TestARelayWithNoSinkAsksForNoHealthAndDropsIt(t *testing.T) {
	f := newFixture(t, nil)
	ws, resp, err := f.dial(mustIssue(t, f.keys, "cafe"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if got := resp.Header.Get(wire.HeaderHealth); got != "" {
		t.Fatalf("a relay with no sink said it reads health: %q", got)
	}
	f.waitFor("cafe registered", func() bool { return f.srv.agentFor("cafe") != nil })

	// A venue that sends one anyway is not disconnected over it.
	sendHealth(t, ws, summary(time.Now()))
	time.Sleep(30 * time.Millisecond)
	a := f.srv.agentFor("cafe")
	if a == nil || a.ctx.Err() != nil {
		t.Fatal("the venue was disconnected over a health frame the relay did not ask for")
	}
}

func TestASlowSinkGetsTheNewestSummaryNotTheMiddleOne(t *testing.T) {
	k := newKeeper()
	k.hold = make(chan struct{})
	f := newFixture(t, func(o *Options) { o.Entitlements = k })
	ws := f.connect("cafe")

	base := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	sendHealth(t, ws, summary(base))
	// The sink is inside the first call before the next two arrive, so the
	// order is not a matter of luck.
	select {
	case <-k.entered:
	case <-time.After(testTimeout):
		t.Fatal("the sink was not called")
	}
	sendHealth(t, ws, summary(base.Add(time.Minute)))
	sendHealth(t, ws, summary(base.Add(2*time.Minute)))
	f.waitFor("the third summary waiting", func() bool {
		a := f.srv.agentFor("cafe")
		return a != nil && len(a.health) == 1
	})
	// Let the first call through, then the one waiting.
	k.hold <- struct{}{}
	k.hold <- struct{}{}

	if first := k.next(t); !first.health.At.Equal(base) {
		t.Errorf("first summary at %v, want %v", first.health.At, base)
	}
	if second := k.next(t); !second.health.At.Equal(base.Add(2 * time.Minute)) {
		t.Errorf("the sink was handed the summary at %v, want the newest at %v", second.health.At, base.Add(2*time.Minute))
	}
	k.none(t, 30*time.Millisecond)
}

func TestAnUnreadableSummaryIsPassedOver(t *testing.T) {
	k := newKeeper()
	f := newFixture(t, func(o *Options) { o.Entitlements = k })
	ws := f.connect("cafe")

	if err := writeFrame(ws, wire.Frame{Type: wire.TypeHealth, Payload: []byte(`{"at":`)}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)
	sendHealth(t, ws, summary(at))

	if kh := k.next(t); !kh.health.At.Equal(at) {
		t.Errorf("the sink was handed %+v, want the readable one", kh)
	}
	if a := f.srv.agentFor("cafe"); a == nil || a.ctx.Err() != nil {
		t.Fatal("the venue was disconnected over an unreadable health frame")
	}
}
