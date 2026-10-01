package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/service/audit"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

func TestTheAuditTrailOverHTTP(t *testing.T) {
	w := newTill(t)
	orders, err := svcorder.New(svcorder.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	trail, err := audit.New(w.db, orders)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithAudit(trail))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	rec := send(w.handler, http.MethodGet, "/api/v1/audit?from=2026-09-01&to=2026-09-30&kind=void&limit=10", "", owner)
	var page audit.Page
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil || page.Entries == nil || len(page.Counts) != 6 {
		t.Fatalf("trail: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the trail may be cached")
	}

	refusals := []struct {
		name, query string
		headers     map[string]string
		status      int
		code        ErrorCode
	}{
		{"no range", "", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a kind we do not have", "?from=2026-09-01&to=2026-09-30&kind=theft", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a cursor that is not one", "?from=2026-09-01&to=2026-09-30&before=soon", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"someone not here", "?from=2026-09-01&to=2026-09-30&staff=stf_NOBODY01", owner, http.StatusNotFound, CodeStaffNotFound},
		{"a cashier", "?from=2026-09-01&to=2026-09-30", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/audit"+tt.query, "", tt.headers)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("%d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}
