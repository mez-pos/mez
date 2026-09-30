package menus

import (
	"context"
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
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
}

// TaxClass is a rate items share.
type TaxClass struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
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
	// ModifierGroups are the ids of the groups it offers, in order. They are
	// edited in Task 2c; here they are shown.
	ModifierGroups []int64 `json:"modifier_groups"`
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
	}
	for i, r := range rows.Stations {
		out.Stations[i] = Station{ID: r.ID, Code: r.Code, Name: r.Name, Active: r.Active}
	}
	for i, r := range rows.Courses {
		out.Courses[i] = Course{ID: r.ID, Number: r.Number, Name: r.Name}
	}
	for i, r := range rows.TaxClasses {
		out.TaxClasses[i] = TaxClass{ID: r.ID, Name: r.Name, Rate: tax.Rate(r.Rate), Exempt: r.Exempt}
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
			ModifierGroups: linked,
		}
	}
	return out, nil
}
