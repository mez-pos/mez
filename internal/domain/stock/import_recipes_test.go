package stock

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

// kitchen is a menu and a store to give recipes against: a latte that has
// one, two dishes called Toast, and a retired syrup.
func kitchen() RecipesVenue {
	return RecipesVenue{
		Dishes: []RecipeDish{
			{ID: 1, Name: "Espresso", Category: "Coffee"},
			{ID: 2, Name: "Latte", Category: "Coffee", HasRecipe: true},
			{ID: 3, Name: "Masala dosa", Category: "Breakfast"},
			{ID: 4, Name: "Toast", Category: "Breakfast"},
			{ID: 5, Name: "Toast", Category: "Kids"},
		},
		Ingredients: []RecipeIngredient{
			{ID: 10, Name: "Coffee beans", Unit: Gram, Active: true},
			{ID: 11, Name: "Milk", Unit: Millilitre, Active: true},
			{ID: 12, Name: "Eggs", Unit: Each, Active: true},
			{ID: 13, Name: "Old syrup", Unit: Millilitre, Active: false},
		},
	}
}

const recipeHeader = "Dish,Ingredient,Amount\n"

func TestReadRecipesGivesEachDishItsRows(t *testing.T) {
	plan := ReadRecipes([]byte(recipeHeader+
		"Espresso,Coffee beans,18\n"+
		"masala dosa,MILK,\"1,000.5\"\n"+
		// A dish's rows need not be together.
		" espresso , milk ,0.5\n"+
		"Masala dosa,Eggs,2\n"), kitchen())
	if len(plan.Problems) != 0 || len(plan.Skipped) != 0 || plan.Blank != 0 {
		t.Fatalf("problems %+v, skipped %+v, blank %d", plan.Problems, plan.Skipped, plan.Blank)
	}
	if len(plan.Recipes) != 2 {
		t.Fatalf("recipes: %+v", plan.Recipes)
	}
	espresso, dosa := plan.Recipes[0], plan.Recipes[1]
	// The menu's spelling of the dish and the store's of the ingredient.
	if espresso.ItemID != 1 || espresso.Dish != "Espresso" || len(espresso.Parts) != 2 {
		t.Fatalf("espresso: %+v", espresso)
	}
	if espresso.Parts[0] != (RecipeLine{Row: 2, StockItemID: 10, Ingredient: "Coffee beans", Unit: Gram, Qty: 18 * PerUnit}) {
		t.Errorf("first part: %+v", espresso.Parts[0])
	}
	if espresso.Parts[1] != (RecipeLine{Row: 4, StockItemID: 11, Ingredient: "Milk", Unit: Millilitre, Qty: 500}) {
		t.Errorf("second part: %+v", espresso.Parts[1])
	}
	if dosa.ItemID != 3 || len(dosa.Parts) != 2 || dosa.Parts[0].Qty != 1000500 || dosa.Parts[1].Unit != Each {
		t.Errorf("dosa: %+v", dosa)
	}
	if !plan.OK() {
		t.Error("a plan with recipes and nothing wrong is not OK")
	}
}

func TestReadRecipesProblems(t *testing.T) {
	tests := []struct {
		name, row    string
		column, says string
	}{
		{"no dish", ",Milk,200", "Dish", "Say which dish"},
		{"a dish that is not on the menu", "Cortado,Milk,200", "Dish", "no dish called"},
		{"two dishes of one name", "Toast,Eggs,1", "Dish", "2 dishes are called Toast (in Breakfast, Kids)"},
		{"no ingredient", "Espresso,,18", "Ingredient", "Say which ingredient"},
		{"an ingredient that is not in the store", "Espresso,Oat milk,200", "Ingredient", "no ingredient called"},
		{"a retired ingredient", "Espresso,Old syrup,10", "Ingredient", "retired"},
		{"no amount", "Espresso,Milk,", "Amount", "Give how much one uses"},
		{"an amount of none", "Espresso,Milk,0", "Amount", "above zero"},
		{"an amount below zero", "Espresso,Milk,-5", "Amount", "above zero"},
		{"an amount in words", "Espresso,Milk,a splash", "Amount", "not an amount"},
		{"an amount with its unit typed in", "Espresso,Milk,200 ml", "Amount", "not an amount"},
		{"an amount with a decimal comma", "Espresso,Milk,\"0,5\"", "Amount", "point"},
		{"an amount finer than a thousandth", "Espresso,Milk,0.0001", "Amount", "3 decimal places"},
		{"an amount too large to keep", "Espresso,Milk,99999999999999", "Amount", "too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadRecipes([]byte(recipeHeader+tt.row+"\n"), kitchen())
			if len(plan.Recipes) != 0 {
				t.Errorf("a row with a problem gives no recipe: %+v", plan.Recipes)
			}
			if len(plan.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %+v", len(plan.Problems), plan.Problems)
			}
			p := plan.Problems[0]
			if p.Row != 2 || p.Column != tt.column || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, column %q, %q; want row 2, column %q, saying %q", p.Row, p.Column, p.Message, tt.column, tt.says)
			}
		})
	}
}

func TestReadRecipesReportsEveryProblemOnARow(t *testing.T) {
	plan := ReadRecipes([]byte(recipeHeader+"Cortado,Oat milk,a splash\n"), kitchen())
	var columns []string
	for _, p := range plan.Problems {
		columns = append(columns, p.Column)
	}
	if got := strings.Join(columns, "|"); got != "Dish|Ingredient|Amount" {
		t.Errorf("problems on %s", got)
	}
}

func TestReadRecipesPartOfARecipeIsNotARecipe(t *testing.T) {
	// One row of the espresso's is wrong: the good row is not given alone.
	plan := ReadRecipes([]byte(recipeHeader+"Espresso,Coffee beans,18\nEspresso,Oat milk,200\nMasala dosa,Eggs,2\n"), kitchen())
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 3 {
		t.Fatalf("problems: %+v", plan.Problems)
	}
	if len(plan.Recipes) != 1 || plan.Recipes[0].Dish != "Masala dosa" {
		t.Errorf("recipes: %+v", plan.Recipes)
	}
	if plan.OK() {
		t.Error("a plan with a problem is OK")
	}
}

func TestReadRecipesTheSameIngredientTwiceInADish(t *testing.T) {
	plan := ReadRecipes([]byte(recipeHeader+"Espresso,Milk,100\nMasala dosa,Milk,50\nEspresso,milk,200\n"), kitchen())
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 4 || plan.Problems[0].Column != "Ingredient" || !strings.Contains(plan.Problems[0].Message, "row 2") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadRecipesLeavesADishThatHasOneAlone(t *testing.T) {
	// Not checked further: its ingredient is not one, and it does not matter.
	plan := ReadRecipes([]byte(recipeHeader+"Latte,Unicorn milk,lots\nLATTE,Milk,200\nEspresso,Coffee beans,18\n"), kitchen())
	if len(plan.Problems) != 0 || len(plan.Recipes) != 1 || plan.Recipes[0].Dish != "Espresso" {
		t.Fatalf("recipes %+v, problems %+v", plan.Recipes, plan.Problems)
	}
	if len(plan.Skipped) != 2 || plan.Skipped[0] != (RecipeSkip{Row: 2, Dish: "Latte"}) || plan.Skipped[1].Row != 3 {
		t.Errorf("skipped: %+v", plan.Skipped)
	}
}

func TestReadRecipesARowLeftBlankIsLeftAlone(t *testing.T) {
	// The template's rows, with one filled in.
	plan := ReadRecipes([]byte(recipeHeader+"Espresso,,\nMasala dosa,Eggs,2\n"), kitchen())
	if len(plan.Problems) != 0 || plan.Blank != 1 || len(plan.Recipes) != 1 || plan.Recipes[0].Dish != "Masala dosa" {
		t.Errorf("recipes %+v, blank %d, problems %+v", plan.Recipes, plan.Blank, plan.Problems)
	}

	// Every row left blank: nothing wrong, and nothing to do.
	none := ReadRecipes([]byte(recipeHeader+"Espresso,,\nMasala dosa,,\n"), kitchen())
	if len(none.Problems) != 0 || none.Blank != 2 || none.OK() {
		t.Errorf("all blank: %+v", none)
	}
}

func TestReadRecipesFilesItCannotRead(t *testing.T) {
	tests := []struct{ name, data, says string }{
		{"empty", "", "empty"},
		{"a header and nothing under it", recipeHeader, "no rows"},
		{"no amount column", "Dish,Ingredient\nEspresso,Milk\n", "Amount"},
		{"a column it does not know", "Dish,Ingredient,Amount,Unit\nEspresso,Milk,200,ml\n", "Unit"},
		{"a spreadsheet, not a CSV", "PK\x03\x04\x14\x00", "CSV UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadRecipes([]byte(tt.data), kitchen())
			if len(plan.Recipes) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Row != 0 || !strings.Contains(plan.Problems[0].Message, tt.says) {
				t.Errorf("recipes %+v, problems %+v", plan.Recipes, plan.Problems)
			}
		})
	}
}

func TestReadRecipesAsASpreadsheetSavesIt(t *testing.T) {
	plan := ReadRecipes([]byte("\xef\xbb\xbfAMOUNT;dish;Ingredient\r\n18;Espresso;Coffee beans\r\n"), kitchen())
	if len(plan.Problems) != 0 || len(plan.Recipes) != 1 || plan.Recipes[0].Parts[0].Qty != 18*PerUnit {
		t.Errorf("recipes %+v, problems %+v", plan.Recipes, plan.Problems)
	}
}

// TestRecipesTemplate: a row for each dish with no recipe, in menu order,
// that reads back as nothing wrong and nothing to do until it is filled in.
func TestRecipesTemplate(t *testing.T) {
	rows := RecipesTemplate(kitchen())
	// The latte has a recipe; a row could not say which Toast.
	if len(rows) != 2 || rows[0][0] != "Espresso" || rows[1][0] != "Masala dosa" {
		t.Fatalf("rows: %q", rows)
	}
	for _, row := range rows {
		if len(row) != len(RecipeColumns) || row[1] != "" || row[2] != "" {
			t.Errorf("row: %q", row)
		}
	}

	file := func(rows [][]string) []byte {
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		w.UseCRLF = true
		if err := w.WriteAll(append([][]string{RecipeColumns}, rows...)); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	asItComes := ReadRecipes(file(rows), kitchen())
	if len(asItComes.Problems) != 0 || asItComes.Blank != 2 || len(asItComes.Recipes) != 0 {
		t.Errorf("as it comes: %+v", asItComes)
	}

	// Filled in, with a row added under a dish for a second ingredient.
	filled := [][]string{{"Espresso", "Coffee beans", "18"}, {"Espresso", "Milk", "20"}, rows[1]}
	plan := ReadRecipes(file(filled), kitchen())
	if len(plan.Problems) != 0 || len(plan.Recipes) != 1 || len(plan.Recipes[0].Parts) != 2 || plan.Blank != 1 {
		t.Errorf("filled in: %+v", plan)
	}

	if rows := RecipesTemplate(RecipesVenue{}); len(rows) != 0 {
		t.Errorf("a venue with no dishes: %q", rows)
	}
}
