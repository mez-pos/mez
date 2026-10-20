package relay

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// book is entitlements a test can change while venues are connected.
type book struct {
	mu     sync.Mutex
	venues map[string]Entitlement
	err    error
	asked  int
}

func (b *book) Entitled(_ context.Context, venueID string) (Entitlement, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.asked++
	if b.err != nil {
		return Entitlement{}, b.err
	}
	return b.venues[venueID], nil
}

func (b *book) set(venueID string, e Entitlement) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.venues == nil {
		b.venues = map[string]Entitlement{}
	}
	b.venues[venueID] = e
}

func (b *book) fail(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.err = err
}

func (b *book) times() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.asked
}

// dialWith connects as a venue with a key of its own making, saying whether
// it reads notices.
func (f *fixture) dialWith(venueID string, remote, notices bool) (*websocket.Conn, *http.Response, error) {
	f.t.Helper()
	key, err := f.keys.Issue(rand.Reader, venueID, remote)
	if err != nil {
		f.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	header := http.Header{"Authorization": {"Bearer " + key}}
	if notices {
		header.Set(wire.HeaderNotices, "1")
	}
	ws, resp, err := websocket.Dial(ctx, f.http.URL+"/agent", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		ws.SetReadLimit(wire.MaxFrame)
		f.t.Cleanup(func() { _ = ws.CloseNow() })
	}
	return ws, resp, err
}

// toldOn reads what the relay said of the venue's standing on its answer to
// the dial.
func toldOn(t *testing.T, resp *http.Response) wire.Entitlement {
	t.Helper()
	var e wire.Entitlement
	raw := resp.Header.Get(wire.HeaderEntitlement)
	if raw == "" {
		t.Fatalf("the answer to the dial carries no %s", wire.HeaderEntitlement)
	}
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("%s is not an entitlement: %v: %s", wire.HeaderEntitlement, err, raw)
	}
	return e
}

func soon(d time.Duration) time.Time { return time.Now().Add(d) }

// TestAVenueIsAskedAboutWhenItConnects: a venue with a good key is let in
// only while it is entitled, and is told where it stands either way.
func TestAVenueIsAskedAboutWhenItConnects(t *testing.T) {
	paid := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	entitlements := &book{}
	entitlements.set("cafe", Entitlement{Until: paid.AddDate(1, 0, 7), PaidUntil: paid.AddDate(1, 0, 0), Remote: true, State: "active"})
	entitlements.set("lapsed", Entitlement{Until: time.Now().Add(-time.Hour), PaidUntil: paid, Remote: true, State: "suspended", Reason: "payment_failed"})
	f := newFixture(t, func(o *Options) { o.Entitlements = entitlements })

	t.Run("entitled", func(t *testing.T) {
		_, resp, err := f.dialWith("cafe", true, false)
		if err != nil {
			t.Fatalf("an entitled venue could not connect: %v", err)
		}
		got := toldOn(t, resp)
		if got.State != "active" || !got.Remote || got.Until == nil || got.PaidUntil == nil || !got.PaidUntil.Equal(paid.AddDate(1, 0, 0)) {
			t.Errorf("told %+v", got)
		}
	})

	t.Run("past its date", func(t *testing.T) {
		_, resp, err := f.dialWith("lapsed", true, false)
		if err == nil {
			t.Fatal("a venue past its date was let in")
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("response = %v, want 403", resp)
		}
		got := toldOn(t, resp)
		if got.State != "suspended" || got.Reason != "payment_failed" {
			t.Errorf("told %+v, want suspended for a failed payment", got)
		}
		if code := errorCode(t, resp); code != wire.CodeNotEntitled {
			t.Errorf("code = %q, want %q", code, wire.CodeNotEntitled)
		}
		if f.srv.agentFor("lapsed") != nil {
			t.Error("a venue that was turned away is registered")
		}
	})

	t.Run("nobody has heard of it", func(t *testing.T) {
		_, resp, err := f.dialWith("stranger", false, false)
		if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("a venue with no entitlement: %v %v, want 403", resp, err)
		}
		if got := toldOn(t, resp); got.Until != nil || got.Remote {
			t.Errorf("told %+v, want no date and no back office", got)
		}
	})

	t.Run("a store that is down lets a good key in", func(t *testing.T) {
		entitlements.fail(errors.New("the disk has gone"))
		defer entitlements.fail(nil)
		_, resp, err := f.dialWith("lapsed", true, false)
		if err != nil {
			t.Fatalf("with entitlements unreadable, a good key was turned away: %v", err)
		}
		if got := toldOn(t, resp); !got.Remote || got.Until != nil {
			t.Errorf("told %+v, want what the key says and no date", got)
		}
		if !strings.Contains(f.logs.String(), "entitlements could not be read") {
			t.Error("the failure was not logged")
		}
	})
}

// TestAVenuePastItsDateIsClosed: a connected venue whose date passes is
// closed at that moment with the reason, having been told its standing if
// it reads notices, and its guests are told it is offline.
func TestAVenuePastItsDateIsClosed(t *testing.T) {
	entitlements := &book{}
	entitlements.set("cafe", Entitlement{Until: soon(250 * time.Millisecond), Remote: true, State: "grace", Reason: "payment_failed"})
	// An hour between askings: what closes it is the date, known in advance.
	f := newFixture(t, func(o *Options) { o.Entitlements = entitlements })

	ws, _, err := f.dialWith("cafe", true, true)
	if err != nil {
		t.Fatal(err)
	}
	f.waitFor("registered", func() bool { return f.srv.agentFor("cafe") != nil })
	entitlements.set("cafe", Entitlement{Until: time.Now().Add(-time.Second), Remote: true, State: "suspended", Reason: "payment_failed"})

	frame, err := readFrame(ws)
	if err != nil {
		t.Fatalf("waiting to be told: %v", err)
	}
	told, err := wire.DecodeNotice(frame)
	if err != nil || told.State != "suspended" || told.Reason != "payment_failed" {
		t.Errorf("told %+v, %v; want suspended for a failed payment", told, err)
	}
	_, err = readFrame(ws)
	if websocket.CloseStatus(err) != wire.CloseNotEntitled {
		t.Fatalf("closed with %v, want %d", err, wire.CloseNotEntitled)
	}
	var closed websocket.CloseError
	if !errors.As(err, &closed) || closed.Reason != "payment_failed" {
		t.Errorf("close reason = %q, want payment_failed", closed.Reason)
	}

	f.waitFor("unregistered", func() bool { return f.srv.agentFor("cafe") == nil })
	resp := f.do(http.MethodGet, "/v/cafe/", nil, nil)
	if code := resp.StatusCode; code != http.StatusServiceUnavailable {
		t.Errorf("a guest of a closed venue: %d, want 503", code)
	}
	_ = resp.Body.Close()
}

// TestAConnectedVenueIsAskedAboutAgain: every round, with no date in sight.
// A back office that is no longer carried turns into 404 without the venue
// being disconnected, a change is told to a venue that reads notices and to
// no other, and a store that fails closes nobody.
func TestAConnectedVenueIsAskedAboutAgain(t *testing.T) {
	entitlements := &book{}
	entitlements.set("club", Entitlement{Until: NoEnd, Remote: true, State: "active"})
	entitlements.set("old", Entitlement{Until: NoEnd, Remote: true, State: "active"})
	f := newFixture(t, func(o *Options) {
		o.Entitlements = entitlements
		o.EntitlementEvery = 30 * time.Millisecond
	})

	club, _, err := f.dialWith("club", true, true)
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := f.dialWith("old", true, false)
	if err != nil {
		t.Fatal(err)
	}
	f.waitFor("both registered", func() bool { return f.srv.agentFor("club") != nil && f.srv.agentFor("old") != nil })
	f.waitFor("asked about again", func() bool { return entitlements.times() >= 6 })

	// Nothing changed: nothing is said, and both are still carried.
	if !f.srv.agentFor("club").remote.Load() {
		t.Fatal("the back office stopped being carried with nothing changed")
	}

	entitlements.fail(errors.New("the disk has gone"))
	before := entitlements.times()
	f.waitFor("asked while the store is down", func() bool { return entitlements.times() >= before+3 })
	if f.srv.agentFor("club") == nil || f.srv.agentFor("old") == nil {
		t.Fatal("a store that failed disconnected a venue")
	}
	entitlements.fail(nil)

	paid := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"club", "old"} {
		entitlements.set(id, Entitlement{Until: NoEnd, PaidUntil: paid, Remote: false, State: "active"})
	}
	f.waitFor("the back office no longer carried", func() bool {
		return !f.srv.agentFor("club").remote.Load() && !f.srv.agentFor("old").remote.Load()
	})

	frame, err := readFrame(club)
	if err != nil {
		t.Fatalf("the venue that reads notices was told nothing: %v", err)
	}
	told, err := wire.DecodeNotice(frame)
	if err != nil || told.Remote || told.PaidUntil == nil || !told.PaidUntil.Equal(paid) {
		t.Errorf("told %+v, %v; want no back office and paid until %v", told, err, paid)
	}

	// The staff paths are now what they are on a guests-only key, and the
	// venue is not asked: the first thing the old one hears is a guest.
	resp := f.do(http.MethodGet, "/v/old/api/v1/menu", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("the staff API of a venue whose back office is not carried: %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
	go func() {
		resp, err := http.Get(f.http.URL + "/v/old/t/abc")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	first, err := readFrame(old)
	if err != nil || first.Type != wire.TypeRequest {
		t.Errorf("the venue that reads no notices was sent %v, %v; want only a guest's request", first.Type, err)
	}
	_ = respond(old, first.Stream, answer{status: http.StatusOK})
}

// TestARelayWithNoEntitlementsCarriesEveryKey: as it always did, and says so.
func TestARelayWithNoEntitlementsCarriesEveryKey(t *testing.T) {
	f := newFixture(t, nil)
	_, resp, err := f.dialWith("cafe", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := toldOn(t, resp); !got.Remote || got.Until != nil || got.State != "" {
		t.Errorf("told %+v, want carried with no date", got)
	}
	_, resp, err = f.dialWith("corner", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := toldOn(t, resp); got.Remote {
		t.Errorf("a guests-only key was told its back office is carried: %+v", got)
	}
}

func TestParseEntitlements(t *testing.T) {
	file := `
# venue      until                  back office
my-cafe      2026-11-12             remote
corner-bar   forever                guests   # the neighbours
old-place    never
by-the-hour  2026-11-12T18:30:00Z
*            2027-01-01
`
	venues, rest, err := parseEntitlements(strings.NewReader(file))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]Entitlement{
		"my-cafe":     {Until: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Remote: true},
		"corner-bar":  {Until: NoEnd},
		"old-place":   {Remote: true, Reason: ReasonNever},
		"by-the-hour": {Until: time.Date(2026, 11, 12, 18, 30, 0, 0, time.UTC), Remote: true},
	}
	if len(venues) != len(want) {
		t.Fatalf("%d venues, want %d: %+v", len(venues), len(want), venues)
	}
	for id, w := range want {
		if got := venues[id]; !got.same(w) {
			t.Errorf("%s = %+v, want %+v", id, got, w)
		}
	}
	if rest == nil || !rest.Until.Equal(time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)) || !rest.Remote {
		t.Errorf("* = %+v", rest)
	}
	// A day is carried through its end.
	day := venues["my-cafe"]
	if !day.covers(time.Date(2026, 11, 12, 23, 59, 59, 0, time.UTC)) || day.covers(time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)) {
		t.Error("a venue carried until a day is not carried through the end of it and no further")
	}
	if venues["old-place"].covers(time.Now()) {
		t.Error("never is carried")
	}

	for name, bad := range map[string]string{
		"one word":        "my-cafe",
		"four words":      "my-cafe forever remote please",
		"not a date":      "my-cafe tomorrow",
		"not a scope":     "my-cafe forever staff",
		"not a venue id":  "My_Cafe forever",
		"a venue twice":   "my-cafe forever\nmy-cafe never",
		"the rest twice":  "* forever\n* never",
		"a line too long": "my-cafe forever # " + strings.Repeat("x", maxEntitlementLine),
	} {
		if _, _, err := parseEntitlements(strings.NewReader(bad)); err == nil {
			t.Errorf("%s: %q was read as entitlements", name, bad)
		}
	}
}

// TestStaticEntitlements: the file answers, a venue it does not name is not
// carried unless * says so, an edit is read at the next asking, and an edit
// that cannot be read leaves what was there.
func TestStaticEntitlements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "entitlements")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		// A file system's clock is coarse, and two writes inside one tick
		// with the same length would look like none.
		later := time.Now().Add(time.Duration(len(body)) * time.Second)
		if err := os.Chtimes(path, later, later); err != nil {
			t.Fatal(err)
		}
	}
	logs := &syncBuffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	ctx := context.Background()

	if _, err := LoadStaticEntitlements(path, log); err == nil {
		t.Fatal("a file that is not there was loaded")
	}
	write("my-cafe never-ever\n")
	if _, err := LoadStaticEntitlements(path, log); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("a file that cannot be read: %v, want an error naming the line", err)
	}

	write("my-cafe forever\ncorner 2020-01-01 guests\n")
	s, err := LoadStaticEntitlements(path, log)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
	now := time.Now()
	if e, _ := s.Entitled(ctx, "my-cafe"); !e.covers(now) || !e.Remote {
		t.Errorf("my-cafe = %+v, want carried", e)
	}
	if e, _ := s.Entitled(ctx, "corner"); e.covers(now) || e.reason() != ReasonExpired {
		t.Errorf("corner = %+v, want past its date", e)
	}
	if e, _ := s.Entitled(ctx, "stranger"); e.covers(now) || e.Reason != ReasonNotListed {
		t.Errorf("a venue not named = %+v, want not carried and saying why", e)
	}

	write("my-cafe never\n* forever guests\n")
	if e, _ := s.Entitled(ctx, "my-cafe"); e.covers(now) || e.Reason != ReasonNever {
		t.Errorf("after an edit, my-cafe = %+v, want never", e)
	}
	if e, _ := s.Entitled(ctx, "stranger"); !e.covers(now) || e.Remote {
		t.Errorf("after an edit, a venue not named = %+v, want carried for guests", e)
	}

	write("this is not a line of the file at all\n")
	if e, _ := s.Entitled(ctx, "stranger"); !e.covers(now) {
		t.Errorf("after an edit that cannot be read, a venue not named = %+v, want what was last read", e)
	}
	if !strings.Contains(logs.String(), "what was last read stands") {
		t.Error("an edit that cannot be read was not logged")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.Entitled(ctx, "stranger"); !e.covers(now) {
		t.Errorf("with the file gone, a venue not named = %+v, want what was last read", e)
	}
}

// TestEntitlementAsTold: what the venue is told leaves out a date that is
// not one, and says of the back office what the relay will in fact do.
func TestEntitlementAsTold(t *testing.T) {
	paid := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	got := Entitlement{Until: paid.AddDate(0, 0, 7), PaidUntil: paid, Remote: true, State: "grace", Reason: "payment_failed"}.told(false)
	if got.Remote || got.Until == nil || !got.Until.Equal(paid.AddDate(0, 0, 7)) || got.PaidUntil == nil || got.State != "grace" || got.Reason != "payment_failed" {
		t.Errorf("told %+v", got)
	}
	if got := (Entitlement{Until: NoEnd, Remote: true}).told(true); got.Until != nil || got.PaidUntil != nil || !got.Remote {
		t.Errorf("no end told as %+v", got)
	}
	if got := (Entitlement{}).told(false); got.Until != nil {
		t.Errorf("never told as %+v", got)
	}
	raw, err := json.Marshal(Entitlement{Until: NoEnd, Remote: true}.told(true))
	if err != nil || string(raw) != `{"remote":true}` {
		t.Errorf("on the wire: %s, %v", raw, err)
	}
}
