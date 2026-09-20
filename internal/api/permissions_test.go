package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// TestAPairingCodeNeedsThePermission is the back office endpoint 3a left
// behind, and the whole chain in one: a paired device, a signed-in staff
// member, and the permission the action needs.
func TestAPairingCodeNeedsThePermission(t *testing.T) {
	w := newTill(t)
	body := `{"name":"Kitchen screen","kind":"kitchen"}`

	cashier := w.signIn(t, "2468")
	rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", body, w.asStaff(cashier.Token))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cashier: status %d, want 403: %s", rec.Code, rec.Body)
	}
	if e := errorOf(t, rec); e.Code != CodeForbidden || e.Field != string(auth.PermDeviceManage) {
		t.Errorf("error = %+v", e)
	}

	owner := w.signInAs(t, "stf_PRIYA001", "2468")
	rec = send(w.handler, http.MethodPost, "/api/v1/devices/codes", body, w.asStaff(owner.Token))
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner: status %d, want 201: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a response carrying a pairing code may be cached")
	}
	var code pairingCodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &code); err != nil || code.ExpiresAt.IsZero() {
		t.Fatalf("code response = %s (%v)", rec.Body, err)
	}

	// And the code pairs a second device, which is what it is for.
	paired := pairOK(t, w.handler, code.Code)
	if paired.Device.Kind != "kitchen" || paired.Device.Name != "Kitchen screen" {
		t.Errorf("paired = %+v", paired.Device)
	}

	t.Run("a kind that cannot be paired", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes",
			`{"name":"Table 4","kind":"guest"}`, w.asStaff(owner.Token))
		if rec.Code != http.StatusBadRequest || errorOf(t, rec).Code != CodeInvalidRequest {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("without signing in", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", body, w.asDevice())
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeSignInRequired {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})
}

// overrideRoute mounts a handler that authorises an override for p, wrapped
// the way a real endpoint in task 4 will be. It is built here rather than
// routed, because no endpoint needing an override exists yet.
func (w *till) overrideRoute(p auth.Permission) http.Handler {
	return w.srv.requireDevice(w.srv.requireStaff(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			Override
			Reason string `json:"reason"`
		}
		if err := decodeJSON(rw, r, &req); err != nil {
			writeError(rw, w.srv.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
			return
		}
		by, ok := w.srv.authorise(rw, r, req.Override, p)
		if !ok {
			return
		}
		meta := Meta(r)
		writeJSON(rw, w.srv.log, http.StatusOK, map[string]any{
			"authorised_by": by.ID,
			"actor_id":      meta.ActorID,
			"venue_id":      meta.VenueID,
			"device_id":     meta.DeviceID,
			"key":           meta.IdempotencyKey,
		})
	})))
}

// TestAnOverrideChecksTheAuthoriserNotTheCaller is the manager override: a
// cashier signed in at the till, a manager's PIN on the void.
func TestAnOverrideChecksTheAuthoriserNotTheCaller(t *testing.T) {
	w := newTill(t)
	cashier := w.signIn(t, "2468")
	h := w.overrideRoute(auth.PermLineVoid)

	header := w.asStaff(cashier.Token)
	header[HeaderIdempotencyKey] = "void-1"

	t.Run("the manager authorises it", func(t *testing.T) {
		rec := send(h, http.MethodPost, "/void", `{"authorised_by":"stf_PRIYA001","pin":"2468","reason":"spilled"}`, header)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		// The command is attributed to the manager, the request to the
		// cashier who is signed in, and both are sent to the service.
		if got["authorised_by"] != float64(2) || got["actor_id"] != float64(1) {
			t.Errorf("authorised by %v, actor %v; want the manager and the cashier", got["authorised_by"], got["actor_id"])
		}
		if got["venue_id"] != float64(1) || got["device_id"] != float64(1) || got["key"] != "void-1" {
			t.Errorf("meta = %v", got)
		}
	})

	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"the cashier cannot authorise their own void", `{"authorised_by":"stf_ASHA0001","pin":"2468"}`, http.StatusForbidden, CodeForbidden},
		{"a manager who mistypes", `{"authorised_by":"stf_PRIYA001","pin":"9999"}`, http.StatusUnauthorized, CodeInvalidPIN},
		{"a manager who does not exist", `{"authorised_by":"stf_NOBODY00","pin":"2468"}`, http.StatusUnauthorized, CodeInvalidPIN},
		{"nobody at all", `{}`, http.StatusUnauthorized, CodeInvalidPIN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(h, http.MethodPost, "/void", tt.body, header)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
			if e := errorOf(t, rec); e.Code != tt.code {
				t.Errorf("code %q, want %q", e.Code, tt.code)
			}
		})
	}

	t.Run("an override needs a signed-in device too", func(t *testing.T) {
		rec := send(h, http.MethodPost, "/void", `{"authorised_by":"stf_PRIYA001","pin":"2468"}`, w.asDevice())
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeSignInRequired {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("and its failures count toward the lockout", func(t *testing.T) {
		for range auth.DefaultStaffLockout.MaxFailures {
			send(h, http.MethodPost, "/void", `{"authorised_by":"stf_PRIYA001","pin":"9999"}`, header)
		}
		rec := send(h, http.MethodPost, "/void", `{"authorised_by":"stf_PRIYA001","pin":"2468"}`, header)
		if rec.Code != http.StatusTooManyRequests || errorOf(t, rec).Code != CodeLockedOut {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Error("a lockout with no Retry-After")
		}
	})
}

// TestMetaCarriesOnlyWhatWasProved is the bridge to the order service.
func TestMetaCarriesOnlyWhatWasProved(t *testing.T) {
	w := newTill(t)
	in := w.signIn(t, "2468")

	var got []string
	h := w.srv.requireDevice(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		m := Meta(r)
		got = append(got, m.IdempotencyKey)
		if m.VenueID != 1 || m.DeviceID != 1 {
			t.Errorf("meta = %+v, want the paired device's venue and id", m)
		}
		if _, signedIn := SessionFrom(r.Context()); !signedIn && m.ActorID != 0 {
			t.Errorf("actor %d with no session", m.ActorID)
		}
		rw.WriteHeader(http.StatusNoContent)
	}))

	header := w.asDevice()
	header[HeaderIdempotencyKey] = "abc-123"
	send(h, http.MethodPost, "/x", "", header)
	send(h, http.MethodPost, "/x", "", w.asDevice())
	if len(got) != 2 || got[0] != "abc-123" || got[1] != "" {
		t.Errorf("idempotency keys = %v", got)
	}

	// With a session, the staff member is on it.
	staffHandler := w.srv.requireDevice(w.srv.requireStaff(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if m := Meta(r); m.ActorID != 1 {
			t.Errorf("actor = %d, want the signed-in cashier", m.ActorID)
		}
		rw.WriteHeader(http.StatusNoContent)
	})))
	send(staffHandler, http.MethodPost, "/x", "", w.asStaff(in.Token))

	// Outside a request, Meta is empty rather than wrong.
	if m := Meta(httptest.NewRequest(http.MethodGet, "/", nil)); m != (svcorder.Meta{}) {
		t.Errorf("Meta with nothing proved = %+v", m)
	}
}

// TestPermissionMiddlewareFailuresThatAreNotTheClientsFault covers the two
// ways the chain can be wrong rather than the request.
func TestPermissionMiddlewareFailuresThatAreNotTheClientsFault(t *testing.T) {
	w := newTill(t)
	ok := http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusNoContent) })

	// Mounted outside requireStaff: a routing bug, answered as a 500.
	naked := w.srv.requireDevice(w.srv.requirePermission(auth.PermOrderOpen, ok))
	rec := send(naked, http.MethodGet, "/x", "", w.asDevice())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("permission check with no session: %d", rec.Code)
	}

	// The same for an override outside requireDevice.
	loose := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if _, proceeded := w.srv.authorise(loose, req, Override{}, auth.PermLineVoid); proceeded || loose.Code != http.StatusInternalServerError {
		t.Errorf("override with no device: proceeded %v, status %d", proceeded, loose.Code)
	}

	// And a staff layer that cannot read permissions at all.
	broken := newTestServer(t, testConfig(), WithDevices(workingDevices{}), WithStaff(brokenPermissions{}))
	h := broken.requireDevice(broken.requireStaff(broken.requirePermission(auth.PermOrderOpen, ok)))
	rec = send(h, http.MethodGet, "/x", "", map[string]string{
		"Authorization": "Bearer nfd_anything", HeaderStaffToken: "nfs_anything",
	})
	if rec.Code != http.StatusInternalServerError || errorOf(t, rec).Code != CodeInternal {
		t.Errorf("status %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "disk") {
		t.Error("the internal error leaked to the client")
	}
}

// brokenPermissions admits any session but cannot read the role behind it.
type brokenPermissions struct{ brokenStaff }

func (brokenPermissions) Session(context.Context, auth.Device, string) (auth.Session, error) {
	return auth.Session{Staff: auth.Staff{ID: 1, VenueID: 1, RoleID: 1}}, nil
}
