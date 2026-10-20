package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/service/cloudsignin"
)

const cloudRoute = "/api/v1/sessions/cloud"

// cloudTill is a till's world paired to mez Cloud: it has a cloud owner, and
// a server that takes the cloud's sign-in. priv is the cloud's key.
type cloudTill struct {
	*till
	priv   ed25519.PrivateKey
	key    ed25519.PublicKey
	paired bool
	owner  auth.Staff
}

func newCloudTill(t *testing.T) *cloudTill {
	t.Helper()
	w := &cloudTill{till: newTill(t), paired: true}
	var err error
	if w.key, w.priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
		t.Fatal(err)
	}
	if w.owner, err = w.staff.PairCloudOwner(context.Background(), 1, "Priya Menon"); err != nil {
		t.Fatal(err)
	}
	svc, err := cloudsignin.New(cloudsignin.Options{
		DB: w.db, Staff: w.staff,
		Key: func() (ed25519.PublicKey, bool) { return w.key, w.paired },
	})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithCloudSignIn(svc))
	return w
}

// assertion is one mez Cloud would sign now for the till's venue.
func (w *cloudTill) assertion(t *testing.T, key ed25519.PrivateKey, venue string, issued time.Time) string {
	t.Helper()
	a, err := cloud.NewAssertion(venue, issued, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := cloud.SignAssertion(key, a)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (w *cloudTill) good(t *testing.T) string {
	t.Helper()
	return w.assertion(t, w.priv, "ven_AAAAAAAA", time.Now())
}

func withAssertion(presented string) map[string]string {
	return viaRelay(map[string]string{"Authorization": cloud.AssertionScheme + " " + presented})
}

// asCloud is the headers of a request from mez Cloud after it has signed
// in: the session's token as the device's and as the session's (ADR-324).
func asCloud(token string) map[string]string {
	return viaRelay(map[string]string{"Authorization": "Bearer " + token, HeaderStaffToken: token})
}

func (w *cloudTill) signInFromCloud(t *testing.T) cloudSessionResponse {
	t.Helper()
	rec := send(w.handler, http.MethodPost, cloudRoute, "", withAssertion(w.good(t)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("cloud sign-in: status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a response carrying a session token may be cached")
	}
	var res cloudSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestTheCloudSignsTheOwnerInOverHTTP(t *testing.T) {
	w := newCloudTill(t)
	in := w.signInFromCloud(t)

	if in.Staff.Name != "Priya Menon" || in.Staff.ID != w.owner.PublicID || in.Staff.Role != "owner" {
		t.Errorf("staff = %+v", in.Staff)
	}
	if in.Device.Name != auth.CloudDeviceName || in.Device.Kind != auth.KindBackOffice {
		t.Errorf("device = %+v", in.Device)
	}
	if len(in.Permissions) != len(auth.Permissions()) {
		t.Errorf("%d permissions, want every one", len(in.Permissions))
	}
	if left := time.Until(in.ExpiresAt); left < auth.RemoteSessionTTL-time.Minute || left > auth.RemoteSessionTTL {
		t.Errorf("the session has %s left, want twelve hours", left)
	}
	if !strings.HasPrefix(in.Token, "nfs_") {
		t.Errorf("the token is %q, want a session's", in.Token)
	}

	// The session is used through the relay like any other from away: who
	// is signed in, and one of the desk's own routes behind a permission.
	rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", asCloud(in.Token))
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions/current: status %d, body %s", rec.Code, rec.Body)
	}
	var current sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.Staff.ID != w.owner.PublicID || current.Token != "" {
		t.Errorf("current = %+v", current)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices/me", "", asCloud(in.Token)); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), auth.CloudDeviceName) {
		t.Errorf("devices/me: status %d, body %s", rec.Code, rec.Body)
	}

	// What stays at the venue stays there for the cloud's owner too.
	rec = send(w.handler, http.MethodPost, "/api/v1/orders", `{}`, asCloud(in.Token))
	if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
		t.Errorf("opening an order from the cloud: status %d, body %s", rec.Code, rec.Body)
	}

	// Signing out ends it.
	if rec := send(w.handler, http.MethodDelete, "/api/v1/sessions/current", "", asCloud(in.Token)); rec.Code != http.StatusNoContent {
		t.Fatalf("sign out: status %d, body %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", asCloud(in.Token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("after signing out: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestCloudSignInRefusalsOverHTTP(t *testing.T) {
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tests := []struct {
		name    string
		setup   func(t *testing.T, w *cloudTill)
		headers func(t *testing.T, w *cloudTill) map[string]string
		status  int
		code    ErrorCode
	}{
		{name: "no Authorization", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			headers: func(*testing.T, *cloudTill) map[string]string { return viaRelay(nil) }},
		{name: "a device's token in its place", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			headers: func(_ *testing.T, w *cloudTill) map[string]string { return viaRelay(w.asDevice()) }},
		{name: "another key's", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			headers: func(t *testing.T, w *cloudTill) map[string]string {
				return withAssertion(w.assertion(t, stranger, "ven_AAAAAAAA", now))
			}},
		{name: "for another venue", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			headers: func(t *testing.T, w *cloudTill) map[string]string {
				return withAssertion(w.assertion(t, w.priv, "ven_ZZZZZZZZ", now))
			}},
		{name: "expired", status: http.StatusUnauthorized, code: CodeCloudClock,
			headers: func(t *testing.T, w *cloudTill) map[string]string {
				return withAssertion(w.assertion(t, w.priv, "ven_AAAAAAAA", now.Add(-10*time.Minute)))
			}},
		{name: "a venue that is not paired", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			setup:   func(_ *testing.T, w *cloudTill) { w.paired = false },
			headers: func(t *testing.T, w *cloudTill) map[string]string { return withAssertion(w.good(t)) }},
		{name: "a cloud device that was revoked", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			setup:   func(t *testing.T, w *cloudTill) { w.revokeCloudDevice(t) },
			headers: func(t *testing.T, w *cloudTill) map[string]string { return withAssertion(w.good(t)) }},
		{name: "an owner moved to a role with nothing of the desk's", status: http.StatusUnauthorized, code: CodeCloudSignInRefused,
			setup: func(t *testing.T, w *cloudTill) {
				if _, err := w.db.Writer().ExecContext(context.Background(),
					`UPDATE staff SET role_id = (SELECT id FROM roles WHERE name = 'cashier') WHERE cloud = 1`); err != nil {
					t.Fatal(err)
				}
			},
			headers: func(t *testing.T, w *cloudTill) map[string]string { return withAssertion(w.good(t)) }},
		// At the venue there is a till, and a PIN.
		{name: "not through the relay", status: http.StatusForbidden, code: CodeCloudSignInRefused,
			headers: func(t *testing.T, w *cloudTill) map[string]string {
				return map[string]string{"Authorization": cloud.AssertionScheme + " " + w.good(t)}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newCloudTill(t)
			if tt.setup != nil {
				tt.setup(t, w)
			}
			rec := send(w.handler, http.MethodPost, cloudRoute, "", tt.headers(t, w))
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("status %d, body %s; want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			if strings.Contains(rec.Body.String(), "nfs_") {
				t.Error("a refusal carries a token")
			}
			var open int
			if err := w.db.Reader().QueryRowContext(context.Background(),
				`SELECT COUNT(*) FROM staff_sessions WHERE ended_at IS NULL`).Scan(&open); err != nil {
				t.Fatal(err)
			}
			if open != 0 {
				t.Errorf("a refused sign-in left %d sessions open", open)
			}
		})
	}
}

func (w *cloudTill) revokeCloudDevice(t *testing.T) {
	t.Helper()
	if _, err := w.db.Writer().ExecContext(context.Background(),
		`UPDATE devices SET revoked_at = '2026-10-12 09:00:00+00:00' WHERE cloud = 1`); err != nil {
		t.Fatal(err)
	}
}

func TestAnAssertionIsGoodOnceOverHTTP(t *testing.T) {
	w := newCloudTill(t)
	presented := w.good(t)
	if rec := send(w.handler, http.MethodPost, cloudRoute, "", withAssertion(presented)); rec.Code != http.StatusCreated {
		t.Fatalf("first: status %d, body %s", rec.Code, rec.Body)
	}
	rec := send(w.handler, http.MethodPost, cloudRoute, "", withAssertion(presented))
	if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeCloudSignInRefused {
		t.Errorf("replayed: status %d, body %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodGet, cloudRoute, "", withAssertion(w.good(t))); rec.Code != http.StatusMethodNotAllowed &&
		rec.Code != http.StatusForbidden {
		t.Errorf("GET: status %d, body %s", rec.Code, rec.Body)
	}
}

// TestOnlyTheCloudsSessionStandsForADevice: a session's token where a
// device's goes opens the cloud's device, for the cloud's session, through
// the relay, and nothing else (ADR-324).
func TestOnlyTheCloudsSessionStandsForADevice(t *testing.T) {
	w := newCloudTill(t)
	cloudIn := w.signInFromCloud(t)

	// A PIN's session, opened from away on a paired back office.
	code, err := w.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Laptop", DeviceKind: auth.KindBackOffice})
	if err != nil {
		t.Fatal(err)
	}
	laptop := pairOK(t, w.handler, code.Code).Token
	rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"2468"}`, viaRelay(bearerOf(laptop)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("a PIN from away: status %d, body %s", rec.Code, rec.Body)
	}
	var pinIn sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &pinIn); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		headers map[string]string
		status  int
	}{
		{"the cloud's session, through the relay", asCloud(cloudIn.Token), http.StatusOK},
		{"the cloud's session, at the venue", map[string]string{"Authorization": "Bearer " + cloudIn.Token, HeaderStaffToken: cloudIn.Token}, http.StatusUnauthorized},
		{"a PIN's session as both", asCloud(pinIn.Token), http.StatusUnauthorized},
		{"the cloud's session as the device, a PIN's as the session", viaRelay(map[string]string{
			"Authorization": "Bearer " + cloudIn.Token, HeaderStaffToken: pinIn.Token}), http.StatusUnauthorized},
		{"a paired device, the cloud's session on it", viaRelay(map[string]string{
			"Authorization": "Bearer " + laptop, HeaderStaffToken: cloudIn.Token}), http.StatusUnauthorized},
		{"the cloud's session as the device, and no session", viaRelay(bearerOf(cloudIn.Token)), http.StatusUnauthorized},
		{"a token that is nobody's", asCloud("nfs_" + strings.Repeat("A", 43)), http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", tt.headers)
			if rec.Code != tt.status {
				t.Errorf("status %d, body %s; want %d", rec.Code, rec.Body, tt.status)
			}
		})
	}

	// Nobody signs in with a PIN on the cloud's device, right PIN or wrong:
	// that would be a session of the venue's own owner, on a device nobody
	// paired, outliving the cloud's.
	for _, pin := range []string{"2468", "0000"} {
		rec := send(w.handler, http.MethodPost, "/api/v1/sessions", `{"staff_id":"stf_PRIYA001","pin":"`+pin+`"}`, viaRelay(bearerOf(cloudIn.Token)))
		if rec.Code != http.StatusForbidden || errorOf(t, rec).Code != CodeRemoteNotAllowed {
			t.Errorf("PIN %s on the cloud's device: status %d, body %s", pin, rec.Code, rec.Body)
		}
	}
	var onCloudDevice, attempts int
	if err := w.db.Reader().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM staff_sessions s JOIN devices d ON d.id = s.device_id JOIN staff st ON st.id = s.staff_id WHERE d.cloud = 1 AND st.cloud = 0`).Scan(&onCloudDevice); err != nil {
		t.Fatal(err)
	}
	if err := w.db.Reader().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sign_in_attempts a JOIN devices d ON d.id = a.device_id WHERE d.cloud = 1`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if onCloudDevice != 0 || attempts != 0 {
		t.Errorf("%d PIN sessions and %d PIN attempts on the cloud's device, want none", onCloudDevice, attempts)
	}

	// Revoking the cloud's device stops the session at once, and
	// deactivating the person would too.
	w.revokeCloudDevice(t)
	if rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", asCloud(cloudIn.Token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("after the device was revoked: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestTheAccessLogSaysTheCloudVouched(t *testing.T) {
	w := newCloudTill(t)
	w.log.Reset()
	in := w.signInFromCloud(t)
	send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", asCloud(in.Token))
	send(w.handler, http.MethodPost, cloudRoute, "", withAssertion("v1.e30.AAAA"))

	lines := requestLines(t, w.log)
	if len(lines) != 3 {
		t.Fatalf("%d request lines, want 3", len(lines))
	}
	for i, line := range lines[:2] {
		if line["via"] != ViaRelay || line["by"] != ByCloud || line["actor_id"] == nil || line["device_id"] == nil {
			t.Errorf("line %d = %v, want via=relay, by=cloud, and who and on what", i, line)
		}
	}
	if _, present := lines[2]["by"]; present || lines[2]["via"] != ViaRelay {
		t.Errorf("a refused sign-in logged as %v", lines[2])
	}
	// Neither the assertion nor the token is in the log.
	if logged := w.log.String(); strings.Contains(logged, in.Token) || strings.Contains(logged, "v1.e30") {
		t.Error("the log carries an assertion or a session's token")
	}

	// A PIN's request says nothing of the cloud.
	w.log.Reset()
	pin := w.signIn(t, "2468")
	send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asStaff(pin.Token))
	for _, line := range requestLines(t, w.log) {
		if _, present := line["by"]; present {
			t.Errorf("a PIN's request logged as %v", line)
		}
	}
}

func TestCloudSignInIsRateLimited(t *testing.T) {
	w := newCloudTill(t)
	last := 0
	for range cloudSignInLimit.requests + 1 {
		last = send(w.handler, http.MethodPost, cloudRoute, "", withAssertion("v1.e30.AAAA")).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("after %d tries: status %d, want 429", cloudSignInLimit.requests+1, last)
	}
}

// The People and Devices lists say which of theirs are the cloud's.
func TestTheListsMarkWhatIsTheClouds(t *testing.T) {
	w := newCloudTill(t)
	in := w.signInFromCloud(t)

	rec := send(w.handler, http.MethodGet, "/api/v1/devices", "", asCloud(in.Token))
	if rec.Code != http.StatusOK {
		t.Fatalf("devices: status %d, body %s", rec.Code, rec.Body)
	}
	var roster struct {
		Devices []deviceRecordResponse `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &roster); err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, d := range roster.Devices {
		if d.Cloud != (d.Name == auth.CloudDeviceName) {
			t.Errorf("device %q: cloud %v", d.Name, d.Cloud)
		}
		if d.Cloud {
			marked++
			if !d.This || d.LastSeenAt == nil {
				t.Errorf("the cloud's device, asked from it: this %v, last seen %v", d.This, d.LastSeenAt)
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d devices marked as the cloud's, want 1", marked)
	}
}
