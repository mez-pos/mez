package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/service/menus"
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
	photos, err := menus.NewPhotoStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := menus.New(menus.Options{DB: w.db, Photos: photos})
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

// TestItemCardFieldsOverHTTP: the description, dietary mark and flags the
// owner saves reach the editor and the terminal's menu (ADR-147), and a bad
// one is refused on its own field.
func TestItemCardFieldsOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	body := fmt.Sprintf(`{"category_id":%d,"name":"Masala dosa","price":{"minor":14000,"currency":"INR"},"hsn":"","tax_class_id":%d,`+
		`"station_id":%d,"course_id":%d,"description":"Crisp rice crepe.","diet":"veg","gluten_free":true,"popular":true}`,
		food, m.taxClass, m.station, m.course)
	id := m.edit(t, http.MethodPost, "/api/v1/menu/items", body, http.StatusCreated)

	editor := m.editor(t)
	if len(editor.Items) != 1 || editor.Items[0].Description != "Crisp rice crepe." || editor.Items[0].Diet != "veg" ||
		!editor.Items[0].GlutenFree || !editor.Items[0].Popular {
		t.Fatalf("editor items = %+v", editor.Items)
	}
	rec := send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if len(till.Items) != 1 || till.Items[0].Description != "Crisp rice crepe." || till.Items[0].Diet != "veg" ||
		!till.Items[0].GlutenFree || !till.Items[0].Popular {
		t.Errorf("till items = %+v", till.Items)
	}

	bad := []struct{ name, from, to, field string }{
		{"a mark that does not exist", `"diet":"veg"`, `"diet":"keto"`, "diet"},
		{"a description past 140", `"description":"Crisp rice crepe."`, `"description":"` + strings.Repeat("a", 141) + `"`, "description"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(m.handler, http.MethodPut, fmt.Sprintf("/api/v1/menu/items/%d", id), strings.Replace(body, tt.from, tt.to, 1), m.owner)
			if got := errorOf(t, rec); rec.Code != http.StatusUnprocessableEntity || got.Code != CodeInvalidMenu || got.Field != tt.field {
				t.Errorf("%d %+v, want 422 invalid_menu on %q", rec.Code, got, tt.field)
			}
		})
	}
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

// TestModifierGroupsOverHTTP: the owner builds a group, offers it on an
// item, and the terminal's menu carries it; refusals name their row.
func TestModifierGroupsOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Coffee"}`, http.StatusCreated)
	latte := m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(food, "Latte", 18000), http.StatusCreated)
	milk := m.edit(t, http.MethodPost, "/api/v1/menu/groups",
		`{"name":"Milk","min_select":1,"max_select":1,"modifiers":[`+
			`{"name":"Dairy","price":{"minor":0,"currency":"INR"},"is_default":true},`+
			`{"name":"Oat","price":{"minor":4000,"currency":"INR"},"is_default":false}]}`, http.StatusCreated)
	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/items/%d/groups", latte), fmt.Sprintf(`{"ids":[%d]}`, milk), http.StatusOK)

	e := m.editor(t)
	if len(e.Groups) != 1 || len(e.Groups[0].Modifiers) != 2 || e.Groups[0].Modifiers[1].Price.Minor != 4000 {
		t.Fatalf("editor groups = %+v", e.Groups)
	}
	oat := e.Groups[0].Modifiers[1].ID

	rec := send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if len(till.Items) != 1 || len(till.Items[0].ModifierGroups) != 1 || till.Items[0].ModifierGroups[0] != milk || len(till.ModifierGroups) != 1 {
		t.Errorf("till = items %+v, groups %+v", till.Items, till.ModifierGroups)
	}

	// Drop dairy, keep oat by id: one choice left, and min 1 still met.
	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/groups/%d", milk),
		fmt.Sprintf(`{"name":"Milk","min_select":1,"max_select":1,"modifiers":[{"id":%d,"name":"Oat","price":{"minor":4500,"currency":"INR"},"is_default":true}]}`, oat),
		http.StatusOK)
	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/groups/%d/retire", milk), "", http.StatusOK)
	m.edit(t, http.MethodPost, fmt.Sprintf("/api/v1/menu/groups/%d/restore", milk), "", http.StatusOK)
	if g := m.editor(t).Groups[0]; len(g.Modifiers) != 1 || g.Modifiers[0].ID != oat || g.Modifiers[0].Price.Minor != 4500 || !g.Active {
		t.Errorf("after edits = %+v", g)
	}

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"asks for more than it offers", http.MethodPost, "/api/v1/menu/groups",
			`{"name":"X","min_select":2,"max_select":2,"modifiers":[{"name":"A","price":{"minor":0,"currency":"INR"},"is_default":false}]}`,
			http.StatusUnprocessableEntity, CodeInvalidMenu, "min_select"},
		{"a nameless choice", http.MethodPost, "/api/v1/menu/groups",
			`{"name":"X","min_select":0,"max_select":1,"modifiers":[{"name":"","price":{"minor":0,"currency":"INR"},"is_default":false}]}`,
			http.StatusUnprocessableEntity, CodeInvalidMenu, "modifiers.0.name"},
		{"a missing group", http.MethodPost, "/api/v1/menu/groups/999/retire", "", http.StatusNotFound, CodeGroupNotFound, ""},
		{"a group twice on an item", http.MethodPut, fmt.Sprintf("/api/v1/menu/items/%d/groups", latte), fmt.Sprintf(`{"ids":[%d,%d]}`, milk, milk),
			http.StatusUnprocessableEntity, CodeInvalidMenu, "ids"},
		{"groups on a missing item", http.MethodPut, "/api/v1/menu/items/999/groups", `{"ids":[]}`, http.StatusNotFound, CodeItemNotFound, ""},
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

	t.Run("a cashier may not", func(t *testing.T) {
		cashier := m.asStaff(m.signIn(t, "2468").Token)
		rec := send(m.handler, http.MethodPost, fmt.Sprintf("/api/v1/menu/groups/%d/retire", milk), "", cashier)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status %d, want 403", rec.Code)
		}
	})
}

func TestSoldOutOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	dosa := m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(food, "Dosa", 12000), http.StatusCreated)

	rec := send(m.handler, http.MethodPost, fmt.Sprintf("/api/v1/menu/items/%d/sold-out", dosa), "", m.owner)
	var out soldOutResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.SoldOutUntil == nil {
		t.Fatalf("sold out: %d %s", rec.Code, rec.Body)
	}
	rec = send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if len(till.Items) != 1 || till.Items[0].SoldOutUntil == nil || !till.Items[0].SoldOutUntil.Equal(*out.SoldOutUntil) {
		t.Errorf("till items = %+v", till.Items)
	}

	rec = send(m.handler, http.MethodPost, fmt.Sprintf("/api/v1/menu/items/%d/on-sale", dosa), "", m.owner)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "sold_out_until") {
		t.Errorf("on sale: %d %s", rec.Code, rec.Body)
	}
	rec = send(m.handler, http.MethodPost, "/api/v1/menu/items/999/sold-out", "", m.owner)
	if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeItemNotFound {
		t.Errorf("missing item: %d %s", rec.Code, rec.Body)
	}
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestItemPhotosOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	food := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	dosa := m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(food, "Dosa", 12000), http.StatusCreated)
	path := fmt.Sprintf("/api/v1/menu/items/%d/photo", dosa)
	data := testJPEG(t, 480, 360)

	upload := func(body []byte, contentType string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		m.handler.ServeHTTP(rec, req)
		return rec
	}

	rec := upload(data, "image/jpeg", m.owner)
	var out photoResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Photo) != 68 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}

	// The terminal's menu names it, and it is served to anyone, for good.
	rec = send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil || till.Items[0].Photo != out.Photo {
		t.Fatalf("till menu: %s (%v)", rec.Body, err)
	}
	rec = send(m.handler, http.MethodGet, "/api/v1/menu/photos/"+out.Photo, "", nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("serve: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("serve headers = %v", rec.Header())
	}

	refusals := []struct {
		name        string
		body        []byte
		contentType string
		headers     map[string]string
		status      int
	}{
		{"not a JPEG content type", data, "image/png", m.owner, http.StatusUnsupportedMediaType},
		{"over the byte limit", make([]byte, menus.MaxPhotoBytes+10), "image/jpeg", m.owner, http.StatusRequestEntityTooLarge},
		{"not a JPEG at all", []byte("hello"), "image/jpeg", m.owner, http.StatusUnprocessableEntity},
		{"a device alone", data, "image/jpeg", m.asDevice(), http.StatusUnauthorized},
		{"a cashier", data, "image/jpeg", m.asStaff(m.signIn(t, "2468").Token), http.StatusForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			if rec := upload(tt.body, tt.contentType, tt.headers); rec.Code != tt.status {
				t.Errorf("status %d %s, want %d", rec.Code, rec.Body, tt.status)
			}
		})
	}

	for _, name := range []string{"..%2Fmez.db", "nothing.jpg", strings.Repeat("0", 64) + ".jpg"} {
		if rec := send(m.handler, http.MethodGet, "/api/v1/menu/photos/"+name, "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET photo %q: %d, want 404", name, rec.Code)
		}
	}

	m.edit(t, http.MethodDelete, path, "", http.StatusOK)
	if e := m.editor(t); e.Items[0].Photo != "" {
		t.Errorf("after delete, photo = %q", e.Items[0].Photo)
	}
}

// TestStationsAndCoursesOverHTTP: the owner adds, renames, orders, retires and
// restores stations and courses; a retire says where items go, and answers
// with what moved.
func TestStationsAndCoursesOverHTTP(t *testing.T) {
	m := newEditorTill(t)

	cat := m.edit(t, http.MethodPost, "/api/v1/menu/categories", `{"name":"Food"}`, http.StatusCreated)
	m.edit(t, http.MethodPost, "/api/v1/menu/items", m.itemBody(cat, "Dosa", 12000), http.StatusCreated)

	bar := m.edit(t, http.MethodPost, "/api/v1/menu/stations", `{"name":" Juice Bar "}`, http.StatusCreated)
	starters := m.edit(t, http.MethodPost, "/api/v1/menu/courses", `{"name":"Starters"}`, http.StatusCreated)
	e := m.editor(t)
	if len(e.Stations) != 2 || e.Stations[1].Code != "juice-bar" || e.Stations[1].Name != "Juice Bar" ||
		len(e.Courses) != 2 || e.Courses[1].Number != 2 || !e.Courses[1].Active {
		t.Fatalf("stations %+v, courses %+v", e.Stations, e.Courses)
	}

	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/stations/%d", bar), `{"name":"Juices"}`, http.StatusOK)
	m.edit(t, http.MethodPut, fmt.Sprintf("/api/v1/menu/courses/%d", starters), `{"name":"Small plates"}`, http.StatusOK)
	m.edit(t, http.MethodPut, "/api/v1/menu/stations/order", fmt.Sprintf(`{"ids":[%d,%d]}`, bar, m.station), http.StatusOK)
	m.edit(t, http.MethodPut, "/api/v1/menu/courses/order", fmt.Sprintf(`{"ids":[%d,%d]}`, starters, m.course), http.StatusOK)
	e = m.editor(t)
	if e.Stations[0].ID != bar || e.Stations[0].Name != "Juices" || e.Courses[0].ID != starters || e.Courses[0].Name != "Small plates" || e.Courses[0].Number != 1 {
		t.Fatalf("after renaming and ordering: stations %+v, courses %+v", e.Stations, e.Courses)
	}

	retire := func(path, body string) (int, map[string]any, Error) {
		t.Helper()
		rec := send(m.handler, http.MethodPost, path, body, m.owner)
		if rec.Code >= 400 {
			return rec.Code, nil, errorOf(t, rec)
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}
	stationPath := func(id int64, verb string) string { return fmt.Sprintf("/api/v1/menu/stations/%d/%s", id, verb) }
	coursePath := func(id int64, verb string) string { return fmt.Sprintf("/api/v1/menu/courses/%d/%s", id, verb) }

	// The kitchen has an item: it cannot go without saying where.
	status, _, apiErr := retire(stationPath(m.station, "retire"), "")
	if status != 422 || apiErr.Code != CodeInvalidMenu || apiErr.Field != "move_to" {
		t.Errorf("a station with items and no destination: %d %s %q", status, apiErr.Code, apiErr.Field)
	}
	status, out, _ := retire(stationPath(m.station, "retire"), fmt.Sprintf(`{"move_to":%d}`, bar))
	if status != 200 || out["items_moved"] != float64(1) || out["printer_moved"] != false || out["id"] != float64(m.station) {
		t.Errorf("retiring the kitchen: %d %v", status, out)
	}
	status, out, _ = retire(coursePath(m.course, "retire"), fmt.Sprintf(`{"move_to":%d}`, starters))
	if status != 200 || out["items_moved"] != float64(1) {
		t.Errorf("retiring mains: %d %v", status, out)
	}
	e = m.editor(t)
	if e.Stations[1].Active || e.Courses[1].Active || e.Items[0].StationID != bar || e.Items[0].CourseID != starters {
		t.Errorf("after retiring: stations %+v, courses %+v, item %+v", e.Stations, e.Courses, e.Items[0])
	}

	// Each venue keeps one in service, and an unknown field is not ignored.
	if status, _, apiErr = retire(stationPath(bar, "retire"), ""); status != 422 || apiErr.Field != "id" {
		t.Errorf("the only station: %d %s %q", status, apiErr.Code, apiErr.Field)
	}
	if status, _, apiErr = retire(stationPath(bar, "retire"), `{"move_to":1,"printer":2}`); status != 400 || apiErr.Code != CodeInvalidRequest {
		t.Errorf("an unknown field: %d %s", status, apiErr.Code)
	}

	m.edit(t, http.MethodPost, stationPath(m.station, "restore"), "", http.StatusOK)
	m.edit(t, http.MethodPost, coursePath(m.course, "restore"), "", http.StatusOK)
	if e = m.editor(t); !e.Stations[1].Active || !e.Courses[1].Active {
		t.Errorf("after restoring: stations %+v, courses %+v", e.Stations, e.Courses)
	}
}

func TestStationAndCourseRefusalsOverHTTP(t *testing.T) {
	m := newEditorTill(t)
	refusals := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"a code in use", "POST", "/api/v1/menu/stations", `{"code":"kitchen","name":"Again"}`, 422, CodeInvalidMenu, "code"},
		{"a bad code", "POST", "/api/v1/menu/stations", `{"code":"Juice Bar","name":"Juice"}`, 422, CodeInvalidMenu, "code"},
		{"no station name", "POST", "/api/v1/menu/stations", `{"name":""}`, 422, CodeInvalidMenu, "name"},
		{"no course name", "POST", "/api/v1/menu/courses", `{"name":""}`, 422, CodeInvalidMenu, "name"},
		{"renaming a missing station", "PUT", "/api/v1/menu/stations/999", `{"name":"X"}`, 404, CodeStationNotFound, ""},
		{"renaming a missing course", "PUT", "/api/v1/menu/courses/999", `{"name":"X"}`, 404, CodeCourseNotFound, ""},
		{"retiring a missing station", "POST", "/api/v1/menu/stations/999/retire", "", 404, CodeStationNotFound, ""},
		{"restoring a missing course", "POST", "/api/v1/menu/courses/999/restore", "", 404, CodeCourseNotFound, ""},
		{"a stale station order", "PUT", "/api/v1/menu/stations/order", `{"ids":[]}`, 409, CodeMenuOrderStale, ""},
		{"a stale course order", "PUT", "/api/v1/menu/courses/order", `{"ids":[1,1]}`, 409, CodeMenuOrderStale, ""},
		{"a station id that is not a number", "PUT", "/api/v1/menu/stations/x", `{"name":"X"}`, 400, CodeInvalidRequest, "id"},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(m.handler, tt.method, tt.path, tt.body, m.owner)
			apiErr := errorOf(t, rec)
			if rec.Code != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", rec.Code, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}

	t.Run("courses cannot be renumbered under an open ticket", func(t *testing.T) {
		starters := m.edit(t, http.MethodPost, "/api/v1/menu/courses", `{"name":"Starters"}`, http.StatusCreated)
		if _, err := m.db.Writer().ExecContext(context.Background(),
			`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
			 VALUES ('ord_COURSE01', 1, 'counter', 'open', 1, ?, ?)`, seededAt, seededAt); err != nil {
			t.Fatal(err)
		}
		rec := send(m.handler, http.MethodPut, "/api/v1/menu/courses/order", fmt.Sprintf(`{"ids":[%d,%d]}`, starters, m.course), m.owner)
		if apiErr := errorOf(t, rec); rec.Code != 409 || apiErr.Code != CodeTicketsOpen {
			t.Errorf("%d %s", rec.Code, apiErr.Code)
		}
	})

	t.Run("a cashier cannot", func(t *testing.T) {
		cashier := m.asStaff(m.signIn(t, "2468").Token)
		for _, path := range []string{"/api/v1/menu/stations", "/api/v1/menu/courses"} {
			if rec := send(m.handler, http.MethodPost, path, `{"name":"X"}`, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("POST %s: %d", path, rec.Code)
			}
		}
	})
}
