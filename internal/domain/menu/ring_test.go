package menu

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

var ringAt = time.Date(2026, 10, 5, 6, 0, 0, 0, time.UTC)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// cafe is a menu with a coffee that needs a milk and may take up to two
// extras, a brownie with no choices, and a soup that is sold out until
// tomorrow.
func cafe() Menu {
	tomorrow := ringAt.Add(18 * time.Hour)
	return Menu{
		Version: 3, Currency: "INR",
		Items: []Item{
			{ID: 1, Name: "Flat white", Price: inr(18000), HSN: "996331", TaxRate: tax.RateFromBP(500), Station: "bar", Course: 1, Groups: []int64{10, 11}},
			{ID: 2, Name: "Brownie", Price: inr(12000), Station: "kitchen", TaxExempt: true},
			{ID: 3, Name: "Soup", Price: inr(9000), Station: "kitchen", SoldOutUntil: &tomorrow},
		},
		Groups: []ModifierGroup{
			{ID: 11, Name: "Extras", MinSelect: 0, MaxSelect: 2, Modifiers: []Modifier{
				{ID: 110, Name: "Extra shot", Price: inr(5000)},
				{ID: 111, Name: "Vanilla", Price: inr(3000)},
				{ID: 112, Name: "Less milk", Price: inr(-1000)},
			}},
			{ID: 10, Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []Modifier{
				{ID: 100, Name: "Dairy", IsDefault: true, Price: inr(0)},
				{ID: 101, Name: "Oat", Price: inr(4000)},
			}},
			{ID: 12, Name: "Another dish's", MinSelect: 0, MaxSelect: 1, Modifiers: []Modifier{{ID: 120, Name: "Cream", Price: inr(2000)}}},
		},
	}
}

func TestRing(t *testing.T) {
	// Asked for extras first and out of order: the line has the item's
	// groups in its own order, and each group's choices in the menu's.
	got, err := cafe().Ring(Pick{ItemID: 1, ModifierIDs: []int64{111, 110, 101}, Qty: 2}, ringAt)
	if err != nil {
		t.Fatal(err)
	}
	want := order.LineAdded{
		ItemID: 1, Name: "Flat white", Qty: 2, UnitPrice: inr(18000),
		Modifiers: []order.Modifier{
			{ModifierID: 101, Name: "Oat", Price: inr(4000)},
			{ModifierID: 110, Name: "Extra shot", Price: inr(5000)},
			{ModifierID: 111, Name: "Vanilla", Price: inr(3000)},
		},
		Course: 1, Station: "bar", TaxRate: tax.RateFromBP(500), HSN: "996331",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	value, err := LineValue(got)
	if err != nil {
		t.Fatal(err)
	}
	if value != inr(60000) {
		t.Errorf("two at 180 + 40 + 50 + 30 come to %v, want 600.00", value)
	}
}

func TestRingWithNoChoices(t *testing.T) {
	got, err := cafe().Ring(Pick{ItemID: 2, Qty: 1}, ringAt)
	if err != nil {
		t.Fatal(err)
	}
	// An empty list and not none: it is what a till sends, and what the
	// event stores.
	if got.Modifiers == nil || len(got.Modifiers) != 0 {
		t.Errorf("modifiers = %#v, want an empty list", got.Modifiers)
	}
	if !got.TaxExempt || got.Station != "kitchen" || got.Course != 0 {
		t.Errorf("line = %+v", got)
	}
}

func TestRingANegativeChoice(t *testing.T) {
	got, err := cafe().Ring(Pick{ItemID: 1, ModifierIDs: []int64{100, 112}, Qty: 3}, ringAt)
	if err != nil {
		t.Fatal(err)
	}
	value, err := LineValue(got)
	if err != nil {
		t.Fatal(err)
	}
	if value != inr(51000) {
		t.Errorf("three at 180 + 0 - 10 come to %v, want 510.00", value)
	}
}

func TestRingRefuses(t *testing.T) {
	tests := []struct {
		name string
		pick Pick
		want error
		item string
	}{
		{"a dish the menu does not have", Pick{ItemID: 99, Qty: 1}, ErrNoSuchItem, ""},
		{"a sold-out dish", Pick{ItemID: 3, Qty: 1}, ErrSoldOut, "Soup"},
		{"none of it", Pick{ItemID: 2, Qty: 0}, ErrPickQty, "Brownie"},
		{"a negative count", Pick{ItemID: 2, Qty: -1}, ErrPickQty, "Brownie"},
		{"more than the cart allows", Pick{ItemID: 2, Qty: MaxPickQty + 1}, ErrPickQty, "Brownie"},
		{"a required group left empty", Pick{ItemID: 1, Qty: 1}, ErrChoices, "Flat white"},
		{"two of a choose-one", Pick{ItemID: 1, ModifierIDs: []int64{100, 101}, Qty: 1}, ErrChoices, "Flat white"},
		{"more than a group allows", Pick{ItemID: 1, ModifierIDs: []int64{100, 110, 111, 112}, Qty: 1}, ErrChoices, "Flat white"},
		{"the same choice twice", Pick{ItemID: 1, ModifierIDs: []int64{100, 110, 110}, Qty: 1}, ErrChoices, "Flat white"},
		{"a choice from another dish's group", Pick{ItemID: 1, ModifierIDs: []int64{100, 120}, Qty: 1}, ErrChoices, "Flat white"},
		{"a choice on a dish with none", Pick{ItemID: 2, ModifierIDs: []int64{100}, Qty: 1}, ErrChoices, "Brownie"},
		{"a choice that does not exist", Pick{ItemID: 1, ModifierIDs: []int64{100, 999}, Qty: 1}, ErrChoices, "Flat white"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cafe().Ring(tt.pick, ringAt)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			var pe *PickError
			if !errors.As(err, &pe) || pe.Item != tt.item || pe.ItemID != tt.pick.ItemID {
				t.Errorf("refused as %+v, want it to name %q", pe, tt.item)
			}
		})
	}
}

func TestRingOnceSoldOutIsOver(t *testing.T) {
	m := cafe()
	if _, err := m.Ring(Pick{ItemID: 3, Qty: 1}, *m.Items[2].SoldOutUntil); err != nil {
		t.Errorf("at the moment it is back on sale: %v", err)
	}
}

func TestLineValueRefusesOverflow(t *testing.T) {
	_, err := LineValue(order.LineAdded{Qty: 20, UnitPrice: inr(1 << 62)})
	if !errors.Is(err, money.ErrOverflow) {
		t.Errorf("err = %v, want an overflow", err)
	}
}
