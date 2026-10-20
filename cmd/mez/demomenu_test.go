package main

import (
	"context"
	"testing"
	"unicode/utf8"

	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/service/menus"
)

// TestDemoMenuCardsLookFinished: the demo is what a screen is built and shown
// on, and most venues will have no photos on day one (rebuild.md, rule 5), so
// every item carries the description and dietary mark its card draws, and a
// few are popular (ADR-147).
func TestDemoMenuCardsLookFinished(t *testing.T) {
	dir, args := setupArgs(t, "-demo-menu")
	runSetupIn(t, args)
	m, err := opened(t, dir).Menu(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	marks := map[menu.Diet]int{}
	popular, glutenFree := 0, 0
	for _, it := range m.Items {
		if n := utf8.RuneCountInString(it.Description); n == 0 || n > menus.MaxDescriptionLength {
			t.Errorf("%s: a description of %d characters", it.Name, n)
		}
		if it.Diet == menu.DietNone || !it.Diet.Valid() {
			t.Errorf("%s: diet %q", it.Name, it.Diet)
		}
		marks[it.Diet]++
		if it.Popular {
			popular++
		}
		if it.GlutenFree {
			glutenFree++
		}
	}
	if marks[menu.DietVeg] == 0 || marks[menu.DietNonVeg] == 0 || marks[menu.DietVegan] == 0 {
		t.Errorf("the demo should show every mark: %v", marks)
	}
	if popular < 3 || popular > len(m.Items)/2 {
		t.Errorf("%d of %d items are popular: a few, so the flag means something", popular, len(m.Items))
	}
	if glutenFree == 0 || glutenFree == len(m.Items) {
		t.Errorf("%d of %d items are gluten free", glutenFree, len(m.Items))
	}
}
