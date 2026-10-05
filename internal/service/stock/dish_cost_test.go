package stock

import (
	"context"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/stock"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// kitchen is a venue with two dishes, worked out by hand.
//
//	Beans    900.00 a kg     Milk   60.00 a litre     Syrup   no cost
//	Latte             18 g of beans, 200 ml of milk    16.20 + 12.00 = 28.20
//	Vanilla latte     18 g of beans, 15 ml of syrup   16.20, and syrup not costed
type kitchen struct {
	fixture
	beans, milk, syrup  int64
	latte, vanillaLatte int64
}

func newKitchen(t *testing.T) kitchen {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	mk := func(name string, unit stock.Unit, cost int64) int64 {
		id, err := f.s.CreateItem(ctx, 1, ItemInput{Name: name, Unit: unit, Cost: inr(cost)})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	k := kitchen{fixture: f, beans: mk("Beans", stock.Gram, 90000), milk: mk("Milk", stock.Millilitre, 6000)}
	k.syrup = f.item(t, 1, "Syrup", stock.Millilitre, 0)

	if _, err := f.d.CreateMenu(ctx, 1, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food", Rate: tax.RateFromBP(500)}},
		Categories: []db.CategorySpec{{Name: "Coffee", Items: []db.ItemSpec{
			{Name: "Latte", Price: 18000, TaxClass: "Food", Station: "bar", Course: 1},
			{Name: "Vanilla latte", Price: 20000, TaxClass: "Food", Station: "bar", Course: 1},
		}}},
	}, now); err != nil {
		t.Fatal(err)
	}
	m, err := f.d.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range m.Items {
		switch it.Name {
		case "Latte":
			k.latte = it.ID
		case "Vanilla latte":
			k.vanillaLatte = it.ID
		}
	}
	for _, err := range []error{
		f.s.SetItemRecipe(ctx, 1, k.latte, []RecipePart{{StockItemID: k.beans, Qty: 18000}, {StockItemID: k.milk, Qty: 200000}}),
		f.s.SetItemRecipe(ctx, 1, k.vanillaLatte, []RecipePart{{StockItemID: k.beans, Qty: 18000}, {StockItemID: k.syrup, Qty: 15000}}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return k
}

// sell closes an order at an instant with these lines.
func (k kitchen) sell(t *testing.T, orderID int64, at string, lines ...stock.SoldLine) {
	t.Helper()
	when, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	err = k.d.WriteTx(context.Background(), func(q *gen.Queries) error {
		return DeductSale(context.Background(), q, Sale{VenueID: 1, OrderID: orderID, StaffID: k.asha, Lines: lines, At: when})
	})
	if err != nil {
		t.Fatalf("selling order %d: %v", orderID, err)
	}
}

func (k kitchen) costs(t *testing.T, from, to string) DishCosts {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	var got DishCosts
	err = k.d.WriteTx(context.Background(), func(q *gen.Queries) error {
		var err error
		got, err = CostByDish(context.Background(), q, DishRange{VenueID: 1, From: from, To: to, Loc: loc, DayStart: shift.DefaultDayStart, Currency: "INR"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestWhatEachDishCostToMake(t *testing.T) {
	k := newKitchen(t)
	// 29 Sept: two lattes on one order, one latte and one vanilla latte on another.
	k.sell(t, 1, "2026-09-29T06:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 2})
	k.sell(t, 2, "2026-09-29T07:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1}, stock.SoldLine{ItemID: k.vanillaLatte, Qty: 1})
	// 2 Oct: outside the range asked for.
	k.sell(t, 3, "2026-10-02T06:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 5})

	got := k.costs(t, "2026-09-29", "2026-09-30")
	if got.KeptFrom != "2026-09-29" || got.NoDish != 0 {
		t.Errorf("kept from %q with %d movements under no dish; want 2026-09-29 and none", got.KeptFrom, got.NoDish)
	}
	// Three lattes: 3 x 28.20 = 84.60, in two movements a stock item an order.
	l := got.Dishes[k.latte]
	if l.Cost != inr(8460) || l.Uncosted != 0 || l.Movements != 4 || l.Units != 3 || l.PutBack != 0 {
		t.Errorf("latte = %+v; want 84.60 for 3 over 4 movements, all costed", l)
	}
	// One vanilla latte: 16.20 of beans, and the syrup has no cost.
	v := got.Dishes[k.vanillaLatte]
	if v.Cost != inr(1620) || v.Uncosted != 1 || v.Movements != 2 || v.Units != 1 {
		t.Errorf("vanilla latte = %+v; want 16.20 for 1 over 2 movements, 1 not costed", v)
	}
	if len(got.Dishes) != 2 {
		t.Errorf("dishes %+v, want two", got.Dishes)
	}
}

// A line voided after a reopen is put back at the next close: the dish's
// cost comes down by what went back.
func TestStockPutBackComesOffTheDishsCost(t *testing.T) {
	k := newKitchen(t)
	k.sell(t, 1, "2026-09-29T06:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 2})
	k.sell(t, 1, "2026-09-29T08:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})

	l := k.costs(t, "2026-09-29", "2026-09-29").Dishes[k.latte]
	if l.Cost != inr(2820) || l.Movements != 4 || l.Units != 1 || l.PutBack != 2 {
		t.Errorf("latte = %+v; want 28.20 for 1 after one of two went back, over 4 movements, 2 of them put back", l)
	}
}

// Stock taken before the ledger kept the dish is under no dish: it is counted
// apart, and the report says from which day dishes are kept.
func TestSalesFromBeforeTheDishWasKept(t *testing.T) {
	k := newKitchen(t)
	ctx := context.Background()
	// An old sale, written as the ledger wrote it then: no dish.
	if err := k.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.InsertStockMovement(ctx, gen.InsertStockMovementParams{
			VenueID: 1, StockItemID: k.beans, Kind: string(stock.Sold), Delta: -18000, StaffID: k.asha, OrderID: 9,
			CreatedAt: time.Date(2026, 9, 28, 6, 0, 0, 0, time.UTC),
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	k.sell(t, 1, "2026-09-30T06:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})

	got := k.costs(t, "2026-09-28", "2026-09-30")
	if got.NoDish != 1 || got.KeptFrom != "2026-09-30" {
		t.Errorf("%d movements under no dish, kept from %q; want 1 and 2026-09-30", got.NoDish, got.KeptFrom)
	}
	if _, ok := got.Dishes[0]; ok {
		t.Error("stock under no dish is listed as a dish")
	}
	if got.Dishes[k.latte].Cost != inr(2820) {
		t.Errorf("latte = %+v, want 28.20", got.Dishes[k.latte])
	}

	// A venue that has never sold by dish says so with no day.
	if quiet := newKitchen(t).costs(t, "2026-09-28", "2026-09-30"); quiet.KeptFrom != "" || len(quiet.Dishes) != 0 {
		t.Errorf("a venue with no sales: %+v", quiet)
	}
}

// 03:00 in Kolkata on 30 Sept is still 29 Sept's trade.
func TestADishIsCostedOnTheTradingDayItSold(t *testing.T) {
	k := newKitchen(t)
	k.sell(t, 1, "2026-09-29T21:30:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})
	if got := k.costs(t, "2026-09-29", "2026-09-29"); got.Dishes[k.latte].Cost != inr(2820) {
		t.Errorf("29 Sept: %+v, want the latte sold at 03:00 the next morning", got.Dishes)
	}
	if got := k.costs(t, "2026-09-30", "2026-09-30"); len(got.Dishes) != 0 {
		t.Errorf("30 Sept: %+v, want nothing", got.Dishes)
	}
}

// A dish sold before it had a recipe took no stock: the ledger costed none of
// it. Sold again once it has one, the cost covers that unit alone, and Units
// says so, so nobody sets one unit's cost against two units' sales.
func TestUnitsAreOnlyThoseTheLedgerCosted(t *testing.T) {
	k := newKitchen(t)
	ctx := context.Background()
	if err := k.s.SetItemRecipe(ctx, 1, k.latte, nil); err != nil {
		t.Fatal(err)
	}
	k.sell(t, 1, "2026-09-29T06:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})
	if err := k.s.SetItemRecipe(ctx, 1, k.latte, []RecipePart{{StockItemID: k.beans, Qty: 18000}, {StockItemID: k.milk, Qty: 200000}}); err != nil {
		t.Fatal(err)
	}
	k.sell(t, 2, "2026-09-29T07:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})

	l := k.costs(t, "2026-09-29", "2026-09-29").Dishes[k.latte]
	if l.Units != 1 || l.Cost != inr(2820) {
		t.Errorf("latte = %+v; want 28.20 covering 1 unit of the 2 sold", l)
	}
	// Closing the second order again records nothing more.
	k.sell(t, 2, "2026-09-29T09:00:00Z", stock.SoldLine{ItemID: k.latte, Qty: 1})
	if again := k.costs(t, "2026-09-29", "2026-09-29").Dishes[k.latte]; again.Units != 1 || again.Movements != l.Movements {
		t.Errorf("after closing again: %+v, want no change from %+v", again, l)
	}
}
