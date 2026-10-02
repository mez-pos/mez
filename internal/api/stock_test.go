package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	domstock "github.com/byjoellobo/mez/internal/domain/stock"
	"github.com/byjoellobo/mez/internal/service/stock"
)

func newStockTill(t *testing.T) (*till, map[string]string) {
	t.Helper()
	w := newTill(t)
	svc, err := stock.New(w.db, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithStock(svc))
	return w, w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
}

// TestStockOverHTTP: the owner adds an item, receives it, counts it, and
// reads its history a page at a time; on hand follows the ledger.
func TestStockOverHTTP(t *testing.T) {
	w, owner := newStockTill(t)
	post := func(path, body string, want int) *json.Decoder {
		t.Helper()
		rec := send(w.handler, http.MethodPost, path, body, owner)
		if rec.Code != want {
			t.Fatalf("POST %s: %d %s, want %d", path, rec.Code, rec.Body, want)
		}
		return json.NewDecoder(rec.Body)
	}

	var created stockEditResponse
	if err := post("/api/v1/stock/items", `{"name":"Milk","unit":"ml","low_level":2000000,"cost":{"minor":6000,"currency":"INR"}}`, http.StatusCreated).Decode(&created); err != nil {
		t.Fatal(err)
	}
	moves := fmt.Sprintf("/api/v1/stock/items/%d/movements", created.ID)
	post(moves, `{"kind":"received","qty":6000000}`, http.StatusCreated)
	post(moves, `{"kind":"wasted","qty":500000,"reason":"expired"}`, http.StatusCreated)
	var counted stock.Recorded
	if err := post(moves, `{"kind":"counted","qty":5400000}`, http.StatusCreated).Decode(&counted); err != nil {
		t.Fatal(err)
	}
	if counted.Delta != -100_000 || counted.OnHand != 5_400_000 {
		t.Errorf("count = %+v; 5.5 litres on the ledger, 5.4 counted", counted)
	}

	rec := send(w.handler, http.MethodGet, "/api/v1/stock", "", owner)
	var list stockItemsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	if it := list.Items[0]; it.Name != "Milk" || it.Unit != domstock.Millilitre || it.OnHand != 5_400_000 || it.Low {
		t.Errorf("milk = %+v", it)
	}
	// 60.00 a litre, in the currency the list names.
	if it := list.Items[0]; list.Currency != "INR" || it.Cost.Minor != 6000 || it.Cost.Currency != "INR" {
		t.Errorf("milk costs %+v in a list costed in %q", it.Cost, list.Currency)
	}
	dollars := send(w.handler, http.MethodPost, "/api/v1/stock/items", `{"name":"Cream","unit":"ml","cost":{"minor":100,"currency":"USD"}}`, owner)
	if e := errorOf(t, dollars); dollars.Code < 400 || e.Code != CodeInvalidStock || e.Field != "cost" {
		t.Errorf("a cost in dollars: %d %s, want the cost refused", dollars.Code, dollars.Body)
	}

	rec = send(w.handler, http.MethodGet, moves+"?limit=2", "", owner)
	var page movementsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Movements) != 2 || page.NextBefore == 0 {
		t.Fatalf("page 1: %s", rec.Body)
	}
	if page.Movements[0].Kind != domstock.Counted || page.Movements[0].StaffName != "Priya" {
		t.Errorf("newest = %+v", page.Movements[0])
	}
	rec = send(w.handler, http.MethodGet, fmt.Sprintf("%s?limit=2&before=%d", moves, page.NextBefore), "", owner)
	page = movementsResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Movements) != 1 || page.NextBefore != 0 ||
		page.Movements[0].Kind != domstock.Received {
		t.Errorf("page 2: %s", rec.Body)
	}

	send(w.handler, http.MethodPut, fmt.Sprintf("/api/v1/stock/items/%d", created.ID), `{"name":"Whole milk","unit":"ml","low_level":6000000}`, owner)
	send(w.handler, http.MethodPost, fmt.Sprintf("/api/v1/stock/items/%d/retire", created.ID), "", owner)
	send(w.handler, http.MethodPost, fmt.Sprintf("/api/v1/stock/items/%d/restore", created.ID), "", owner)
	rec = send(w.handler, http.MethodGet, "/api/v1/stock", "", owner)
	list = stockItemsResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || list.Items[0].Name != "Whole milk" || !list.Items[0].Low || !list.Items[0].Active {
		t.Errorf("after edits: %s", rec.Body)
	}
}

func TestStockRefusals(t *testing.T) {
	w, owner := newStockTill(t)
	rec := send(w.handler, http.MethodPost, "/api/v1/stock/items", `{"name":"Sugar","unit":"g","low_level":0}`, owner)
	var sugar stockEditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sugar); err != nil {
		t.Fatal(err)
	}
	moves := fmt.Sprintf("/api/v1/stock/items/%d/movements", sugar.ID)

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"a unit we do not have", http.MethodPost, "/api/v1/stock/items", `{"name":"Salt","unit":"kg"}`, http.StatusUnprocessableEntity, CodeInvalidStock, "unit"},
		{"a name taken", http.MethodPost, "/api/v1/stock/items", `{"name":"SUGAR","unit":"g"}`, http.StatusConflict, CodeStockNameTaken, "name"},
		{"waste with no reason", http.MethodPost, moves, `{"kind":"wasted","qty":1}`, http.StatusUnprocessableEntity, CodeInvalidStock, "reason"},
		{"a sale by hand", http.MethodPost, moves, `{"kind":"sold","qty":1}`, http.StatusUnprocessableEntity, CodeInvalidStock, "kind"},
		{"a missing item", http.MethodPost, "/api/v1/stock/items/999/movements", `{"kind":"received","qty":1}`, http.StatusNotFound, CodeStockItemNotFound, ""},
		{"a missing item's history", http.MethodGet, "/api/v1/stock/items/999/movements", "", http.StatusNotFound, CodeStockItemNotFound, ""},
		{"a cursor that is not one", http.MethodGet, moves + "?before=soon", "", http.StatusBadRequest, CodeInvalidRequest, "before"},
		{"a negative limit", http.MethodGet, moves + "?limit=-1", "", http.StatusBadRequest, CodeInvalidRequest, "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, tt.method, tt.path, tt.body, owner)
			if rec.Code != tt.status {
				t.Fatalf("status %d %s, want %d", rec.Code, rec.Body, tt.status)
			}
			if got := errorOf(t, rec); got.Code != tt.code || got.Field != tt.field {
				t.Errorf("error = %+v, want %s on %q", got, tt.code, tt.field)
			}
		})
	}

	t.Run("a retired item takes no movements", func(t *testing.T) {
		send(w.handler, http.MethodPost, fmt.Sprintf("/api/v1/stock/items/%d/retire", sugar.ID), "", owner)
		rec := send(w.handler, http.MethodPost, moves, `{"kind":"received","qty":1}`, owner)
		if rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeStockItemRetired {
			t.Errorf("status %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("a cashier may not", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		rec := send(w.handler, http.MethodGet, "/api/v1/stock", "", cashier)
		if rec.Code != http.StatusForbidden || errorOf(t, rec).Field != "stock.manage" {
			t.Errorf("status %d %s", rec.Code, rec.Body)
		}
	})
}

func TestRecipesOverHTTP(t *testing.T) {
	w, owner := newStockTill(t)
	ctx := context.Background()
	if _, err := w.db.CreateMenu(ctx, 1, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food"}},
		Groups:     []db.ModifierGroupSpec{{Name: "Milk", MaxSelect: 1, Modifiers: []db.ModifierSpec{{Name: "Oat"}}}},
		Categories: []db.CategorySpec{{Name: "Coffee", Items: []db.ItemSpec{
			{Name: "Latte", Price: 18000, TaxClass: "Food", Station: "bar", Course: 1, Groups: []string{"Milk"}},
		}}},
	}, seededAt); err != nil {
		t.Fatal(err)
	}
	menu, err := w.db.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	latte, oat := menu.Items[0].ID, menu.Groups[0].Modifiers[0].ID

	rec := send(w.handler, http.MethodPost, "/api/v1/stock/items", `{"name":"Oat milk","unit":"ml"}`, owner)
	var milk stockEditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &milk); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"parts":[{"stock_item_id":%d,"qty":200000}]}`, milk.ID)
	for _, path := range []string{fmt.Sprintf("/api/v1/stock/recipes/items/%d", latte), fmt.Sprintf("/api/v1/stock/recipes/modifiers/%d", oat)} {
		if rec := send(w.handler, http.MethodPut, path, body, owner); rec.Code != http.StatusOK {
			t.Fatalf("PUT %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	rec = send(w.handler, http.MethodGet, "/api/v1/stock/recipes", "", owner)
	var book stock.RecipeBook
	if err := json.Unmarshal(rec.Body.Bytes(), &book); err != nil || len(book.Items) != 1 || len(book.Modifiers) != 1 ||
		book.Modifiers[0].ID != oat || book.Items[0].Parts[0].Qty != 200000 {
		t.Fatalf("recipes: %s", rec.Body)
	}

	tests := []struct {
		name, path, body string
		status           int
		code             ErrorCode
		field            string
	}{
		{"a missing menu item", "/api/v1/stock/recipes/items/999", body, http.StatusNotFound, CodeItemNotFound, ""},
		{"a missing modifier", "/api/v1/stock/recipes/modifiers/999", body, http.StatusNotFound, CodeModifierNotFound, ""},
		{"none of it", fmt.Sprintf("/api/v1/stock/recipes/items/%d", latte), fmt.Sprintf(`{"parts":[{"stock_item_id":%d,"qty":0}]}`, milk.ID),
			http.StatusUnprocessableEntity, CodeInvalidStock, "parts.0.qty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodPut, tt.path, tt.body, owner)
			if rec.Code != tt.status {
				t.Fatalf("status %d %s, want %d", rec.Code, rec.Body, tt.status)
			}
			if got := errorOf(t, rec); got.Code != tt.code || got.Field != tt.field {
				t.Errorf("error = %+v, want %s on %q", got, tt.code, tt.field)
			}
		})
	}
}

func TestClosingStockOverHTTP(t *testing.T) {
	w, owner := newStockTill(t)
	res, err := w.db.Writer().ExecContext(context.Background(),
		`INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at)
		 VALUES ('shf_HTTPCLS1', 1, 'open', '2026-09-30', 1, '2026-09-30 06:00:00+00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	shift, _ := res.LastInsertId()
	rec := send(w.handler, http.MethodPost, "/api/v1/stock/items", `{"name":"Milk","unit":"ml","low_level":0,"count_at_close":true}`, owner)
	var milk stockEditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &milk); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/shift/%d/stock-count", shift)

	rec = send(w.handler, http.MethodGet, path, "", owner)
	var sheet stock.ClosingSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &sheet); err != nil || sheet.Done || len(sheet.Items) != 1 || sheet.Items[0].ID != milk.ID {
		t.Fatalf("sheet: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "on_hand") {
		t.Error("the closing count is blind: the sheet must not carry on hand")
	}

	rec = send(w.handler, http.MethodPost, path, fmt.Sprintf(`{"counts":[{"stock_item_id":%d,"qty":3000000}]}`, milk.ID), owner)
	sheet = stock.ClosingSheet{}
	if err := json.Unmarshal(rec.Body.Bytes(), &sheet); err != nil || rec.Code != http.StatusCreated || !sheet.Done || sheet.Results[0].Delta != 3_000_000 {
		t.Fatalf("count: %d %s", rec.Code, rec.Body)
	}

	refusals := []struct {
		name, method, path, body string
		headers                  map[string]string
		status                   int
		code                     ErrorCode
	}{
		{"counting again", http.MethodPost, path, fmt.Sprintf(`{"counts":[{"stock_item_id":%d,"qty":0}]}`, milk.ID), owner, http.StatusConflict, CodeStockCounted},
		{"a missing shift", http.MethodGet, "/api/v1/shift/999/stock-count", "", owner, http.StatusNotFound, CodeShiftUnknown},
		{"a cashier", http.MethodGet, path, "", w.asStaff(w.signIn(t, "2468").Token), http.StatusForbidden, CodeForbidden},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, tt.method, tt.path, tt.body, tt.headers)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("%d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}

func TestTheClosingStockReportOverHTTP(t *testing.T) {
	w, owner := newStockTill(t)
	const base = "/api/v1/reports/closing-stock"

	rec := send(w.handler, http.MethodGet, base+"?from=2026-09-01&to=2026-09-30", "", owner)
	var got stock.ClosingReport
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("report: %d %s", rec.Code, rec.Body)
	}
	if got.From != "2026-09-01" || got.Currency != "INR" || got.Shifts == nil || got.Items == nil {
		t.Errorf("a quiet September: %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the report may be cached")
	}

	rec = send(w.handler, http.MethodGet, base+"/csv?from=2026-09-01&to=2026-09-30", "", owner)
	var file csvResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &file) != nil {
		t.Fatalf("csv: %d %s", rec.Code, rec.Body)
	}
	if file.Filename != "closing-stock-2026-09-01-to-2026-09-30.csv" || !strings.HasPrefix(file.Text, byteOrderMark+"Trading day,Shift,Item,Unit,Expected,Counted,Variance,Cost (INR),") {
		t.Errorf("file %q: %q", file.Filename, file.Text)
	}

	// It is a report: a cashier, who cannot read the Z-report, cannot read it.
	cashier := w.asStaff(w.signIn(t, "2468").Token)
	refusals := []struct {
		name, path string
		headers    map[string]string
		status     int
		code       ErrorCode
	}{
		{"no range", base, owner, http.StatusBadRequest, CodeInvalidFilter},
		{"backwards", base + "?from=2026-09-30&to=2026-09-01", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"over a year", base + "?from=2025-01-01&to=2026-09-01", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a cashier", base + "?from=2026-09-01&to=2026-09-30", cashier, http.StatusForbidden, CodeForbidden},
		{"a file of no range", base + "/csv", owner, http.StatusBadRequest, CodeInvalidFilter},
		{"a file for a cashier", base + "/csv?from=2026-09-01&to=2026-09-30", cashier, http.StatusForbidden, CodeForbidden},
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
