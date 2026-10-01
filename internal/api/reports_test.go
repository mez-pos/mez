package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/service/reports"
)

func TestTheSalesReportsOverHTTP(t *testing.T) {
	w := newTill(t)
	rs, err := reports.New(w.db)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithReports(rs))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	rec := send(w.handler, http.MethodGet, "/api/v1/reports/sales?from=2026-09-01&to=2026-09-30&every=week", "", owner)
	var got reports.Sales
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("sales: %d %s", rec.Code, rec.Body)
	}
	if got.Every != "week" || len(got.Series) != 5 || got.Hours == nil || got.Totals.ByItem == nil {
		t.Errorf("a quiet September by week: %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the reports may be cached")
	}

	refusals := []struct {
		name, query string
		headers     map[string]string
		status      int
		code        ErrorCode
	}{
		{"no range", "", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"backwards", "?from=2026-09-30&to=2026-09-01", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"by the hour", "?from=2026-09-01&to=2026-09-30&every=hour", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a cashier", "?from=2026-09-01&to=2026-09-30", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/reports/sales"+tt.query, "", tt.headers)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("%d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}
