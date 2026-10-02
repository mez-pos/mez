package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/byjoellobo/mez/internal/service/people"
)

func newPeopleTill(t *testing.T) (*till, map[string]string) {
	t.Helper()
	w := newTill(t)
	svc, err := people.New(w.db, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithPeople(svc))
	return w, w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
}

func (w *till) people(t *testing.T, headers map[string]string) peopleResponse {
	t.Helper()
	rec := send(w.handler, http.MethodGet, "/api/v1/people", "", headers)
	var out peopleResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("people: %d %s", rec.Code, rec.Body)
	}
	return out
}

func roleID(t *testing.T, p peopleResponse, name string) int64 {
	t.Helper()
	for _, r := range p.Roles {
		if r.Name == name {
			return r.ID
		}
	}
	t.Fatalf("no role %s", name)
	return 0
}

// TestPeopleOverHTTP: the owner adds a manager, who signs in with the PIN
// given and can manage cashiers but not the owner; a new PIN signs the
// cashier out at once.
func TestPeopleOverHTTP(t *testing.T) {
	w, owner := newPeopleTill(t)
	all := w.people(t, owner)
	if len(all.Staff) != 2 || len(all.Roles) != 4 {
		t.Fatalf("people = %+v", all)
	}

	body := `{"name":"Meera","role_id":` + itoa(roleID(t, all, "manager")) + `,"pin":"1111"}`
	rec := send(w.handler, http.MethodPost, "/api/v1/people/staff", body, owner)
	var added staffAddedResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &added) != nil || added.ID == "" {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	manager := w.asStaff(w.signInAs(t, added.ID, "1111").Token)

	// Within a manager's own: a cashier's PIN. Asha is signed in, and a new
	// PIN signs her out.
	asha := w.asStaff(w.signIn(t, "2468").Token)
	if rec := send(w.handler, http.MethodPut, "/api/v1/people/staff/stf_ASHA0001/pin", `{"pin":"8642"}`, manager); rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/sessions/current", "", asha); rec.Code != http.StatusUnauthorized {
		t.Errorf("Asha's old session: %d, want signed out", rec.Code)
	}
	w.signIn(t, "8642")

	refusals := []struct {
		name, method, path, body string
		headers                  map[string]string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"a manager resets the owner's PIN", http.MethodPut, "/api/v1/people/staff/stf_PRIYA001/pin", `{"pin":"0000"}`, manager, http.StatusForbidden, CodeBeyondYou, ""},
		{"a manager adds an owner", http.MethodPost, "/api/v1/people/staff", `{"name":"X","role_id":` + itoa(roleID(t, all, "owner")) + `,"pin":"1234"}`, manager, http.StatusForbidden, CodeBeyondYou, ""},
		{"a PIN of letters", http.MethodPut, "/api/v1/people/staff/stf_ASHA0001/pin", `{"pin":"abcd"}`, owner, http.StatusUnprocessableEntity, CodeInvalidPerson, "pin"},
		{"a person who is not here", http.MethodPut, "/api/v1/people/staff/stf_NOBODY01/pin", `{"pin":"1234"}`, owner, http.StatusNotFound, CodeStaffNotFound, ""},
		{"the owner deactivates herself", http.MethodPost, "/api/v1/people/staff/stf_PRIYA001/deactivate", "", owner, http.StatusUnprocessableEntity, CodeInvalidPerson, "active"},
		{"a role name taken", http.MethodPost, "/api/v1/people/roles", `{"name":"cashier","permissions":[]}`, owner, http.StatusConflict, CodeRoleNameTaken, "name"},
		{"deleting a role people hold", http.MethodDelete, "/api/v1/people/roles/" + itoa(roleID(t, all, "cashier")), "", owner, http.StatusConflict, CodeRoleInUse, ""},
		{"a cashier", http.MethodGet, "/api/v1/people", "", w.asStaff(w.signIn(t, "8642").Token), http.StatusForbidden, CodeForbidden, "staff.manage"},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, tt.method, tt.path, tt.body, tt.headers)
			if rec.Code != tt.status {
				t.Fatalf("status %d %s, want %d", rec.Code, rec.Body, tt.status)
			}
			if got := errorOf(t, rec); got.Code != tt.code || got.Field != tt.field {
				t.Errorf("error = %+v, want %s on %q", got, tt.code, tt.field)
			}
		})
	}

	// Roles: add, edit, delete one nobody holds.
	rec = send(w.handler, http.MethodPost, "/api/v1/people/roles", `{"name":"Barista","permissions":["order.open","kitchen.view"]}`, owner)
	var role roleEditResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &role) != nil {
		t.Fatalf("add role: %d %s", rec.Code, rec.Body)
	}
	path := "/api/v1/people/roles/" + itoa(role.ID)
	if rec := send(w.handler, http.MethodPut, path, `{"name":"Head barista","permissions":["order.open"]}`, owner); rec.Code != http.StatusOK {
		t.Errorf("edit role: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodDelete, path, "", owner); rec.Code != http.StatusOK {
		t.Errorf("delete role: %d %s", rec.Code, rec.Body)
	}

	// Deactivating Meera ends her session; bringing her back lets her in.
	if rec := send(w.handler, http.MethodPost, "/api/v1/people/staff/"+added.ID+"/deactivate", "", owner); rec.Code != http.StatusOK {
		t.Fatalf("deactivate: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/people", "", manager); rec.Code != http.StatusUnauthorized {
		t.Errorf("a deactivated manager's session: %d, want signed out", rec.Code)
	}
	if rec := send(w.handler, http.MethodPost, "/api/v1/people/staff/"+added.ID+"/reactivate", "", owner); rec.Code != http.StatusOK {
		t.Fatalf("reactivate: %d %s", rec.Code, rec.Body)
	}
	w.signInAs(t, added.ID, "1111")
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
