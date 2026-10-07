package menu

import "fmt"

// Prices from a file (phase 10, Task 9, ADR-295): the menu's export with
// its Price column edited comes back, and the prices that differ are the
// changes. Reading it changes nothing. It answers a plan: each item whose
// price the file changes, with the price it has and the price it would
// have; how many rows leave their item as it is; the rows that are no item
// on the menu; and every problem with its row and column.
//
// Only the price is read. A file never renames, moves, retires or removes
// an item, and a row for something that is not on the menu adds nothing:
// that is the import's work.

// PriceVenue is what a file of prices is read against: every item on the
// menu, retired ones included.
type PriceVenue struct {
	Currency string
	Items    []PriceItem
}

// PriceItem is an item by its category's name and its own, and its price in
// minor units.
type PriceItem struct {
	ID       int64
	Category string
	Name     string
	Price    int64
}

// PriceChange is one item's price changing. Category and Name are the
// menu's own spelling. Old and New are minor units of the venue's currency.
type PriceChange struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row      int
	ItemID   int64
	Category string
	Name     string
	Old      int64
	New      int64
}

// PriceUnknown is a row that is no item on the menu, as the file spells it.
// It is left out and said, not a problem: an export with rows added for the
// import is still a file of prices.
type PriceUnknown struct {
	Row      int
	Category string
	Name     string
}

// PricePlan is what a file of prices would do.
type PricePlan struct {
	Changes []PriceChange
	// Unchanged is how many rows give their item the price it has.
	Unchanged int
	Unknown   []PriceUnknown
	Problems  []ImportProblem
}

// OK reports whether the plan can be carried out: it changes something and
// has nothing wrong. One bad row refuses the file, as it does an import.
func (p PricePlan) OK() bool { return len(p.Problems) == 0 && len(p.Changes) > 0 }

// ReadPrices reads a CSV file against the menu and answers the prices it
// would change. It never fails: a file it cannot read is a plan with one
// problem, on row 0. It takes what ReadImport takes (the export's columns,
// as a spreadsheet saves them) and reads Category, Name and Price.
func ReadPrices(data []byte, v PriceVenue) PricePlan {
	records, problem := importRecords(data)
	if problem != "" {
		return priceFileProblem(problem)
	}
	columns, problem := readHeading(records[0])
	if problem != "" {
		return priceFileProblem(problem)
	}

	r := newPriceReader(v)
	for i, record := range records[1:] {
		cells := importCells(record, columns)
		if len(cells) == 0 {
			continue
		}
		// Counted with the empty rows, as the spreadsheet numbers them.
		r.read(i+2, cells)
	}
	switch {
	case r.seen == 0:
		return priceFileProblem("The file has no rows under its headings.")
	case r.seen > MaxImportRows:
		return priceFileProblem(fmt.Sprintf("The file has %d rows. Send at most %d at a time.", r.seen, MaxImportRows))
	}
	return r.plan
}

func priceFileProblem(message string) PricePlan {
	return PricePlan{Problems: []ImportProblem{{Message: message}}}
}

// priceReader reads rows into a plan, remembering which items earlier rows
// named.
type priceReader struct {
	currency string
	plan     PricePlan
	// seen is how many rows had anything in them.
	seen int
	// items is every item by category and name. The menu allows two of one
	// name in a category, so a key may hold more than one.
	items map[string][]PriceItem
	// firstRow is the row that first named each item.
	firstRow map[string]int
}

func newPriceReader(v PriceVenue) *priceReader {
	items := make(map[string][]PriceItem, len(v.Items))
	for _, it := range v.Items {
		key := itemKey(it.Category, it.Name)
		items[key] = append(items[key], it)
	}
	return &priceReader{currency: v.Currency, items: items, firstRow: map[string]int{}}
}

// read reads one row into the plan: a change, a row that changes nothing,
// one that is no item, or its problems.
func (r *priceReader) read(row int, cells map[string]string) {
	r.seen++
	if r.seen > MaxImportRows {
		return
	}
	var problems []ImportProblem
	problem := func(column, message string) {
		problems = append(problems, ImportProblem{Row: row, Column: column, Message: message})
	}

	category, name := cells[ColCategory], cells[ColName]
	if category == "" {
		problem(ColCategory, "Give it a category.")
	}
	if name == "" {
		problem(ColName, "Give it a name.")
	}
	if len(problems) > 0 {
		r.plan.Problems = append(r.plan.Problems, problems...)
		return
	}

	// Which item it is comes first: a row that is none is left out whatever
	// its price says.
	key := itemKey(category, name)
	found := r.items[key]
	if len(found) == 0 {
		r.plan.Unknown = append(r.plan.Unknown, PriceUnknown{Row: row, Category: category, Name: name})
		return
	}
	item := found[0]
	switch first, twice := r.firstRow[key]; {
	case len(found) > 1:
		problem(ColName, fmt.Sprintf("%d items in %s are called %s, so this row could mean either. Change their prices in the menu, or rename one first.",
			len(found), item.Category, item.Name))
	case twice:
		problem(ColName, fmt.Sprintf("%s is already in this file, on row %d. Keep one of them.", item.Name, first))
	default:
		r.firstRow[key] = row
	}

	price := importPrice(cells[ColPrice], r.currency, problem)
	switch {
	case len(problems) > 0:
		r.plan.Problems = append(r.plan.Problems, problems...)
	case price == item.Price:
		r.plan.Unchanged++
	default:
		r.plan.Changes = append(r.plan.Changes, PriceChange{
			Row: row, ItemID: item.ID, Category: item.Category, Name: item.Name, Old: item.Price, New: price,
		})
	}
}
