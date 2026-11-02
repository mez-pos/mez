package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/web"
)

// viaRelay marks a request's headers the way the tunnel does.
func viaRelay(h map[string]string) map[string]string {
	out := map[string]string{HeaderVia: ViaRelay}
	for k, v := range h {
		out[k] = v
	}
	return out
}

// registeredRoutes reads every "METHOD /api/v1/..." pattern written in this
// package's own source. ServeMux does not say what was registered on it, and
// a list kept by hand in a test is the list that goes stale; the source is
// what a new route cannot avoid being in.
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	literal := regexp.MustCompile(`"((?:GET|POST|PUT|PATCH|DELETE) /api/v1/[^"]*)"`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, e := range entries {
		name := e.Name()
		// remote.go is the list under test, not a place routes are made.
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "remote.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range literal.FindAllSubmatch(src, -1) {
			seen[string(m[1])] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	if len(out) < 150 {
		t.Fatalf("found %d routes in the source; the scan is broken", len(out))
	}
	return out
}

// staysAtTheVenue is every registered route a relayed request is refused,
// written out. It is the other half of remote.go: a route added to the API
// is in neither list until someone puts it in one, and the test below fails
// until they do, so "can this be done from outside?" is asked of every route
// and answered by a person (ADR-308).
var staysAtTheVenue = []string{
	// A tablet asking the PC at a new address to prove it holds its pairing
	// (ADR-417): a question asked on the venue's network, and bound to it.
	"GET /api/v1/devices/{id}/proof",
	// The tablet app's version (ADR-421): a tablet installs on the venue's
	// network, and its package, which is not under /api, is refused too.
	"GET /api/v1/app",
	// Letting a tablet out of mez (ADR-419): a manager standing at it.
	"GET /api/v1/devices/me/way-out",
	"POST /api/v1/devices/me/way-out",
	// Orders, as anything but something to read.
	"POST /api/v1/orders",
	"GET /api/v1/orders/closed",
	"GET /api/v1/orders/today",
	// Money.
	"POST /api/v1/charges/{id}/resolve",
	"POST /api/v1/paid-notices/{id}/seen",
	"PUT /api/v1/settings/payments",
	"PUT /api/v1/settings/numbering",
	"PUT /api/v1/settings/backup",
	// Pairing to an account: the code is typed at the venue, as a device's
	// is made there. From away it would be a way to move a venue to
	// another account with a manager's PIN and nothing else.
	"POST /api/v1/cloud/pair",
	"POST /api/v1/settings/backup/now",
	// The shift and its drawer.
	"GET /api/v1/shift",
	"GET /api/v1/shift/mine",
	"POST /api/v1/shift/open",
	"POST /api/v1/shift/{id}/cash",
	"POST /api/v1/shift/{id}/close",
	"POST /api/v1/shift/{id}/count",
	"POST /api/v1/shift/{id}/reason",
	"GET /api/v1/shift/{id}/stock-count",
	"POST /api/v1/shift/{id}/stock-count",
	"POST /api/v1/shift/{id}/tips",
	"POST /api/v1/shift/{id}/tips/preview",
	"GET /api/v1/shift/{id}/z-report",
	"POST /api/v1/shift/{id}/z-report/corrections",
	"POST /api/v1/z-reports/{number}/print",
	// The room: tables in service, guests' requests, the kitchen.
	"POST /api/v1/floor/tables/{id}/clear",
	"POST /api/v1/floor/tables/{id}/release",
	"GET /api/v1/guest-requests",
	"POST /api/v1/guest-requests/{id}/accept",
	"POST /api/v1/guest-requests/{id}/decline",
	"GET /api/v1/kitchen/{station}/tickets",
	"POST /api/v1/kitchen/orders/{id}/tickets/{seq}/bump",
	"POST /api/v1/kitchen/orders/{id}/tickets/{seq}/recall",
	"POST /api/v1/kitchen/orders/{id}/tickets/{seq}/start",
	// Printers and what is waiting for them.
	"GET /api/v1/print-jobs",
	"POST /api/v1/print-jobs/{id}/cancel",
	"POST /api/v1/print-jobs/{id}/retry",
	"POST /api/v1/printers",
	"GET /api/v1/printers/discover",
	"POST /api/v1/printers/discover",
	"PUT /api/v1/printers/receipt",
	"PUT /api/v1/printers/{id}",
	"POST /api/v1/printers/{id}/restore",
	"POST /api/v1/printers/{id}/retire",
	"POST /api/v1/printers/{id}/test",
	"PUT /api/v1/stations/{code}/printer",
	// Devices: a code is made, and a device named, by someone in the room.
	"POST /api/v1/devices/codes",
	"POST /api/v1/devices/{id}/rename",
	"GET /api/v1/devices/addresses",
	// First run, a locked PIN, and the stream the relay cannot carry.
	"POST /api/v1/setup",
	// And a venue brought back: at the computer it is brought back to.
	"POST /api/v1/setup/recover/start",
	"POST /api/v1/setup/recover/verify",
	"POST /api/v1/setup/recover/restore",
	"POST /api/v1/setup/recover/outcome",
	"POST /api/v1/people/staff/{id}/unlock",
	"GET /api/v1/stream",
}

// TestEveryRouteHasARemoteVerdict: each route in the API is either one a
// relayed request may use or one written in staysAtTheVenue, never both and
// never neither, and neither list names a route that does not exist.
func TestEveryRouteHasARemoteVerdict(t *testing.T) {
	registered := registeredRoutes(t)
	exists := map[string]bool{}
	for _, p := range registered {
		exists[p] = true
	}
	refused := map[string]bool{}
	for _, p := range staysAtTheVenue {
		if refused[p] {
			t.Errorf("%q is in staysAtTheVenue twice", p)
		}
		refused[p] = true
		if !exists[p] {
			t.Errorf("staysAtTheVenue names %q, which is not a route", p)
		}
		if remoteAllows(p) {
			t.Errorf("%q is in staysAtTheVenue and remote.go allows it", p)
		}
	}
	for p := range remoteRoutes {
		if !exists[p] {
			t.Errorf("remote.go allows %q, which is not a route", p)
		}
	}
	for p := range remoteRefused {
		if !exists[p] {
			t.Errorf("remote.go refuses %q, which is not a route", p)
		}
	}
	for _, p := range registered {
		if !remoteAllows(p) && !refused[p] {
			t.Errorf("%q has no verdict: allow it in remote.go, or add it to staysAtTheVenue here", p)
		}
	}
}

// TestRemoteAllows is the rule itself, on the shapes a pattern can have.
func TestRemoteAllows(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"GET /api/v1/menu", true},
		{"PUT /api/v1/menu/items/{id}", true},
		{"POST /api/v1/menu/a-route-added-next-year", true},
		{"GET /api/v1/stock/items/{id}/movements", true},
		{"GET /api/v1/reports/sales/csv", true},
		{"PUT /api/v1/people/staff/{id}/pin", true},
		{"POST /api/v1/people/staff/{id}/unlock", false},
		{"GET /api/v1/menus", false},
		{"GET /api/v1/stockroom", false},
		{"POST /api/v1/orders", false},
		{"POST /api/v1/orders/{id}/payments", false},
		{"POST /api/v1/a-route-added-next-year", false},
		// What the catch-alls are registered as, and what no route is.
		{"/api/v1/menu", false},
		{"/api/", false},
		{"/", false},
		{"", false},
		{"GET", false},
	}
	for _, tt := range tests {
		if got := remoteAllows(tt.pattern); got != tt.want {
			t.Errorf("remoteAllows(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

// TestRemoteGate: the gate by itself, in front of a mux that answers
// everything, so that what is refused was refused by the gate.
func TestRemoteGate(t *testing.T) {
	mux := http.NewServeMux()
	for _, pattern := range []string{
		"GET /api/v1/menu", "POST /api/v1/orders", "GET /api/v1/health", "/api/v1/health", "/api/",
		"/order/", "/backoffice/", "/kitchen", "/",
	} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "served "+r.Pattern)
		})
	}
	// No guest mount set: a venue serving the guest bundle and nothing more
	// is still reached by its guests.
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	gate := s.remoteGate(mux)
	if remoteGuest != web.MountGuest || remoteBundle+"/" != web.MountBackoffice {
		t.Fatalf("the gate lets through %q and %q; the bundles are at %q and %q",
			remoteGuest, remoteBundle, web.MountGuest, web.MountBackoffice)
	}

	tests := []struct {
		method, path string
		remote       bool // served when relayed
	}{
		{"GET", "/api/v1/menu", true},
		{"GET", "/api/v1/health", true},
		{"GET", "/order/t/abc", true},
		{"POST", "/order/api/t/abc/orders", true},
		{"GET", "/backoffice/", true},
		{"GET", "/backoffice/assets/app.js", true},
		{"POST", "/api/v1/orders", false},
		{"POST", "/api/v1/health", false}, // the 405 catch-all
		{"GET", "/api/v1/nothing", false},
		{"GET", "/kitchen", false},
		{"GET", "/", false},
		{"GET", "/pos", false},
		{"GET", "/backofficer", false},
		// These decode to something under a bundle that is let through, and
		// the mux, which routes on the path as sent, gives them to whatever
		// serves "/": the till. They are judged as sent.
		{"GET", "/backoffice%2f", false},
		{"GET", "/backoffice%2f..%2fassets/app.js", false},
		{"GET", "/backoffice%2Fapi/v1/health", false},
		{"GET", "/order%2ft/abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			// From the venue's own network everything is served, always.
			if rec := send(gate, tt.method, tt.path, "", nil); rec.Code != http.StatusOK || rec.Header().Get(HeaderVia) != "" {
				t.Errorf("on the LAN: status %d, %s %q", rec.Code, HeaderVia, rec.Header().Get(HeaderVia))
			}
			rec := send(gate, tt.method, tt.path, "", viaRelay(nil))
			if rec.Header().Get(HeaderVia) != ViaRelay {
				t.Errorf("a relayed answer does not say so: %q", rec.Header().Get(HeaderVia))
			}
			if tt.remote {
				if rec.Code != http.StatusOK {
					t.Errorf("relayed: status %d, body %s", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
				t.Errorf("relayed: status %d, body %s; want 403 %s", rec.Code, rec.Body, CodeRemoteNotAllowed)
			}
		})
	}

	// Anything under the name is the mark: a value the tunnel never writes
	// is not a way to be treated as local.
	for _, value := range []string{"lan", "", "local", "relay, lan"} {
		req, _ := http.NewRequest("POST", "/api/v1/orders", nil)
		req.Header[HeaderVia] = []string{value}
		if !relayed(req) {
			t.Errorf("%s: %q is not read as relayed", HeaderVia, value)
		}
	}
}

// backOffice pairs a back office device on the till's venue and returns the
// headers of a request from it.
func (w *till) backOffice(t *testing.T) map[string]string {
	t.Helper()
	code, err := w.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Owner's phone", DeviceKind: auth.KindBackOffice})
	if err != nil {
		t.Fatal(err)
	}
	return bearerOf(pairOK(t, w.handler, code.Code).Token)
}

func withStaff(device map[string]string, token string) map[string]string {
	h := map[string]string{HeaderStaffToken: token}
	for k, v := range device {
		h[k] = v
	}
	return h
}

// TestARelayedRequestIsHeldToTheOwnersRules: who may do what through the
// relay, against the real server. Each refusal is set beside the same
// request made at the venue, which goes through (ADR-306, ADR-308).
func TestARelayedRequestIsHeldToTheOwnersRules(t *testing.T) {
	r := newRegister(t)
	office := r.backOffice(t)
	till := r.asDevice()
	signIn := func(device map[string]string, staffID, pin string) *sessionResponse {
		t.Helper()
		rec := send(r.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"`+staffID+`","pin":"`+pin+`"}`, device)
		if rec.Code != http.StatusCreated {
			return nil
		}
		var res sessionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("sign in response: %v", err)
		}
		return &res
	}
	refused := func(t *testing.T, what string, method, path, body string, h map[string]string) {
		t.Helper()
		rec := send(r.handler, method, path, body, h)
		if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
			t.Errorf("%s: status %d, body %s; want 403 %s", what, rec.Code, rec.Body, CodeRemoteNotAllowed)
		}
	}
	served := func(t *testing.T, what string, method, path, body string, h map[string]string, want int) {
		t.Helper()
		if rec := send(r.handler, method, path, body, h); rec.Code != want {
			t.Errorf("%s: status %d, body %s; want %d", what, rec.Code, rec.Body, want)
		}
	}

	t.Run("a till's token opens nothing from outside", func(t *testing.T) {
		served(t, "at the venue", "GET", "/api/v1/staff", "", till, http.StatusOK)
		refused(t, "through the relay", "GET", "/api/v1/staff", "", viaRelay(till))
		refused(t, "signing in through the relay", "POST", "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"2468"}`, viaRelay(till))
		served(t, "a back office through the relay", "GET", "/api/v1/staff", "", viaRelay(office), http.StatusOK)
	})

	t.Run("a cashier's right PIN is refused from outside, and leaves no session", func(t *testing.T) {
		if signIn(office, "stf_ASHA0001", "2468") == nil {
			t.Fatal("the cashier could not sign in at the venue")
		}
		rec := send(r.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"2468"}`, viaRelay(office))
		if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
			t.Fatalf("status %d, body %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "token") {
			t.Errorf("the refusal carries a token: %s", rec.Body)
		}
		// A wrong PIN is the answer it always was: nothing about the role
		// is said to someone who does not hold the PIN.
		wrong := send(r.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"0000"}`, viaRelay(office))
		if wrong.Code != http.StatusUnauthorized || errorOf(t, wrong).Code != CodeInvalidPIN {
			t.Errorf("a wrong PIN through the relay: status %d, body %s", wrong.Code, wrong.Body)
		}
	})

	t.Run("a cashier's session opened at the venue does not travel", func(t *testing.T) {
		in := signIn(office, "stf_ASHA0001", "2468")
		if in == nil {
			t.Fatal("sign in at the venue failed")
		}
		h := withStaff(office, in.Token)
		served(t, "at the venue", "GET", "/api/v1/sessions/current", "", h, http.StatusOK)
		refused(t, "through the relay", "GET", "/api/v1/sessions/current", "", viaRelay(h))
	})

	owner := signIn(viaRelay(office), "stf_PRIYA001", "2468")
	if owner == nil {
		t.Fatal("the owner could not sign in through the relay on a back office device")
	}
	asOwner := withStaff(office, owner.Token)

	t.Run("the owner's session opened at the venue does not travel either", func(t *testing.T) {
		in := signIn(office, "stf_PRIYA001", "2468")
		if in == nil {
			t.Fatal("sign in at the venue failed")
		}
		h := withStaff(office, in.Token)
		served(t, "at the venue", "GET", "/api/v1/sessions/current", "", h, http.StatusOK)
		rec := send(r.handler, http.MethodGet, "/api/v1/sessions/current", "", viaRelay(h))
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeSignInRequired {
			t.Errorf("through the relay: status %d, body %s; want 401 %s", rec.Code, rec.Body, CodeSignInRequired)
		}
	})

	// How long it lasts is internal/auth's test: here the two lengths are
	// the same number, and an assertion on it would prove nothing.
	t.Run("a session opened from away works at the venue", func(t *testing.T) {
		served(t, "at the venue", "GET", "/api/v1/sessions/current", "", asOwner, http.StatusOK)
	})

	t.Run("the owner signs in and does the desk's work", func(t *testing.T) {
		served(t, "who is signed in", "GET", "/api/v1/sessions/current", "", viaRelay(asOwner), http.StatusOK)
		served(t, "reading orders", "GET", "/api/v1/orders/today", "", asOwner, http.StatusOK)
	})

	t.Run("the owner cannot work the till from outside", func(t *testing.T) {
		open := `{"type":"takeaway"}`
		// At the venue the request reaches the handler, whatever it then
		// makes of the body; through the relay it does not get that far.
		if rec := send(r.handler, "POST", "/api/v1/orders", open, asOwner); rec.Code == http.StatusForbidden {
			t.Fatalf("at the venue the owner is refused an order: %s", rec.Body)
		}
		refused(t, "opening an order", "POST", "/api/v1/orders", open, viaRelay(asOwner))
		refused(t, "the till's list of orders", "GET", "/api/v1/orders/today", "", viaRelay(asOwner))
		refused(t, "a payment", "POST", "/api/v1/orders/ord_AAAAAAAA/payments", `{}`, viaRelay(asOwner))
		refused(t, "a void", "POST", "/api/v1/orders/ord_AAAAAAAA/lines/1/void", `{}`, viaRelay(asOwner))
		refused(t, "first run", "POST", "/api/v1/setup", `{}`, viaRelay(nil))
		refused(t, "a route that does not exist", "GET", "/api/v1/nothing-here", "", viaRelay(asOwner))
		refused(t, "the terminal", "GET", "/", "", viaRelay(nil))
	})

	t.Run("the log says which requests came from outside", func(t *testing.T) {
		r.log.Reset()
		send(r.handler, "GET", "/api/v1/sessions/current", "", viaRelay(asOwner))
		send(r.handler, "POST", "/api/v1/orders", `{}`, viaRelay(asOwner))
		send(r.handler, "GET", "/api/v1/sessions/current", "", asOwner)
		lines := requestLines(t, r.log)
		if len(lines) != 3 {
			t.Fatalf("%d request lines, want 3", len(lines))
		}
		if lines[0]["via"] != ViaRelay || lines[1]["via"] != ViaRelay {
			t.Errorf("relayed requests logged as %v and %v", lines[0], lines[1])
		}
		if lines[1]["status"] != float64(http.StatusForbidden) {
			t.Errorf("the refused request logged as %v", lines[1])
		}
		if _, present := lines[2]["via"]; present {
			t.Errorf("a request from the venue carries via: %v", lines[2])
		}
	})

	// Last, since it locks the owner out (ADR-314).
	t.Run("the third wrong PIN from away locks, and says nothing of it", func(t *testing.T) {
		for i := range auth.RemoteMaxFailures {
			rec := send(r.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"000000"}`, viaRelay(office))
			if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeInvalidPIN {
				t.Fatalf("wrong PIN %d through the relay: status %d, body %s", i+1, rec.Code, rec.Body)
			}
		}
		rec := send(r.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"2468"}`, viaRelay(office))
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeInvalidPIN {
			t.Errorf("the right PIN through the relay, locked: status %d, body %s", rec.Code, rec.Body)
		}
		if signIn(office, "stf_PRIYA001", "2468") != nil {
			t.Error("locked from away, the right PIN still signs in at the venue")
		}
		// The session the owner had from away went with the lock.
		gone := send(r.handler, http.MethodGet, "/api/v1/sessions/current", "", viaRelay(asOwner))
		if gone.Code != http.StatusUnauthorized || errorOf(t, gone).Code != CodeSignInRequired {
			t.Errorf("the session open before the lock: status %d, body %s", gone.Code, gone.Body)
		}
	})
}

// TestSetupIsNeverHereThroughTheRelay: a relay on the venue's own machine
// reports a loopback caller, and first run trusts loopback. Through the
// relay the answer is that the caller is not here, and the write is refused
// before it is asked (ADR-308).
func TestSetupIsNeverHereThroughTheRelay(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:7777/api/v1/setup", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "127.0.0.1:0"
	s := &Server{cfg: testConfig(), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if !s.atThisMachine(req) {
		t.Fatal("a loopback caller asking for localhost is not here: the test's own premise is wrong")
	}
	req.Header.Set(HeaderVia, ViaRelay)
	if s.atThisMachine(req) {
		t.Error("a relayed request was taken for one made at this machine")
	}
}
