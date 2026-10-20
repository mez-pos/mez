package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mez-pos/mez/internal/domain/sheet"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// The menu from a spreadsheet (phase 10, Task 7, ADR-288): a CSV file with a
// header row and one item on each row under it. Reading it adds nothing. It
// answers a plan: the rows to add, the categories they need, the rows already
// on the menu, and every problem with the row and column it is in.

// The columns, by the names the template gives them. A header is matched
// without regard to case, spaces, hyphens or underscores.
const (
	ColCategory    = "Category"
	ColName        = "Name"
	ColPrice       = "Price"
	ColTaxClass    = "Tax class"
	ColHSN         = "HSN"
	ColStation     = "Station"
	ColCourse      = "Course"
	ColDescription = "Description"
	ColDiet        = "Diet"
	ColGlutenFree  = "Gluten free"
	ColPopular     = "Popular"
	// ColChoices names the groups of choices an item offers, in the order
	// its choice sheet shows them, with a semicolon between (ADR-302).
	ColChoices = "Choices"
)

// ImportColumns is every column, in the template's order. The first three
// must be in the file; a file may leave any of the others out.
var ImportColumns = []string{
	ColCategory, ColName, ColPrice, ColTaxClass, ColHSN, ColStation, ColCourse,
	ColDescription, ColDiet, ColGlutenFree, ColPopular, ColChoices,
}

var requiredColumns = []string{ColCategory, ColName, ColPrice}

// MaxImportRows is the most rows one file may hold. A venue's whole menu is
// well under it, and it bounds what one request can ask the database to do.
const MaxImportRows = sheet.MaxRows

// MaxDescriptionLength is the longest an item's description may be, in
// characters: one line on a card.
const MaxDescriptionLength = 140

// ImportVenue is what a file is read against: what the venue has now.
type ImportVenue struct {
	Currency string
	// NeedsHSN: the venue's tax profile prints an HSN or SAC code on every
	// invoice line, so every item must have one.
	NeedsHSN bool
	// Each in the order the venue keeps them, retired ones included.
	Categories []ImportChoice
	TaxClasses []ImportChoice
	Stations   []ImportChoice
	Courses    []ImportChoice
	// Groups are the groups of choices, retired ones included.
	Groups []ImportChoice
	// Items are every item on the menu, retired ones included.
	Items []ImportHave
}

// ImportChoice is something a cell may name.
type ImportChoice struct {
	ID   int64
	Name string
	// Also is another thing it answers to: a station's code, a course's number.
	Also   string
	Active bool
}

// ImportHave is an item already on the menu, by its category's name and its own.
type ImportHave struct {
	Category string
	Name     string
}

// ImportRow is one item to add.
type ImportRow struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row int
	// Category is the category's name, and CategoryID its id, or 0 for one
	// the import has to make first.
	Category   string
	CategoryID int64
	Name       string
	// Price is in minor units of the venue's currency.
	Price       int64
	HSN         string
	TaxClassID  int64
	StationID   int64
	CourseID    int64
	Description string
	Diet        Diet
	GlutenFree  bool
	Popular     bool
	// Groups are the ids of the groups of choices it offers, in order.
	Groups []int64
}

// ImportSkip is a row left out because the menu already has that item in
// that category. An import adds; it never changes what is there.
type ImportSkip struct {
	Row      int
	Category string
	Name     string
}

// ImportProblem is why a row cannot be added. Row 0 is a problem with the
// file as a whole, and has no column.
type ImportProblem = sheet.Problem

// ImportPlan is what importing a file would do.
type ImportPlan struct {
	Rows []ImportRow
	// NewCategories are the categories to make, in the order the file first
	// names them.
	NewCategories []string
	Skipped       []ImportSkip
	// Examples is how many rows are the template's own examples, left in
	// the file. They are left out (ADR-305).
	Examples int
	Problems []ImportProblem
}

// OK reports whether the plan can be carried out: it has something to add
// and nothing wrong. One bad row refuses the file, so a menu is never half
// of what its owner wrote.
func (p ImportPlan) OK() bool { return len(p.Problems) == 0 && len(p.Rows) > 0 }

// ReadImport reads a CSV file against the venue and answers what importing
// it would do. It never fails: a file it cannot read is a plan with one
// problem, on row 0.
func ReadImport(data []byte, v ImportVenue) ImportPlan {
	records, problem := sheet.Read(data)
	if problem != "" {
		return fileProblem(problem)
	}
	columns, problem := importHeadings.Read(records[0])
	if problem != "" {
		return fileProblem(problem)
	}

	r := newImportReader(v)
	seen := sheet.Rows(records, columns, r.read)
	if problem := sheet.CountProblem(seen, "Import"); problem != "" {
		return fileProblem(problem)
	}
	return r.plan
}

// importHeadings is what an import's first row may hold. Status is the
// export's own column: taken and not read, since a file never retires an
// item or brings one back (ADR-293).
var importHeadings = sheet.Headings{Columns: ImportColumns, Also: []string{ColStatus}, Required: requiredColumns}

func fileProblem(message string) ImportPlan {
	return ImportPlan{Problems: []ImportProblem{{Message: message}}}
}

// importReader reads rows into a plan, remembering what earlier rows named.
type importReader struct {
	venue ImportVenue
	plan  ImportPlan
	// have is every item on the menu, and firstRow every item an earlier row
	// of the file gave, each by category and name.
	have     map[string]bool
	firstRow map[string]int
	// newCategory is the spelling the file first gave a category it adds.
	newCategory map[string]string
}

func newImportReader(v ImportVenue) *importReader {
	have := make(map[string]bool, len(v.Items))
	for _, it := range v.Items {
		have[itemKey(it.Category, it.Name)] = true
	}
	return &importReader{venue: v, have: have, firstRow: map[string]int{}, newCategory: map[string]string{}}
}

func itemKey(category, name string) string {
	return strings.ToLower(strings.TrimSpace(category)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}

// read reads one row into the plan: an item to add, one already there, or
// its problems, every one of them, so the file is fixed in one pass.
func (r *importReader) read(row int, cells map[string]string) {
	// A template's example is not the venue's item, whatever else it says.
	if sheet.IsExample(cells[ColName]) {
		r.plan.Examples++
		return
	}
	var problems []ImportProblem
	problem := func(column, message string) {
		problems = append(problems, ImportProblem{Row: row, Column: column, Message: message})
	}

	out := ImportRow{Row: row}
	var retired bool
	out.Category, out.CategoryID, retired = r.category(cells[ColCategory], problem)
	out.Name = importName(ColName, cells[ColName], problem)

	// Who it is comes first: a row for an item already there is left alone
	// whatever else it says, and the same item twice is one mistake, not two.
	if out.Category != "" && out.Name != "" {
		key := itemKey(out.Category, out.Name)
		if r.have[key] {
			r.plan.Skipped = append(r.plan.Skipped, ImportSkip{Row: row, Category: out.Category, Name: out.Name})
			return
		}
		if first, twice := r.firstRow[key]; twice {
			problem(ColName, fmt.Sprintf("%s is already in this file, on row %d. Keep one of them.", out.Name, first))
		} else {
			r.firstRow[key] = row
		}
	}
	if retired {
		// First, where the column is, before whatever the name had wrong.
		problems = append([]ImportProblem{{
			Row: row, Column: ColCategory,
			Message: fmt.Sprintf("The category %s is retired. Bring it back in the menu first.", out.Category),
		}}, problems...)
	}

	out.Price = importPrice(cells[ColPrice], r.venue.Currency, problem)
	out.TaxClassID = importChoice(ColTaxClass, "tax class", cells[ColTaxClass], r.venue.TaxClasses, problem)
	out.HSN = importHSN(cells[ColHSN], r.venue.NeedsHSN, problem)
	out.StationID = importChoice(ColStation, "station", cells[ColStation], r.venue.Stations, problem)
	out.CourseID = importChoice(ColCourse, "course", cells[ColCourse], r.venue.Courses, problem)
	out.Description = importDescription(cells[ColDescription], problem)
	out.Diet = importDiet(cells[ColDiet], problem)
	out.GlutenFree = importYes(ColGlutenFree, cells[ColGlutenFree], problem)
	out.Popular = importYes(ColPopular, cells[ColPopular], problem)
	out.Groups = importGroups(cells[ColChoices], r.venue.Groups, problem)

	if len(problems) > 0 {
		r.plan.Problems = append(r.plan.Problems, problems...)
		return
	}
	if out.CategoryID == 0 {
		r.addCategory(out.Category)
	}
	r.plan.Rows = append(r.plan.Rows, out)
}

// reporter is how a cell's reader says what is wrong with it.
type reporter = sheet.Report

// category is the category a cell names: the venue's, by its own spelling
// and id, or a new one by the spelling the file first gave it and id 0.
//
// A retired category is answered as retired, not reported: whether that is
// a problem depends on the row. An item already in it is left alone, as the
// menu's own export gives it back (ADR-293); a new item cannot go into it.
func (r *importReader) category(cell string, problem reporter) (name string, id int64, retired bool) {
	if cell == "" {
		problem(ColCategory, "Give it a category.")
		return "", 0, false
	}
	for _, c := range r.venue.Categories {
		if strings.EqualFold(c.Name, cell) {
			return c.Name, c.ID, !c.Active
		}
	}
	name = importName(ColCategory, cell, problem)
	if first, ok := r.newCategory[strings.ToLower(name)]; ok {
		return first, 0, false
	}
	return name, 0, false
}

// addCategory notes a category the import has to make, once.
func (r *importReader) addCategory(name string) {
	key := strings.ToLower(name)
	if _, ok := r.newCategory[key]; ok {
		return
	}
	r.newCategory[key] = name
	r.plan.NewCategories = append(r.plan.NewCategories, name)
}

// importName is a category's or an item's name, or "" with its problem.
func importName(column, cell string, problem reporter) string {
	switch {
	case cell == "":
		problem(column, "Give it a name.")
	case utf8.RuneCountInString(cell) > MaxNameLength:
		problem(column, fmt.Sprintf("Keep the name to %d characters.", MaxNameLength))
	case strings.ContainsFunc(cell, sheet.IsLineBreak):
		problem(column, "A name is one line: take out the line break.")
	default:
		return cell
	}
	return ""
}

// importPrice reads a price cell as minor units. It reads digits, never a
// float: "120.5" is 12050 paise because the point is moved, not because
// anything was multiplied.
func importPrice(cell, currency string, problem reporter) int64 {
	if cell == "" {
		problem(ColPrice, "Give it a price. Type 0 for something that is free.")
		return 0
	}
	minor, wrong := sheet.Money(cell, currency, "price")
	switch {
	case wrong != "":
		problem(ColPrice, wrong)
	case minor < 0:
		problem(ColPrice, "Give a price of zero or more.")
	default:
		return minor
	}
	return 0
}

// importChoice is the id of the tax class, station or course a cell names,
// by its name or what it also answers to. An empty cell is the only active
// one there is, and a problem when there is a choice to make: a tax rate is
// not something to guess for a whole menu.
func importChoice(column, what, cell string, choices []ImportChoice, problem reporter) int64 {
	var active []ImportChoice
	for _, c := range choices {
		if c.Active {
			active = append(active, c)
		}
	}
	names := make([]string, len(active))
	for i, c := range active {
		names[i] = c.Name
	}
	use := "Use " + strings.Join(names, ", ") + "."

	if len(active) == 0 {
		problem(column, fmt.Sprintf("This venue has no %s to use. Add one in the back office first.", what))
		return 0
	}
	if cell == "" {
		if len(active) == 1 {
			return active[0].ID
		}
		problem(column, fmt.Sprintf("Say which %s. %s", what, use))
		return 0
	}
	for _, c := range choices {
		if !strings.EqualFold(c.Name, cell) && (c.Also == "" || !strings.EqualFold(c.Also, cell)) {
			continue
		}
		if !c.Active {
			problem(column, fmt.Sprintf("The %s %s is retired. %s", what, c.Name, use))
			return 0
		}
		return c.ID
	}
	problem(column, fmt.Sprintf("There is no %s called %q. %s", what, cell, use))
	return 0
}

// importHSN checks an HSN or SAC code: required where the venue's invoices
// show one on every line, and well formed wherever it is given (ADR-108).
func importHSN(cell string, needs bool, problem reporter) string {
	switch {
	case cell == "" && !needs:
		return ""
	case !tax.ValidHSN(cell) && needs:
		problem(ColHSN, "Every item needs an HSN or SAC code of 4, 6 or 8 digits: invoices show it.")
	case !tax.ValidHSN(cell):
		problem(ColHSN, "An HSN or SAC code is 4, 6 or 8 digits. It is not needed here; you can leave it empty.")
	default:
		return cell
	}
	return ""
}

func importDescription(cell string, problem reporter) string {
	switch {
	case utf8.RuneCountInString(cell) > MaxDescriptionLength:
		problem(ColDescription, fmt.Sprintf("Keep the description to %d characters: it is one line on the item's card.", MaxDescriptionLength))
	case strings.ContainsFunc(cell, sheet.IsLineBreak):
		problem(ColDescription, "The description is one line: take out the line breaks.")
	default:
		return cell
	}
	return ""
}

// importDiet reads a dietary mark however it is commonly typed.
func importDiet(cell string, problem reporter) Diet {
	switch sheet.Fold(cell) {
	case "", "none", "no":
		return DietNone
	case "veg", "vegetarian":
		return DietVeg
	case "nonveg", "nonvegetarian":
		return DietNonVeg
	case "vegan":
		return DietVegan
	}
	problem(ColDiet, fmt.Sprintf("%q is not a mark. Type veg, non-veg or vegan, or leave it empty.", cell))
	return DietNone
}

// importYes reads a yes or no. Empty is no.
func importYes(column, cell string, problem reporter) bool {
	yes, ok := sheet.Yes(cell)
	if !ok {
		problem(column, fmt.Sprintf("%q: type yes or no, or leave it empty.", cell))
	}
	return yes
}

// ChoicesSeparator is what stands between the groups an item offers, in a
// Choices cell. A comma would be read as the end of the cell by half the
// spreadsheets that save the file.
const ChoicesSeparator = ";"

// importGroups reads the groups of choices an item offers: their names with
// a semicolon between, in the order its choice sheet shows them. Each must
// be one group on the menu and in use. A name two groups share could mean
// either, and is not guessed at.
func importGroups(cell string, groups []ImportChoice, problem reporter) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for _, name := range strings.Split(cell, ChoicesSeparator) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var found []ImportChoice
		for _, g := range groups {
			if strings.EqualFold(g.Name, name) {
				found = append(found, g)
			}
		}
		switch {
		case len(found) == 0:
			problem(ColChoices, fmt.Sprintf("There is no group of choices called %q. Import the choices first, or check the spelling.", name))
		case len(found) > 1:
			problem(ColChoices, fmt.Sprintf("%d groups of choices are called %s, so this could mean either. Offer it from the item's editor.", len(found), found[0].Name))
		case !found[0].Active:
			problem(ColChoices, fmt.Sprintf("The group of choices %s is retired. Bring it back first.", found[0].Name))
		case seen[found[0].ID]:
			problem(ColChoices, fmt.Sprintf("%s is in this cell twice.", found[0].Name))
		default:
			seen[found[0].ID] = true
			ids = append(ids, found[0].ID)
		}
	}
	return ids
}
