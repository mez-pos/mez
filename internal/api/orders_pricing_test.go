package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// tillMenu is the menu a register test's till rings from: a Menus that a
// test writes items into. The server prices every line a till adds from
// it (ADR-256), never from the request.
//
// Most order tests are not about pricing: they ring a line with the price
// their totals assume. With echo on, a line a test posts first puts its
// item on the menu as the line describes it, so those tests still mean what
// they say. A test about pricing turns echo off and stocks the menu itself.
type tillMenu struct {
	mu   sync.Mutex
	menu menu.Menu
	echo bool
}

func newTillMenu() *tillMenu {
	return &tillMenu{menu: menu.Menu{Version: 1, Currency: "INR"}, echo: true}
}

func (m *tillMenu) Menu(context.Context, int64) (menu.Menu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.menu, nil
}

func (m *tillMenu) MenuVersion(context.Context, int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.menu.Version, nil
}

// stock puts an item and its groups on the menu, in place of any with the
// same ids. The menu is replaced, never edited, so a menu already handed to
// a request does not change under it.
func (m *tillMenu) stock(item menu.Item, groups ...menu.ModifierGroup) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]menu.Item, 0, len(m.menu.Items)+1)
	for _, it := range m.menu.Items {
		if it.ID != item.ID {
			items = append(items, it)
		}
	}
	items = append(items, item)
	replaced := make(map[int64]bool, len(groups))
	for _, g := range groups {
		replaced[g.ID] = true
	}
	kept := make([]menu.ModifierGroup, 0, len(m.menu.Groups)+len(groups))
	for _, g := range m.menu.Groups {
		if !replaced[g.ID] {
			kept = append(kept, g)
		}
	}
	next := m.menu
	next.Version++
	next.Items = items
	next.Groups = append(kept, groups...)
	m.menu = next
}

// stopEcho is for a test that stocks the menu itself: a line it posts is
// priced from what it stocked, and puts nothing on the menu.
func (m *tillMenu) stopEcho() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.echo = false
}

// echoGroupBase keeps an echoed item's one group clear of the ids a test
// stocks by hand.
const echoGroupBase = 100000

// echoLine stocks the item a posted line describes, when echo is on. A body
// that is not a line, or names no dish, is left for the server to refuse.
func (m *tillMenu) echoLine(body string) {
	m.mu.Lock()
	echo := m.echo
	m.mu.Unlock()
	var l order.LineAdded
	if !echo || json.Unmarshal([]byte(body), &l) != nil || l.Name == "" {
		return
	}
	item := menu.Item{
		ID: l.ItemID, Name: l.Name, Price: l.UnitPrice, Station: l.Station, Course: l.Course,
		TaxRate: l.TaxRate, HSN: l.HSN, TaxExempt: l.TaxExempt,
	}
	if len(l.Modifiers) == 0 {
		m.stock(item)
		return
	}
	group := menu.ModifierGroup{ID: echoGroupBase + l.ItemID, Name: "Choices", MaxSelect: int64(len(l.Modifiers))}
	for _, mod := range l.Modifiers {
		group.Modifiers = append(group.Modifiers, menu.Modifier{ID: mod.ModifierID, Name: mod.Name, Price: mod.Price})
	}
	item.Groups = []int64{group.ID}
	m.stock(item, group)
}

// isAddLine is a POST to an order's lines.
func isAddLine(method, path string) bool {
	return method == http.MethodPost && strings.HasPrefix(path, "/api/v1/orders/") && strings.HasSuffix(path, "/lines")
}

// menuItem is an untaxed dish with no choices.
func menuItem(id int64, name string, price int64, station string) menu.Item {
	return menu.Item{ID: id, Name: name, Price: inr(price), Station: station}
}

// The pricing tests' menu: a dosa at ₹120 and 5% GST with two optional
// chutneys, a coffee with a milk to choose, and a vada that has sold out.
const (
	dosaID, coffeeID, vadaID = 7, 8, 9
	chutneyGroup, milkGroup  = 70, 80
	coconutID, sambarID      = 71, 72
	dairyID, oatID           = 81, 82
)

func stockPricingMenu(m *tillMenu, now time.Time) {
	m.stopEcho()
	gst := tax.RateFromBP(500)
	m.stock(menu.Item{
		ID: dosaID, Name: "Masala dosa", Price: inr(12000), HSN: "996331", TaxRate: gst,
		Station: "kitchen", Course: 1, Groups: []int64{chutneyGroup},
	}, menu.ModifierGroup{ID: chutneyGroup, Name: "Chutney", MinSelect: 0, MaxSelect: 2, Modifiers: []menu.Modifier{
		{ID: coconutID, Name: "Coconut", Price: inr(0)},
		{ID: sambarID, Name: "Extra sambar", Price: inr(2000)},
	}})
	m.stock(menu.Item{
		ID: coffeeID, Name: "Filter coffee", Price: inr(4000), HSN: "996331", TaxRate: gst,
		Station: "bar", Course: 1, Groups: []int64{milkGroup},
	}, menu.ModifierGroup{ID: milkGroup, Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []menu.Modifier{
		{ID: dairyID, Name: "Dairy", Price: inr(0)},
		{ID: oatID, Name: "Oat", Price: inr(5000)},
	}})
	back := now.Add(24 * time.Hour)
	m.stock(menu.Item{
		ID: vadaID, Name: "Medu vada", Price: inr(6000), HSN: "996331", TaxRate: gst,
		Station: "kitchen", Course: 1, SoldOutUntil: &back,
	})
}

// TestATillsLineIsPricedFromTheMenu is the HIGH finding's proof: a waiter
// who posts a dish at ₹1 and tax exempt gets the dish at the menu's price
// and tax. Only the item, the choices, how many and the course are the
// till's to say.
func TestATillsLineIsPricedFromTheMenu(t *testing.T) {
	r := newRegister(t)
	stockPricingMenu(r.menu, time.Now())

	tests := []struct {
		name      string
		body      string
		wantName  string
		wantUnit  int64
		wantQty   int64
		wantGross int64
		wantTax   int64
		wantMods  []order.Modifier
		course    int64
		station   string
	}{
		{
			name: "a dish at one rupee, tax exempt, at the bar",
			body: `{"item_id":7,"name":"Free dosa","qty":2,"unit_price":{"minor":100,"currency":"INR"},` +
				`"station":"bar","course":3,"tax_rate":0,"tax_exempt":true,"hsn":"0000"}`,
			wantName: "Masala dosa", wantUnit: 12000, wantQty: 2, wantGross: 24000, wantTax: 1200, course: 3, station: "kitchen",
		},
		{
			name: "a stale lower price from a menu the till has not reloaded",
			body: `{"item_id":7,"name":"Masala dosa","qty":1,"unit_price":{"minor":11000,"currency":"INR"},` +
				`"station":"kitchen","course":1,"tax_rate":5000,"hsn":"996331"}`,
			wantName: "Masala dosa", wantUnit: 12000, wantQty: 1, wantGross: 12000, wantTax: 600, course: 1, station: "kitchen",
		},
		{
			name: "a choice priced at nothing, under another name",
			body: `{"item_id":7,"name":"Masala dosa","qty":1,"unit_price":{"minor":12000,"currency":"INR"},` +
				`"modifiers":[{"modifier_id":72,"name":"Free sambar","price":{"minor":0,"currency":"INR"}}],"course":2}`,
			wantName: "Masala dosa", wantUnit: 12000, wantQty: 1, wantGross: 14000, wantTax: 700, course: 2, station: "kitchen",
			wantMods: []order.Modifier{{ModifierID: sambarID, Name: "Extra sambar", Price: inr(2000)}},
		},
		{
			name:     "only the dish and how many",
			body:     `{"item_id":8,"qty":3,"modifiers":[{"modifier_id":82}]}`,
			wantName: "Filter coffee", wantUnit: 4000, wantQty: 3, wantGross: 27000, wantTax: 1350, course: 0, station: "bar",
			wantMods: []order.Modifier{{ModifierID: oatID, Name: "Oat", Price: inr(5000)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := r.openOrder(t)
			res := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", tt.body)
			if len(res.Order.Lines) != 1 {
				t.Fatalf("lines = %+v", res.Order.Lines)
			}
			l := res.Order.Lines[0]
			if l.Name != tt.wantName || l.UnitPrice != inr(tt.wantUnit) || l.Qty != tt.wantQty || l.Course != tt.course || l.Station != tt.station {
				t.Errorf("line = %+v", l)
			}
			if l.Gross != inr(tt.wantGross) || l.Tax != inr(tt.wantTax) {
				t.Errorf("gross %d tax %d, want %d and %d", l.Gross.Minor, l.Tax.Minor, tt.wantGross, tt.wantTax)
			}
			if len(l.Modifiers) != len(tt.wantMods) {
				t.Fatalf("modifiers = %+v, want %+v", l.Modifiers, tt.wantMods)
			}
			for i := range tt.wantMods {
				if l.Modifiers[i] != tt.wantMods[i] {
					t.Errorf("modifier %d = %+v, want %+v", i, l.Modifiers[i], tt.wantMods[i])
				}
			}
			// The event is the record: what it says is what the menu says.
			if len(res.Events) != 1 {
				t.Fatalf("events = %+v", res.Events)
			}
			added, ok := res.Events[0].Payload.(order.LineAdded)
			if !ok {
				t.Fatalf("event payload = %T", res.Events[0].Payload)
			}
			if added.TaxExempt || added.TaxRate != tax.RateFromBP(500) || added.HSN != "996331" || added.UnitPrice != inr(tt.wantUnit) {
				t.Errorf("line.added = %+v", added)
			}
		})
	}
}

// TestATillsLineTheMenuRefuses: what the menu cannot ring is refused with a
// code the till can act on, and nothing is added.
func TestATillsLineTheMenuRefuses(t *testing.T) {
	r := newRegister(t)
	stockPricingMenu(r.menu, time.Now())
	id := r.openOrder(t)

	tests := []struct {
		name   string
		body   string
		status int
		code   ErrorCode
	}{
		{"a dish the menu does not have", `{"item_id":999,"name":"Lobster","qty":1,"unit_price":{"minor":1,"currency":"INR"}}`,
			http.StatusUnprocessableEntity, CodeDishUnavailable},
		{"no dish at all", `{"name":"Anything","qty":1,"unit_price":{"minor":1,"currency":"INR"}}`,
			http.StatusUnprocessableEntity, CodeDishUnavailable},
		{"a dish that has sold out", `{"item_id":9,"qty":1}`, http.StatusConflict, CodeDishUnavailable},
		{"a choice the dish does not offer", `{"item_id":7,"qty":1,"modifiers":[{"modifier_id":82,"name":"Oat","price":{"minor":0,"currency":"INR"}}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a choice that does not exist", `{"item_id":7,"qty":1,"modifiers":[{"modifier_id":4242}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a choice given twice", `{"item_id":7,"qty":1,"modifiers":[{"modifier_id":72},{"modifier_id":72}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a group left short", `{"item_id":8,"qty":1}`, http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a group over its bound", `{"item_id":8,"qty":1,"modifiers":[{"modifier_id":81},{"modifier_id":82}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"more than a till may ring at once", `{"item_id":7,"qty":1000}`, http.StatusBadRequest, CodeInvalidRequest},
		{"none of it", `{"item_id":7,"qty":0}`, http.StatusUnprocessableEntity, CodeInvalidPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Fatalf("%d %s (%s), want %d %s", status, apiErr.Code, apiErr.Message, tt.status, tt.code)
			}
			if apiErr.Message == "" {
				t.Error("a refusal with no message for the cashier to read")
			}
		})
	}
	read := r.ok(t, http.MethodGet, "/api/v1/orders/"+id, "")
	if len(read.Order.Lines) != 0 {
		t.Errorf("a refused line was added: %+v", read.Order.Lines)
	}

	// Many of one dish is still fine at a till: the guest's bound of
	// menu.MaxPickQty is not the till's.
	many := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", fmt.Sprintf(`{"item_id":7,"qty":%d}`, menu.MaxPickQty+5))
	if l := many.Order.Lines[0]; l.Qty != menu.MaxPickQty+5 || l.UnitPrice != inr(12000) {
		t.Errorf("many dosas = %+v", l)
	}
}

// TestAModifiedLineIsPricedFromTheMenu: the choices a till changes a line
// to come from the menu too. From is the till's: it is the guard against a
// stale ticket, checked against the line itself.
func TestAModifiedLineIsPricedFromTheMenu(t *testing.T) {
	r := newRegister(t)
	now := time.Now()
	stockPricingMenu(r.menu, now)
	id := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", `{"item_id":8,"qty":2,"modifiers":[{"modifier_id":81}]}`) // line 2
	const dairy = `{"modifier_id":81,"name":"Dairy","price":{"minor":0,"currency":"INR"}}`
	path := "/api/v1/orders/" + id + "/lines/2/modifiers"

	tests := []struct {
		name   string
		body   string
		status int
		code   ErrorCode
	}{
		{"a choice the dish does not offer", `{"from":[` + dairy + `],"to":[{"modifier_id":72,"name":"Extra sambar","price":{"minor":0,"currency":"INR"}}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a required choice taken away", `{"from":[` + dairy + `],"to":[]}`, http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"two where the dish takes one", `{"from":[` + dairy + `],"to":[{"modifier_id":81},{"modifier_id":82}]}`,
			http.StatusUnprocessableEntity, CodeChoicesInvalid},
		{"a line that does not exist", `{"from":[],"to":[{"modifier_id":82}]}`, http.StatusConflict, CodeLineNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := path
			if tt.code == CodeLineNotFound {
				p = "/api/v1/orders/" + id + "/lines/99/modifiers"
			}
			status, _, apiErr := r.call(t, http.MethodPost, p, tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Fatalf("%d %s (%s), want %d %s", status, apiErr.Code, apiErr.Message, tt.status, tt.code)
			}
		})
	}

	// Oat for nothing is oat at the menu's price.
	res := r.ok(t, http.MethodPost, path, `{"from":[`+dairy+`],"to":[{"modifier_id":82,"name":"Oat","price":{"minor":0,"currency":"INR"}}]}`)
	l := res.Order.Lines[0]
	want := order.Modifier{ModifierID: oatID, Name: "Oat", Price: inr(5000)}
	if len(l.Modifiers) != 1 || l.Modifiers[0] != want || l.Gross != inr(18000) {
		t.Fatalf("line after the change = %+v", l)
	}

	// A dish that sells out after it was rung can still have its choices
	// changed: it is on the bill already.
	back := now.Add(time.Hour)
	r.menu.stock(menu.Item{
		ID: coffeeID, Name: "Filter coffee", Price: inr(4000), HSN: "996331", TaxRate: tax.RateFromBP(500),
		Station: "bar", Course: 1, Groups: []int64{milkGroup}, SoldOutUntil: &back,
	})
	oat := `{"modifier_id":82,"name":"Oat","price":{"minor":5000,"currency":"INR"}}`
	res = r.ok(t, http.MethodPost, path, `{"from":[`+oat+`],"to":[{"modifier_id":81}]}`)
	if l := res.Order.Lines[0]; len(l.Modifiers) != 1 || l.Modifiers[0].ModifierID != dairyID || l.Gross != inr(8000) {
		t.Errorf("a sold-out dish's choices = %+v", l)
	}
}

// TestWithoutAMenuATillCannotRingAnything: the routes that put money on a
// bill are not mounted without a menu to price them from.
func TestWithoutAMenuATillCannotRingAnything(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	r.till.rebuild(t, WithOrders(r.srv.orders))

	for _, path := range []string{"/api/v1/orders/" + id + "/lines", "/api/v1/orders/" + id + "/lines/2/modifiers"} {
		rec := send(r.handler, http.MethodPost, path, `{"item_id":1,"qty":1}`, r.asStaff(r.staffToken))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want it not mounted", path, rec.Code)
		}
	}
	// The rest of the order routes stand.
	if rec := send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", r.asStaff(r.staffToken)); rec.Code != http.StatusOK {
		t.Errorf("reading the order: %d", rec.Code)
	}
}
