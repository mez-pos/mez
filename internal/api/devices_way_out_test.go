package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const wayOutPath = "/api/v1/devices/me/way-out"

// TestTheWayOutListsWhoMayOpenIt: a tablet locked to mez is let out by a
// staff member whose role may manage devices, and the app lists only them
// (ADR-419).
func TestTheWayOutListsWhoMayOpenIt(t *testing.T) {
	w := newTill(t)
	rec := send(w.handler, http.MethodGet, wayOutPath, "", w.asDevice())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	var list staffListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Staff) != 1 || list.Staff[0].ID != "stf_PRIYA001" || list.Staff[0].Role != "owner" {
		t.Errorf("who may open it = %+v, want Priya the owner alone", list.Staff)
	}
}

// TestTheWayOutOpensForAManagersPIN: the PIN is checked by the PC, as the
// till's overrides are, and who opened it is answered and logged; the PIN
// is in neither.
func TestTheWayOutOpensForAManagersPIN(t *testing.T) {
	w := newTill(t)
	rec := send(w.handler, http.MethodPost, wayOutPath, `{"authorised_by":"stf_PRIYA001","pin":"2468"}`, w.asDevice())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("an answer to a PIN may be cached")
	}
	var out wayOutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("way out: %v", err)
	}
	if out.OpenedBy.ID != "stf_PRIYA001" || out.OpenedBy.Name != "Priya" {
		t.Errorf("opened by %+v, want Priya", out.OpenedBy)
	}
	logged := w.log.String()
	if !strings.Contains(logged, "way out") || strings.Contains(logged, "2468") || strings.Contains(rec.Body.String(), "2468") {
		t.Errorf("the log does not say the way out was opened, or carries the PIN:\n%s", logged)
	}
	// No session is opened: nobody is left signed in on the tablet.
	if sessions := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", w.asDevice()); sessions.Code == http.StatusOK {
		t.Errorf("opening the way out signed someone in: %s", sessions.Body)
	}
}

func TestTheWayOutRefusals(t *testing.T) {
	w := newTill(t)
	tests := []struct {
		name   string
		method string
		body   string
		header map[string]string
		status int
		code   ErrorCode
	}{
		{"a cashier", http.MethodPost, `{"authorised_by":"stf_ASHA0001","pin":"2468"}`, w.asDevice(), http.StatusForbidden, CodeForbidden},
		{"a wrong PIN", http.MethodPost, `{"authorised_by":"stf_PRIYA001","pin":"1111"}`, w.asDevice(), http.StatusUnauthorized, CodeInvalidPIN},
		{"nobody named", http.MethodPost, `{"authorised_by":"","pin":"2468"}`, w.asDevice(), http.StatusUnauthorized, CodeInvalidPIN},
		{"not JSON", http.MethodPost, `pin=2468`, w.asDevice(), http.StatusBadRequest, CodeInvalidRequest},
		{"no tablet", http.MethodPost, `{"authorised_by":"stf_PRIYA001","pin":"2468"}`, nil, http.StatusUnauthorized, CodeUnauthenticated},
		{"no tablet asking who", http.MethodGet, "", nil, http.StatusUnauthorized, CodeUnauthenticated},
		{"through the relay", http.MethodPost, `{"authorised_by":"stf_PRIYA001","pin":"2468"}`, viaRelay(w.asDevice()), http.StatusForbidden, CodeRemoteNotAllowed},
		{"another method", http.MethodDelete, "", w.asDevice(), http.StatusMethodNotAllowed, CodeMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, tt.method, wayOutPath, tt.body, tt.header)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("status %d, body %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}
