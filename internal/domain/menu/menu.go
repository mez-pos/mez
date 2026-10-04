// Package menu is what a venue sells, as a terminal reads it.
//
// It is data, not storage: internal/db builds it and internal/api sends it,
// and neither has to know about the other to agree on its shape.
package menu

import (
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// Menu is everything a terminal sells, as one document at one version.
//
// Items carry the facts a line snapshots, already resolved: the station's
// code, the course's number, the tax class's rate. A terminal rings an item
// up from what is here, with no second lookup, and what it sends is what
// order.LineAdded records.
type Menu struct {
	Version  int64
	Currency string

	Stations   []Station
	Courses    []Course
	Categories []Category
	Items      []Item
	Groups     []ModifierGroup
}

// Station is where items are made.
type Station struct {
	ID   int64
	Code string
	Name string
}

// Course is a numbered course.
type Course struct {
	ID     int64
	Number int64
	Name   string
}

// Category is a tab on the register, in the order it shows.
type Category struct {
	ID   int64
	Name string
}

// Diet is the mark on an item's card: a vegetarian dot, a non-vegetarian one,
// or vegan. DietNone is no mark, which is what an item has until someone says
// (ADR-147). Gluten free is a flag of its own, not a fifth mark.
type Diet string

// The marks. The strings are what is stored and sent.
const (
	DietNone   Diet = ""
	DietVeg    Diet = "veg"
	DietNonVeg Diet = "non_veg"
	DietVegan  Diet = "vegan"
)

// Valid reports whether d is one of the marks, or none.
func (d Diet) Valid() bool {
	switch d {
	case DietNone, DietVeg, DietNonVeg, DietVegan:
		return true
	}
	return false
}

// Item is one thing on sale.
type Item struct {
	ID         int64
	CategoryID int64
	Name       string
	Price      money.Amount
	HSN        string
	TaxRate    tax.Rate
	TaxExempt  bool
	Station    string // the station's code
	Course     int64  // the course's number
	Groups     []int64
	// SoldOutUntil is when a sold-out item is back on sale: the start of the
	// next trading day. Nil is on sale. The terminal compares it with its own
	// clock, so the item comes back with no edit to the menu (ADR-109).
	SoldOutUntil *time.Time
	// Photo is the stored photo's name, or empty (ADR-110).
	Photo string
	// What the card shows (ADR-147): one line of description, a dietary mark,
	// whether it is gluten free, and whether it is among the venue's popular
	// items. All of it optional.
	Description string
	Diet        Diet
	GlutenFree  bool
	Popular     bool
}

// ModifierGroup is a set of choices with bounds on how many to pick.
type ModifierGroup struct {
	ID        int64
	Name      string
	MinSelect int64
	MaxSelect int64
	Modifiers []Modifier
}

// Modifier is one choice. Price is per unit of the line, and may be
// negative.
type Modifier struct {
	ID        int64
	Name      string
	Price     money.Amount
	IsDefault bool
}
