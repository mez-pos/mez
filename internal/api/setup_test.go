package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/service/firstrun"
)

// setupBody is what the wizard posts for a cafe in Bengaluru.
const setupBody = `{"name":"Filter Coffee House","currency":"INR","timezone":"Asia/Kolkata","fy_start_month":4,
	"tax_profile":"gst_in","tax_registration":"29ABCDE1234F1Z5","owner_name":"Priya","owner_pin":"246813",
	"device_name":"Counter 1","device_kind":"terminal"}`

// setupServer is the API over an empty database: no venue, no device, no
// staff, which is the state a new install is in. logs is what it logged.
func setupServer(t *testing.T, cfg config.Config) (h http.Handler, d *db.DB, logs *bytes.Buffer) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatalf("NewDevices: %v", err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatalf("NewStaffAuth: %v", err)
	}
	first, err := firstrun.New(d, devices, nil)
	if err != nil {
		t.Fatalf("firstrun.New: %v", err)
	}
	logs = &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := New(cfg, log, BuildInfo{Version: "1.2.3"}, WithDevices(devices), WithStaff(staff), WithFirstRun(first))
	return srv.Handler(), d, logs
}

// caller is where a request comes from and what its browser says of it.
type caller struct {
	remote string // RemoteAddr
	host   string // the Host asked for
	header map[string]string
}

// atTheTill is the kiosk browser the installer's shortcut opens.
func atTheTill() caller {
	return caller{remote: "127.0.0.1:51000", host: "localhost:7777"}
}

func (c caller) with(key, value string) caller {
	header := map[string]string{key: value}
	for k, v := range c.header {
		if k != key {
			header[k] = v
		}
	}
	return caller{remote: c.remote, host: c.host, header: header}
}

// from sends a request as c would. A body goes as JSON unless c says otherwise.
func from(h http.Handler, c caller, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr, req.Host = c.remote, c.host
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func venuesIn(t *testing.T, d *db.DB) int {
	t.Helper()
	venues, err := d.Queries().ListVenues(context.Background())
	if err != nil {
		t.Fatalf("ListVenues: %v", err)
	}
	return len(venues)
}

func statusOf(t *testing.T, rec *httptest.ResponseRecorder) setupStatusResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/setup: status %d, body %s", rec.Code, rec.Body)
	}
	var out setupStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("setup status: %v", err)
	}
	return out
}

// TestSetupOverHTTPMakesAVenueThatCanSell is the wizard's whole errand, by
// the API alone: ask, post the answers, pair with the code that comes back,
// sign in as the owner with the PIN that was typed.
func TestSetupOverHTTPMakesAVenueThatCanSell(t *testing.T) {
	h, d, logs := setupServer(t, testConfig())
	till := atTheTill()

	if st := statusOf(t, from(h, till, http.MethodGet, "/api/v1/setup", "")); !st.Needed || !st.Here {
		t.Fatalf("status on a new install at the till = %+v, want needed and here", st)
	}

	rec := from(h, till, http.MethodPost, "/api/v1/setup", setupBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a response carrying a pairing code may be cached")
	}
	var made setupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &made); err != nil {
		t.Fatalf("setup response: %v", err)
	}
	if !strings.HasPrefix(made.VenueID, "ven_") || !strings.HasPrefix(made.OwnerID, "stf_") || made.ExpiresAt.IsZero() {
		t.Errorf("setup response = %+v", made)
	}
	// The PIN goes in and never comes out: not in the answer, not in the log.
	if strings.Contains(rec.Body.String(), "246813") {
		t.Errorf("the response carries the PIN: %s", rec.Body)
	}
	if strings.Contains(logs.String(), "246813") || strings.Contains(logs.String(), made.PairingCode) {
		t.Errorf("the log carries the PIN or the pairing code:\n%s", logs)
	}

	venue, err := d.Queries().VenueByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("VenueByID: %v", err)
	}
	if venue.PublicID != made.VenueID || venue.Name != "Filter Coffee House" || venue.TaxProfile != "gst_in" {
		t.Errorf("venue = %+v", venue)
	}
	// The wizard's venue starts with the station and course every item
	// needs, as `mez setup`'s does (ADR-400).
	m, err := d.Menu(context.Background(), venue.ID)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(m.Stations) != 1 || m.Stations[0].Name != "Kitchen" || len(m.Courses) != 1 || m.Courses[0].Name != "Main" {
		t.Errorf("a new venue's stations %+v and courses %+v, want a Kitchen and Main", m.Stations, m.Courses)
	}

	// From here on it is the API any till uses, from anywhere on the LAN.
	paired := pairOK(t, h, made.PairingCode)
	if paired.Device.Name != "Counter 1" || paired.Device.Kind != auth.KindTerminal {
		t.Errorf("paired = %+v", paired.Device)
	}
	in := send(h, http.MethodPost, "/api/v1/sessions",
		`{"staff_id":"`+made.OwnerID+`","pin":"246813"}`, bearerOf(paired.Token))
	if in.Code != http.StatusCreated && in.Code != http.StatusOK {
		t.Fatalf("the owner cannot sign in: status %d, body %s", in.Code, in.Body)
	}
	var session sessionResponse
	if err := json.Unmarshal(in.Body.Bytes(), &session); err != nil {
		t.Fatalf("session: %v", err)
	}
	if session.Staff.Name != "Priya" || len(session.Permissions) != len(auth.Permissions()) {
		t.Errorf("signed in as %+v with %d permissions, want Priya with all %d",
			session.Staff, len(session.Permissions), len(auth.Permissions()))
	}

	if st := statusOf(t, from(h, till, http.MethodGet, "/api/v1/setup", "")); st.Needed {
		t.Errorf("status after setup = %+v, want not needed", st)
	}
}

// TestSetupIsOnlyFromThisMachine: every way of not being a browser at the
// server's own computer is refused, and none of them makes a venue.
func TestSetupIsOnlyFromThisMachine(t *testing.T) {
	dev := testConfig()
	dev.DevOrigin = "http://localhost:5173"

	tests := []struct {
		name string
		cfg  config.Config
		who  caller
		here bool
	}{
		{"the kiosk browser, by localhost", testConfig(), atTheTill(), true},
		{"by 127.0.0.1", testConfig(), caller{remote: "127.0.0.1:51000", host: "127.0.0.1:7777"}, true},
		{"by the IPv6 loopback", testConfig(), caller{remote: "[::1]:51000", host: "[::1]:7777"}, true},
		{"by localhost with no port", testConfig(), caller{remote: "127.0.0.1:51000", host: "LOCALHOST"}, true},
		{"its own page saying so", testConfig(), atTheTill().with("Origin", "http://localhost:7777"), true},
		{"its own page, as a browser marks it", testConfig(), atTheTill().with("Origin", "http://localhost:7777").with("Sec-Fetch-Site", "same-origin"), true},
		{"the address typed into the bar", testConfig(), atTheTill().with("Sec-Fetch-Site", "none"), true},
		{"the dev server's page, cross-site as it is", dev, atTheTill().with("Origin", "http://localhost:5173").with("Sec-Fetch-Site", "same-site"), true},
		{"the dev server's page, where one is configured", dev, atTheTill().with("Origin", "http://localhost:5173"), true},

		{"a tablet on the LAN", testConfig(), caller{remote: "192.168.1.23:40112", host: "192.168.1.10:7777"}, false},
		{"a tablet on the LAN asking for localhost", testConfig(), caller{remote: "192.168.1.23:40112", host: "localhost:7777"}, false},
		{"this machine by its LAN address", testConfig(), caller{remote: "192.168.1.10:51000", host: "192.168.1.10:7777"}, false},
		{"a site whose name points at 127.0.0.1", testConfig(), caller{remote: "127.0.0.1:51000", host: "evil.example:7777"}, false},
		{"another site's page posting blind", testConfig(), atTheTill().with("Origin", "https://evil.example"), false},
		{"another port's page on this machine", testConfig(), atTheTill().with("Origin", "http://localhost:8080"), false},
		{"a sandboxed page", testConfig(), atTheTill().with("Origin", "null"), false},
		{"the dev server's page, where none is configured", testConfig(), atTheTill().with("Origin", "http://localhost:5173"), false},
		{"through a proxy on this machine", testConfig(), atTheTill().with("X-Forwarded-For", "203.0.113.9"), false},
		{"through a proxy that says only Forwarded", testConfig(), atTheTill().with("Forwarded", "for=203.0.113.9"), false},
		{"through a proxy that says only X-Real-Ip", testConfig(), atTheTill().with("X-Real-Ip", "203.0.113.9"), false},
		{"through a proxy that says only X-Forwarded-Proto", testConfig(), atTheTill().with("X-Forwarded-Proto", "https"), false},
		{"through a proxy that says only Via", testConfig(), atTheTill().with("Via", "1.1 proxy"), false},
		{"through a tunnel that names its caller", testConfig(), atTheTill().with("CF-Connecting-IP", "203.0.113.9"), false},
		{"another site's page, by what the browser says of it", testConfig(), atTheTill().with("Sec-Fetch-Site", "cross-site"), false},
		{"another port's page, by what the browser says of it", testConfig(), atTheTill().with("Sec-Fetch-Site", "same-site"), false},
		{"a guest through the relay", testConfig(), caller{remote: "0.0.0.0:0", host: "relay.example"}, false},
		{"an address that is not one", testConfig(), caller{remote: "pipe", host: "localhost:7777"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, d, _ := setupServer(t, tt.cfg)

			// Anyone may ask; only the till is told it is the place.
			if st := statusOf(t, from(h, tt.who, http.MethodGet, "/api/v1/setup", "")); !st.Needed || st.Here != tt.here {
				t.Errorf("status = %+v, want needed and here=%v", st, tt.here)
			}

			rec := from(h, tt.who, http.MethodPost, "/api/v1/setup", setupBody)
			if tt.here {
				if rec.Code != http.StatusCreated || venuesIn(t, d) != 1 {
					t.Fatalf("status %d with %d venues, want 201 and 1: %s", rec.Code, venuesIn(t, d), rec.Body)
				}
				return
			}
			if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeSetupNotHere {
				t.Errorf("status %d, body %s, want 403 %s", rec.Code, rec.Body, CodeSetupNotHere)
			}
			if n := venuesIn(t, d); n != 0 {
				t.Errorf("%d venues after a refused setup", n)
			}
		})
	}
}

func TestSetupRefusals(t *testing.T) {
	// edit swaps one piece of the body for another.
	edit := func(old, new string) string {
		if !strings.Contains(setupBody, old) {
			panic("setupBody has no " + old)
		}
		return strings.Replace(setupBody, old, new, 1)
	}
	tests := []struct {
		name   string
		who    caller
		body   string
		status int
		code   ErrorCode
		field  string
	}{
		{"a form's text/plain", atTheTill().with("Content-Type", "text/plain"), setupBody, http.StatusUnsupportedMediaType, CodeInvalidRequest, ""},
		{"a form's urlencoding", atTheTill().with("Content-Type", "application/x-www-form-urlencoded"), setupBody, http.StatusUnsupportedMediaType, CodeInvalidRequest, ""},
		{"not JSON", atTheTill(), `name=Cafe`, http.StatusBadRequest, CodeInvalidRequest, ""},
		{"a field the server does not take", atTheTill(), edit(`"name"`, `"demo_menu":true,"name"`), http.StatusBadRequest, CodeInvalidRequest, ""},
		{"no name", atTheTill(), edit(`"Filter Coffee House"`, `"  "`), http.StatusUnprocessableEntity, CodeInvalidSetup, "name"},
		{"a timezone that does not exist", atTheTill(), edit(`Asia/Kolkata`, `Mars/Olympus`), http.StatusUnprocessableEntity, CodeInvalidSetup, "timezone"},
		{"a financial year starting in month 13", atTheTill(), edit(`"fy_start_month":4`, `"fy_start_month":13`), http.StatusUnprocessableEntity, CodeInvalidSetup, "fy_start_month"},
		{"GST with a short GSTIN", atTheTill(), edit(`29ABCDE1234F1Z5`, `29ABC`), http.StatusUnprocessableEntity, CodeInvalidSetup, "tax_registration"},
		{"an owner PIN of four digits", atTheTill(), edit(`246813`, `2468`), http.StatusUnprocessableEntity, CodeInvalidSetup, "owner_pin"},
		{"a first device that is a guest", atTheTill(), edit(`"terminal"`, `"guest"`), http.StatusUnprocessableEntity, CodeInvalidSetup, "device_kind"},
		{"a first device with no name", atTheTill(), edit(`"Counter 1"`, `""`), http.StatusUnprocessableEntity, CodeInvalidSetup, "device_name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, d, logs := setupServer(t, testConfig())
			rec := from(h, tt.who, http.MethodPost, "/api/v1/setup", tt.body)
			if rec.Code != tt.status {
				t.Fatalf("status %d, body %s, want %d", rec.Code, rec.Body, tt.status)
			}
			if got := errorOf(t, rec); got.Code != tt.code || got.Field != tt.field || got.Message == "" {
				t.Errorf("error = %+v, want %s on %q", got, tt.code, tt.field)
			}
			if n := venuesIn(t, d); n != 0 {
				t.Errorf("%d venues after a refused setup", n)
			}
			if strings.Contains(logs.String(), "246813") {
				t.Errorf("the log carries the PIN:\n%s", logs)
			}
		})
	}
}

func TestSetupRefusesASecondVenue(t *testing.T) {
	h, d, _ := setupServer(t, testConfig())
	if rec := from(h, atTheTill(), http.MethodPost, "/api/v1/setup", setupBody); rec.Code != http.StatusCreated {
		t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	rec := from(h, atTheTill(), http.MethodPost, "/api/v1/setup", setupBody)
	if rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeAlreadySetUp {
		t.Errorf("second setup: status %d, body %s, want 409 %s", rec.Code, rec.Body, CodeAlreadySetUp)
	}
	if strings.Contains(rec.Body.String(), "pairing_code") {
		t.Errorf("a refused setup handed out a code: %s", rec.Body)
	}
	if n := venuesIn(t, d); n != 1 {
		t.Errorf("%d venues, want 1", n)
	}
}

// TestSetupIsRateLimited: each attempt hashes a PIN, so a caller gets a
// form's worth of them a minute.
func TestSetupIsRateLimited(t *testing.T) {
	h, _, _ := setupServer(t, testConfig())
	bad := strings.Replace(setupBody, `"Filter Coffee House"`, `""`, 1)
	for i := range setupLimit.requests {
		if rec := from(h, atTheTill(), http.MethodPost, "/api/v1/setup", bad); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d: status %d, body %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := from(h, atTheTill(), http.MethodPost, "/api/v1/setup", setupBody)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("attempt %d: status %d, Retry-After %q, want 429 with one",
			setupLimit.requests+1, rec.Code, rec.Header().Get("Retry-After"))
	}
	// Asking whether setup is needed is not an attempt at it.
	if rec := from(h, atTheTill(), http.MethodGet, "/api/v1/setup", ""); rec.Code != http.StatusOK {
		t.Errorf("status after the limit: %d", rec.Code)
	}
}

func TestSetupRoutes(t *testing.T) {
	t.Run("another method", func(t *testing.T) {
		h, _, _ := setupServer(t, testConfig())
		rec := from(h, atTheTill(), http.MethodDelete, "/api/v1/setup", "")
		if rec.Code != http.StatusMethodNotAllowed || errorOf(t, rec).Code != CodeMethodNotAllowed {
			t.Errorf("DELETE: status %d, body %s", rec.Code, rec.Body)
		}
	})
	t.Run("a server built without first run has no such endpoint", func(t *testing.T) {
		h := newTestServer(t, testConfig()).Handler()
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if rec := from(h, atTheTill(), method, "/api/v1/setup", setupBody); rec.Code != http.StatusNotFound {
				t.Errorf("%s: status %d, want 404", method, rec.Code)
			}
		}
	})
}

// brokenFirstRun is first run that fails as told.
type brokenFirstRun struct {
	needed error
	run    error
}

func (b brokenFirstRun) Needed(context.Context) (bool, error) { return true, b.needed }

func (b brokenFirstRun) Run(context.Context, firstrun.Answers, ...firstrun.Extra) (firstrun.Result, error) {
	return firstrun.Result{VenueID: 1, VenuePublicID: "ven_AAAAAAAA"}, b.run
}

func TestSetupFailures(t *testing.T) {
	disk := errors.New("disk I/O error at C:\\ProgramData\\mez")
	tests := []struct {
		name   string
		first  brokenFirstRun
		method string
		status int
		code   ErrorCode
	}{
		{"the status cannot be read", brokenFirstRun{needed: disk}, http.MethodGet, http.StatusInternalServerError, CodeInternal},
		{"the venue cannot be written", brokenFirstRun{run: disk}, http.MethodPost, http.StatusInternalServerError, CodeInternal},
		{"the venue was made and the code was not", brokenFirstRun{run: errors.Join(firstrun.ErrNoCode, disk)}, http.MethodPost, http.StatusInternalServerError, CodeSetupNoCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestServer(t, testConfig(), WithFirstRun(tt.first)).Handler()
			body := ""
			if tt.method == http.MethodPost {
				body = setupBody
			}
			rec := from(h, atTheTill(), tt.method, "/api/v1/setup", body)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("status %d, body %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			// What went wrong inside is for the log, not the caller.
			if strings.Contains(rec.Body.String(), "ProgramData") {
				t.Errorf("the response says where the data is: %s", rec.Body)
			}
		})
	}
}

func TestLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true}, {"localhost:7777", true}, {"LocalHost:7777", true},
		{"127.0.0.1", true}, {"127.0.0.1:7777", true}, {"127.8.9.10:80", true},
		{"[::1]", true}, {"[::1]:7777", true}, {"::1", true},
		{"", false}, {"localhost.evil.example", false}, {"evil.example", false}, {"evil.example:7777", false},
		{"192.168.1.10:7777", false}, {"0.0.0.0:7777", false}, {"notlocalhost", false}, {"localhost:7777:1", false},
	}
	for _, tt := range tests {
		if got := loopbackHost(tt.host); got != tt.want {
			t.Errorf("loopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}
