package api

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/service/stock"
)

const ingredientSheet = "Name,Counted in,Low at,Cost,Count at close\nMilk,ml,2000,60,yes\nEggs,each,12,7.50,\n"

// sendCSV posts a body as a file of the given type.
func sendCSV(w *till, path, contentType, body string, as map[string]string) *httptest.ResponseRecorder {
	header := map[string]string{"Content-Type": contentType}
	maps.Copy(header, as)
	return send(w.handler, http.MethodPost, path, body, header)
}

func itemsImportOf(t *testing.T, rec *httptest.ResponseRecorder) stock.ItemsImport {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s, want 200", rec.Code, rec.Body)
	}
	var out stock.ItemsImport
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func stockNames(t *testing.T, w *till, owner map[string]string) []string {
	t.Helper()
	rec := send(w.handler, http.MethodGet, "/api/v1/stock", "", owner)
	var list stockItemsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	var names []string
	for _, it := range list.Items {
		names = append(names, it.Name)
	}
	return names
}

// TestStockImportOverHTTP: the owner gets the template, checks a file, which
// writes nothing, imports it, and the store has the items with nothing on
// hand.
func TestStockImportOverHTTP(t *testing.T) {
	w, owner := newStockTill(t)

	rec := send(w.handler, http.MethodGet, "/api/v1/stock/import/template", "", owner)
	var file csvResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &file); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("template: %d %s (%v)", rec.Code, rec.Body, err)
	}
	first, _, _ := strings.Cut(file.Text, "\r\n")
	if file.Filename != "mez-ingredients-template.csv" || first != byteOrderMark+"Name,Counted in,Low at,Cost,Count at close" {
		t.Errorf("template: %q, first line %q", file.Filename, first)
	}
	if plan := itemsImportOf(t, sendCSV(w, "/api/v1/stock/import/preview", "text/csv", file.Text, owner)); len(plan.Problems) != 0 || len(plan.ToAdd) != 3 {
		t.Errorf("the template, checked: %+v", plan)
	}

	rec = sendCSV(w, "/api/v1/stock/import/preview", "text/csv; charset=utf-8", ingredientSheet, owner)
	plan := itemsImportOf(t, rec)
	if rec.Header().Get("Cache-Control") != "no-store" || plan.Imported || len(plan.ToAdd) != 2 {
		t.Fatalf("preview: %+v (cache %q)", plan, rec.Header().Get("Cache-Control"))
	}
	if !strings.Contains(rec.Body.String(), `"row":2,"name":"Milk","unit":"ml","low_level":2000000,"cost":{"minor":6000,"currency":"INR"},"count_at_close":true`) {
		t.Errorf("the first row as sent: %s", rec.Body)
	}
	if names := stockNames(t, w, owner); len(names) != 0 {
		t.Fatalf("a preview wrote: %v", names)
	}

	if done := itemsImportOf(t, sendCSV(w, "/api/v1/stock/import", "text/csv", ingredientSheet, owner)); !done.Imported || len(done.ToAdd) != 2 {
		t.Fatalf("import: %+v", done)
	}
	if names := stockNames(t, w, owner); strings.Join(names, "|") != "Eggs|Milk" {
		t.Errorf("the store: %v", names)
	}

	again := itemsImportOf(t, sendCSV(w, "/api/v1/stock/import", "text/csv", ingredientSheet, owner))
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 2 {
		t.Errorf("the same file again: %+v", again)
	}
}

func TestStockImportRefusals(t *testing.T) {
	w, owner := newStockTill(t)
	cashier := w.asStaff(w.signInAs(t, "stf_ASHA0001", "2468").Token)
	tooBig := "Name,Counted in\n" + strings.Repeat("x", stock.MaxImportBytes)

	tests := []struct {
		name, path, contentType, body string
		as                            map[string]string
		status                        int
		code                          ErrorCode
		field                         string
	}{
		{"a file with a problem", "/api/v1/stock/import", "text/csv", ingredientSheet + "Rice,kg,5,90,\n", owner, http.StatusUnprocessableEntity, CodeStockImportRefused, "file"},
		{"JSON, not a file", "/api/v1/stock/import", "application/json", `{}`, owner, http.StatusUnsupportedMediaType, CodeInvalidStock, "file"},
		{"a file too large to import", "/api/v1/stock/import", "text/csv", tooBig, owner, http.StatusRequestEntityTooLarge, CodeInvalidStock, "file"},
		{"a file too large to check", "/api/v1/stock/import/preview", "text/csv", tooBig, owner, http.StatusRequestEntityTooLarge, CodeInvalidStock, "file"},
		{"a cashier importing", "/api/v1/stock/import", "text/csv", ingredientSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"a cashier checking", "/api/v1/stock/import/preview", "text/csv", ingredientSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"no one signed in", "/api/v1/stock/import", "text/csv", ingredientSheet, w.asDevice(), http.StatusUnauthorized, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := sendCSV(w, tt.path, tt.contentType, tt.body, tt.as)
			got := errorOf(t, rec)
			if rec.Code != tt.status || (tt.code != "" && got.Code != tt.code) || (tt.field != "" && got.Field != tt.field) {
				t.Errorf("%d %+v, want %d %s on %q", rec.Code, got, tt.status, tt.code, tt.field)
			}
		})
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/stock/import/template", "", cashier); rec.Code != http.StatusForbidden {
		t.Errorf("a cashier reading the template: %d, want 403", rec.Code)
	}
	if names := stockNames(t, w, owner); len(names) != 0 {
		t.Errorf("a refusal wrote: %v", names)
	}
}

// TestStockImportPreviewSaysWhatIsWrong: the preview of a bad file is a 200
// whose answer is the problems.
func TestStockImportPreviewSaysWhatIsWrong(t *testing.T) {
	w, owner := newStockTill(t)
	plan := itemsImportOf(t, sendCSV(w, "/api/v1/stock/import/preview", "text/csv", ingredientSheet+"Rice,kg,5,90,\n,g,,,\n", owner))
	if len(plan.ToAdd) != 2 || len(plan.Problems) != 2 {
		t.Fatalf("preview: %+v", plan)
	}
	if p := plan.Problems[0]; p.Row != 4 || p.Column != "Counted in" || p.Message == "" {
		t.Errorf("first problem: %+v", p)
	}
	if p := plan.Problems[1]; p.Row != 5 || p.Column != "Name" {
		t.Errorf("second problem: %+v", p)
	}
}
