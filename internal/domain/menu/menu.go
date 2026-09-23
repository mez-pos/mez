// Package menu is what a venue sells, as a terminal reads it.
//
// It is data, not storage: internal/db builds it and internal/api sends it,
// and neither has to know about the other to agree on its shape.
package menu

import (
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
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
