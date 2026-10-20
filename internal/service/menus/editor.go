package menus

import (
	"context"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// Editor is the whole menu as the back office edits it: every category and
// item, retired ones included, and the stations, courses and tax classes an
// item can name. Version is the menu version these rows are at.
type Editor struct {
	Version  int64  `json:"version"`
	Currency string `json:"currency"`
	// NeedsHSN: the venue's tax profile prints an HSN or SAC code on every
	// invoice line, so every item must have one.
	NeedsHSN   bool       `json:"needs_hsn"`
	Stations   []Station  `json:"stations"`
	Courses    []Course   `json:"courses"`
	TaxClasses []TaxClass `json:"tax_classes"`
	Categories []Category `json:"categories"`
	Items      []Item     `json:"items"`
	// Groups are every modifier group, retired ones included, by name.
	Groups []Group `json:"modifier_groups"`
}

// Group is a modifier group with the choices it offers, in order.
type Group struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	MinSelect int64      `json:"min_select"`
	MaxSelect int64      `json:"max_select"`
	Active    bool       `json:"active"`
	Modifiers []Modifier `json:"modifiers"`
}

// Modifier is one choice on offer.
type Modifier struct {
	ID        int64        `json:"id"`
	Name      string       `json:"name"`
	Price     money.Amount `json:"price"`
	IsDefault bool         `json:"is_default"`
}

// Station is where an item is made.
type Station struct {
	ID     int64  `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// Course is a numbered course.
type Course struct {
	ID     int64  `json:"id"`
	Number int64  `json:"number"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// TaxClass is a rate items share.
type TaxClass struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
	Active bool     `json:"active"`
}

// Category is a tab on the register, in the order it shows.
type Category struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// Item is one thing on the menu, in its category's order.
type Item struct {
	ID         int64        `json:"id"`
	CategoryID int64        `json:"category_id"`
	Name       string       `json:"name"`
	Price      money.Amount `json:"price"`
	HSN        string       `json:"hsn"`
	TaxClassID int64        `json:"tax_class_id"`
	StationID  int64        `json:"station_id"`
	CourseID   int64        `json:"course_id"`
	Active     bool         `json:"active"`
	// ModifierGroups are the ids of the groups it offers, in the order its
	// choice sheet shows them, retired groups included.
	ModifierGroups []int64 `json:"modifier_groups"`
	// SoldOutUntil is set while it is sold out for today (ADR-109).
	SoldOutUntil *time.Time `json:"sold_out_until,omitempty"`
	// Photo is the stored photo's name, or empty (ADR-110).
	Photo string `json:"photo,omitempty"`
	// What the card shows (ADR-147).
	Description string    `json:"description"`
	Diet        menu.Diet `json:"diet"`
	GlutenFree  bool      `json:"gluten_free"`
	Popular     bool      `json:"popular"`
}

// Editor reads the venue's menu for editing.
func (s *Service) Editor(ctx context.Context, venueID int64) (Editor, error) {
	rows, err := s.db.MenuEditor(ctx, venueID)
	if err != nil {
		return Editor{}, err
	}
	q := s.db.Queries()
	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return Editor{}, fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return Editor{}, fmt.Errorf("menus: reading the venue's tax profile: %w", err)
	}

	out := Editor{
		Version: rows.Version, Currency: venue.Currency, NeedsHSN: tax.NeedsHSN(taxes.Profile),
		Stations:   make([]Station, len(rows.Stations)),
		Courses:    make([]Course, len(rows.Courses)),
		TaxClasses: make([]TaxClass, len(rows.TaxClasses)),
		Categories: make([]Category, len(rows.Categories)),
		Items:      make([]Item, len(rows.Items)),
		Groups:     groupsOf(rows, venue.Currency),
	}
	for i, r := range rows.Stations {
		out.Stations[i] = Station{ID: r.ID, Code: r.Code, Name: r.Name, Active: r.Active}
	}
	for i, r := range rows.Courses {
		out.Courses[i] = Course{ID: r.ID, Number: r.Number, Name: r.Name, Active: r.Active}
	}
	for i, r := range rows.TaxClasses {
		out.TaxClasses[i] = TaxClass{ID: r.ID, Name: r.Name, Rate: tax.Rate(r.Rate), Exempt: r.Exempt, Active: r.Active}
	}
	for i, r := range rows.Categories {
		out.Categories[i] = Category{ID: r.ID, Name: r.Name, Active: r.Active}
	}
	groups := map[int64][]int64{}
	for _, l := range rows.Links {
		groups[l.ItemID] = append(groups[l.ItemID], l.GroupID)
	}
	for i, r := range rows.Items {
		linked := groups[r.ID]
		if linked == nil {
			linked = []int64{}
		}
		out.Items[i] = Item{
			ID: r.ID, CategoryID: r.CategoryID, Name: r.Name,
			Price: money.Amount{Minor: r.Price, Currency: venue.Currency}, HSN: r.Hsn,
			TaxClassID: r.TaxClassID, StationID: r.StationID, CourseID: r.CourseID, Active: r.Active,
			ModifierGroups: linked, Photo: r.Photo,
			Description: r.Description, Diet: menu.Diet(r.Diet), GlutenFree: r.GlutenFree, Popular: r.Popular,
		}
		if r.SoldOutUntil.Valid {
			until := r.SoldOutUntil.Time.UTC()
			out.Items[i].SoldOutUntil = &until
		}
	}
	return out, nil
}

// groupsOf is every group with its choices on offer, in order.
func groupsOf(rows db.MenuEditorRows, currency string) []Group {
	choices := map[int64][]Modifier{}
	for _, m := range rows.Modifiers {
		choices[m.GroupID] = append(choices[m.GroupID], Modifier{
			ID: m.ID, Name: m.Name, Price: money.Amount{Minor: m.Price, Currency: currency}, IsDefault: m.IsDefault,
		})
	}
	out := make([]Group, len(rows.Groups))
	for i, g := range rows.Groups {
		mods := choices[g.ID]
		if mods == nil {
			mods = []Modifier{}
		}
		out[i] = Group{ID: g.ID, Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Active: g.Active, Modifiers: mods}
	}
	return out
}
