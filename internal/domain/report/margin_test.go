package report

import (
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/zreport"
)

func item(key, name string, qty, net int64) zreport.Group {
	z := inr(0)
	return zreport.Group{Key: key, Name: name, Qty: qty, Gross: inr(net), Discounts: z, Net: inr(net), Tax: z, Total: inr(net)}
}

func TestMargins(t *testing.T) {
	amount := func(minor int64) *money.Amount { a := inr(minor); return &a }
	bp := func(n int64) *int64 { return &n }
	tests := []struct {
		name  string
		item  zreport.Group
		cost  *DishCost
		short bool
		want  ItemMargin
	}{
		{
			name: "a dish fully costed has its margin", item: item("10", "Latte", 3, 54000),
			cost: &DishCost{Cost: inr(8460), Movements: 4},
			want: ItemMargin{Key: "10", Name: "Latte", Qty: 3, Net: inr(54000), Costing: CostFull, Cost: amount(8460), Margin: amount(45540), MarginBP: bp(8433)},
		},
		{
			name: "a dish with no recipe has no cost", item: item("11", "Water", 5, 10000),
			want: ItemMargin{Key: "11", Name: "Water", Qty: 5, Net: inr(10000), Costing: CostNone},
		},
		{
			name: "an ingredient with no cost leaves the cost short and the margin unsaid", item: item("12", "Vanilla latte", 1, 20000),
			cost: &DishCost{Cost: inr(1620), Movements: 2, Uncosted: 1},
			want: ItemMargin{Key: "12", Name: "Vanilla latte", Qty: 1, Net: inr(20000), Costing: CostPart, Cost: amount(1620)},
		},
		{
			name: "a range reaching before dishes were kept trusts no cost", item: item("10", "Latte", 3, 54000),
			cost: &DishCost{Cost: inr(2820), Movements: 2}, short: true,
			want: ItemMargin{Key: "10", Name: "Latte", Qty: 3, Net: inr(54000), Costing: CostPart, Cost: amount(2820)},
		},
		{
			name: "a dish that cost more than it took has a margin below zero", item: item("13", "Offer thali", 1, 10000),
			cost: &DishCost{Cost: inr(12500), Movements: 3},
			want: ItemMargin{Key: "13", Name: "Offer thali", Qty: 1, Net: inr(10000), Costing: CostFull, Cost: amount(12500), Margin: amount(-2500), MarginBP: bp(-2500)},
		},
		{
			name: "a dish given away has a margin and no percentage", item: item("14", "Comped chai", 2, 0),
			cost: &DishCost{Cost: inr(900), Movements: 2},
			want: ItemMargin{Key: "14", Name: "Comped chai", Qty: 2, Net: inr(0), Costing: CostFull, Cost: amount(900), Margin: amount(-900)},
		},
		{
			name: "stock all put back is a cost of nothing, fully known", item: item("15", "Toast", 1, 5000),
			cost: &DishCost{Cost: inr(0), Movements: 2},
			want: ItemMargin{Key: "15", Name: "Toast", Qty: 1, Net: inr(5000), Costing: CostFull, Cost: amount(0), Margin: amount(5000), MarginBP: bp(10000)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			costs := map[string]DishCost{}
			if tt.cost != nil {
				costs[tt.item.Key] = *tt.cost
			}
			got := Margins([]zreport.Group{tt.item}, costs, tt.short)
			if len(got) != 1 {
				t.Fatalf("%d rows", len(got))
			}
			g, w := got[0], tt.want
			same := func(a, b *money.Amount) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
			sameBP := (g.MarginBP == nil) == (w.MarginBP == nil) && (g.MarginBP == nil || *g.MarginBP == *w.MarginBP)
			if g.Key != w.Key || g.Name != w.Name || g.Qty != w.Qty || g.Net != w.Net || g.Costing != w.Costing ||
				!same(g.Cost, w.Cost) || !same(g.Margin, w.Margin) || !sameBP {
				t.Errorf("got  %s\nwant %s", show(g), show(w))
			}
		})
	}
}

func show(m ItemMargin) string {
	s := m.Name + " " + string(m.Costing) + " net " + m.Net.String()
	if m.Cost != nil {
		s += " cost " + m.Cost.String()
	}
	if m.Margin != nil {
		s += " margin " + m.Margin.String()
	}
	if m.MarginBP != nil {
		s += " bp " + itoa(*m.MarginBP)
	}
	return s
}

func itoa(n int64) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestMarginsKeepTheReportsOrder(t *testing.T) {
	items := []zreport.Group{item("2", "Thali", 1, 20000), item("1", "Chai", 9, 9000)}
	got := Margins(items, nil, false)
	if len(got) != 2 || got[0].Key != "2" || got[1].Key != "1" {
		t.Errorf("order %+v", got)
	}
	if out := Margins(nil, nil, false); out == nil || len(out) != 0 {
		t.Errorf("no items = %+v, want an empty list", out)
	}
}

func TestBasisPointsRoundHalfAwayFromZero(t *testing.T) {
	for _, tt := range []struct{ part, whole, want int64 }{
		{1, 3, 3333}, {2, 3, 6667}, {1, 8, 1250}, {-1, 3, -3333}, {-2, 3, -6667}, {1, 20000, 1}, {1, 40000, 0}, {3, 40000, 1}, {-3, 40000, -1},
	} {
		if got := basisPoints(tt.part, tt.whole); got != tt.want {
			t.Errorf("%d of %d = %d, want %d", tt.part, tt.whole, got, tt.want)
		}
	}
}
