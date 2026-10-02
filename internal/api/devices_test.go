package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// pairingServer is the API over a real database with one venue, and the
// Devices behind it for making codes.
func pairingServer(t *testing.T) (http.Handler, *auth.Devices) {
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
	at := time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_AAAAAAAA", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: at, UpdatedAt: at,
		})
		return err
	})
	if err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}
	devs, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatalf("NewDevices: %v", err)
	}
	return newTestServer(t, testConfig(), WithDevices(devs)).Handler(), devs
}

func newCode(t *testing.T, devs *auth.Devices) string {
	t.Helper()
	code, err := devs.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Counter 1", DeviceKind: auth.KindTerminal})
	if err != nil {
		t.Fatalf("CreatePairingCode: %v", err)
	}
	return code.Code
}

func send(h http.Handler, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func bearerOf(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// errorOf decodes the standard error envelope.
func errorOf(t *testing.T, rec *httptest.ResponseRecorder) Error {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the error envelope: %v: %s", err, rec.Body)
	}
	return body.Error
}

func pairOK(t *testing.T, h http.Handler, code string) pairResponse {
	t.Helper()
	rec := send(h, http.MethodPost, "/api/v1/devices/pair", `{"code":"`+code+`"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("pair: status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a response carrying a token may be cached")
	}
	var res pairResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("pair response: %v", err)
	}
	return res
}

func TestPairThenUseTheToken(t *testing.T) {
	h, devs := pairingServer(t)
	paired := pairOK(t, h, strings.ToLower(newCode(t, devs)))
	if paired.Device.Name != "Counter 1" || paired.Device.Kind != "terminal" || !strings.HasPrefix(paired.Device.ID, "dev_") {
		t.Errorf("device = %+v", paired.Device)
	}

	for _, scheme := range []string{"Bearer ", "bearer ", "BEARER  "} {
		rec := send(h, http.MethodGet, "/api/v1/devices/me", "", map[string]string{"Authorization": scheme + paired.Token})
		if rec.Code != http.StatusOK {
			t.Fatalf("me with %q: status %d, body %s", scheme, rec.Code, rec.Body)
		}
		var me deviceResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil || me != paired.Device {
			t.Errorf("me = %+v, %v; want %+v", me, err, paired.Device)
		}
	}
}

func TestPairingErrors(t *testing.T) {
	h, devs := pairingServer(t)
	used := newCode(t, devs)
	pairOK(t, h, used)

	tests := []struct {
		name  string
		body  string
		code  ErrorCode
		field string
	}{
		{"a used code", `{"code":"` + used + `"}`, CodeInvalidPairingCode, "code"},
		{"an unknown code", `{"code":"ZZZZ-ZZZZ"}`, CodeInvalidPairingCode, "code"},
		{"no code", `{}`, CodeInvalidPairingCode, "code"},
		{"not JSON", `code=7K3M`, CodeInvalidRequest, ""},
		{"an unknown field", `{"code":"ZZZZ-ZZZZ","kind":"back_office"}`, CodeInvalidRequest, ""},
		{"two objects", `{"code":"A"}{"code":"B"}`, CodeInvalidRequest, ""},
		{"a body too large", `{"code":"` + strings.Repeat("A", maxBodyBytes) + `"}`, CodeInvalidRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(h, http.MethodPost, "/api/v1/devices/pair", tt.body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body)
			}
			if e := errorOf(t, rec); e.Code != tt.code || e.Field != tt.field {
				t.Errorf("error = %+v, want code %q field %q", e, tt.code, tt.field)
			}
		})
	}
}

func TestTheDeviceMiddleware(t *testing.T) {
	h, devs := pairingServer(t)
	paired := pairOK(t, h, newCode(t, devs))
	other := pairOK(t, h, newCode(t, devs))
	revoke(t, devs, other)

	tests := []struct {
		name   string
		header map[string]string
		code   ErrorCode
	}{
		{"no header", nil, CodeUnauthenticated},
		{"another scheme", map[string]string{"Authorization": "Basic " + paired.Token}, CodeUnauthenticated},
		{"a scheme and nothing", map[string]string{"Authorization": "Bearer "}, CodeUnauthenticated},
		{"the token alone", map[string]string{"Authorization": paired.Token}, CodeUnauthenticated},
		{"a token of nothing", bearerOf("nfd_" + strings.Repeat("A", 43)), CodeUnauthenticated},
		{"a revoked device", bearerOf(other.Token), CodeDeviceRevoked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(h, http.MethodGet, "/api/v1/devices/me", "", tt.header)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("a 401 without WWW-Authenticate")
			}
			if e := errorOf(t, rec); e.Code != tt.code {
				t.Errorf("code %q, want %q", e.Code, tt.code)
			}
		})
	}

	if rec := send(h, http.MethodGet, "/api/v1/health", "", nil); rec.Code != http.StatusOK {
		t.Errorf("health needs a token now: %d", rec.Code)
	}
	if rec := send(h, http.MethodPost, "/api/v1/devices/me", "", bearerOf(paired.Token)); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST me: %d, want 405", rec.Code)
	}
	if rec := send(h, http.MethodGet, "/api/v1/devices/pair", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET pair: %d, want 405", rec.Code)
	}
}

// revoke unpairs a device, using Authenticate to find it from its token.
func revoke(t *testing.T, devs *auth.Devices, p pairResponse) {
	t.Helper()
	ctx := context.Background()
	dev, err := devs.Authenticate(ctx, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := devs.Revoke(ctx, dev.VenueID, dev.ID); err != nil {
		t.Fatal(err)
	}
}

// brokenDevices fails the way a database that has gone away does.
type brokenDevices struct{}

var errDiskGone = errors.New("disk I/O error")

func (brokenDevices) Authenticate(context.Context, string) (auth.Device, error) {
	return auth.Device{}, errDiskGone
}
func (brokenDevices) Redeem(context.Context, string) (auth.Paired, error) {
	return auth.Paired{}, errDiskGone
}
func (brokenDevices) CreatePairingCode(context.Context, auth.NewCode) (auth.PairingCode, error) {
	return auth.PairingCode{}, errDiskGone
}

func TestAuthFailuresThatAreNotTheClientsFault(t *testing.T) {
	h := newTestServer(t, testConfig(), WithDevices(brokenDevices{})).Handler()
	for _, rec := range []*httptest.ResponseRecorder{
		send(h, http.MethodGet, "/api/v1/devices/me", "", bearerOf("nfd_x")),
		send(h, http.MethodPost, "/api/v1/devices/pair", `{"code":"7K3M9QX2"}`, nil),
	} {
		if rec.Code != http.StatusInternalServerError || errorOf(t, rec).Code != CodeInternal {
			t.Errorf("status %d body %s, want a 500 internal", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "disk") {
			t.Error("the internal error leaked to the client")
		}
	}
}

func TestWithoutDevicesTheRoutesDoNotExist(t *testing.T) {
	h := newTestServer(t, testConfig()).Handler()
	for _, path := range []string{"/api/v1/devices/me", "/api/v1/devices/pair"} {
		if rec := send(h, http.MethodGet, path, "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
}
