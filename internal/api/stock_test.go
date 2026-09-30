package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	domstock "github.com/byjoellobo/nofiltr-pos/internal/domain/stock"
	"github.com/byjoellobo/nofiltr-pos/internal/service/stock"
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
	if err := post("/api/v1/stock/items", `{"name":"Milk","unit":"ml","low_level":2000000}`, http.StatusCreated).Decode(&created); err != nil {
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
