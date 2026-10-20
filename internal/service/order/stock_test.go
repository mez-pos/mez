package order

import (
	"context"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/stock"
	"github.com/mez-pos/mez/internal/domain/tax"
	svcstock "github.com/mez-pos/mez/internal/service/stock"
)

// pantry is the order service over a menu with recipes: a dosa takes 150 g
// of batter; a coffee 10 g of powder; its Dairy choice 100 ml of milk and
// its Oat choice 100 ml of oat milk.
type pantry struct {
	s       *Service
	d       *db.DB
	m       Meta
	staffID int64
	dosa    int64
	coffee  int64
	dairy   int64
	oat     int64
	stock   map[string]int64
}

func newPantry(t *testing.T) pantry {
	t.Helper()
	ctx := context.Background()
	s, d, _ := newService(t)
	k := pantry{s: s, d: d, stock: map[string]int64{}}

	if _, err := d.CreateMenu(ctx, 1, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "kitchen", Name: "Kitchen"}, {Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food", Rate: tax.RateFromBP(500)}},
		Groups: []db.ModifierGroupSpec{{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
			{Name: "Dairy", IsDefault: true}, {Name: "Oat", Price: 4000},
		}}},
		Categories: []db.CategorySpec{{Name: "All", Items: []db.ItemSpec{
			{Name: "Masala dosa", Price: 12000, HSN: "996331", TaxClass: "Food", Station: "kitchen", Course: 1},
			{Name: "Filter coffee", Price: 4000, HSN: "996331", TaxClass: "Food", Station: "bar", Course: 1, Groups: []string{"Milk"}},
		}}},
	}, now); err != nil {
		t.Fatal(err)
	}
	m, err := d.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range m.Items {
		switch it.Name {
		case "Masala dosa":
			k.dosa = it.ID
		case "Filter coffee":
			k.coffee = it.ID
		}
	}
	for _, md := range m.Groups[0].Modifiers {
		switch md.Name {
		case "Dairy":
			k.dairy = md.ID
		case "Oat":
			k.oat = md.ID
		}
	}

	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		role, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: "owner", Permissions: `[]`, CreatedAt: now})
		if err != nil {
			return err
		}
		staff, err := q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: "stf_KITCHEN1", VenueID: 1, RoleID: role.ID, Name: "Asha", PinHash: "x", Active: true, CreatedAt: now, UpdatedAt: now,
		})
		k.staffID = staff.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	k.m = Meta{VenueID: 1, ActorID: k.staffID, DeviceID: 3}

	st, err := svcstock.New(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, unit := range map[string]stock.Unit{"batter": stock.Gram, "powder": stock.Gram, "milk": stock.Millilitre, "oat milk": stock.Millilitre} {
		id, err := st.CreateItem(ctx, 1, svcstock.ItemInput{Name: name, Unit: unit})
		if err != nil {
			t.Fatal(err)
		}
		k.stock[name] = id
	}
	part := func(name string, units int64) []svcstock.RecipePart {
		return []svcstock.RecipePart{{StockItemID: k.stock[name], Qty: stock.Qty(units) * stock.PerUnit}}
	}
	for _, err := range []error{
		st.SetItemRecipe(ctx, 1, k.dosa, part("batter", 150)),
		st.SetItemRecipe(ctx, 1, k.coffee, part("powder", 10)),
		st.SetModifierRecipe(ctx, 1, k.dairy, part("milk", 100)),
		st.SetModifierRecipe(ctx, 1, k.oat, part("oat milk", 100)),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return k
}

func (k pantry) line(item int64, name string, qty int64, mods ...int64) AddLine {
	l := AddLine{ItemID: item, Name: name, Qty: qty, UnitPrice: inr(4000), Station: "bar", TaxRate: 5000, HSN: "996331"}
	for _, m := range mods {
		l.Modifiers = append(l.Modifiers, order.Modifier{ModifierID: m, Name: "choice", Price: inr(0)})
	}
	return l
}

// used is what the order's sold movements have taken, by stock item name,
// in whole units, and how many movements there are.
func (k pantry) used(t *testing.T, orderID int64) (map[string]int64, int) {
	t.Helper()
	rows, err := k.d.Queries().SoldForOrder(context.Background(), gen.SoldForOrderParams{VenueID: 1, OrderID: orderID})
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := k.d.Writer().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM stock_movements WHERE order_id = ? AND kind = 'sold'`, orderID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for name, id := range k.stock {
		for _, r := range rows {
			if r.StockItemID == id && r.Taken != 0 {
				out[name] = r.Taken / int64(stock.PerUnit)
			}
		}
	}
	return out, n
}

func TestACloseTakesWhatTheOrderUsed(t *testing.T) {
	k := newPantry(t)
	ctx := context.Background()
	id := must(t)(k.s.Open(ctx, k.m, OpenOrder{Channel: order.ChannelCounter})).OrderID
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.dosa, "Masala dosa", 2)))            // line 2
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.coffee, "Filter coffee", 1, k.oat))) // line 3
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.coffee, "Filter coffee", 1, k.dairy)))
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.dosa, "Masala dosa", 1))) // line 5, voided
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.coffee, "Filter coffee", 1, k.dairy)))
	must(t)(k.s.VoidLine(ctx, k.m, id, VoidLine{LineID: 5, ReasonCode: "wrong_item", AuthorisedBy: k.staffID}))
	res := must(t)(k.s.CompLine(ctx, k.m, id, CompLine{LineID: 6, ReasonCode: "regular", AuthorisedBy: k.staffID}))

	if got, n := k.used(t, id); n != 0 || len(got) != 0 {
		t.Fatalf("before the close the order has taken %v: stock goes at the close", got)
	}
	must(t)(k.s.TenderPayment(ctx, k.m, id, TenderPayment{Method: order.PaymentCash, Amount: res.State.Totals.BalanceDue}))
	must(t)(k.s.Close(ctx, k.m, id))

	// Two dosas; three coffees, one oat and two dairy (the comped one was
	// made); the voided dosa takes nothing.
	want := map[string]int64{"batter": 300, "powder": 30, "oat milk": 100, "milk": 200}
	got, n := k.used(t, id)
	if n != 4 || len(got) != len(want) {
		t.Fatalf("took %v in %d movements, want %v in 4", got, n, want)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: took %d, want %d", name, got[name], w)
		}
	}

	// Each movement keeps the dish its stock went into, a choice's stock
	// going to its line's dish (ADR-189): batter to the dosa, the rest to
	// the coffee.
	dishOf := map[string]int64{"batter": k.dosa, "powder": k.coffee, "milk": k.coffee, "oat milk": k.coffee}
	for name, dish := range dishOf {
		var got int64
		if err := k.d.Writer().QueryRowContext(ctx,
			`SELECT menu_item_id FROM stock_movements WHERE order_id = ? AND stock_item_id = ?`, id, k.stock[name]).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != dish {
			t.Errorf("%s went to dish %d, want %d", name, got, dish)
		}
	}

	// Reopened and closed again with the same lines: nothing more is taken.
	must(t)(k.s.Reopen(ctx, k.m, id, Reopen{Reason: "wrong tender", AuthorisedBy: k.staffID}))
	must(t)(k.s.Close(ctx, k.m, id))
	if again, n := k.used(t, id); n != 4 || again["batter"] != 300 {
		t.Errorf("after closing again: %v in %d movements; the second close must write nothing", again, n)
	}

	// The movements are Asha's, in the open shift.
	var staff, shift int64
	if err := k.d.Writer().QueryRowContext(ctx,
		`SELECT staff_id, shift_id FROM stock_movements WHERE order_id = ? LIMIT 1`, id).Scan(&staff, &shift); err != nil {
		t.Fatal(err)
	}
	if staff != k.staffID || shift == 0 {
		t.Errorf("movement by staff %d in shift %d", staff, shift)
	}
}

// A bill of nothing but comps totals zero and closes with no invoice; the
// food was still made.
func TestAnAllCompOrderStillTakesItsStock(t *testing.T) {
	k := newPantry(t)
	ctx := context.Background()
	id := must(t)(k.s.Open(ctx, k.m, OpenOrder{Channel: order.ChannelCounter})).OrderID
	must(t)(k.s.AddLine(ctx, k.m, id, k.line(k.dosa, "Masala dosa", 1)))
	must(t)(k.s.CompLine(ctx, k.m, id, CompLine{LineID: 2, ReasonCode: "staff_meal", AuthorisedBy: k.staffID}))
	closed := must(t)(k.s.Close(ctx, k.m, id))
	if closed.State.Invoiced {
		t.Fatal("a zero bill must close without an invoice")
	}
	if got, _ := k.used(t, id); got["batter"] != 150 {
		t.Errorf("took %v, want 150 g of batter", got)
	}
}
