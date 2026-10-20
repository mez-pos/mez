package menu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mez-pos/mez/internal/domain/sheet"
)

// Groups of choices from a spreadsheet (phase 10, Task 11, ADR-302): a CSV
// file with a row for each choice, and on it the group the choice is in.
// Reading it adds nothing. It answers a plan: the groups to add, the groups
// already on the menu, and every problem with its row and column.
//
// The import's rules hold (ADR-288): it adds a group and never changes one
// that is there, one bad row refuses the file, and nothing is guessed. What
// a group is allowed to be is CleanGroup's, as it is for the editor.

// The choices file's columns.
const (
	ColGroup       = "Group"
	ColFewest      = "Fewest"
	ColMost        = "Most"
	ColChoice      = "Choice"
	ColChoicePrice = "Price"
	ColStart       = "Picked to start"
)

// ChoiceColumns is every column, in the template's order. Group and Choice
// must be in the file; Fewest and Most must be too, since every group needs
// them said once.
var ChoiceColumns = []string{ColGroup, ColFewest, ColMost, ColChoice, ColChoicePrice, ColStart}

var choiceHeadings = sheet.Headings{Columns: ChoiceColumns, Required: []string{ColGroup, ColFewest, ColMost, ColChoice}}

// ChoicesVenue is what a file of choices is read against.
type ChoicesVenue struct {
	Currency string
	// Groups are the names of every group on the menu, retired ones included.
	Groups []string
}

// GroupImport is a group a file adds.
type GroupImport struct {
	// Row is the row that first names it; the header is row 1.
	Row   int
	Group GroupDraft
}

// GroupSkip is a row left out because the menu already has a group of that
// name.
type GroupSkip struct {
	Row   int
	Group string
}

// ChoicesPlan is what importing a file of choices would do.
type ChoicesPlan struct {
	// Groups are in the order the file first names each.
	Groups  []GroupImport
	Skipped []GroupSkip
	// Examples is how many rows are of the template's own example groups,
	// left in the file. They are left out (ADR-305).
	Examples int
	Problems []ImportProblem
}

// OK reports whether the plan can be carried out: a group to add and
// nothing wrong.
func (p ChoicesPlan) OK() bool { return len(p.Problems) == 0 && len(p.Groups) > 0 }

// ReadChoices reads a CSV file against the menu and answers the groups it
// would add. It never fails: a file it cannot read is a plan with one
// problem, on row 0.
func ReadChoices(data []byte, v ChoicesVenue) ChoicesPlan {
	file := func(message string) ChoicesPlan { return ChoicesPlan{Problems: []ImportProblem{{Message: message}}} }
	records, problem := sheet.Read(data)
	if problem != "" {
		return file(problem)
	}
	columns, problem := choiceHeadings.Read(records[0])
	if problem != "" {
		return file(problem)
	}

	r := choicesReader{currency: v.Currency, have: make(map[string]bool, len(v.Groups)), groups: map[string]*readGroup{}}
	for _, name := range v.Groups {
		r.have[groupKey(name)] = true
	}
	seen := sheet.Rows(records, columns, r.read)
	if problem := sheet.CountProblem(seen, "Import"); problem != "" {
		return file(problem)
	}
	return r.finish()
}

func groupKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// readGroup is a group as the file has given it so far.
type readGroup struct {
	name string
	// firstRow names it first; rows holds the row of each choice, in order.
	firstRow int
	rows     []int
	choices  []ChoiceDraft
	// fewest and most are the bounds once a row has given them, and the row
	// that did.
	fewest, most       int64
	fewestRow, mostRow int
	// bad: a row of it had a problem. Part of a group is not a group.
	bad bool
}

type choicesReader struct {
	currency string
	have     map[string]bool
	groups   map[string]*readGroup
	order    []string
	skipped  []GroupSkip
	examples int
	problems []ImportProblem
}

// read reads one row: a choice of a group, a row for a group already there,
// or its problems.
func (r *choicesReader) read(row int, cells map[string]string) {
	var problems []ImportProblem
	problem := func(column, message string) {
		problems = append(problems, ImportProblem{Row: row, Column: column, Message: message})
	}

	name := cells[ColGroup]
	if sheet.IsExample(name) {
		r.examples++
		return
	}
	if name == "" {
		problem(ColGroup, "Say which group this choice is in.")
	} else if r.have[groupKey(name)] {
		// Which group it is comes first: one already there is left alone
		// whatever the row says.
		r.skipped = append(r.skipped, GroupSkip{Row: row, Group: name})
		return
	}

	var g *readGroup
	if name != "" {
		g = r.groupOf(name, row)
		r.bound(g, row, ColFewest, cells[ColFewest], &g.fewest, &g.fewestRow, problem)
		r.bound(g, row, ColMost, cells[ColMost], &g.most, &g.mostRow, problem)
	}
	choice := ChoiceDraft{Name: cells[ColChoice], Price: choicePrice(cells[ColChoicePrice], r.currency, problem)}
	if choice.Name == "" {
		problem(ColChoice, "Give the choice a name.")
	}
	choice.IsDefault = importYes(ColStart, cells[ColStart], problem)

	if len(problems) > 0 {
		r.problems = append(r.problems, problems...)
		if g != nil {
			g.bad = true
		}
		return
	}
	g.rows = append(g.rows, row)
	g.choices = append(g.choices, choice)
}

func (r *choicesReader) groupOf(name string, row int) *readGroup {
	key := groupKey(name)
	g, ok := r.groups[key]
	if !ok {
		g = &readGroup{name: name, firstRow: row}
		r.groups[key] = g
		r.order = append(r.order, key)
	}
	return g
}

// bound reads a group's fewest or most from a row. It is the group's, so it
// is said once: a row may leave it empty, and a row that gives it again
// must agree with the row that gave it first.
func (r *choicesReader) bound(g *readGroup, row int, column, cell string, value *int64, from *int, problem sheet.Report) {
	if cell == "" {
		return
	}
	n, err := strconv.ParseInt(cell, 10, 64)
	if err != nil || n < 0 {
		problem(column, fmt.Sprintf("%q is not a number of choices. Type a whole number like 0, 1 or 2.", cell))
		return
	}
	switch {
	case *from == 0:
		*value, *from = n, row
	case *value != n:
		problem(column, fmt.Sprintf("Row %d says %d for %s. Say it once, on the group's first row, or the same on each.", *from, *value, g.name))
	}
}

// choicePrice reads what a choice adds to the line's price. Empty is
// nothing; below zero takes off ("no cheese, less 20").
func choicePrice(cell, currency string, problem sheet.Report) int64 {
	if cell == "" {
		return 0
	}
	minor, wrong := sheet.Money(cell, currency, "price")
	if wrong != "" {
		problem(ColChoicePrice, wrong)
		return 0
	}
	return minor
}

// finish checks each group whole, as the editor would on Save, and answers
// the plan.
func (r *choicesReader) finish() ChoicesPlan {
	plan := ChoicesPlan{Skipped: r.skipped, Examples: r.examples, Problems: r.problems}
	for _, key := range r.order {
		g := r.groups[key]
		if g.bad {
			continue
		}
		if p := g.problem(); p != nil {
			plan.Problems = append(plan.Problems, *p)
			continue
		}
		draft, _ := CleanGroup(GroupDraft{Name: g.name, MinSelect: g.fewest, MaxSelect: g.most, Choices: g.choices})
		plan.Groups = append(plan.Groups, GroupImport{Row: g.firstRow, Group: draft})
	}
	return plan
}

// problem is what is wrong with a group as a whole, on the row and column
// an owner would fix it at; or nil.
func (g *readGroup) problem() *ImportProblem {
	switch {
	case g.fewestRow == 0:
		return &ImportProblem{Row: g.firstRow, Column: ColFewest, Message: fmt.Sprintf("Say the fewest of %s a guest must pick: 0 if it is optional, 1 if they must choose.", g.name)}
	case g.mostRow == 0:
		return &ImportProblem{Row: g.firstRow, Column: ColMost, Message: fmt.Sprintf("Say the most of %s a guest may pick.", g.name)}
	}
	_, p := CleanGroup(GroupDraft{Name: g.name, MinSelect: g.fewest, MaxSelect: g.most, Choices: g.choices})
	if p == nil {
		return nil
	}
	// CleanGroup names a form's field; a file has rows and columns.
	switch {
	case p.Field == "name":
		return &ImportProblem{Row: g.firstRow, Column: ColGroup, Message: p.Message}
	case p.Field == "min_select":
		return &ImportProblem{Row: g.fewestRow, Column: ColFewest, Message: p.Message}
	case p.Field == "max_select":
		return &ImportProblem{Row: g.mostRow, Column: ColMost, Message: p.Message}
	case p.Field == "modifiers":
		// Too many picked to start, or no choices at all.
		return &ImportProblem{Row: g.firstRow, Column: ColStart, Message: p.Message}
	}
	var i int
	if _, err := fmt.Sscanf(p.Field, "modifiers.%d.name", &i); err == nil && i < len(g.rows) {
		return &ImportProblem{Row: g.rows[i], Column: ColChoice, Message: p.Message}
	}
	return &ImportProblem{Row: g.firstRow, Column: ColGroup, Message: p.Message}
}

// ChoicesTemplate is the example rows of the file a venue fills in, a cell
// for each of ChoiceColumns: one group a guest must choose from, with its
// bounds said once and a choice picked to start, and one that is optional.
func ChoicesTemplate() [][]string {
	return [][]string{
		{sheet.ExamplePrefix + " Size", "1", "1", "Regular", "0", "yes"},
		{sheet.ExamplePrefix + " Size", "", "", "Large", "30", "no"},
		{sheet.ExamplePrefix + " Extras", "0", "2", "Extra shot", "40", ""},
		{sheet.ExamplePrefix + " Extras", "", "", "Vanilla syrup", "30", ""},
	}
}
