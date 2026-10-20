package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/service/menus"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// pricedTill is the editor's till with a menu on it: Dosa at 140.00 and
// Idli at 80.50 in Food, Filter coffee at 40.00 in Drinks.
func pricedTill(t *testing.T) *editorTill {
	t.Helper()
	m := newEditorTill(t)
	importOf(t, m.sendFile("/api/v1/menu/import", "text/csv", importSheet, nil))
	return m
}

const priceSheet = "Category,Name,Price\nFood,Masala dosa,150\nFood,Idli,80.50\nDrinks,Filter coffee,45\nDrinks,Lassi,60\n"

func pricesOf(t *testing.T, rec *httptest.ResponseRecorder) menus.Prices {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s, want 200", rec.Code, rec.Body)
	}
	var out menus.Prices
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func applyPath(version int64) string { return fmt.Sprintf("/api/v1/menu/prices?version=%d", version) }

// TestMenuPricesOverHTTP: the owner checks a file of prices, which changes
// nothing, then applies it, and the terminal's menu has them at a new
// version.
func TestMenuPricesOverHTTP(t *testing.T) {
	m := pricedTill(t)
	before := m.editor(t).Version

	rec := m.sendFile("/api/v1/menu/prices/preview", "text/csv; charset=utf-8", priceSheet, nil)
	plan := pricesOf(t, rec)
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("a preview may be cached: %q", rec.Header().Get("Cache-Control"))
	}
	if plan.Applied || plan.Version != before || len(plan.Changes) != 2 || plan.Unchanged != 1 || len(plan.Unknown) != 1 || len(plan.Problems) != 0 {
		t.Fatalf("preview: %+v", plan)
	}
	if !strings.Contains(rec.Body.String(), `"category":"Food","name":"Masala dosa","old":{"minor":14000,"currency":"INR"},"new":{"minor":15000,"currency":"INR"}`) {
		t.Errorf("the first change as sent: %s", rec.Body)
	}
	if e := m.editor(t); e.Items[0].Price.Minor != 14000 || e.Version != before {
		t.Fatalf("a preview changed the menu: %+v at version %d", e.Items[0].Price, e.Version)
	}

	done := pricesOf(t, m.sendFile(applyPath(plan.Version), "text/csv", priceSheet, nil))
	if !done.Applied || len(done.Changes) != 2 {
		t.Fatalf("applied: %+v", done)
	}
	e := m.editor(t)
	if e.Version != before+1 || len(e.Items) != 3 || e.Items[0].Price.Minor != 15000 || e.Items[1].Price.Minor != 8050 || e.Items[2].Price.Minor != 4500 {
		t.Fatalf("after: version %d (was %d), items %+v", e.Version, before, e.Items)
	}

	rec = send(m.handler, http.MethodGet, "/api/v1/menu", "", m.asDevice())
	var till menuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &till); err != nil {
		t.Fatal(err)
	}
	if till.Version != e.Version || till.Items[0].Price.Minor != 15000 {
		t.Errorf("the terminal's menu: version %d, first price %+v", till.Version, till.Items[0].Price)
	}

	// The same file again, at the new version: nothing left to change.
	again := pricesOf(t, m.sendFile(applyPath(e.Version), "text/csv", priceSheet, nil))
	if again.Applied || len(again.Changes) != 0 || again.Unchanged != 3 {
		t.Errorf("the same file again: %+v", again)
	}
	if got := m.editor(t).Version; got != e.Version {
		t.Errorf("version %d after the same file again, was %d", got, e.Version)
	}
}

func TestMenuPricesRefusals(t *testing.T) {
	m := pricedTill(t)
	version := m.editor(t).Version
	cashier := m.asStaff(m.signInAs(t, "stf_ASHA0001", "2468").Token)
	bad := priceSheet + "Food,Idli,lots\n"
	tooBig := "Category,Name,Price\n" + strings.Repeat("x", menus.MaxImportBytes)
	apply := applyPath(version)

	tests := []struct {
		name, path, contentType, body string
		as                            map[string]string
		status                        int
		code                          ErrorCode
		field                         string
	}{
		{"a file with a problem", apply, "text/csv", bad, nil, http.StatusUnprocessableEntity, CodeMenuPricesRefused, "file"},
		{"a menu that has moved on", applyPath(version - 1), "text/csv", priceSheet, nil, http.StatusConflict, CodeMenuPricesStale, ""},
		{"no version", "/api/v1/menu/prices", "text/csv", priceSheet, nil, http.StatusBadRequest, CodeInvalidRequest, "version"},
		{"a version that is not a number", "/api/v1/menu/prices?version=latest", "text/csv", priceSheet, nil, http.StatusBadRequest, CodeInvalidRequest, "version"},
		{"JSON, not a file", apply, "application/json", `{}`, nil, http.StatusUnsupportedMediaType, CodeInvalidMenu, "file"},
		{"a file too large to apply", apply, "text/csv", tooBig, nil, http.StatusRequestEntityTooLarge, CodeInvalidMenu, "file"},
		{"a file too large to check", "/api/v1/menu/prices/preview", "text/csv", tooBig, nil, http.StatusRequestEntityTooLarge, CodeInvalidMenu, "file"},
		{"a cashier applying", apply, "text/csv", priceSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"a cashier checking", "/api/v1/menu/prices/preview", "text/csv", priceSheet, cashier, http.StatusForbidden, CodeForbidden, ""},
		{"no one signed in", apply, "text/csv", priceSheet, m.asDevice(), http.StatusUnauthorized, "", ""},
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
	if e := m.editor(t); e.Items[0].Price.Minor != 14000 || e.Version != version {
		t.Errorf("a refusal changed the menu: %+v at version %d (was %d)", e.Items[0].Price, e.Version, version)
	}
}

// TestMenuPricesPreviewSaysWhatIsWrong: the preview of a bad file is a 200
// whose answer is the problems.
func TestMenuPricesPreviewSaysWhatIsWrong(t *testing.T) {
	m := pricedTill(t)
	plan := pricesOf(t, m.sendFile("/api/v1/menu/prices/preview", "text/csv", "Category,Name,Price\nFood,Masala dosa,150\nFood,Idli,lots\n", nil))
	if len(plan.Changes) != 1 || len(plan.Problems) != 1 || plan.Problems[0].Row != 3 || plan.Problems[0].Column != "Price" || plan.Problems[0].Message == "" {
		t.Errorf("changes %+v, problems %+v", plan.Changes, plan.Problems)
	}
}

// TestAnOpenOrderKeepsThePriceItWasRungUpAt: prices changed from a file
// reach the next line rung up and no line already on an order
// (docs/02-domain.md, "Prices and the snapshot rule").
func TestAnOpenOrderKeepsThePriceItWasRungUpAt(t *testing.T) {
	m := pricedTill(t)
	ctx := context.Background()
	if err := m.db.WriteTx(ctx, func(q *gen.Queries) error { return seedTables(ctx, q, 1) }); err != nil {
		t.Fatal(err)
	}
	m.openTestShift(t)
	orders, err := svcorder.New(svcorder.Options{DB: m.db})
	if err != nil {
		t.Fatal(err)
	}
	photos, err := menus.NewPhotoStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := menus.New(menus.Options{DB: m.db, Photos: photos})
	if err != nil {
		t.Fatal(err)
	}
	m.rebuild(t, WithMenuEditor(svc), WithMenus(m.db), WithOrders(orders))
	m.owner = m.asStaff(m.signInAs(t, "stf_PRIYA001", "2468").Token)

	dosa := m.editor(t).Items[0]
	order := func(method, path, body string) commandResponse {
		t.Helper()
		rec := send(m.handler, method, path, body, m.owner)
		if rec.Code >= 400 {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body)
		}
		var out commandResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	line := func(price int64) string {
		return fmt.Sprintf(`{"item_id":%d,"name":"Masala dosa","qty":1,"unit_price":{"minor":%d,"currency":"INR"},"station":"kitchen","course":1,"tax_rate":5000}`, dosa.ID, price)
	}
	id := order(http.MethodPost, "/api/v1/orders", `{"channel":"counter","table_id":4,"covers":2}`).Order.ID
	before := order(http.MethodPost, "/api/v1/orders/"+id+"/lines", line(14000))
	if len(before.Order.Lines) != 1 || before.Order.Lines[0].UnitPrice.Minor != 14000 {
		t.Fatalf("the line as rung up: %+v", before.Order.Lines)
	}

	done := pricesOf(t, m.sendFile(applyPath(m.editor(t).Version), "text/csv", priceSheet, nil))
	if !done.Applied {
		t.Fatalf("applied: %+v", done)
	}

	// The next line is rung up at the new price, beside the old one at the old.
	after := order(http.MethodPost, "/api/v1/orders/"+id+"/lines", line(15000))
	if len(after.Order.Lines) != 2 || after.Order.Lines[0].UnitPrice.Minor != 14000 || after.Order.Lines[1].UnitPrice.Minor != 15000 {
		t.Errorf("lines after the prices changed: %+v", after.Order.Lines)
	}
}
