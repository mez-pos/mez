package menus

import (
	"context"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/menu"
)

// TestItemCardFieldsAreSavedAndRead: what the editor saves is what the
// editor and the till read back, and a save replaces them all, so a field
// left out clears it (ADR-147).
func TestItemCardFieldsAreSavedAndRead(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")

	in := w.item(plain, cat, "Masala dosa", 14000, "")
	in.Description = "  Crisp rice crepe, spiced potato.  "
	in.Diet = menu.DietVegan
	in.GlutenFree = true
	in.Popular = true
	id := w.create(t, plain, in)

	got := findItem(w.editor(t, plain), id)
	if got.Description != "Crisp rice crepe, spiced potato." || got.Diet != menu.DietVegan || !got.GlutenFree || !got.Popular {
		t.Errorf("editor item = %+v; the description is kept trimmed", got)
	}
	till, err := w.d.Menu(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	var seen menu.Item
	for _, it := range till.Items {
		if it.ID == id {
			seen = it
		}
	}
	if seen.Description != got.Description || seen.Diet != menu.DietVegan || !seen.GlutenFree || !seen.Popular {
		t.Errorf("the till reads %+v", seen)
	}

	// A save is the whole item: the fields left out are cleared.
	bare := w.item(plain, cat, "Masala dosa", 14000, "")
	if err := w.s.UpdateItem(ctx, plain, id, bare); err != nil {
		t.Fatal(err)
	}
	cleared := findItem(w.editor(t, plain), id)
	if cleared.Description != "" || cleared.Diet != menu.DietNone || cleared.GlutenFree || cleared.Popular {
		t.Errorf("after a save with none: %+v", cleared)
	}
}

func TestItemCardFieldsAreChecked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	tests := []struct {
		name  string
		edit  func(*ItemInput)
		field string
	}{
		{"a mark that does not exist", func(in *ItemInput) { in.Diet = "keto" }, "diet"},
		{"a description past 140 characters", func(in *ItemInput) { in.Description = strings.Repeat("a", MaxDescriptionLength+1) }, "description"},
		{"a description with a line break", func(in *ItemInput) { in.Description = "Crisp\nrice crepe" }, "description"},
		{"a description with a Unicode line separator", func(in *ItemInput) { in.Description = "Crisp rice crepe" }, "description"},
		{"a description with a paragraph separator", func(in *ItemInput) { in.Description = "Crisp rice crepe" }, "description"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := w.item(plain, cat, "Dosa", 12000, "")
			tt.edit(&in)
			if _, err := w.s.CreateItem(ctx, plain, in); fieldOf(err) != tt.field {
				t.Errorf("refused field = %q (%v), want %q", fieldOf(err), err, tt.field)
			}
		})
	}
	t.Run("the limit, counted in characters", func(t *testing.T) {
		in := w.item(plain, cat, "Dosa", 12000, "")
		in.Description = strings.Repeat("é", MaxDescriptionLength)
		if _, err := w.s.CreateItem(ctx, plain, in); err != nil {
			t.Errorf("140 characters were refused: %v", err)
		}
	})
	t.Run("a zero-width joiner, which Indic scripts need, is allowed", func(t *testing.T) {
		in := w.item(plain, cat, "Kerala meals", 9000, "")
		in.Description = "ന‍മ" // Malayalam letters with a joiner between them
		if _, err := w.s.CreateItem(ctx, plain, in); err != nil {
			t.Errorf("a joiner was refused: %v", err)
		}
	})
	t.Run("a description of spaces is none", func(t *testing.T) {
		in := w.item(plain, cat, "Idli", 8000, "")
		in.Description = "   "
		id := w.create(t, plain, in)
		if got := findItem(w.editor(t, plain), id).Description; got != "" {
			t.Errorf("description = %q", got)
		}
	})
}
