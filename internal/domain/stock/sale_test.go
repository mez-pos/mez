package stock

import (
	"reflect"
	"testing"
)

// The venue: beans (1), milk (2), oat milk (3), syrup (4). A latte (10)
// uses 18 g of beans; its Milk group has Dairy (100) at 200 ml of milk and
// Oat (101) at 200 ml of oat milk; Vanilla (102) is 15 ml of syrup; Extra
// shot (103) is 9 g more beans. Water (11) has no recipe.
var recipes = Recipes{
	Items: map[int64][]Part{10: {{1, 18 * PerUnit}}},
	Modifiers: map[int64][]Part{
		100: {{2, 200 * PerUnit}},
		101: {{3, 200 * PerUnit}},
		102: {{4, 15 * PerUnit}},
		103: {{1, 9 * PerUnit}},
	},
}

// use is a stock item used for a dish: use(1, 10) is beans for the latte.
func use(stockItem, dish int64) Use { return Use{StockItemID: stockItem, ItemID: dish} }

// A flat white (12) uses 18 g of beans too, so the same beans go to two dishes.
var cafe = Recipes{
	Items:     map[int64][]Part{10: {{1, 18 * PerUnit}}, 12: {{1, 18 * PerUnit}}},
	Modifiers: recipes.Modifiers,
}

func TestUsage(t *testing.T) {
	tests := []struct {
		name  string
		lines []SoldLine
		want  map[Use]Qty
	}{
		{"one latte with dairy", []SoldLine{{ItemID: 10, ModifierIDs: []int64{100}, Qty: 1}},
			map[Use]Qty{use(1, 10): 18 * PerUnit, use(2, 10): 200 * PerUnit}},
		{"two oat lattes with vanilla and a shot", []SoldLine{{ItemID: 10, ModifierIDs: []int64{101, 102, 103}, Qty: 2}},
			map[Use]Qty{use(1, 10): 54 * PerUnit, use(3, 10): 400 * PerUnit, use(4, 10): 30 * PerUnit}},
		{"lines of one dish add up", []SoldLine{
			{ItemID: 10, ModifierIDs: []int64{100}, Qty: 1},
			{ItemID: 10, ModifierIDs: []int64{101}, Qty: 3},
		}, map[Use]Qty{use(1, 10): 72 * PerUnit, use(2, 10): 200 * PerUnit, use(3, 10): 600 * PerUnit}},
		{"the same stock for two dishes is kept apart", []SoldLine{
			{ItemID: 10, Qty: 1},
			{ItemID: 12, ModifierIDs: []int64{100}, Qty: 2},
		}, map[Use]Qty{use(1, 10): 18 * PerUnit, use(1, 12): 36 * PerUnit, use(2, 12): 400 * PerUnit}},
		{"a modifier's stock is its dish's", []SoldLine{{ItemID: 12, ModifierIDs: []int64{103}, Qty: 1}},
			map[Use]Qty{use(1, 12): 27 * PerUnit}},
		{"no recipe uses nothing", []SoldLine{{ItemID: 11, Qty: 5}}, map[Use]Qty{}},
		{"a modifier with no recipe adds nothing", []SoldLine{{ItemID: 10, ModifierIDs: []int64{999}, Qty: 1}},
			map[Use]Qty{use(1, 10): 18 * PerUnit}},
		{"a line of none uses nothing", []SoldLine{{ItemID: 10, Qty: 0}}, map[Use]Qty{}},
		{"no lines", nil, map[Use]Qty{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Usage(tt.lines, cafe); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Usage = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaleDeltas(t *testing.T) {
	tests := []struct {
		name            string
		usage, deducted map[Use]Qty
		want            []SaleDelta
	}{
		{"the first close takes it all", map[Use]Qty{use(1, 10): 18000, use(2, 10): 200000}, nil,
			[]SaleDelta{{1, 10, -18000}, {2, 10, -200000}}},
		{"a second close with the same lines writes nothing", map[Use]Qty{use(1, 10): 18000}, map[Use]Qty{use(1, 10): 18000}, nil},
		{"a line added after a reopen takes only the rest", map[Use]Qty{use(1, 10): 36000}, map[Use]Qty{use(1, 10): 18000},
			[]SaleDelta{{1, 10, -18000}}},
		{"a line voided after a reopen puts its stock back", map[Use]Qty{use(1, 10): 18000}, map[Use]Qty{use(1, 10): 36000},
			[]SaleDelta{{1, 10, 18000}}},
		{"an ingredient no longer used goes back whole", map[Use]Qty{use(1, 10): 18000}, map[Use]Qty{use(1, 10): 18000, use(2, 10): 200000},
			[]SaleDelta{{2, 10, 200000}}},
		{"one dish voided, another kept, of the same stock", map[Use]Qty{use(1, 12): 18000}, map[Use]Qty{use(1, 10): 18000, use(1, 12): 18000},
			[]SaleDelta{{1, 10, 18000}}},
		{"in stock item order, then by dish", map[Use]Qty{use(2, 12): 5, use(1, 12): 5, use(1, 10): 5}, nil,
			[]SaleDelta{{1, 10, -5}, {1, 12, -5}, {2, 12, -5}}},
		// An order closed before stock was kept by dish has its stock under
		// no dish (0). Closed again, that goes back and is taken by dish: the
		// stock on hand does not move.
		{"stock taken under no dish is moved to its dish", map[Use]Qty{use(1, 10): 18000}, map[Use]Qty{use(1, 0): 18000},
			[]SaleDelta{{1, 0, 18000}, {1, 10, -18000}}},
		{"nothing used, nothing taken", nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SaleDeltas(tt.usage, tt.deducted)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SaleDeltas = %v, want %v", got, tt.want)
			}
			// Whatever is written, the stock on hand moves by usage less deducted.
			net := map[int64]Qty{}
			for _, d := range got {
				net[d.StockItemID] += d.Delta
			}
			want := map[int64]Qty{}
			for u, q := range tt.deducted {
				want[u.StockItemID] += q
			}
			for u, q := range tt.usage {
				want[u.StockItemID] -= q
			}
			for id, q := range want {
				if net[id] != q {
					t.Errorf("stock item %d moves by %d, want %d", id, net[id], q)
				}
			}
		})
	}
}

func TestRecipeProblem(t *testing.T) {
	if p := RecipeProblem([]Part{{1, 18000}, {2, 500}}); p != nil {
		t.Errorf("a good recipe: %v", p)
	}
	if p := RecipeProblem(nil); p != nil {
		t.Errorf("an empty recipe is no recipe: %v", p)
	}
	tests := []struct {
		name  string
		parts []Part
		field string
	}{
		{"one stock item twice", []Part{{1, 1}, {2, 1}, {1, 5}}, "parts.2.stock_item_id"},
		{"none of it", []Part{{1, 0}}, "parts.0.qty"},
		{"below zero", []Part{{1, 5}, {2, -1}}, "parts.1.qty"},
		{"too much", []Part{{1, MaxQty + 1}}, "parts.0.qty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if p := RecipeProblem(tt.parts); p == nil || p.Field != tt.field {
				t.Errorf("problem = %v, want one on %q", p, tt.field)
			}
		})
	}
}
