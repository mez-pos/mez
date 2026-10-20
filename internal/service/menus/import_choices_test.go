package menus

import (
	"context"
	"encoding/csv"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/money"
)

const choicesSheet = "Group,Fewest,Most,Choice,Price,Picked to start\n" +
	"Size,1,1,Regular,0,yes\n" +
	"Size,,,Large,30,\n" +
	"Extras,0,2,Extra shot,40,\n" +
	"Extras,,,No cheese,-20,\n"

func (w *world) groupNamed(t *testing.T, venue int64, name string) Group {
	t.Helper()
	for _, g := range w.editor(t, venue).Groups {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("no group called %q", name)
	return Group{}
}

func TestPreviewChoicesWritesNothing(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)

	out, err := w.s.PreviewChoices(context.Background(), plain, []byte(choicesSheet))
	if err != nil {
		t.Fatal(err)
	}
	if out.Imported || len(out.ToAdd) != 2 || len(out.Problems) != 0 || len(out.Skipped) != 0 {
		t.Fatalf("preview: %+v", out)
	}
	size := out.ToAdd[0]
	if size.Row != 2 || size.Name != "Size" || size.MinSelect != 1 || size.MaxSelect != 1 || len(size.Choices) != 2 ||
		!size.Choices[0].IsDefault || size.Choices[1].Price.Minor != 3000 || size.Choices[1].Price.Currency != "INR" {
		t.Errorf("size: %+v", size)
	}
	if extras := out.ToAdd[1]; extras.Choices[1].Price.Minor != -2000 {
		t.Errorf("extras: %+v", extras)
	}
	if e := w.editor(t, plain); len(e.Groups) != 0 {
		t.Errorf("a preview wrote: %+v", e.Groups)
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: a preview is not an edit", got, before, w.nudged)
	}
}

func TestImportChoicesAddsGroupsAsOneEdit(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)

	out, err := w.s.ImportChoices(context.Background(), plain, []byte(choicesSheet))
	if err != nil {
		t.Fatalf("importing: %v (problems %+v)", err, out.Problems)
	}
	if !out.Imported || len(out.ToAdd) != 2 {
		t.Fatalf("import: %+v", out)
	}
	if got := w.version(t, plain); got != before+1 {
		t.Errorf("version %d, want %d: one import is one edit", got, before+1)
	}
	if !slices.Equal(w.nudged, []int64{plain}) {
		t.Errorf("nudged %v, want venue %d once", w.nudged, plain)
	}

	size := w.groupNamed(t, plain, "Size")
	if size.MinSelect != 1 || size.MaxSelect != 1 || !size.Active || len(size.Modifiers) != 2 {
		t.Fatalf("size: %+v", size)
	}
	// In the file's order, which is the order the choice sheet shows them.
	if m := size.Modifiers; m[0].Name != "Regular" || !m[0].IsDefault || m[1].Name != "Large" || m[1].Price.Minor != 3000 || m[1].IsDefault {
		t.Errorf("size's choices: %+v", m)
	}
	if extras := w.groupNamed(t, plain, "Extras"); extras.MinSelect != 0 || extras.MaxSelect != 2 || extras.Modifiers[1].Price.Minor != -2000 {
		t.Errorf("extras: %+v", extras)
	}
}

func TestImportChoicesRefusesAFileWithAProblem(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)
	// Sweetness asks for two and offers one: a group the till could never complete.
	out, err := w.s.ImportChoices(context.Background(), plain, []byte(choicesSheet+"Sweetness,2,2,Less sugar,0,\n"))
	if !errors.Is(err, ErrImportRefused) {
		t.Fatalf("err = %v, want ErrImportRefused", err)
	}
	if out.Imported || len(out.Problems) != 1 || out.Problems[0].Row != 6 || out.Problems[0].Column != menu.ColFewest {
		t.Errorf("refusal: %+v", out)
	}
	if e := w.editor(t, plain); len(e.Groups) != 0 {
		t.Errorf("a refused import wrote: %+v", e.Groups)
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: nothing changed", got, before, w.nudged)
	}
}

func TestImportChoicesLeavesAGroupThatIsThereAlone(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if _, err := w.s.CreateGroup(ctx, plain, GroupInput{Name: "size", MinSelect: 0, MaxSelect: 3, Modifiers: []ModifierInput{{Name: "Tiny", Price: money.Amount{Currency: "INR"}}}}); err != nil {
		t.Fatal(err)
	}
	w.nudged = nil

	out, err := w.s.ImportChoices(ctx, plain, []byte(choicesSheet))
	if err != nil {
		t.Fatal(err)
	}
	if !out.Imported || len(out.ToAdd) != 1 || out.ToAdd[0].Name != "Extras" || len(out.Skipped) != 2 {
		t.Fatalf("import: %+v", out)
	}
	if g := w.groupNamed(t, plain, "size"); g.MaxSelect != 3 || len(g.Modifiers) != 1 || g.Modifiers[0].Name != "Tiny" {
		t.Errorf("an import changed a group that was there: %+v", g)
	}

	// The same file again: both groups are there now.
	before := w.version(t, plain)
	again, err := w.s.ImportChoices(ctx, plain, []byte(choicesSheet))
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 4 || w.version(t, plain) != before {
		t.Errorf("the same file again: %+v", again)
	}
}

func TestImportChoicesKeepsToItsVenue(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if _, err := w.s.CreateGroup(ctx, gst, GroupInput{Name: "Size", MinSelect: 1, MaxSelect: 1, Modifiers: []ModifierInput{{Name: "One", Price: money.Amount{Currency: "INR"}}}}); err != nil {
		t.Fatal(err)
	}
	// Venue 2 has a Size; venue 1 does not, so it is added there and only there.
	out, err := w.s.ImportChoices(ctx, plain, []byte(choicesSheet))
	if err != nil || len(out.ToAdd) != 2 {
		t.Fatalf("import: %v %+v", err, out)
	}
	if g := w.editor(t, gst).Groups; len(g) != 1 || len(g[0].Modifiers) != 1 {
		t.Errorf("venue 2 was written to: %+v", g)
	}
}

// TestChoicesThenItems is the order a new venue works in: the choices, then
// the items that offer them. The items' file names the groups, and what it
// exports names them back.
func TestChoicesThenItems(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if _, err := w.s.ImportChoices(ctx, plain, []byte(choicesSheet)); err != nil {
		t.Fatal(err)
	}
	const items = "Category,Name,Price,Choices\nCoffee,Latte,180,Extras; Size\nCoffee,Espresso,120,\nCoffee,Mocha,200,Sweetness\n"

	// One row names a group the menu does not have.
	out, err := w.s.ImportMenu(ctx, plain, []byte(items))
	if !errors.Is(err, ErrImportRefused) || len(out.Problems) != 1 || out.Problems[0].Row != 4 || out.Problems[0].Column != menu.ColChoices {
		t.Fatalf("a group the menu does not have: %v %+v", err, out.Problems)
	}

	done := w.importMenu(t, plain, strings.Replace(items, "Sweetness", "size", 1))
	if !done.Imported || len(done.ToAdd) != 3 {
		t.Fatalf("import: %+v", done)
	}
	size, extras := w.groupNamed(t, plain, "Size").ID, w.groupNamed(t, plain, "Extras").ID
	offered := map[string][]int64{}
	for _, it := range w.editor(t, plain).Items {
		offered[it.Name] = it.ModifierGroups
	}
	// In the order the cell gave them.
	if !slices.Equal(offered["Latte"], []int64{extras, size}) || len(offered["Espresso"]) != 0 || !slices.Equal(offered["Mocha"], []int64{size}) {
		t.Errorf("offered: %v (size %d, extras %d)", offered, size, extras)
	}

	rows := w.export(t, plain)
	choices := slices.Index(menu.ExportColumns, menu.ColChoices)
	if rows[0][1] != "Latte" || rows[0][choices] != "Extras; Size" || rows[1][choices] != "" {
		t.Errorf("the export: %q", rows)
	}
	// A retired group is not named: a row could not offer it.
	if err := w.s.SetGroupActive(ctx, plain, extras, false); err != nil {
		t.Fatal(err)
	}
	if got := w.export(t, plain)[0][choices]; got != "Size" {
		t.Errorf("with Extras retired the export names %q", got)
	}
	// And the export still goes back in as nothing wrong and nothing to add.
	var file strings.Builder
	if err := csv.NewWriter(&file).WriteAll(append([][]string{menu.ExportColumns}, w.export(t, plain)...)); err != nil {
		t.Fatal(err)
	}
	if back, err := w.s.PreviewImport(ctx, plain, []byte(file.String())); err != nil || len(back.Problems) != 0 || len(back.ToAdd) != 0 {
		t.Errorf("the export, checked: %v %+v", err, back)
	}
}

func TestChoicesTemplateImportsAtItsOwnVenue(t *testing.T) {
	w := newWorld(t)
	var file strings.Builder
	if err := csv.NewWriter(&file).WriteAll(append([][]string{menu.ChoiceColumns}, unmarked(w.s.ChoicesTemplate(), 0)...)); err != nil {
		t.Fatal(err)
	}
	out, err := w.s.PreviewChoices(context.Background(), plain, []byte(file.String()))
	if err != nil || len(out.Problems) != 0 || len(out.ToAdd) != 2 {
		t.Errorf("the template, previewed: %v %+v", err, out)
	}
}

func TestImportChoicesRefusesAFileTooLarge(t *testing.T) {
	w := newWorld(t)
	big := []byte("Group,Fewest,Most,Choice\n" + strings.Repeat("x", MaxImportBytes))
	if _, err := w.s.PreviewChoices(context.Background(), plain, big); fieldOf(err) != "file" {
		t.Errorf("preview: %v", err)
	}
	if _, err := w.s.ImportChoices(context.Background(), plain, big); fieldOf(err) != "file" {
		t.Errorf("import: %v", err)
	}
}
