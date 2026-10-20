package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// menuTill is a paired till whose venue has a coffee menu.
func menuTill(t *testing.T) *till {
	t.Helper()
	w := newTill(t)
	_, err := w.db.CreateMenu(context.Background(), 1, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food 5%", Rate: tax.RateFromBP(500)}},
		Groups: []db.ModifierGroupSpec{{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
			{Name: "Dairy", IsDefault: true}, {Name: "Oat", Price: 4000},
		}}},
		Categories: []db.CategorySpec{{Name: "Coffee", Items: []db.ItemSpec{
			{Name: "Flat white", Price: 18000, HSN: "996331", TaxClass: "Food 5%", Station: "bar", Course: 1, Groups: []string{"Milk"}},
		}}},
	}, seededAt)
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	w.rebuild(t, WithMenus(w.db))
	return w
}

func TestReadTheMenu(t *testing.T) {
	w := menuTill(t)

	rec := send(w.handler, http.MethodGet, "/api/v1/menu", "", w.asDevice())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("ETag"); got != `"menu-2"` {
		t.Errorf("ETag = %q", got)
	}
	var m menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if m.Version != 2 || m.Currency != "INR" || len(m.Items) != 1 || len(m.ModifierGroups) != 1 {
		t.Fatalf("menu = %+v", m)
	}
	item := m.Items[0]
	if item.Station != "bar" || item.Course != 1 || item.TaxRate != tax.RateFromBP(500) ||
		item.Price.Minor != 18000 || item.HSN != "996331" || len(item.ModifierGroups) != 1 {
		t.Errorf("item = %+v", item)
	}

	// The wire shape a terminal reads: snake_case, lists never null.
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, key := range []string{"stations", "courses", "categories", "items", "modifier_groups"} {
		if _, ok := raw[key].([]any); !ok {
			t.Errorf("%s = %v, want a list", key, raw[key])
		}
	}
}

func TestTheMenuAnswersNotModified(t *testing.T) {
	w := menuTill(t)
	tests := []struct {
		name   string
		etag   string
		status int
	}{
		{"current", `"menu-2"`, http.StatusNotModified},
		{"stale", `"menu-1"`, http.StatusOK},
		{"nonsense", `"lunch"`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := w.asDevice()
			h["If-None-Match"] = tt.etag
			rec := send(w.handler, http.MethodGet, "/api/v1/menu", "", h)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d", rec.Code, tt.status)
			}
			if rec.Header().Get("ETag") != `"menu-2"` {
				t.Errorf("ETag = %q", rec.Header().Get("ETag"))
			}
			if tt.status == http.StatusNotModified && rec.Body.Len() != 0 {
				t.Errorf("a 304 with a body: %s", rec.Body)
			}
		})
	}
}

func TestTheMenuIsGuarded(t *testing.T) {
	w := menuTill(t)
	tests := []struct {
		name   string
		method string
		header map[string]string
		status int
		code   ErrorCode
	}{
		{"no device", http.MethodGet, nil, http.StatusUnauthorized, CodeUnauthenticated},
		{"a bad token", http.MethodGet, bearerOf("nope"), http.StatusUnauthorized, CodeUnauthenticated},
		{"a write", http.MethodPost, w.asDevice(), http.StatusMethodNotAllowed, CodeMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, tt.method, "/api/v1/menu", "", tt.header)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d", rec.Code, tt.status)
			}
			if got := errorOf(t, rec).Code; got != tt.code {
				t.Errorf("code %q, want %q", got, tt.code)
			}
		})
	}
}

type brokenMenus struct{}

func (brokenMenus) Menu(context.Context, int64) (menu.Menu, error) {
	return menu.Menu{}, errors.New("disk on fire")
}
func (brokenMenus) MenuVersion(context.Context, int64) (int64, error) {
	return 0, errors.New("disk on fire")
}

func TestAMenuThatCannotBeReadIsAServerError(t *testing.T) {
	w := newTill(t)
	w.rebuild(t, WithMenus(brokenMenus{}))
	for _, etag := range []string{"", `"menu-1"`} {
		h := w.asDevice()
		if etag != "" {
			h["If-None-Match"] = etag
		}
		rec := send(w.handler, http.MethodGet, "/api/v1/menu", "", h)
		if rec.Code != http.StatusInternalServerError || errorOf(t, rec).Code != CodeInternal {
			t.Errorf("etag %q: status %d, body %s", etag, rec.Code, rec.Body)
		}
	}
	if !strings.Contains(w.log.String(), "reading the menu failed") {
		t.Error("the failure was not logged")
	}
}

func TestWithoutMenusTheRouteDoesNotExist(t *testing.T) {
	w := newTill(t)
	rec := send(w.handler, http.MethodGet, "/api/v1/menu", "", w.asDevice())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}
