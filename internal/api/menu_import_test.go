package api

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/service/menus"
)

// The fixture's venue has one tax class, one station and one course, and
// shows no HSN, so a row needs only the three required columns.
const importSheet = "Category,Name,Price,Diet\nFood,Masala dosa,140,veg\nFood,Idli,80.50,veg\nDrinks,Filter coffee,40,\n"

// sendFile sends a body as a file of the given type, as the owner unless
// told who.
func (m *editorTill) sendFile(path, contentType, body string, as map[string]string) *httptest.ResponseRecorder {
	header := map[string]string{"Content-Type": contentType}
	if as == nil {
		as = m.owner
	}
	maps.Copy(header, as)
	return send(m.handler, http.MethodPost, path, body, header)
}

func importOf(t *testing.T, rec *httptest.ResponseRecorder) menus.Import {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s, want 200", rec.Code, rec.Body)
	}
	var out menus.Import
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMenuImportOverHTTP: the owner checks a file, which writes nothing,
// then imports it, and the terminal's menu has the items at a new version.
func TestMenuImportOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	before := m.editor(t).Version

	rec := m.sendFile("/api/v1/menu/import/preview", "text/csv; charset=utf-8", importSheet, nil)
	plan := importOf(t, rec)
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("a preview may be cached: %q", rec.Header().Get("Cache-Control"))
	}
	if plan.Imported || len(plan.ToAdd) != 3 || len(plan.NewCategories) != 1 || plan.NewCategories[0] != "Drinks" || len(plan.Problems) != 0 {
		t.Fatalf("preview: %+v", plan)
	}
	if !strings.Contains(rec.Body.String(), `"row":3,"category":"Food","name":"Idli","price":{"minor":8050,"currency":"INR"}`) {
		t.Errorf("the second row as sent: %s", rec.Body)
	}
	if e := m.editor(t); len(e.Items) != 0 || e.Version != before {
		t.Fatalf("a preview wrote: %d items at version %d (was %d)", len(e.Items), e.Version, before)
	}

	done := importOf(t, m.sendFile("/api/v1/menu/import", "text/csv", importSheet, nil))
	if !done.Imported || len(done.ToAdd) != 3 {
		t.Fatalf("import: %+v", done)
	}
	e := m.editor(t)
	if len(e.Items) != 3 || len(e.Categories) != 2 || e.Version != before+1 {
		t.Fatalf("after the import: %d items, %d categories, version %d (was %d)", len(e.Items), len(e.Categories), e.Version, before)
	}
	if it := e.Items[1]; it.Name != "Idli" || it.Price.Minor != 8050 || it.Diet != "veg" || it.TaxClassID != m.taxClass || it.StationID != m.station || it.CourseID != m.course {
		t.Errorf("idli: %+v", it)
	}

	rec = send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if len(till.Items) != 3 || till.Version != e.Version {
		t.Errorf("the terminal's menu: %d items at version %d, want 3 at %d", len(till.Items), till.Version, e.Version)
	}

	// The same file again: a retry adds nothing and is not an error.
	again := importOf(t, m.sendFile("/api/v1/menu/import", "text/csv", importSheet, nil))
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 3 {
		t.Errorf("the same file again: %+v", again)
	}
	if e := m.editor(t); len(e.Items) != 3 || e.Version != before+1 {
		t.Errorf("after the same file again: %d items at version %d", len(e.Items), e.Version)
	}
}

func TestMenuImportRefusals(t *testing.T) {
	m := newEditorTill(t)
	cashier := m.asStaff(m.signInAs(t, "stf_ASHA0001", "2468").Token)
	bad := importSheet + "Food,Vada,lots,\nFood,,10,\n"
	tooBig := "Category,Name,Price\n" + strings.Repeat("x", menus.MaxImportBytes)

	tests := []struct {
		name, path, contentType, body string
		as                            map[string]string
		status                        int
		code                          ErrorCode
		field                         string
	}{
		{"a file with problems", "/api/v1/menu/import", "text/csv", bad, nil, http.StatusUnprocessableEntity, CodeMenuImportRefused, "file"},
		{"JSON, not a file", "/api/v1/menu/import", "application/json", `{}`, nil, http.StatusUnsupportedMediaType, CodeInvalidMenu, "file"},
		{"no type at all", "/api/v1/menu/import/preview", "", importSheet, nil, http.StatusUnsupportedMediaType, CodeInvalidMenu, "file"},
		{"a file too large to import", "/api/v1/menu/import", "text/csv", tooBig, nil, http.StatusRequestEntityTooLarge, CodeInvalidMenu, "file"},
		{"a file too large to check", "/api/v1/menu/import/preview", "text/csv", tooBig, nil, http.StatusRequestEntityTooLarge, CodeInvalidMenu, "file"},
		{"a cashier importing", "/api/v1/menu/import", "text/csv", importSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"a cashier checking", "/api/v1/menu/import/preview", "text/csv", importSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"no one signed in", "/api/v1/menu/import", "text/csv", importSheet, m.asDevice(), http.StatusUnauthorized, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := m.sendFile(tt.path, tt.contentType, tt.body, tt.as)
			got := errorOf(t, rec)
			if rec.Code != tt.status || (tt.code != "" && got.Code != tt.code) || (tt.field != "" && got.Field != tt.field) {
				t.Errorf("%d %+v, want %d %s on %q", rec.Code, got, tt.status, tt.code, tt.field)
			}
		})
	}
	if e := m.editor(t); len(e.Items) != 0 || len(e.Categories) != 0 {
		t.Errorf("a refusal wrote: %d items, %d categories", len(e.Items), len(e.Categories))
	}
}

// TestMenuImportPreviewSaysWhatIsWrong: the preview of a bad file is a 200
// whose answer is the problems, each with its row and column.
func TestMenuImportPreviewSaysWhatIsWrong(t *testing.T) {
	m := newEditorTill(t)
	plan := importOf(t, m.sendFile("/api/v1/menu/import/preview", "text/csv", importSheet+"Food,Vada,lots,\nFood,,10,\n", nil))
	if len(plan.ToAdd) != 3 || len(plan.Problems) != 2 {
		t.Fatalf("preview: %+v", plan)
	}
	if p := plan.Problems[0]; p.Row != 5 || p.Column != "Price" || p.Message == "" {
		t.Errorf("first problem: %+v", p)
	}
	if p := plan.Problems[1]; p.Row != 6 || p.Column != "Name" {
		t.Errorf("second problem: %+v", p)
	}

	notAFile := importOf(t, m.sendFile("/api/v1/menu/import/preview", "text/csv", "PK\x03\x04\x00\x00", nil))
	if len(notAFile.Problems) != 1 || notAFile.Problems[0].Row != 0 || !strings.Contains(notAFile.Problems[0].Message, "CSV UTF-8") {
		t.Errorf("a spreadsheet sent as CSV: %+v", notAFile.Problems)
	}
}
