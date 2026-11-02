package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthReportsIdentityAndUptime(t *testing.T) {
	start := time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)
	clock := &stoppedClock{at: start}

	s := newTestServer(t, testConfig(), WithClock(clock.now))
	clock.advance(90 * time.Second)

	resp := get(t, s, "/api/v1/health")

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q, want JSON", got)
	}

	var body healthResponse
	decode(t, resp, &body)

	equal(t, "status", body.Status, "ok")
	equal(t, "version", body.Version, "1.2.3")
	equal(t, "commit", body.Commit, "abc123")
	if !body.StartedAt.Equal(start) {
		t.Errorf("started_at = %v, want %v", body.StartedAt, start)
	}
	if body.UptimeSeconds != 90 {
		t.Errorf("uptime_seconds = %d, want 90", body.UptimeSeconds)
	}
}

// TestHealthReportsTheDatabaseCheck covers the check Phase 02 made real. The
// server is given a function rather than a database so that this package keeps
// knowing nothing about storage, and so that a hung database cannot be
// something this endpoint waits on.
func TestHealthReportsTheDatabaseCheck(t *testing.T) {
	tests := []struct {
		name  string
		check func() string
		want  string
	}{
		{name: "healthy", check: func() string { return CheckOK }, want: CheckOK},
		{name: "failing", check: func() string { return CheckFailing }, want: CheckFailing},
		{name: "not wired up", check: nil, want: CheckUnknown},

		// A check that answers with something outside the vocabulary is a bug
		// in the caller. The endpoint publishes a fixed set of words, and
		// leaking an unknown one would break a dashboard that switches on it.
		{name: "nonsense", check: func() string { return "probably fine" }, want: CheckUnknown},
		{name: "empty", check: func() string { return "" }, want: CheckUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []Option{}
			if tt.check != nil {
				opts = append(opts, WithDBCheck(tt.check))
			}

			resp := get(t, newTestServer(t, testConfig(), opts...), "/api/v1/health")

			var body healthResponse
			decode(t, resp, &body)

			equal(t, "checks.db", body.Checks.DB, tt.want)
		})
	}
}

// TestHealthChecksAreUnknownForNow pins the contract the dashboard is written
// against. Phase 02 makes db real; disk, printer and tunnel wait for their
// phases and must say they do not know rather than claim to be fine.
func TestHealthChecksAreUnknownForNow(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")

	var body healthResponse
	decode(t, resp, &body)

	for _, tt := range []struct {
		name string
		got  string
	}{
		{"db", body.Checks.DB},
		{"disk", body.Checks.Disk},
		{"printer", body.Checks.Printer},
		{"tunnel", body.Checks.Tunnel},
	} {
		if tt.got != CheckUnknown {
			t.Errorf("checks.%s = %q, want %q", tt.name, tt.got, CheckUnknown)
		}
	}
}

// TestHealthFieldsAreSnakeCase: 03-conventions.md fixes the wire format, and
// the frontend types are generated from these structs. Asserting the decoded Go
// struct would pass whatever the tags said, so this reads the raw keys.
func TestHealthFieldsAreSnakeCase(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")

	var raw map[string]json.RawMessage
	decode(t, resp, &raw)

	for _, key := range []string{"status", "version", "started_at", "uptime_seconds", "checks"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response has no %q: keys are %v", key, keysOf(raw))
		}
	}
}

// TestHealthStartsWithStatusOK holds the first bytes of the answer: the
// tablet app knows the PC is mez again by a 200 that starts with them
// (android-app's Probe.MEZ_HEALTH, ADR-413), whatever follows.
func TestHealthStartsWithStatusOK(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")

	if body := resp.Body.String(); !strings.HasPrefix(body, `{"status":"ok"`) {
		t.Errorf("health answers %q, want it to start with {\"status\":\"ok\"", body)
	}
}

// TestHealthOmitsUnstampedBuildInfo keeps commit:"" and build_date:"" off the
// wire until Task 5 stamps them, so a client can tell "not built with it" from
// "built with an empty value".
func TestHealthOmitsUnstampedBuildInfo(t *testing.T) {
	s := New(testConfig(), quietLogger(), BuildInfo{Version: "dev"})
	resp := get(t, s, "/api/v1/health")

	var raw map[string]json.RawMessage
	decode(t, resp, &raw)

	for _, key := range []string{"commit", "build_date"} {
		if _, ok := raw[key]; ok {
			t.Errorf("response carries %q, want it omitted while unstamped", key)
		}
	}
}

// TestHealthDoesNotTouchSubsystems is a design assertion in test form: health is
// asked precisely when something is already hanging, so it must answer from
// memory. The proof here is that it answers with no dependencies wired at all.
func TestHealthAnswersWithNothingWired(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/api/v1/health")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func decode(t *testing.T, resp *httptest.ResponseRecorder, into any) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), into); err != nil {
		t.Fatalf("decoding %q: %v", resp.Body.String(), err)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestHealthSaysWhichVenue: a tablet that finds a mez at a new address asks
// it which venue it serves before carrying its pairing there (ADR-416). The
// venue's network is told; a request through the relay is not.
func TestHealthSaysWhichVenue(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		via  []string // X-Mez-Via, as the tunnel sets it
		want string
	}{
		{name: "on the venue's network", opts: []Option{WithVenue("ven_7K3M9Q")}, want: "ven_7K3M9Q"},
		{name: "through the relay", opts: []Option{WithVenue("ven_7K3M9Q")}, via: []string{ViaRelay}, want: ""},
		// relayed() counts any claim to have come that way, and so does health.
		{name: "saying anything of the relay", opts: []Option{WithVenue("ven_7K3M9Q")}, via: []string{"lan", "relay"}, want: ""},
		{name: "no venue yet", want: ""},
		{name: "an empty name names none", opts: []Option{WithVenue("ven_7K3M9Q"), WithVenue("")}, want: "ven_7K3M9Q"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			for _, v := range tt.via {
				r.Header.Add(HeaderVia, v)
			}
			resp := do(t, newTestServer(t, testConfig(), tt.opts...), r)

			var body map[string]any
			decode(t, resp, &body)
			got, present := body["venue"]
			if tt.want == "" {
				if present {
					t.Errorf("venue = %v, want none", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("venue = %v, want %q", got, tt.want)
			}
		})
	}
}
