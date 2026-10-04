package db

import (
	"context"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// TestItemCardFieldsReadBack: the description, dietary mark, gluten-free and
// popular flags an item card shows go in with the menu and come out of the
// terminal's read; an item that says none of them has none (ADR-147).
func TestItemCardFieldsReadBack(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()
	spec := MenuSpec{
		Stations:   []StationSpec{{Code: "kitchen", Name: "Kitchen"}},
		Courses:    []CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []TaxClassSpec{{Name: "Food", Rate: tax.RateFromBP(500)}},
		Categories: []CategorySpec{{Name: "Food", Items: []ItemSpec{
			{Name: "Masala dosa", Price: 14000, TaxClass: "Food", Station: "kitchen", Course: 1,
				Description: "Crisp rice crepe, spiced potato.", Diet: menu.DietVeg, GlutenFree: true, Popular: true},
			{Name: "Egg toast", Price: 18000, TaxClass: "Food", Station: "kitchen", Course: 1, Diet: menu.DietNonVeg},
			{Name: "Plain", Price: 100, TaxClass: "Food", Station: "kitchen", Course: 1},
		}}},
	}
	if _, err := d.CreateMenu(ctx, 1, spec, menuNow); err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	m, err := d.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	dosa, egg, plain := m.Items[0], m.Items[1], m.Items[2]
	if dosa.Description != "Crisp rice crepe, spiced potato." || dosa.Diet != menu.DietVeg || !dosa.GlutenFree || !dosa.Popular {
		t.Errorf("dosa = %+v", dosa)
	}
	if egg.Diet != menu.DietNonVeg || egg.Description != "" || egg.GlutenFree || egg.Popular {
		t.Errorf("egg toast = %+v", egg)
	}
	if plain.Description != "" || plain.Diet != menu.DietNone || plain.GlutenFree || plain.Popular {
		t.Errorf("an item that says nothing has %+v", plain)
	}
}

// TestItemCardFieldsAreConstrained: the table refuses what the editor would,
// so a write that skipped the editor cannot leave a card that cannot be drawn.
func TestItemCardFieldsAreConstrained(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()
	if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, set string }{
		{"a mark that does not exist", `diet = 'keto'`},
		{"a description past 140 characters", `description = '` + strings.Repeat("a", 141) + `'`},
		{"gluten free that is neither", `gluten_free = 2`},
		{"popular that is neither", `popular = 2`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := d.Writer().ExecContext(ctx, `UPDATE items SET `+tt.set+` WHERE id = 1`); err == nil {
				t.Error("the table accepted it")
			}
		})
	}
	// The limits themselves are allowed.
	if _, err := d.Writer().ExecContext(ctx,
		`UPDATE items SET description = '`+strings.Repeat("a", 140)+`', diet = 'vegan', gluten_free = 1, popular = 1 WHERE id = 1`); err != nil {
		t.Errorf("the limits were refused: %v", err)
	}
}
