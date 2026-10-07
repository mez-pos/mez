package stock

import (
	"fmt"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/sheet"
)

// Recipes from a spreadsheet (phase 10, Task 10c, ADR-300): a CSV file with
// a row for each ingredient a dish takes: the dish, the ingredient, and how
// much of it one of the dish uses. Reading it writes nothing. It answers a
// plan: the recipes it gives, the dishes left alone because they have one,
// and every problem with its row and column.
//
// The import's rules hold (ADR-288): it gives a recipe to a dish that has
// none and never changes one that is there, one bad row refuses the file,
// and nothing is guessed. Dishes only: what a choice takes (the milk on
// Oat) is set on the screen.

// The recipe file's columns.
const (
	ColDish       = "Dish"
	ColIngredient = "Ingredient"
	ColAmount     = "Amount"
)

// RecipeColumns is every column, in the template's order. All three must be
// in the file.
var RecipeColumns = []string{ColDish, ColIngredient, ColAmount}

var recipeHeadings = sheet.Headings{Columns: RecipeColumns, Required: RecipeColumns}

// RecipesVenue is what a file of recipes is read against.
type RecipesVenue struct {
	// Dishes are the items on sale, in menu order.
	Dishes []RecipeDish
	// Ingredients are every stock item, retired ones included.
	Ingredients []RecipeIngredient
}

// RecipeDish is a menu item a recipe may be for.
type RecipeDish struct {
	ID       int64
	Name     string
	Category string
	// HasRecipe: it takes something from stock already.
	HasRecipe bool
}

// RecipeIngredient is a stock item a recipe may take.
type RecipeIngredient struct {
	ID     int64
	Name   string
	Unit   Unit
	Active bool
}

// DishRecipe is a recipe a file gives a dish.
type DishRecipe struct {
	ItemID int64
	Dish   string
	Parts  []RecipeLine
}

// RecipeLine is one ingredient of a recipe a file gives.
type RecipeLine struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row         int
	StockItemID int64
	Ingredient  string
	Unit        Unit
	Qty         Qty
}

// RecipeSkip is a row left out because its dish already has a recipe.
type RecipeSkip struct {
	Row  int
	Dish string
}

// RecipesPlan is what importing a file of recipes would do.
type RecipesPlan struct {
	// Recipes are in the order the file first names each dish.
	Recipes []DishRecipe
	Skipped []RecipeSkip
	// Blank is how many rows name a dish and nothing it takes: the
	// template's rows for dishes the owner left alone.
	Blank    int
	Problems []sheet.Problem
}

// OK reports whether the plan can be carried out: a recipe to give and
// nothing wrong.
func (p RecipesPlan) OK() bool { return len(p.Problems) == 0 && len(p.Recipes) > 0 }

// ReadRecipes reads a CSV file against the menu and the store and answers
// the recipes it would give. It never fails: a file it cannot read is a
// plan with one problem, on row 0.
func ReadRecipes(data []byte, v RecipesVenue) RecipesPlan {
	file := func(message string) RecipesPlan { return RecipesPlan{Problems: []sheet.Problem{{Message: message}}} }
	records, problem := sheet.Read(data)
	if problem != "" {
		return file(problem)
	}
	columns, problem := recipeHeadings.Read(records[0])
	if problem != "" {
		return file(problem)
	}

	r := newRecipesReader(v)
	seen := sheet.Rows(records, columns, r.read)
	if problem := sheet.CountProblem(seen, "Import"); problem != "" {
		return file(problem)
	}
	return r.finish()
}

type recipesReader struct {
	dishes      map[string][]RecipeDish
	ingredients map[string]RecipeIngredient
	// recipes are the dishes the file has named so far, by id, and order
	// the ids as it first named them.
	recipes  map[int64]*DishRecipe
	order    []int64
	skipped  []RecipeSkip
	blank    int
	problems []sheet.Problem
	// bad are the dishes a row of which had a problem. Part of a recipe is
	// not a recipe, so the file is refused; this keeps them out of the plan.
	bad map[int64]bool
}

func newRecipesReader(v RecipesVenue) *recipesReader {
	r := &recipesReader{
		dishes: make(map[string][]RecipeDish, len(v.Dishes)), ingredients: make(map[string]RecipeIngredient, len(v.Ingredients)),
		recipes: map[int64]*DishRecipe{}, bad: map[int64]bool{},
	}
	for _, d := range v.Dishes {
		key := nameKey(d.Name)
		r.dishes[key] = append(r.dishes[key], d)
	}
	for _, in := range v.Ingredients {
		r.ingredients[nameKey(in.Name)] = in
	}
	return r
}

// read reads one row: an ingredient of a dish's recipe, a row for a dish
// that has one or was left blank, or its problems.
func (r *recipesReader) read(row int, cells map[string]string) {
	var problems []sheet.Problem
	problem := func(column, message string) {
		problems = append(problems, sheet.Problem{Row: row, Column: column, Message: message})
	}

	dish, found := r.dish(cells[ColDish], problem)
	ingredientCell, amountCell := cells[ColIngredient], cells[ColAmount]
	if found {
		switch {
		// Which dish it is comes first: one that has a recipe is left alone
		// whatever the row says, as the import leaves an item that is there.
		case dish.HasRecipe:
			r.skipped = append(r.skipped, RecipeSkip{Row: row, Dish: dish.Name})
			return
		case ingredientCell == "" && amountCell == "":
			r.blank++
			return
		}
	}

	ingredient, ok := r.ingredient(ingredientCell, problem)
	qty := recipeAmount(amountCell, problem)
	if found && ok {
		for _, p := range r.recipeOf(dish).Parts {
			if p.StockItemID == ingredient.ID {
				problem(ColIngredient, fmt.Sprintf("%s is already in the recipe of %s, on row %d. Keep one of them.", ingredient.Name, dish.Name, p.Row))
				break
			}
		}
	}
	if len(problems) > 0 {
		r.problems = append(r.problems, problems...)
		if found {
			r.bad[dish.ID] = true
		}
		return
	}
	recipe := r.recipeOf(dish)
	recipe.Parts = append(recipe.Parts, RecipeLine{Row: row, StockItemID: ingredient.ID, Ingredient: ingredient.Name, Unit: ingredient.Unit, Qty: qty})
}

// recipeOf is the recipe the file is giving a dish, begun if this is its
// first row.
func (r *recipesReader) recipeOf(d RecipeDish) *DishRecipe {
	recipe, ok := r.recipes[d.ID]
	if !ok {
		recipe = &DishRecipe{ItemID: d.ID, Dish: d.Name}
		r.recipes[d.ID] = recipe
		r.order = append(r.order, d.ID)
	}
	return recipe
}

// dish is the menu item a cell names. Two dishes of one name is something
// the menu allows; a row for that name could mean either, and is not
// guessed at.
func (r *recipesReader) dish(cell string, problem sheet.Report) (RecipeDish, bool) {
	if cell == "" {
		problem(ColDish, "Say which dish.")
		return RecipeDish{}, false
	}
	found := r.dishes[nameKey(cell)]
	switch len(found) {
	case 0:
		problem(ColDish, fmt.Sprintf("There is no dish called %q on the menu. Add it to the menu first, or check the spelling.", cell))
		return RecipeDish{}, false
	case 1:
		return found[0], true
	}
	where := make([]string, len(found))
	for i, d := range found {
		where[i] = d.Category
	}
	problem(ColDish, fmt.Sprintf("%d dishes are called %s (in %s), so this row could mean any of them. Set their recipes on the Recipes screen.",
		len(found), found[0].Name, strings.Join(where, ", ")))
	return RecipeDish{}, false
}

func (r *recipesReader) ingredient(cell string, problem sheet.Report) (RecipeIngredient, bool) {
	if cell == "" {
		problem(ColIngredient, "Say which ingredient.")
		return RecipeIngredient{}, false
	}
	in, ok := r.ingredients[nameKey(cell)]
	switch {
	case !ok:
		problem(ColIngredient, fmt.Sprintf("There is no ingredient called %q. Add it on Stock levels first, or check the spelling.", cell))
	case !in.Active:
		problem(ColIngredient, fmt.Sprintf("The ingredient %s is retired. Bring it back first.", in.Name))
	default:
		return in, true
	}
	return RecipeIngredient{}, false
}

// recipeAmount reads how much of an ingredient one of the dish takes, in
// the ingredient's own unit: above zero, to a thousandth.
func recipeAmount(cell string, problem sheet.Report) Qty {
	if cell == "" {
		problem(ColAmount, "Give how much one uses.")
		return 0
	}
	n, wrong := sheet.Decimal(cell, 3, "amount")
	switch {
	case wrong != "":
		problem(ColAmount, wrong)
	case n <= 0:
		problem(ColAmount, "Give how much one uses, above zero.")
	case Qty(n) > MaxQty:
		problem(ColAmount, fmt.Sprintf("%q is too large.", cell))
	default:
		return Qty(n)
	}
	return 0
}

func (r *recipesReader) finish() RecipesPlan {
	plan := RecipesPlan{Skipped: r.skipped, Blank: r.blank, Problems: r.problems}
	for _, id := range r.order {
		if recipe := r.recipes[id]; !r.bad[id] && len(recipe.Parts) > 0 {
			plan.Recipes = append(plan.Recipes, *recipe)
		}
	}
	return plan
}

// RecipesTemplate is the rows of the file a venue fills in, a cell for each
// of RecipeColumns: a row for every dish that has no recipe, in menu order,
// with nothing in its Ingredient and Amount. The owner fills those in, and
// adds a row under a dish for each further ingredient; a dish left as it
// came is left alone. Two dishes of one name are left out: a row could not
// say which.
func RecipesTemplate(v RecipesVenue) [][]string {
	named := make(map[string]int, len(v.Dishes))
	for _, d := range v.Dishes {
		named[nameKey(d.Name)]++
	}
	var rows [][]string
	for _, d := range v.Dishes {
		if !d.HasRecipe && named[nameKey(d.Name)] == 1 {
			rows = append(rows, []string{d.Name, "", ""})
		}
	}
	return rows
}
