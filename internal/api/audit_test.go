package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/service/audit"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

func TestTheAuditTrailAsCSV(t *testing.T) {
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

	rec := send(w.handler, http.MethodGet, "/api/v1/audit/csv?from=2026-09-01&to=2026-09-30&kind=void", "", owner)
	var got csvResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("csv: %d %s", rec.Code, rec.Body)
	}
	if got.Filename != "audit-2026-09-01-to-2026-09-30.csv" || !strings.HasPrefix(got.Text, byteOrderMark+"Trading day,At,Kind,Order,") {
		t.Errorf("file %q: %q", got.Filename, got.Text)
	}

	refusals := []struct {
		name, query string
		headers     map[string]string
		status      int
		code        ErrorCode
	}{
		{"no range", "", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a kind we do not have", "?from=2026-09-01&to=2026-09-30&kind=theft", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"someone not here", "?from=2026-09-01&to=2026-09-30&staff=stf_NOBODY01", owner, http.StatusNotFound, CodeStaffNotFound},
		{"a cashier", "?from=2026-09-01&to=2026-09-30", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/audit/csv"+tt.query, "", tt.headers)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("%d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}

func TestTheAnalysisOverHTTP(t *testing.T) {
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
	const base = "/api/v1/audit/analysis"

	rec := send(w.handler, http.MethodGet, base+"?from=2026-09-01&to=2026-09-30", "", owner)
	var got audit.Analysis
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("analysis: %d %s", rec.Code, rec.Body)
	}
	if got.From != "2026-09-01" || len(got.ByKind) != len(audit.Kinds()) || got.ByReason == nil || got.By == nil || got.AuthorisedBy == nil {
		t.Errorf("a quiet September: %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the analysis may be cached")
	}

	rec = send(w.handler, http.MethodGet, base+"/csv?from=2026-09-01&to=2026-09-30&by=authorised_by", "", owner)
	var file csvResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &file) != nil {
		t.Fatalf("csv: %d %s", rec.Code, rec.Body)
	}
	if file.Filename != "corrections-authorised_by-2026-09-01-to-2026-09-30.csv" || !strings.HasPrefix(file.Text, byteOrderMark+"Authorised by,Count,Amount (") {
		t.Errorf("file %q: %q", file.Filename, file.Text)
	}

	refusals := []struct {
		name, path string
		headers    map[string]string
		status     int
		code       ErrorCode
	}{
		{"no range", base, owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a kind we do not have", base + "?from=2026-09-01&to=2026-09-30&kind=theft", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"someone not here", base + "?from=2026-09-01&to=2026-09-30&staff=stf_NOBODY01", owner, http.StatusNotFound, CodeStaffNotFound},
		{"a cashier", base + "?from=2026-09-01&to=2026-09-30", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
		{"a file of no grouping", base + "/csv?from=2026-09-01&to=2026-09-30", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a file of a grouping we do not have", base + "/csv?from=2026-09-01&to=2026-09-30&by=kind", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a file backwards", base + "/csv?from=2026-09-30&to=2026-09-01&by=reason", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a file for a cashier", base + "/csv?from=2026-09-01&to=2026-09-30&by=reason", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, tt.path, "", tt.headers)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("%d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}

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
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil || page.Entries == nil || len(page.Counts) != len(audit.Kinds()) {
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
