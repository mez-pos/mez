package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/service/menus"
)

// editorTill is the till with a menu editor and the ids an item can name.
type editorTill struct {
	*till
	owner                     map[string]string
	station, course, taxClass int64
}

func newEditorTill(t *testing.T) *editorTill {
	t.Helper()
	w := newTill(t)
	ctx := context.Background()
	m := &editorTill{till: w}
	if err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		if m.station, err = q.CreateStation(ctx, gen.CreateStationParams{VenueID: 1, Code: "kitchen", Name: "Kitchen", CreatedAt: seededAt, UpdatedAt: seededAt}); err != nil {
			return err
		}
		if m.course, err = q.CreateCourse(ctx, gen.CreateCourseParams{VenueID: 1, Number: 1, Name: "Mains", CreatedAt: seededAt, UpdatedAt: seededAt}); err != nil {
			return err
		}
		m.taxClass, err = q.CreateTaxClass(ctx, gen.CreateTaxClassParams{VenueID: 1, Name: "Food", Rate: 5000, CreatedAt: seededAt, UpdatedAt: seededAt})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	svc, err := menus.New(menus.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithMenuEditor(svc), WithMenus(w.db))
	m.owner = w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	return m
}

func (m *editorTill) itemBody(category int64, name string, price int64) string {
	return fmt.Sprintf(`{"category_id":%d,"name":%q,"price":{"minor":%d,"currency":"INR"},"hsn":"","tax_class_id":%d,"station_id":%d,"course_id":%d}`,
		category, name, price, m.taxClass, m.station, m.course)
}

// edit sends an edit as the owner and returns the id it answered with.
func (m *editorTill) edit(t *testing.T, method, path, body string, want int) int64 {
	t.Helper()
	rec := send(m.handler, method, path, body, m.owner)
	if rec.Code != want {
		t.Fatalf("%s %s: %d %s, want %d", method, path, rec.Code, rec.Body, want)
	}
	var out menuEditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return out.ID
}

func (m *editorTill) editor(t *testing.T) menus.Editor {
	t.Helper()
	rec := send(m.handler, http.MethodGet, "/api/v1/menu/editor", "", m.owner)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("editor: %d %s (cache %q)", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
	}
	var e menus.Editor
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestMenuEditorOverHTTP: the owner builds a menu, the terminal's menu
// follows at a new version, and a retired item leaves it.
func TestMenuEditorOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	drinks := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Drinks"}`, http.StatusCreated)
	dosa := m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(food, "Dosa", 12000), http.StatusCreated)
	lassi := m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(drinks, "Lassi", 9000), http.StatusCreated)

	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/items/%d", dosa), m.itemBody(food, "Masala dosa", 14000), http.StatusOK)
	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/categories/%d", drinks), `{"name":"Cold drinks"}`, http.StatusOK)
	m.edit(t, http.MethodPut, "/api/v1/menu/categories/order", fmt.Sprintf(`{"ids":[%d,%d]}`, drinks, food), http.StatusOK)
	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/categories/%d/items/order", food), fmt.Sprintf(`{"ids":[%d]}`, dosa), http.StatusOK)
	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/items/%d/retire", lassi), "", http.StatusOK)

	e := m.editor(t)
	if len(e.Categories) != 2 || e.Categories[0].Name != "Cold drinks" || e.Categories[1].ID != food {
		t.Errorf("categories = %+v", e.Categories)
	}
	if len(e.Items) != 2 {
		t.Fatalf("items = %+v; the retired one stays in the editor", e.Items)
	}

	// The terminal's menu: the new name and price, and no retired lassi.
	rec := send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if till.Version != e.Version || len(till.Items) != 1 || till.Items[0].Name != "Masala dosa" || till.Items[0].Price.Minor != 14000 {
		t.Errorf("till menu = version %d (editor %d), items %+v", till.Version, e.Version, till.Items)
	}

	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/items/%d/restore", lassi), "", http.StatusOK)
	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/categories/%d/retire", food), "", http.StatusOK)
	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/categories/%d/restore", food), "", http.StatusOK)
}

func TestMenuEditorRefusals(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"a nameless category", http.MethodPost, "/api/v1/menu/categories", `{"name":""}`, http.StatusUnprocessableEntity, CodeInvalidMenu, "name"},
		{"a price below zero", http.MethodPost, "/api/v1/menu/items", m.itemBody(food, "Dosa", -1), http.StatusUnprocessableEntity, CodeInvalidMenu, "price"},
		{"an item in no category", http.MethodPost, "/api/v1/menu/items", m.itemBody(999, "Dosa", 100), http.StatusUnprocessableEntity, CodeInvalidMenu, "category_id"},
		{"a missing item", http.MethodPut, "/api/v1/menu/items/999", m.itemBody(food, "Dosa", 100), http.StatusNotFound, CodeItemNotFound, ""},
		{"retiring a missing item", http.MethodPost, "/api/v1/menu/items/999/retire", "", http.StatusNotFound, CodeItemNotFound, ""},
		{"a missing category", http.MethodPut, "/api/v1/menu/categories/999", `{"name":"X"}`, http.StatusNotFound, CodeCategoryNotFound, ""},
		{"a stale category order", http.MethodPut, "/api/v1/menu/categories/order", `{"ids":[]}`, http.StatusConflict, CodeMenuOrderStale, ""},
		{"an id that is not one", http.MethodPut, "/api/v1/menu/items/abc", m.itemBody(food, "Dosa", 100), http.StatusBadRequest, CodeInvalidRequest, "id"},
		{"a body that is not JSON", http.MethodPost, "/api/v1/menu/categories", `{`, http.StatusBadRequest, CodeInvalidRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(m.handler, tt.method, tt.path, tt.body, m.owner)
			if rec.Code != tt.status {
				t.Fatalf("status %d %s, want %d", rec.Code, rec.Body, tt.status)
			}
			if got := errorOf(t, rec); got.Code != tt.code || got.Field != tt.field {
				t.Errorf("error = %+v, want %s on %q", got, tt.code, tt.field)
			}
		})
	}

	t.Run("a cashier may not edit the menu", func(t *testing.T) {
		cashier := m.asStaff(m.signIn(t, "2468").Token)
		for _, req := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/v1/menu/editor", ""},
			{http.MethodPost, "/api/v1/menu/categories", `{"name":"Mine"}`},
			{http.MethodPost, fmt.Sprintf("/api/v1/menu/categories/%d/retire", food), ""},
		} {
			rec := send(m.handler, req.method, req.path, req.body, cashier)
			if rec.Code != http.StatusForbidden || errorOf(t, rec).Field != "menu.edit" {
				t.Errorf("%s %s: %d %s, want 403 naming menu.edit", req.method, req.path, rec.Code, rec.Body)
			}
		}
	})
	t.Run("a device alone may not edit the menu", func(t *testing.T) {
		rec := send(m.handler, http.MethodGet, "/api/v1/menu/editor", "", m.asDevice())
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", rec.Code)
		}
	})
}

func TestWithoutAMenuEditorTheRoutesDoNotExist(t *testing.T) {
	w := newTill(t)
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	if rec := send(w.handler, http.MethodGet, "/api/v1/menu/editor", "", owner); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
