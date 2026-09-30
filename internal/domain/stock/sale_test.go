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

func TestUsage(t *testing.T) {
	tests := []struct {
		name  string
		lines []SoldLine
		want  map[int64]Qty
	}{
		{"one latte with dairy", []SoldLine{{ItemID: 10, ModifierIDs: []int64{100}, Qty: 1}},
			map[int64]Qty{1: 18 * PerUnit, 2: 200 * PerUnit}},
		{"two oat lattes with vanilla and a shot", []SoldLine{{ItemID: 10, ModifierIDs: []int64{101, 102, 103}, Qty: 2}},
			map[int64]Qty{1: 54 * PerUnit, 3: 400 * PerUnit, 4: 30 * PerUnit}},
		{"lines add up", []SoldLine{
			{ItemID: 10, ModifierIDs: []int64{100}, Qty: 1},
			{ItemID: 10, ModifierIDs: []int64{101}, Qty: 3},
		}, map[int64]Qty{1: 72 * PerUnit, 2: 200 * PerUnit, 3: 600 * PerUnit}},
		{"no recipe uses nothing", []SoldLine{{ItemID: 11, Qty: 5}}, map[int64]Qty{}},
		{"a modifier with no recipe adds nothing", []SoldLine{{ItemID: 10, ModifierIDs: []int64{999}, Qty: 1}},
			map[int64]Qty{1: 18 * PerUnit}},
		{"a line of none uses nothing", []SoldLine{{ItemID: 10, Qty: 0}}, map[int64]Qty{}},
		{"no lines", nil, map[int64]Qty{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Usage(tt.lines, recipes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Usage = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaleDeltas(t *testing.T) {
	tests := []struct {
		name            string
		usage, deducted map[int64]Qty
		want            []SaleDelta
	}{
		{"the first close takes it all", map[int64]Qty{1: 18000, 2: 200000}, nil,
			[]SaleDelta{{1, -18000}, {2, -200000}}},
		{"a second close with the same lines writes nothing", map[int64]Qty{1: 18000}, map[int64]Qty{1: 18000}, nil},
		{"a line added after a reopen takes only the rest", map[int64]Qty{1: 36000}, map[int64]Qty{1: 18000},
			[]SaleDelta{{1, -18000}}},
		{"a line voided after a reopen puts its stock back", map[int64]Qty{1: 18000}, map[int64]Qty{1: 36000},
			[]SaleDelta{{1, 18000}}},
		{"an ingredient no longer used goes back whole", map[int64]Qty{1: 18000}, map[int64]Qty{1: 18000, 2: 200000},
			[]SaleDelta{{2, 200000}}},
		{"nothing used, nothing taken", nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SaleDeltas(tt.usage, tt.deducted); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SaleDeltas = %v, want %v", got, tt.want)
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
