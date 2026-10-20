package tunnel

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/relay"
	"github.com/mez-pos/mez/internal/relay/wire"
)

// book is the relay's entitlements, which a test changes while a venue is
// connected.
type book struct {
	mu     sync.Mutex
	venues map[string]relay.Entitlement
}

func (b *book) Entitled(_ context.Context, venueID string) (relay.Entitlement, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.venues[venueID], nil
}

func (b *book) set(venueID string, e relay.Entitlement) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.venues == nil {
		b.venues = map[string]relay.Entitlement{}
	}
	b.venues[venueID] = e
}

// waitStanding waits for the venue to have been told something that ok
// accepts.
func waitStanding(t *testing.T, c *Client, what string, ok func(Standing) bool) Standing {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		if st := c.Status().Standing; st != nil && ok(*st) {
			return *st
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting to be told: %s; standing is %+v", what, c.Status().Standing)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

var (
	paidTo    = time.Date(2030, 11, 12, 0, 0, 0, 0, time.UTC)
	carriedTo = paidTo.AddDate(0, 0, 7)
)

// TestTheVenueIsToldWhereItStands: on connecting, and again when it changes
// while connected, with nothing dropped to say it. A back office the relay
// stops carrying is said, and the venue stays connected (ADR-316).
func TestTheVenueIsToldWhereItStands(t *testing.T) {
	entitled := &book{}
	entitled.set("cafe", relay.Entitlement{Until: carriedTo, PaidUntil: paidTo, Remote: true, State: "active"})
	r := startRelayWith(t, entitled, 20*time.Millisecond)
	c := r.venue(server(), func(o *Options) { o.Key = r.remoteKey("cafe") })

	connected := waitState(t, c, StateConnected)
	got := waitStanding(t, c, "its standing on connecting", func(Standing) bool { return true })
	if got.State != "active" || !got.PaidUntil.Equal(paidTo) || !got.Until.Equal(carriedTo) || !got.BackOffice || got.Reason != "" {
		t.Errorf("told on connecting: %+v", got)
	}

	later := paidTo.AddDate(0, 1, 0)
	entitled.set("cafe", relay.Entitlement{Until: later.AddDate(0, 0, 7), PaidUntil: later, Remote: true, State: "active"})
	waitStanding(t, c, "a later date once paid", func(s Standing) bool { return s.PaidUntil.Equal(later) })

	entitled.set("cafe", relay.Entitlement{Until: later.AddDate(0, 0, 7), PaidUntil: later, Remote: false, State: "active"})
	waitStanding(t, c, "the back office no longer carried", func(s Standing) bool { return !s.BackOffice })

	// Told three things and disconnected for none of them.
	if now := c.Status(); now.State != StateConnected || !now.Since.Equal(connected.Since) {
		t.Errorf("after being told: %+v, want connected since %v", now, connected.Since)
	}
	if status, _ := r.mustGuest(http.MethodGet, "/", nil); status != http.StatusOK {
		t.Errorf("a guest after the notices: %d", status)
	}
}

// TestAVenueNotCarriedAsksAgainWithinTheMinute: turned away at the dial with
// a good key, it says so and why, and comes back to ask in under a minute
// and not in the five a refused key waits.
func TestAVenueNotCarriedAsksAgainWithinTheMinute(t *testing.T) {
	entitled := &book{}
	entitled.set("cafe", relay.Entitlement{Until: time.Now().Add(-time.Hour), PaidUntil: paidTo.AddDate(-10, 0, 0), Remote: true, State: "suspended", Reason: "payment_failed"})
	r := startRelayWith(t, entitled, time.Hour)
	c := r.venue(server(), nil)

	s := waitState(t, c, StateNotEntitled)
	if wait := s.RetryAt.Sub(s.Since); wait > time.Minute || wait < 30*time.Second {
		t.Errorf("a venue not carried asks again in %v, want from half a minute to a minute", wait)
	}
	if !strings.Contains(s.LastError, "payment_failed") {
		t.Errorf("LastError = %q, want the relay's reason", s.LastError)
	}
	if s.Standing == nil || s.Standing.State != "suspended" || s.Standing.Reason != "payment_failed" || !s.Standing.Until.Before(time.Now()) {
		t.Errorf("standing = %+v, want what the relay said as it turned the venue away", s.Standing)
	}
	if strings.Contains(s.LastError, "mzr_") {
		t.Error("the status repeats the key")
	}
}

// TestAVenueComesBackByItselfOncePaid: turned away, then carried again, it
// connects at its next asking with nobody touching it.
func TestAVenueComesBackByItselfOncePaid(t *testing.T) {
	entitled := &book{}
	entitled.set("cafe", relay.Entitlement{Reason: "never"})
	r := startRelayWith(t, entitled, time.Hour)
	c := r.venue(server(), func(o *Options) { o.EntitledBackoff = 20 * time.Millisecond })

	waitState(t, c, StateNotEntitled)
	if status, _ := r.mustGuest(http.MethodGet, "/", nil); status != http.StatusServiceUnavailable {
		t.Errorf("a guest of a venue that is not carried: %d, want 503", status)
	}

	entitled.set("cafe", relay.Entitlement{Until: carriedTo, PaidUntil: paidTo, Remote: true, State: "active"})
	waitState(t, c, StateConnected)
	got := waitStanding(t, c, "its standing once carried", func(s Standing) bool { return s.State == "active" })
	if !got.PaidUntil.Equal(paidTo) {
		t.Errorf("told %+v", got)
	}
	if status, _ := r.mustGuest(http.MethodGet, "/", nil); status != http.StatusOK {
		t.Errorf("a guest once it is carried again: %d", status)
	}
}

// TestAVenueCutOffWhileConnected: its date passes, the relay closes it with
// its own code, and the venue says not entitled and why, where a dropped
// line would say waiting.
func TestAVenueCutOffWhileConnected(t *testing.T) {
	entitled := &book{}
	entitled.set("cafe", relay.Entitlement{Until: carriedTo, PaidUntil: paidTo, Remote: true, State: "grace", Reason: "payment_failed"})
	r := startRelayWith(t, entitled, 20*time.Millisecond)
	c := r.venue(server(), nil)
	waitState(t, c, StateConnected)

	entitled.set("cafe", relay.Entitlement{Until: time.Now().Add(-time.Second), PaidUntil: paidTo, Remote: true, State: "suspended", Reason: "payment_failed"})
	s := waitState(t, c, StateNotEntitled)
	if wait := s.RetryAt.Sub(s.Since); wait > time.Minute {
		t.Errorf("cut off while connected, it asks again in %v, want within the minute", wait)
	}
	if !strings.Contains(s.LastError, "payment_failed") {
		t.Errorf("LastError = %q, want the relay's reason", s.LastError)
	}
	// Told before it was closed, so the screen has more than the reason.
	if s.Standing == nil || s.Standing.State != "suspended" {
		t.Errorf("standing = %+v, want suspended", s.Standing)
	}
}

// TestARelayThatSaysNothing: one from before there was anything to say
// leaves nothing known, and a header that cannot be read is nothing too.
func TestARelayThatSaysNothing(t *testing.T) {
	if got := standingFrom(nil); got != nil {
		t.Errorf("no answer at all: %+v", got)
	}
	if got := standingFrom(&http.Response{Header: http.Header{}}); got != nil {
		t.Errorf("no header: %+v", got)
	}
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set(wire.HeaderEntitlement, "{not json")
	if got := standingFrom(resp); got != nil {
		t.Errorf("a header that cannot be read: %+v", got)
	}
	resp.Header.Set(wire.HeaderEntitlement, `{"remote":true}`)
	got := standingFrom(resp)
	if got == nil || !got.BackOffice || !got.Until.IsZero() || !got.PaidUntil.IsZero() || got.State != "" {
		t.Errorf("a relay with a file and no date: %+v", got)
	}

	// A relay with no entitlements at all still answers, with no date.
	r := startRelay(t)
	c := r.venue(server(), nil)
	waitState(t, c, StateConnected)
	told := waitStanding(t, c, "anything", func(Standing) bool { return true })
	if !told.Until.IsZero() || told.State != "" || told.BackOffice {
		t.Errorf("a relay that carries every key told a guests-only venue %+v", told)
	}
}
