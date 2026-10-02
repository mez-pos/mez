package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/byjoellobo/mez/internal/service/settings"
)

// TestVenueSettingsOverHTTP: the owner reads and renames the venue, a bad
// value comes back with its field, a cashier cannot touch it, and any paired
// device may read the venue's name.
func TestVenueSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(w.db, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string, header map[string]string) (int, settingsResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, header)
		if rec.Code >= 400 {
			return rec.Code, settingsResponse{}, errorOf(t, rec)
		}
		var out settingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "/api/v1/settings", "", owner)
	if status != http.StatusOK || out.Venue.Name == "" || out.Venue.Currency == "" || len(out.Settings) == 0 {
		t.Fatalf("read: %d %+v", status, out)
	}
	for _, e := range out.Settings {
		if e.Set {
			t.Errorf("%s is set on a new venue", e.Key)
		}
	}

	status, out, _ = call(http.MethodPut, "/api/v1/settings/venue",
		`{"name":" Blue Door ","legal_name":"Blue Door LLP","timezone":"Europe/London"}`, owner)
	if status != http.StatusOK || out.Venue.Name != "Blue Door" || out.Venue.LegalName != "Blue Door LLP" || out.Venue.Timezone != "Europe/London" {
		t.Fatalf("renamed: %d %+v", status, out.Venue)
	}

	refusals := []struct {
		name, body string
		status     int
		code       ErrorCode
		field      string
	}{
		{"no name", `{"name":"","timezone":"Asia/Kolkata"}`, 422, CodeInvalidSetting, "name"},
		{"not a timezone", `{"name":"A","timezone":"Mars/Olympus"}`, 422, CodeInvalidSetting, "timezone"},
		{"an unknown field", `{"name":"A","timezone":"Asia/Kolkata","currency":"USD"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(http.MethodPut, "/api/v1/settings/venue", tt.body, owner)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	if _, out, _ = call(http.MethodGet, "/api/v1/settings", "", owner); out.Venue.Name != "Blue Door" || out.Venue.Currency == "USD" {
		t.Errorf("a refused change was kept: %+v", out.Venue)
	}

	t.Run("a cashier cannot read or change them", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, req := range [][2]string{{http.MethodGet, "/api/v1/settings"}, {http.MethodPut, "/api/v1/settings/venue"}} {
			if rec := send(w.handler, req[0], req[1], `{"name":"X","timezone":"Asia/Kolkata"}`, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: %d", req[0], req[1], rec.Code)
			}
		}
	})

	t.Run("a device with nobody signed in reads the name and no more", func(t *testing.T) {
		rec := send(w.handler, http.MethodGet, "/api/v1/venue", "", w.asDevice())
		var name venueNameResponse
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &name) != nil || name.Name != "Blue Door" {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		if rec := send(w.handler, http.MethodGet, "/api/v1/settings", "", w.asDevice()); rec.Code != http.StatusUnauthorized {
			t.Errorf("settings with nobody signed in: %d", rec.Code)
		}
		if rec := send(w.handler, http.MethodGet, "/api/v1/venue", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("the venue name with no device: %d", rec.Code)
		}
		if rec := send(w.handler, http.MethodPost, "/api/v1/venue", "", w.asDevice()); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST /venue: %d", rec.Code)
		}
	})
}
