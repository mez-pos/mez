package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// till is a signed-in world: a paired terminal, and Asha with PIN 2468.
type till struct {
	srv         *Server
	handler     http.Handler
	deviceToken string
	log         *bytes.Buffer

	db      *db.DB
	devices *auth.Devices
	staff   *auth.StaffAuth
}

// seededAt is when the test world was created.
var seededAt = time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)

func newTill(t *testing.T) *till {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: t.TempDir() + "/" + db.FileName})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	at := seededAt
	pin, err := auth.HashPIN("2468")
	if err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_AAAAAAAA", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: at, UpdatedAt: at,
		})
		if err != nil {
			return err
		}
		// The default roles, so a permission check has something real to
		// read: Asha is a cashier, Priya owns the place.
		roles := map[string]int64{}
		for _, template := range auth.DefaultRoles() {
			set, err := auth.NewPermissionSet(permissionKeys(template.Permissions))
			if err != nil {
				return err
			}
			raw, err := set.JSON()
			if err != nil {
				return err
			}
			role, err := q.CreateRole(ctx, gen.CreateRoleParams{
				VenueID: v.ID, Name: template.Name, Permissions: raw, CreatedAt: at,
			})
			if err != nil {
				return err
			}
			roles[template.Name] = role.ID
		}
		for _, member := range []struct {
			publicID, name, role string
		}{
			{"stf_ASHA0001", "Asha", "cashier"},
			{"stf_PRIYA001", "Priya", "owner"},
		} {
			if _, err := q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: member.publicID, VenueID: v.ID, RoleID: roles[member.role], Name: member.name,
				PinHash: pin, Active: true, CreatedAt: at, UpdatedAt: at,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	devs, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	logged := &bytes.Buffer{}
	w := &till{log: logged, db: d, devices: devs, staff: staff}
	w.rebuild(t)

	code, err := devs.CreatePairingCode(ctx, auth.NewCode{VenueID: 1, DeviceName: "Counter 1", DeviceKind: auth.KindTerminal})
	if err != nil {
		t.Fatal(err)
	}
	w.deviceToken = pairOK(t, w.handler, code.Code).Token
	return w
}

// rebuild swaps in a server with more wired into it. Everything already in
// the database, the paired device included, carries over.
func (w *till) rebuild(t *testing.T, extra ...Option) {
	t.Helper()
	opts := append([]Option{WithDevices(w.devices), WithStaff(w.staff)}, extra...)
	w.srv = New(testConfig(), slog.New(slog.NewJSONHandler(w.log, nil)), BuildInfo{Version: "1.2.3"}, opts...)
	w.handler = w.srv.Handler()
}

// asDevice is the headers of a request from the paired terminal.
func (w *till) asDevice() map[string]string { return bearerOf(w.deviceToken) }

// asStaff adds a staff session token to the device's headers.
func (w *till) asStaff(token string) map[string]string {
	h := w.asDevice()
	h[HeaderStaffToken] = token
	return h
}

// permissionKeys is a template's permissions as the strings a role stores.
func permissionKeys(perms []auth.Permission) []string {
	keys := make([]string, len(perms))
	for i, p := range perms {
		keys[i] = string(p)
	}
	return keys
}

// signIn signs Asha the cashier in and returns the response.
func (w *till) signIn(t *testing.T, pin string) sessionResponse {
	t.Helper()
	return w.signInAs(t, "stf_ASHA0001", pin)
}

func (w *till) signInAs(t *testing.T, staffID, pin string) sessionResponse {
	t.Helper()
	rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"`+staffID+`","pin":"`+pin+`"}`, w.asDevice())
	if rec.Code != http.StatusCreated {
		t.Fatalf("sign in: status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a response carrying a session token may be cached")
	}
	var res sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("sign in response: %v", err)
	}
	return res
}

func TestSignInOverHTTP(t *testing.T) {
	w := newTill(t)

	rec := send(w.handler, http.MethodGet, "/api/v1/staff", "", w.asDevice())
	var list staffListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Staff) != 2 || list.Staff[0].ID != "stf_ASHA0001" {
		t.Fatalf("staff list = %s (%v)", rec.Body, err)
	}
	// The sign-in screen shows the role under the name (rebuild Task 1a).
	if roles := []string{list.Staff[0].Role, list.Staff[1].Role}; roles[0] != "cashier" || roles[1] != "owner" {
		t.Errorf("the list's roles = %v, want cashier and owner", roles)
	}

	in := w.signIn(t, "2468")
	if in.Token == "" || in.Staff.Name != "Asha" || in.Staff.Role != "cashier" || in.ExpiresAt.IsZero() {
		t.Fatalf("signed in = %+v", in)
	}

	rec = send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asStaff(in.Token))
	var current sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("current: %d %s (%v)", rec.Code, rec.Body, err)
	}
	if current.Staff != in.Staff || current.Token != "" {
		t.Errorf("current = %+v; the token must not be echoed back", current)
	}

	if rec := send(w.handler, http.MethodDelete, "/api/v1/sessions/current", "", w.asStaff(in.Token)); rec.Code != http.StatusNoContent {
		t.Fatalf("sign out: %d %s", rec.Code, rec.Body)
	}
	rec = send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asStaff(in.Token))
	if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeSignInRequired {
		t.Errorf("after signing out: %d %s", rec.Code, rec.Body)
	}
}

func TestSignInRefusals(t *testing.T) {
	w := newTill(t)

	t.Run("a wrong PIN", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"1111"}`, w.asDevice())
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeInvalidPIN {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "Asha") {
			t.Error("the refusal says whether that staff member exists")
		}
	})

	t.Run("nobody", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_NOBODY00","pin":"2468"}`, w.asDevice())
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeInvalidPIN {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("not from a paired device", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"2468"}`, nil)
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeUnauthenticated {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("a body that is not the request", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff":"x"}`, w.asDevice())
		if rec.Code != http.StatusBadRequest || errorOf(t, rec).Code != CodeInvalidRequest {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("and then the lockout", func(t *testing.T) {
		for range auth.DefaultStaffLockout.MaxFailures {
			send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"1111"}`, w.asDevice())
		}
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_ASHA0001","pin":"2468"}`, w.asDevice())
		if rec.Code != http.StatusTooManyRequests || errorOf(t, rec).Code != CodeLockedOut {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
		if err != nil || retry < 1 || retry > int(auth.DefaultStaffLockout.Window.Seconds()) {
			t.Errorf("Retry-After = %q (%v)", rec.Header().Get("Retry-After"), err)
		}
	})
}

func TestStaffRoutesNeedBothTokens(t *testing.T) {
	w := newTill(t)
	in := w.signIn(t, "2468")

	tests := []struct {
		name   string
		header map[string]string
		status int
		code   ErrorCode
	}{
		{"no tokens at all", nil, http.StatusUnauthorized, CodeUnauthenticated},
		{"the device alone", w.asDevice(), http.StatusUnauthorized, CodeSignInRequired},
		{"a staff token of nothing", w.asStaff("nfs_" + strings.Repeat("A", 43)), http.StatusUnauthorized, CodeSignInRequired},
		{"a device token as a staff token", w.asStaff(w.deviceToken), http.StatusUnauthorized, CodeSignInRequired},
		{"both", w.asStaff(in.Token), http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", tt.header)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
			if tt.code != "" && errorOf(t, rec).Code != tt.code {
				t.Errorf("code %q, want %q", errorOf(t, rec).Code, tt.code)
			}
		})
	}

	// A staff token is no use on another terminal, even a paired one.
	second := newTill(t)
	if rec := send(second.handler, http.MethodGet, "/api/v1/sessions/current", "", second.asStaff(in.Token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("a session token worked on another terminal: %d", rec.Code)
	}
}

// TestTheAccessLogSaysWhoItWas is the operator's side of authentication.
func TestTheAccessLogSaysWhoItWas(t *testing.T) {
	w := newTill(t)
	in := w.signIn(t, "2468")
	w.log.Reset()
	send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asStaff(in.Token))
	send(w.handler, http.MethodGet, "/api/v1/health", "", nil)

	lines := requestLines(t, w.log)
	if len(lines) != 2 {
		t.Fatalf("%d request lines, want 2", len(lines))
	}
	if lines[0]["venue_id"] != float64(1) || lines[0]["device_id"] != float64(1) || lines[0]["actor_id"] != float64(1) {
		t.Errorf("authenticated request logged as %v", lines[0])
	}
	for _, key := range []string{"venue_id", "device_id", "actor_id"} {
		if _, present := lines[1][key]; present {
			t.Errorf("the health check carries %s", key)
		}
	}
}

// requestLines decodes the access log lines out of a JSON log.
func requestLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if m["msg"] == "request" {
			out = append(out, m)
		}
	}
	return out
}

func TestPairingIsRateLimited(t *testing.T) {
	w := newTill(t)
	body := `{"code":"ZZZZ-ZZZZ"}`

	// Pairing this terminal already spent one of the window's requests.
	for i := range pairLimit.requests - 1 {
		if rec := send(w.handler, http.MethodPost, "/api/v1/devices/pair", body, nil); rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status %d, want the ordinary refusal", i+1, rec.Code)
		}
	}
	rec := send(w.handler, http.MethodPost, "/api/v1/devices/pair", body, nil)
	if rec.Code != http.StatusTooManyRequests || errorOf(t, rec).Code != CodeRateLimited {
		t.Fatalf("after the limit: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}
}

func TestTheLimiter(t *testing.T) {
	at := time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)
	now := at
	l := newLimiter(func() time.Time { return now })
	lim := limit{requests: 2, per: time.Minute}

	for i := range 2 {
		if ok, _ := l.allow("a", lim); !ok {
			t.Fatalf("request %d was refused", i+1)
		}
	}
	ok, wait := l.allow("a", lim)
	if ok || wait != time.Minute {
		t.Errorf("third request: ok %v, wait %v", ok, wait)
	}
	if ok, _ := l.allow("b", lim); !ok {
		t.Error("another address was refused for the first's spending")
	}
	now = at.Add(time.Minute)
	if ok, _ := l.allow("a", lim); !ok {
		t.Error("the window did not turn over")
	}

	// A flood of keys is pruned rather than kept forever.
	now = at.Add(2 * time.Minute)
	for i := range maxLimiterKeys {
		l.allow("flood"+strconv.Itoa(i), lim)
	}
	now = at.Add(10 * time.Minute)
	l.allow("last", lim)
	if len(l.windows) > maxLimiterKeys {
		t.Errorf("%d windows held, want at most %d", len(l.windows), maxLimiterKeys)
	}
	if got := retryAfterSeconds(time.Millisecond); got != 1 {
		t.Errorf("retryAfterSeconds(1ms) = %d, want 1", got)
	}
}

func TestASessionSaysWhatItsRoleMayDo(t *testing.T) {
	w := newTill(t)
	cashier := w.signIn(t, "2468")
	owner := w.signInAs(t, "stf_PRIYA001", "2468")

	tests := []struct {
		name  string
		token string
		want  []auth.Permission
	}{
		{"a cashier", cashier.Token, cashierPermissions(t)},
		{"the owner", owner.Token, auth.Permissions()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asStaff(tt.token))
			var current sessionResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("current: %d %s (%v)", rec.Code, rec.Body, err)
			}
			if !slices.Equal(current.Permissions, tt.want) {
				t.Errorf("permissions = %v, want %v", current.Permissions, tt.want)
			}
		})
	}
	if !slices.Equal(owner.Permissions, auth.Permissions()) {
		t.Errorf("sign-in permissions = %v, want the owner's", owner.Permissions)
	}

	// Any paired device can list staff; what each may do stays off it.
	rec := send(w.handler, http.MethodGet, "/api/v1/staff", "", w.asDevice())
	if strings.Contains(rec.Body.String(), "permissions") {
		t.Errorf("the staff list names permissions: %s", rec.Body)
	}
}

// cashierPermissions is the default cashier role, in canonical order.
func cashierPermissions(t *testing.T) []auth.Permission {
	t.Helper()
	for _, template := range auth.DefaultRoles() {
		if template.Name == "cashier" {
			set, err := auth.NewPermissionSet(permissionKeys(template.Permissions))
			if err != nil {
				t.Fatal(err)
			}
			return set.Keys()
		}
	}
	t.Fatal("no default cashier role")
	return nil
}

// brokenStaff fails the way a database that has gone away does.
type brokenStaff struct{}

func (brokenStaff) ActiveStaff(context.Context, int64) ([]auth.Staff, error) {
	return nil, errDiskGone
}
func (brokenStaff) WithPermission(context.Context, int64, auth.Permission) ([]auth.Staff, error) {
	return nil, errDiskGone
}
func (brokenStaff) SignIn(context.Context, auth.Device, string, string) (auth.SignedIn, error) {
	return auth.SignedIn{}, errDiskGone
}
func (brokenStaff) Session(context.Context, auth.Device, string) (auth.Session, error) {
	return auth.Session{}, errDiskGone
}
func (brokenStaff) SignOut(context.Context, auth.Session) error { return errDiskGone }
func (brokenStaff) Permissions(context.Context, int64) (auth.PermissionSet, error) {
	return nil, errDiskGone
}
func (brokenStaff) Authorise(context.Context, auth.Device, string, string, auth.Permission) (auth.Staff, error) {
	return auth.Staff{}, errDiskGone
}

// workingDevices admits any token, so the staff layer behind it can be
// exercised on its own.
type workingDevices struct{}

func (workingDevices) Authenticate(context.Context, string) (auth.Device, error) {
	return auth.Device{ID: 1, VenueID: 1, PublicID: "dev_TEST0001", Name: "Counter 1", Kind: auth.KindTerminal}, nil
}
func (workingDevices) Redeem(context.Context, string) (auth.Paired, error) {
	return auth.Paired{}, errors.New("not used")
}
func (workingDevices) CreatePairingCode(context.Context, auth.NewCode) (auth.PairingCode, error) {
	return auth.PairingCode{}, errors.New("not used")
}
func (workingDevices) Roster(context.Context, int64) (auth.Roster, error) {
	return auth.Roster{}, errors.New("not used")
}
func (workingDevices) Rename(context.Context, int64, string, string, int64) error {
	return errors.New("not used")
}
func (workingDevices) RevokeByPublicID(context.Context, int64, string, int64, int64) error {
	return errors.New("not used")
}

func TestStaffFailuresThatAreNotTheClientsFault(t *testing.T) {
	h := newTestServer(t, testConfig(), WithDevices(workingDevices{}), WithStaff(brokenStaff{})).Handler()
	header := bearerOf("nfd_anything")
	staffHeader := map[string]string{"Authorization": "Bearer nfd_anything", HeaderStaffToken: "nfs_anything"}

	for _, rec := range []*httptest.ResponseRecorder{
		send(h, http.MethodGet, "/api/v1/staff", "", header),
		send(h, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_X","pin":"1234"}`, header),
		send(h, http.MethodGet, "/api/v1/sessions/current", "", staffHeader),
	} {
		if rec.Code != http.StatusInternalServerError || errorOf(t, rec).Code != CodeInternal {
			t.Errorf("status %d body %s, want a 500 internal", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "disk") {
			t.Error("the internal error leaked to the client")
		}
	}
}

func TestWithoutStaffTheRoutesDoNotExist(t *testing.T) {
	h := newTestServer(t, testConfig(), WithDevices(workingDevices{})).Handler()
	for _, path := range []string{"/api/v1/staff", "/api/v1/sessions", "/api/v1/sessions/current"} {
		if rec := send(h, http.MethodGet, path, "", bearerOf("nfd_anything")); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
}
