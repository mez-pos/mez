package stock

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/stock"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// unitCosts is the cost kept on each of an item's movements, oldest first.
func (f fixture) unitCosts(t *testing.T, itemID int64) []int64 {
	t.Helper()
	rows, err := f.d.Writer().QueryContext(context.Background(), `SELECT unit_cost FROM stock_movements WHERE stock_item_id = ? ORDER BY id`, itemID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var c int64
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f fixture) costOf(t *testing.T, itemID int64) money.Amount {
	t.Helper()
	items, err := f.s.Items(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == itemID {
			return it.Cost
		}
	}
	t.Fatalf("no item %d", itemID)
	return money.Amount{}
}

func TestAnItemsCost(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// With none given it is not known, and still in the venue's currency.
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	if got := f.costOf(t, beans); got != inr(0) {
		t.Errorf("a cost not given is %+v, want 0 INR", got)
	}
	milk, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Milk", Unit: stock.Millilitre, Cost: inr(6000)})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.costOf(t, milk); got != inr(6000) {
		t.Errorf("milk costs %+v, want 60.00 a litre", got)
	}
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Millilitre, Cost: inr(6400)}); err != nil {
		t.Fatal(err)
	}
	if got := f.costOf(t, milk); got != inr(6400) {
		t.Errorf("milk now costs %+v, want 64.00", got)
	}
	// Left out of an edit, it is not known again: the form sends it whole.
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Millilitre}); err != nil {
		t.Fatal(err)
	}
	if got := f.costOf(t, milk); got != inr(0) {
		t.Errorf("milk now costs %+v, want not known", got)
	}
	if c, err := f.s.Currency(ctx, 1); err != nil || c != "INR" {
		t.Errorf("currency %q, %v", c, err)
	}

	refusals := []struct {
		name string
		cost money.Amount
	}{
		{"below zero", inr(-1)},
		{"too large", inr(stock.MaxCost + 1)},
		{"another currency", money.Amount{Minor: 100, Currency: "USD"}},
		{"another currency, nothing", money.Amount{Currency: "USD"}},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			var inv *Invalid
			_, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Sugar " + tt.name, Unit: stock.Gram, Cost: tt.cost})
			if !errors.As(err, &inv) || inv.Field != "cost" {
				t.Errorf("create: err = %v, want the cost refused", err)
			}
			err = f.s.UpdateItem(ctx, 1, beans, ItemInput{Name: "Beans", Unit: stock.Gram, Cost: tt.cost})
			if !errors.As(err, &inv) || inv.Field != "cost" {
				t.Errorf("update: err = %v, want the cost refused", err)
			}
		})
	}
}

// A movement keeps the cost as it was when it was written; a later change
// of cost touches none of them.
func TestAMovementKeepsItsCost(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	milk, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Milk", Unit: stock.Millilitre, Cost: inr(6000)})
	if err != nil {
		t.Fatal(err)
	}
	record := func(in MovementInput) {
		t.Helper()
		if _, err := f.s.Record(ctx, 1, f.asha, milk, in); err != nil {
			t.Fatal(err)
		}
	}
	record(MovementInput{Kind: stock.Received, Qty: 5000 * stock.PerUnit})
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Millilitre, Cost: inr(6400)}); err != nil {
		t.Fatal(err)
	}
	record(MovementInput{Kind: stock.Counted, Qty: 4500 * stock.PerUnit})
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Millilitre}); err != nil {
		t.Fatal(err)
	}
	record(MovementInput{Kind: stock.Wasted, Qty: 100 * stock.PerUnit, Reason: stock.Spilled})

	got := f.unitCosts(t, milk)
	want := []int64{6000, 6400, 0}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("unit costs %v, want %v", got, want)
	}
}
