package menus

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
)

func choice(name string, price int64, isDefault bool) ModifierInput {
	return ModifierInput{Name: name, Price: money.Amount{Minor: price, Currency: "INR"}, IsDefault: isDefault}
}

func milkInput() GroupInput {
	return GroupInput{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []ModifierInput{
		choice("Dairy", 0, true), choice("Oat", 4000, false), choice("Soy", 3000, false),
	}}
}

func (w *world) group(t *testing.T, venue int64, id int64) Group {
	t.Helper()
	for _, g := range w.editor(t, venue).Groups {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no group %d in the editor", id)
	return Group{}
}

// tillMenu is the menu a terminal reads.
func (w *world) tillMenu(t *testing.T, venue int64) menu.Menu {
	t.Helper()
	m, err := w.d.Menu(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func choiceNames(g Group) []string {
	out := make([]string, len(g.Modifiers))
	for i, m := range g.Modifiers {
		out[i] = m.Name
	}
	return out
}

func TestAGroupIsSavedWhole(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	id, err := w.s.CreateGroup(ctx, plain, milkInput())
	if err != nil {
		t.Fatal(err)
	}
	g := w.group(t, plain, id)
	if g.Name != "Milk" || g.MinSelect != 1 || !g.Active || !slices.Equal(choiceNames(g), []string{"Dairy", "Oat", "Soy"}) || !g.Modifiers[0].IsDefault {
		t.Fatalf("created = %+v", g)
	}
	dairy, oat, soy := g.Modifiers[0].ID, g.Modifiers[1].ID, g.Modifiers[2].ID

	// Reorder, reprice oat, drop soy, add almond.
	in := GroupInput{Name: "Milk choice", MinSelect: 0, MaxSelect: 1, Modifiers: []ModifierInput{
		{ID: oat, Name: "Oat", Price: money.Amount{Minor: 5000, Currency: "INR"}},
		{ID: dairy, Name: "Dairy", Price: money.Amount{Minor: 0, Currency: "INR"}, IsDefault: true},
		choice("Almond", 6000, false),
	}}
	if err := w.s.UpdateGroup(ctx, plain, id, in); err != nil {
		t.Fatal(err)
	}
	g = w.group(t, plain, id)
	if g.Name != "Milk choice" || g.MinSelect != 0 || !slices.Equal(choiceNames(g), []string{"Oat", "Dairy", "Almond"}) {
		t.Errorf("updated = %+v", g)
	}
	if g.Modifiers[0].ID != oat || g.Modifiers[0].Price.Minor != 5000 || g.Modifiers[1].ID != dairy {
		t.Errorf("choices kept their ids: %+v", g.Modifiers)
	}
	for _, m := range g.Modifiers {
		if m.ID == soy {
			t.Error("soy was left out, so it is retired and off the form")
		}
	}

	// Soy's id is still the group's, retired: listing it again puts it back
	// on offer under its old id, as UpdateModifier says.
	stale := in
	stale.Modifiers = append(slices.Clone(in.Modifiers[:2]), ModifierInput{ID: soy, Name: "Soy", Price: money.Amount{Minor: 0, Currency: "INR"}})
	if err := w.s.UpdateGroup(ctx, plain, id, stale); err != nil {
		t.Errorf("a retired choice of this group listed by id comes back: %v", err)
	}
	if got := choiceNames(w.group(t, plain, id)); !slices.Equal(got, []string{"Oat", "Dairy", "Soy"}) {
		t.Errorf("choices = %v", got)
	}
}

func TestGroupsAreChecked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	other, err := w.s.CreateGroup(ctx, gst, milkInput())
	if err != nil {
		t.Fatal(err)
	}
	mine, err := w.s.CreateGroup(ctx, plain, milkInput())
	if err != nil {
		t.Fatal(err)
	}
	foreign := w.group(t, gst, other).Modifiers[0].ID

	tests := []struct {
		name  string
		edit  func(*GroupInput)
		field string
	}{
		{"the domain's rules", func(in *GroupInput) { in.MinSelect = 4; in.MaxSelect = 4 }, "min_select"},
		{"a choice's name", func(in *GroupInput) { in.Modifiers[1].Name = " " }, "modifiers.1.name"},
		{"a price in another currency", func(in *GroupInput) { in.Modifiers[2].Price.Currency = "USD" }, "modifiers.2.price"},
		{"another group's choice", func(in *GroupInput) { in.Modifiers[0].ID = foreign }, "modifiers.0.id"},
		{"a choice listed twice", func(in *GroupInput) {
			id := w.group(t, plain, mine).Modifiers[0].ID
			in.Modifiers[0].ID, in.Modifiers[1].ID = id, id
		}, "modifiers.1.id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := w.version(t, plain)
			in := milkInput()
			tt.edit(&in)
			if err := w.s.UpdateGroup(ctx, plain, mine, in); fieldOf(err) != tt.field {
				t.Errorf("err = %v, want a refusal on %q", err, tt.field)
			}
			if w.version(t, plain) != before {
				t.Error("a refused save must change nothing")
			}
		})
	}

	t.Run("a new group cannot claim an existing choice", func(t *testing.T) {
		in := milkInput()
		in.Modifiers[0].ID = foreign
		if _, err := w.s.CreateGroup(ctx, plain, in); fieldOf(err) != "modifiers.0.id" {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("another venue's group is not found", func(t *testing.T) {
		if err := w.s.UpdateGroup(ctx, plain, other, milkInput()); !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("update: %v", err)
		}
		if err := w.s.SetGroupActive(ctx, plain, other, false); !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("retire: %v", err)
		}
	})
}

// What the till offers is the test that matters: an item's groups in order,
// a retired group gone from it and back when brought back.
func TestItemsOfferGroups(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	coffee := w.category(t, plain, "Coffee")
	latte := w.create(t, plain, w.item(plain, coffee, "Latte", 18000, ""))
	milk, err := w.s.CreateGroup(ctx, plain, milkInput())
	if err != nil {
		t.Fatal(err)
	}
	extras, err := w.s.CreateGroup(ctx, plain, GroupInput{Name: "Extras", MaxSelect: 2, Modifiers: []ModifierInput{choice("Extra shot", 3000, false)}})
	if err != nil {
		t.Fatal(err)
	}

	offered := func() []int64 {
		for _, it := range w.tillMenu(t, plain).Items {
			if it.ID == latte {
				return it.Groups
			}
		}
		t.Fatal("latte is not on the till")
		return nil
	}

	steps := []struct {
		name string
		do   func() error
		want []int64
	}{
		{"offer extras then milk", func() error { return w.s.SetItemGroups(ctx, plain, latte, []int64{extras, milk}) }, []int64{extras, milk}},
		{"reorder", func() error { return w.s.SetItemGroups(ctx, plain, latte, []int64{milk, extras}) }, []int64{milk, extras}},
		{"retire milk", func() error { return w.s.SetGroupActive(ctx, plain, milk, false) }, []int64{extras}},
		{"bring milk back", func() error { return w.s.SetGroupActive(ctx, plain, milk, true) }, []int64{milk, extras}},
		{"offer none", func() error { return w.s.SetItemGroups(ctx, plain, latte, []int64{}) }, []int64{}},
	}
	for _, st := range steps {
		if err := st.do(); err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got := offered(); !slices.Equal(got, st.want) {
			t.Errorf("after %s the till offers %v, want %v", st.name, got, st.want)
		}
	}

	if err := w.s.SetItemGroups(ctx, plain, latte, []int64{milk, milk}); fieldOf(err) != "ids" {
		t.Errorf("a group twice: %v", err)
	}
	other, err := w.s.CreateGroup(ctx, gst, milkInput())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.s.SetItemGroups(ctx, plain, latte, []int64{other}); fieldOf(err) != "ids" {
		t.Errorf("another venue's group: %v", err)
	}
	if err := w.s.SetItemGroups(ctx, plain, 999, []int64{milk}); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("a missing item: %v", err)
	}

	// The till's choice sheet shows milk's choices as saved.
	if err := w.s.SetItemGroups(ctx, plain, latte, []int64{milk}); err != nil {
		t.Fatal(err)
	}
	for _, g := range w.tillMenu(t, plain).Groups {
		if g.ID == milk && (len(g.Modifiers) != 3 || g.Modifiers[1].Name != "Oat" || g.Modifiers[1].Price.Minor != 4000 || !g.Modifiers[0].IsDefault) {
			t.Errorf("the till's milk = %+v", g)
		}
	}
	if e := w.editor(t, plain); !slices.Equal(findItem(e, latte).ModifierGroups, []int64{milk}) {
		t.Errorf("the editor's latte offers %v", findItem(e, latte).ModifierGroups)
	}
}
