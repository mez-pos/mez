package tunnel

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/relay"
	"github.com/mez-pos/mez/internal/relay/wire"
)

// keeper carries every venue and keeps what each says of its health: the
// relay as mez-cloud runs it (ADR-361).
type keeper struct {
	got chan wire.Health
}

func newKeeper() *keeper { return &keeper{got: make(chan wire.Health, 64)} }

func (k *keeper) Entitled(context.Context, string) (relay.Entitlement, error) {
	return relay.Entitlement{Until: relay.NoEnd, Remote: true}, nil
}

func (k *keeper) VenueHealth(_ context.Context, _ string, h wire.Health) error {
	k.got <- h
	return nil
}

func (k *keeper) next(t *testing.T) wire.Health {
	t.Helper()
	select {
	case h := <-k.got:
		return h
	case <-time.After(testTimeout):
		t.Fatal("the relay was not sent a summary")
		return wire.Health{}
	}
}

func (k *keeper) none(t *testing.T, for_ time.Duration) {
	t.Helper()
	select {
	case h := <-k.got:
		t.Fatalf("the relay was sent a summary it should not have been: %+v", h)
	case <-time.After(for_):
	}
}

// source is a venue's health as a test sets it, counting how often it was asked.
type source struct {
	mu    sync.Mutex
	state string
	err   error
	asked atomic.Int64
}

func (s *source) set(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *source) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *source) health(context.Context) (wire.Health, error) {
	s.asked.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return wire.Health{}, s.err
	}
	return wire.Health{At: time.Now(), Timezone: "Asia/Kolkata", Backup: wire.BackupHealth{State: s.state}}, nil
}

func healthVenue(r *testRelay, src *source, every, poll time.Duration) *Client {
	return r.venue(server(), func(o *Options) {
		o.Health = src.health
		o.HealthEvery = every
		o.HealthPoll = poll
	})
}

// TestTheVenueSendsItsHealthOnConnectingAndWhenItChanges: a relay that asked
// is sent the summary at once, and again within a poll of a change, and
// nothing between while nothing changes.
func TestTheVenueSendsItsHealthOnConnectingAndWhenItChanges(t *testing.T) {
	k := newKeeper()
	r := startRelayWith(t, k, 0)
	src := &source{state: "ok"}
	c := healthVenue(r, src, time.Hour, 5*time.Millisecond)
	waitState(t, c, StateConnected)

	if h := k.next(t); h.Backup.State != "ok" || h.Timezone != "Asia/Kolkata" {
		t.Fatalf("the first summary was %+v", h)
	}
	k.none(t, 40*time.Millisecond)

	src.set("failing")
	if h := k.next(t); h.Backup.State != "failing" {
		t.Fatalf("after a change the relay was sent %+v", h)
	}
	k.none(t, 40*time.Millisecond)
	if src.asked.Load() < 3 {
		t.Errorf("the health was read %d times; it should be read every poll", src.asked.Load())
	}
}

// TestAnUnchangedHealthIsSentAgainEveryHealthEvery: a summary that has not
// changed goes again when HealthEvery has passed, so a relay hearing nothing
// knows the venue is gone.
func TestAnUnchangedHealthIsSentAgainEveryHealthEvery(t *testing.T) {
	k := newKeeper()
	r := startRelayWith(t, k, 0)
	src := &source{state: "ok"}
	c := healthVenue(r, src, 30*time.Millisecond, 5*time.Millisecond)
	waitState(t, c, StateConnected)

	first := k.next(t)
	second := k.next(t)
	if second.Backup.State != first.Backup.State {
		t.Fatalf("the second summary differs: %+v, then %+v", first, second)
	}
	if !second.At.After(first.At) {
		t.Errorf("the second summary's clock %v is not after the first's %v", second.At, first.At)
	}
}

// TestARelayThatDidNotAskIsSentNoHealth: the relay said nothing about health
// on the dial, so the venue sends none and does not even read it.
func TestARelayThatDidNotAskIsSentNoHealth(t *testing.T) {
	r := startRelayWith(t, &book{}, 0)
	bk := r.entitled.(*book)
	bk.set("cafe", relay.Entitlement{Until: relay.NoEnd, Remote: true})
	src := &source{state: "ok"}
	c := healthVenue(r, src, 5*time.Millisecond, 5*time.Millisecond)
	waitState(t, c, StateConnected)

	time.Sleep(40 * time.Millisecond)
	if n := src.asked.Load(); n != 0 {
		t.Errorf("the health was read %d times for a relay that did not ask", n)
	}
	// And the line is as good as ever.
	if status, _ := r.mustGuest(http.MethodGet, "/order/", nil); status != http.StatusOK {
		t.Errorf("a guest got %d", status)
	}
}

// TestAHealthThatCannotBeReadIsTriedAgain: an error reading the venue's
// health sends nothing and is not the end of it.
func TestAHealthThatCannotBeReadIsTriedAgain(t *testing.T) {
	k := newKeeper()
	r := startRelayWith(t, k, 0)
	src := &source{state: "ok"}
	src.fail(errors.New("the database is busy"))
	c := healthVenue(r, src, time.Hour, 5*time.Millisecond)
	waitState(t, c, StateConnected)

	k.none(t, 30*time.Millisecond)
	src.fail(nil)
	if h := k.next(t); h.Backup.State != "ok" {
		t.Fatalf("once readable the relay was sent %+v", h)
	}
}

// TestTheHealthIsSentAgainOnEachConnection: a relay that comes back is told
// at once, as a relay that restarted has forgotten.
func TestTheHealthIsSentAgainOnEachConnection(t *testing.T) {
	k := newKeeper()
	r := startRelayWith(t, k, 0)
	src := &source{state: "ok"}
	c := healthVenue(r, src, time.Hour, 5*time.Millisecond)
	waitState(t, c, StateConnected)
	k.next(t)

	r.kill()
	waitState(t, c, StateWaiting)
	r.restart()
	waitState(t, c, StateConnected)
	if h := k.next(t); h.Backup.State != "ok" {
		t.Fatalf("on reconnecting the relay was sent %+v", h)
	}
}

func TestFingerprintIgnoresTheClock(t *testing.T) {
	a := wire.Health{At: time.Now(), Backup: wire.BackupHealth{State: "ok"}}
	b := a
	b.At = a.At.Add(time.Minute)
	if string(fingerprint(a)) != string(fingerprint(b)) {
		t.Error("two summaries of one state at different times fingerprint differently")
	}
	b.Backup.State = "failing"
	if string(fingerprint(a)) == string(fingerprint(b)) {
		t.Error("two summaries of different states fingerprint alike")
	}
}
