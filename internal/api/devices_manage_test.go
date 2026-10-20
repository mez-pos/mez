package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/auth"
)

// pairAnother pairs one more device at venue 1 and returns its token and id.
func (w *till) pairAnother(t *testing.T, name string, kind auth.DeviceKind) (token, id string) {
	t.Helper()
	code, err := w.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: name, DeviceKind: kind})
	if err != nil {
		t.Fatal(err)
	}
	p := pairOK(t, w.handler, code.Code)
	return p.Token, p.Device.ID
}

func (w *till) deviceList(t *testing.T, headers map[string]string) deviceListResponse {
	t.Helper()
	rec := send(w.handler, http.MethodGet, "/api/v1/devices", "", headers)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("list: %d %s (cache %q)", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
	}
	var out deviceListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestDevicesOverHTTP: the owner lists the venue's devices, renames one and
// revokes another, and what each answers with is the list as it now stands.
func TestDevicesOverHTTP(t *testing.T) {
	w := newTill(t)
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	kitchenToken, kitchenID := w.pairAnother(t, "Pass", auth.KindKitchen)

	// A code made and not used is waiting.
	if rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", `{"name":"Bar tablet","kind":"terminal"}`, owner); rec.Code != http.StatusCreated {
		t.Fatalf("a code: %d %s", rec.Code, rec.Body)
	}

	list := w.deviceList(t, owner)
	if len(list.Devices) != 2 || list.Devices[0].Name != "Counter 1" || !list.Devices[0].This || list.Devices[0].Kind != auth.KindTerminal ||
		list.Devices[1].ID != kitchenID || list.Devices[1].This || list.Devices[1].RevokedAt != nil || list.Devices[1].PairedAt.IsZero() {
		t.Fatalf("devices = %+v", list.Devices)
	}
	if len(list.Waiting) != 1 || list.Waiting[0].Name != "Bar tablet" || list.Waiting[0].Kind != auth.KindTerminal || list.Waiting[0].ExpiresAt.IsZero() {
		t.Fatalf("waiting = %+v", list.Waiting)
	}
	// The code itself is nowhere in the answer.
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices", "", owner); strings.Contains(rec.Body.String(), "code") {
		t.Errorf("the list mentions a code: %s", rec.Body)
	}

	// Rename: the answer is the list.
	rec := send(w.handler, http.MethodPost, "/api/v1/devices/"+kitchenID+"/rename", `{"name":"  Kitchen pass  "}`, owner)
	var renamed deviceListResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &renamed) != nil || renamed.Devices[1].Name != "Kitchen pass" {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}

	// Revoke: it stops working, and is listed last as revoked.
	rec = send(w.handler, http.MethodPost, "/api/v1/devices/"+kitchenID+"/revoke", "", owner)
	var revoked deviceListResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &revoked) != nil || len(revoked.Devices) != 2 ||
		revoked.Devices[1].ID != kitchenID || revoked.Devices[1].RevokedAt == nil {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices/me", "", bearerOf(kitchenToken)); rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeDeviceRevoked {
		t.Errorf("the revoked device: %d", rec.Code)
	}
	// Revoking again changes nothing and is not an error.
	if rec = send(w.handler, http.MethodPost, "/api/v1/devices/"+kitchenID+"/revoke", "", owner); rec.Code != http.StatusOK {
		t.Errorf("revoking again: %d %s", rec.Code, rec.Body)
	}
}

func TestDeviceManagementRefusals(t *testing.T) {
	w := newTill(t)
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	_, kitchenID := w.pairAnother(t, "Pass", auth.KindKitchen)
	self := w.deviceList(t, owner).Devices[0].ID
	if _, err := w.devices.Roster(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	// Revoked, then renamed.
	if rec := send(w.handler, http.MethodPost, "/api/v1/devices/"+kitchenID+"/revoke", "", owner); rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}

	refusals := []struct {
		name, path, body string
		status           int
		code             ErrorCode
		field            string
	}{
		{"revoking the device in use", "/api/v1/devices/" + self + "/revoke", "", 409, CodeCannotRevokeSelf, ""},
		{"a device that is not there", "/api/v1/devices/dev_NOSUCH01/revoke", "", 404, CodeDeviceNotFound, ""},
		{"renaming one that is not there", "/api/v1/devices/dev_NOSUCH01/rename", `{"name":"x"}`, 404, CodeDeviceNotFound, ""},
		{"renaming a revoked device", "/api/v1/devices/" + kitchenID + "/rename", `{"name":"Again"}`, 409, CodeDeviceUnpaired, ""},
		{"an empty name", "/api/v1/devices/" + self + "/rename", `{"name":"  "}`, 422, CodeInvalidRequest, "name"},
		{"a name too long", "/api/v1/devices/" + self + "/rename", `{"name":"` + strings.Repeat("a", 65) + `"}`, 422, CodeInvalidRequest, "name"},
		{"an unknown field", "/api/v1/devices/" + self + "/rename", `{"name":"x","kind":"kitchen"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodPost, tt.path, tt.body, owner)
			if apiErr := errorOf(t, rec); rec.Code != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", rec.Code, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	if list := w.deviceList(t, owner); list.Devices[0].Name != "Counter 1" {
		t.Errorf("a refused rename changed the name: %q", list.Devices[0].Name)
	}
	// The device in use still works after the refusal.
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices/me", "", owner); rec.Code != http.StatusOK {
		t.Errorf("the device in use stopped working: %d", rec.Code)
	}
}

// A pairing code refused for its name says so under the name, not the kind.
func TestAPairingCodeRefusalNamesItsField(t *testing.T) {
	w := newTill(t)
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	for body, field := range map[string]string{`{"name":" ","kind":"terminal"}`: "name", `{"name":"Toaster","kind":"toaster"}`: "kind"} {
		rec := send(w.handler, http.MethodPost, "/api/v1/devices/codes", body, owner)
		if apiErr := errorOf(t, rec); rec.Code != http.StatusBadRequest || apiErr.Field != field {
			t.Errorf("%s: %d %q, want field %q", body, rec.Code, apiErr.Field, field)
		}
	}
}

func TestDeviceManagementNeedsDeviceManage(t *testing.T) {
	w := newTill(t)
	cashier := w.asStaff(w.signIn(t, "2468").Token)
	_, kitchenID := w.pairAnother(t, "Pass", auth.KindKitchen)
	for _, r := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/devices", ""},
		{http.MethodPost, "/api/v1/devices/" + kitchenID + "/rename", `{"name":"x"}`},
		{http.MethodPost, "/api/v1/devices/" + kitchenID + "/revoke", ""},
	} {
		if rec := send(w.handler, r.method, r.path, r.body, cashier); rec.Code != http.StatusForbidden {
			t.Errorf("a cashier %s %s: %d", r.method, r.path, rec.Code)
		}
	}
	// No staff session at all, and no device: refused before anything is read.
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices", "", w.asDevice()); rec.Code != http.StatusUnauthorized {
		t.Errorf("a device with no staff signed in: %d", rec.Code)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/devices", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no device at all: %d", rec.Code)
	}
	// And the wrong verbs are not allowed.
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	if rec := send(w.handler, http.MethodDelete, "/api/v1/devices", "", owner); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /devices: %d", rec.Code)
	}
}
