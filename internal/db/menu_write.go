package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// ErrMenuSpec is a menu spec that refers to something it does not define.
var ErrMenuSpec = errors.New("db: menu spec")

// MenuSpec is a whole menu to add, written by name rather than by id: an
// item names its category, station, course, tax class and modifier groups,
// and CreateMenu resolves them. It is what the demo menu is written in, and
// what an import will be. Prices are minor units of the venue's currency.
type MenuSpec struct {
	Stations   []StationSpec
	Courses    []CourseSpec
	TaxClasses []TaxClassSpec
	Groups     []ModifierGroupSpec
	Categories []CategorySpec
}

// StationSpec is a station, by its code.
type StationSpec struct {
	Code string
	Name string
}

// CourseSpec is a course, by its number.
type CourseSpec struct {
	Number int64
	Name   string
}

// TaxClassSpec is a tax class, by its name.
type TaxClassSpec struct {
	Name   string
	Rate   tax.Rate
	Exempt bool
}

// ModifierGroupSpec is a group of choices, by its name.
type ModifierGroupSpec struct {
	Name      string
	MinSelect int64
	MaxSelect int64
	Modifiers []ModifierSpec
}

// ModifierSpec is one choice.
type ModifierSpec struct {
	Name      string
	Price     int64
	IsDefault bool
}

// CategorySpec is a category and the items in it, in order.
type CategorySpec struct {
	Name  string
	Items []ItemSpec
}

// ItemSpec is an item. Station, Course, TaxClass and Groups name entries
// elsewhere in the spec.
type ItemSpec struct {
	Name     string
	Price    int64
	HSN      string
	TaxClass string
	Station  string
	Course   int64
	Groups   []string
}

// CreateMenu adds a menu to a venue in one transaction and bumps the menu
// version, returning the new one. A spec naming something it does not
// define is refused and nothing is written.
func (d *DB) CreateMenu(ctx context.Context, venueID int64, spec MenuSpec, now time.Time) (int64, error) {
	var version int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := WriteMenu(ctx, q, venueID, spec, now)
		version = v
		return err
	})
	if err != nil {
		return 0, err
	}
	return version, nil
}

// WriteMenu is CreateMenu inside a transaction the caller already has:
// setup writes the demo menu in the same one that creates the venue, so a
// venue never exists with half a menu. It bumps the menu version too.
func WriteMenu(ctx context.Context, q *gen.Queries, venueID int64, spec MenuSpec, now time.Time) (int64, error) {
	w := menuWriter{q: q, venueID: venueID, now: now.UTC(),
		stations: map[string]int64{}, courses: map[int64]int64{},
		classes: map[string]int64{}, groups: map[string]int64{}}
	if err := w.write(ctx, spec); err != nil {
		return 0, fmt.Errorf("db: writing the menu of venue %d: %w", venueID, err)
	}
	version, err := q.BumpMenuVersion(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("db: bumping the menu version of venue %d: %w", venueID, err)
	}
	return version, nil
}

type menuWriter struct {
	q       *gen.Queries
	venueID int64
	now     time.Time

	stations map[string]int64
	courses  map[int64]int64
	classes  map[string]int64
	groups   map[string]int64
}

func (w *menuWriter) write(ctx context.Context, spec MenuSpec) error {
	if err := w.writeStations(ctx, spec.Stations); err != nil {
		return err
	}
	if err := w.writeCourses(ctx, spec.Courses); err != nil {
		return err
	}
	if err := w.writeTaxClasses(ctx, spec.TaxClasses); err != nil {
		return err
	}
	if err := w.writeGroups(ctx, spec.Groups); err != nil {
		return err
	}
	for i, c := range spec.Categories {
		if err := w.writeCategory(ctx, c, int64(i)); err != nil {
			return err
		}
	}
	return nil
}

func (w *menuWriter) writeStations(ctx context.Context, stations []StationSpec) error {
	for i, s := range stations {
		id, err := w.q.CreateStation(ctx, gen.CreateStationParams{
			VenueID: w.venueID, Code: s.Code, Name: s.Name, SortOrder: int64(i), CreatedAt: w.now, UpdatedAt: w.now,
		})
		if err != nil {
			return fmt.Errorf("station %q: %w", s.Code, err)
		}
		w.stations[s.Code] = id
	}
	return nil
}

func (w *menuWriter) writeCourses(ctx context.Context, courses []CourseSpec) error {
	for _, c := range courses {
		id, err := w.q.CreateCourse(ctx, gen.CreateCourseParams{
			VenueID: w.venueID, Number: c.Number, Name: c.Name, CreatedAt: w.now, UpdatedAt: w.now,
		})
		if err != nil {
			return fmt.Errorf("course %d: %w", c.Number, err)
		}
		w.courses[c.Number] = id
	}
	return nil
}

func (w *menuWriter) writeTaxClasses(ctx context.Context, classes []TaxClassSpec) error {
	for _, t := range classes {
		if err := t.Rate.Validate(); err != nil {
			return fmt.Errorf("tax class %q: %w", t.Name, err)
		}
		id, err := w.q.CreateTaxClass(ctx, gen.CreateTaxClassParams{
			VenueID: w.venueID, Name: t.Name, Rate: int64(t.Rate), Exempt: t.Exempt, CreatedAt: w.now, UpdatedAt: w.now,
		})
		if err != nil {
			return fmt.Errorf("tax class %q: %w", t.Name, err)
		}
		w.classes[t.Name] = id
	}
	return nil
}

func (w *menuWriter) writeGroups(ctx context.Context, groups []ModifierGroupSpec) error {
	for _, g := range groups {
		id, err := w.q.CreateModifierGroup(ctx, gen.CreateModifierGroupParams{
			VenueID: w.venueID, Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect,
			CreatedAt: w.now, UpdatedAt: w.now,
		})
		if err != nil {
			return fmt.Errorf("modifier group %q: %w", g.Name, err)
		}
		w.groups[g.Name] = id
		for i, m := range g.Modifiers {
			// The orders domain refuses a negative modifier price, so a menu
			// that offered one would offer a choice no till could ring up.
			if m.Price < 0 {
				return fmt.Errorf("%w: modifier %q in %q costs %d, below nothing", ErrMenuSpec, m.Name, g.Name, m.Price)
			}
			if _, err := w.q.CreateModifier(ctx, gen.CreateModifierParams{
				VenueID: w.venueID, GroupID: id, Name: m.Name, Price: m.Price, IsDefault: m.IsDefault,
				SortOrder: int64(i), CreatedAt: w.now, UpdatedAt: w.now,
			}); err != nil {
				return fmt.Errorf("modifier %q in %q: %w", m.Name, g.Name, err)
			}
		}
	}
	return nil
}

func (w *menuWriter) writeCategory(ctx context.Context, c CategorySpec, order int64) error {
	categoryID, err := w.q.CreateCategory(ctx, gen.CreateCategoryParams{
		VenueID: w.venueID, Name: c.Name, SortOrder: order, CreatedAt: w.now, UpdatedAt: w.now,
	})
	if err != nil {
		return fmt.Errorf("category %q: %w", c.Name, err)
	}
	for i, it := range c.Items {
		if err := w.writeItem(ctx, categoryID, it, int64(i)); err != nil {
			return fmt.Errorf("item %q: %w", it.Name, err)
		}
	}
	return nil
}

func (w *menuWriter) writeItem(ctx context.Context, categoryID int64, it ItemSpec, order int64) error {
	station, ok := w.stations[it.Station]
	if !ok {
		return fmt.Errorf("%w: no station %q", ErrMenuSpec, it.Station)
	}
	course, ok := w.courses[it.Course]
	if !ok {
		return fmt.Errorf("%w: no course %d", ErrMenuSpec, it.Course)
	}
	class, ok := w.classes[it.TaxClass]
	if !ok {
		return fmt.Errorf("%w: no tax class %q", ErrMenuSpec, it.TaxClass)
	}
	itemID, err := w.q.CreateItem(ctx, gen.CreateItemParams{
		VenueID: w.venueID, CategoryID: categoryID, Name: it.Name, Price: it.Price, Hsn: it.HSN,
		TaxClassID: class, StationID: station, CourseID: course, SortOrder: order,
		CreatedAt: w.now, UpdatedAt: w.now,
	})
	if err != nil {
		return err
	}
	for i, name := range it.Groups {
		group, ok := w.groups[name]
		if !ok {
			return fmt.Errorf("%w: no modifier group %q", ErrMenuSpec, name)
		}
		if err := w.q.AttachModifierGroup(ctx, gen.AttachModifierGroupParams{
			VenueID: w.venueID, ItemID: itemID, GroupID: group, SortOrder: int64(i),
		}); err != nil {
			return fmt.Errorf("attaching %q: %w", name, err)
		}
	}
	return nil
}
