package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/menu"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// ErrVenueNotFound is a menu asked for a venue that does not exist.
var ErrVenueNotFound = errors.New("db: venue not found")

// MenuVersion is the venue's menu version, for a cheap "has it changed".
func (d *DB) MenuVersion(ctx context.Context, venueID int64) (int64, error) {
	v, err := d.Queries().MenuVersion(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("db: menu version of venue %d: %w", venueID, err)
	}
	return v, nil
}

// Menu reads the venue's menu. Every read is in one read-only transaction,
// so the version returned is the version of the rows returned: a menu
// edited halfway through the read cannot come back as the old number over
// half the new rows.
func (d *DB) Menu(ctx context.Context, venueID int64) (menu.Menu, error) {
	tx, err := d.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return menu.Menu{}, fmt.Errorf("db: starting the menu read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	m, err := readMenu(ctx, gen.New(tx), venueID)
	if err != nil {
		return menu.Menu{}, err
	}
	if err := tx.Commit(); err != nil {
		return menu.Menu{}, fmt.Errorf("db: finishing the menu read: %w", err)
	}
	return m, nil
}

// menuRows is every row the menu is built from.
type menuRows struct {
	stations   []gen.ActiveStationsRow
	courses    []gen.CoursesForVenueRow
	taxClasses []gen.TaxClassesForVenueRow
	categories []gen.ActiveCategoriesRow
	items      []gen.ActiveItemsRow
	groups     []gen.ActiveModifierGroupsRow
	modifiers  []gen.ActiveModifiersRow
	links      []gen.ItemModifierGroupsForVenueRow
}

func readMenu(ctx context.Context, q *gen.Queries, venueID int64) (menu.Menu, error) {
	venue, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return menu.Menu{}, fmt.Errorf("%w: %d", ErrVenueNotFound, venueID)
	}
	if err != nil {
		return menu.Menu{}, fmt.Errorf("db: venue %d: %w", venueID, err)
	}
	version, err := q.MenuVersion(ctx, venueID)
	if err != nil {
		return menu.Menu{}, fmt.Errorf("db: menu version of venue %d: %w", venueID, err)
	}
	rows, err := readMenuRows(ctx, q, venueID)
	if err != nil {
		return menu.Menu{}, fmt.Errorf("db: menu of venue %d: %w", venueID, err)
	}
	return assembleMenu(version, venue.Currency, rows)
}

func readMenuRows(ctx context.Context, q *gen.Queries, venueID int64) (menuRows, error) {
	var r menuRows
	var err error
	if r.stations, err = q.ActiveStations(ctx, venueID); err != nil {
		return r, fmt.Errorf("stations: %w", err)
	}
	if r.courses, err = q.CoursesForVenue(ctx, venueID); err != nil {
		return r, fmt.Errorf("courses: %w", err)
	}
	if r.taxClasses, err = q.TaxClassesForVenue(ctx, venueID); err != nil {
		return r, fmt.Errorf("tax classes: %w", err)
	}
	if r.categories, err = q.ActiveCategories(ctx, venueID); err != nil {
		return r, fmt.Errorf("categories: %w", err)
	}
	if r.items, err = q.ActiveItems(ctx, venueID); err != nil {
		return r, fmt.Errorf("items: %w", err)
	}
	if r.groups, err = q.ActiveModifierGroups(ctx, venueID); err != nil {
		return r, fmt.Errorf("modifier groups: %w", err)
	}
	if r.modifiers, err = q.ActiveModifiers(ctx, venueID); err != nil {
		return r, fmt.Errorf("modifiers: %w", err)
	}
	if r.links, err = q.ItemModifierGroupsForVenue(ctx, venueID); err != nil {
		return r, fmt.Errorf("item modifier groups: %w", err)
	}
	return r, nil
}

// assembleMenu nests the rows. An item whose station has been retired is
// left off the menu, because a line rung up for it would go to a station
// no screen is watching; the same goes for a link to a retired group.
func assembleMenu(version int64, currency string, r menuRows) (menu.Menu, error) {
	m := menu.Menu{
		Version: version, Currency: currency,
		Stations: make([]menu.Station, 0, len(r.stations)), Courses: make([]menu.Course, 0, len(r.courses)),
		Categories: make([]menu.Category, 0, len(r.categories)), Items: make([]menu.Item, 0, len(r.items)),
		Groups: make([]menu.ModifierGroup, 0, len(r.groups)),
	}
	stations := make(map[int64]string, len(r.stations))
	for _, s := range r.stations {
		stations[s.ID] = s.Code
		m.Stations = append(m.Stations, menu.Station{ID: s.ID, Code: s.Code, Name: s.Name})
	}
	courses := make(map[int64]int64, len(r.courses))
	for _, c := range r.courses {
		courses[c.ID] = c.Number
		m.Courses = append(m.Courses, menu.Course{ID: c.ID, Number: c.Number, Name: c.Name})
	}
	classes := make(map[int64]gen.TaxClassesForVenueRow, len(r.taxClasses))
	for _, t := range r.taxClasses {
		classes[t.ID] = t
	}
	for _, c := range r.categories {
		m.Categories = append(m.Categories, menu.Category{ID: c.ID, Name: c.Name})
	}

	groups, err := assembleGroups(currency, r.groups, r.modifiers)
	if err != nil {
		return menu.Menu{}, err
	}
	links := make(map[int64][]int64)
	for _, l := range r.links {
		if _, live := groups.index[l.GroupID]; live {
			links[l.ItemID] = append(links[l.ItemID], l.GroupID)
		}
	}
	m.Groups = groups.list

	for _, row := range r.items {
		station, ok := stations[row.StationID]
		if !ok {
			continue
		}
		item, err := assembleItem(currency, row, station, courses, classes)
		if err != nil {
			return menu.Menu{}, err
		}
		item.Groups = links[row.ID]
		if item.Groups == nil {
			item.Groups = []int64{}
		}
		m.Items = append(m.Items, item)
	}
	return m, nil
}

func assembleItem(currency string, row gen.ActiveItemsRow, station string,
	courses map[int64]int64, classes map[int64]gen.TaxClassesForVenueRow) (menu.Item, error) {
	course, ok := courses[row.CourseID]
	if !ok {
		return menu.Item{}, fmt.Errorf("db: item %d names course %d, which the menu does not have", row.ID, row.CourseID)
	}
	class, ok := classes[row.TaxClassID]
	if !ok {
		return menu.Item{}, fmt.Errorf("db: item %d names tax class %d, which the menu does not have", row.ID, row.TaxClassID)
	}
	price, err := money.New(row.Price, currency)
	if err != nil {
		return menu.Item{}, fmt.Errorf("db: price of item %d: %w", row.ID, err)
	}
	return menu.Item{
		ID: row.ID, CategoryID: row.CategoryID, Name: row.Name, Price: price, HSN: row.Hsn,
		TaxRate: tax.Rate(class.Rate), TaxExempt: class.Exempt, Station: station, Course: course,
	}, nil
}

type assembledGroups struct {
	list  []menu.ModifierGroup
	index map[int64]int
}

func assembleGroups(currency string, groups []gen.ActiveModifierGroupsRow, mods []gen.ActiveModifiersRow) (assembledGroups, error) {
	out := assembledGroups{list: make([]menu.ModifierGroup, 0, len(groups)), index: make(map[int64]int, len(groups))}
	for _, g := range groups {
		out.index[g.ID] = len(out.list)
		out.list = append(out.list, menu.ModifierGroup{
			ID: g.ID, Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Modifiers: []menu.Modifier{},
		})
	}
	for _, row := range mods {
		i, ok := out.index[row.GroupID]
		if !ok {
			continue // a live modifier in a retired group
		}
		if row.Price < 0 {
			// Written before the writer refused them (ADR-071). A line with
			// it would be refused, so the till is never offered it.
			continue
		}
		price, err := money.New(row.Price, currency)
		if err != nil {
			return assembledGroups{}, fmt.Errorf("db: price of modifier %d: %w", row.ID, err)
		}
		out.list[i].Modifiers = append(out.list[i].Modifiers, menu.Modifier{
			ID: row.ID, Name: row.Name, Price: price, IsDefault: row.IsDefault,
		})
	}
	return out, nil
}
