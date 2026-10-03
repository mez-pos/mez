// Package menus is the back office's menu editor (phase 10, Task 2,
// ADR-106): categories and items, created, changed, retired, brought back
// and put in order.
//
// Every write goes through db.DB.EditMenu, which bumps the menu version in
// the same transaction, and then nudges the stream, so every terminal reads
// the new menu. A line already rung up keeps what it copied
// (docs/02-domain.md, "Prices and the snapshot rule"): nothing here reaches
// an open ticket.
package menus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/tax"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// The refusals. The API maps them to codes; Invalid carries the field.
var (
	ErrCategoryNotFound = errors.New("no such category at this venue")
	ErrItemNotFound     = errors.New("no such item at this venue")
	// ErrStaleOrder is an ordering that is not exactly the rows there are:
	// the menu changed since the editor read it.
	ErrStaleOrder = errors.New("the order does not name exactly the rows there are")
)

// Invalid is an edit that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// MaxNameLength is the longest category or item name, in characters: the
// same limit as a modifier group's, which fits a register tile and a
// ticket line.
const MaxNameLength = menu.MaxNameLength

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options configures a Service.
type Options struct {
	DB    *db.DB
	Clock Clock
	// OnChange is told the venue after every committed edit: the hub's
	// Notify, so connected devices learn the menu's new version at once.
	OnChange func(venueID int64)
	// Photos is where item photos are kept. Without it, a photo cannot be
	// set, and none is served.
	Photos *PhotoStore
}

// Service is the menu editor.
type Service struct {
	db       *db.DB
	clock    Clock
	onChange func(venueID int64)
	photos   *PhotoStore
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil {
		return nil, errors.New("menus: a database is required")
	}
	if o.Clock == nil {
		o.Clock = systemClock{}
	}
	if o.OnChange == nil {
		o.OnChange = func(int64) {}
	}
	return &Service{db: o.DB, clock: o.Clock, onChange: o.OnChange, photos: o.Photos}, nil
}

// CategoryInput is a category as the editor sends it.
type CategoryInput struct {
	Name string `json:"name"`
}

// ItemInput is an item as the editor sends it. The ids are the venue's
// category, tax class, station and course.
type ItemInput struct {
	CategoryID int64        `json:"category_id"`
	Name       string       `json:"name"`
	Price      money.Amount `json:"price"`
	HSN        string       `json:"hsn"`
	TaxClassID int64        `json:"tax_class_id"`
	StationID  int64        `json:"station_id"`
	CourseID   int64        `json:"course_id"`
}

// CreateCategory adds a category at the end of the list.
func (s *Service) CreateCategory(ctx context.Context, venueID int64, in CategoryInput) (int64, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return 0, err
	}
	now := s.now()
	var id int64
	err = s.edit(ctx, venueID, func(q *gen.Queries) error {
		next, err := q.NextCategorySort(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: next category place: %w", err)
		}
		id, err = q.CreateCategory(ctx, gen.CreateCategoryParams{
			VenueID: venueID, Name: name, SortOrder: next, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, err
}

// RenameCategory changes a category's name.
func (s *Service) RenameCategory(ctx context.Context, venueID, id int64, in CategoryInput) error {
	name, err := cleanName(in.Name)
	if err != nil {
		return err
	}
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.RenameCategory(ctx, gen.RenameCategoryParams{Name: name, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrCategoryNotFound)
	})
}

// SetCategoryActive retires a category, taking every item in it off the
// till with it, or brings it back with the items it had.
func (s *Service) SetCategoryActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetCategoryActive(ctx, gen.SetCategoryActiveParams{Active: active, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrCategoryNotFound)
	})
}

// OrderCategories puts the categories in the order given. ids must name
// every category the venue has, retired ones included, once each.
func (s *Service) OrderCategories(ctx context.Context, venueID int64, ids []int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		have, err := q.CategoryIDs(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: reading categories: %w", err)
		}
		if !samePermutation(have, ids) {
			return ErrStaleOrder
		}
		now := s.now()
		for i, id := range ids {
			n, err := q.SetCategorySort(ctx, gen.SetCategorySortParams{SortOrder: int64(i), Now: now, VenueID: venueID, ID: id})
			if err := oneRow(n, err, ErrCategoryNotFound); err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateItem adds an item at the end of its category.
func (s *Service) CreateItem(ctx context.Context, venueID int64, in ItemInput) (int64, error) {
	var id int64
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		item, err := s.checkItem(ctx, q, venueID, in)
		if err != nil {
			return err
		}
		next, err := q.NextItemSort(ctx, gen.NextItemSortParams{VenueID: venueID, CategoryID: item.CategoryID})
		if err != nil {
			return fmt.Errorf("menus: next item place: %w", err)
		}
		now := s.now()
		id, err = q.CreateItem(ctx, gen.CreateItemParams{
			VenueID: venueID, CategoryID: item.CategoryID, Name: item.Name, Price: item.Price.Minor, Hsn: item.HSN,
			TaxClassID: item.TaxClassID, StationID: item.StationID, CourseID: item.CourseID, SortOrder: next,
			CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, err
}

// UpdateItem changes an item. The next line rung up copies the change;
// every line already on a ticket keeps what it copied. An item moved to
// another category goes to the end of it.
func (s *Service) UpdateItem(ctx context.Context, venueID, id int64, in ItemInput) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		current, err := q.ItemForEdit(ctx, gen.ItemForEditParams{VenueID: venueID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrItemNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("menus: reading item %d: %w", id, err)
		}
		item, err := s.checkItem(ctx, q, venueID, in)
		if err != nil {
			return err
		}
		sort := current.SortOrder
		if item.CategoryID != current.CategoryID {
			if sort, err = q.NextItemSort(ctx, gen.NextItemSortParams{VenueID: venueID, CategoryID: item.CategoryID}); err != nil {
				return fmt.Errorf("menus: next item place: %w", err)
			}
		}
		n, err := q.UpdateItem(ctx, gen.UpdateItemParams{
			CategoryID: item.CategoryID, Name: item.Name, Price: item.Price.Minor, Hsn: item.HSN,
			TaxClassID: item.TaxClassID, StationID: item.StationID, CourseID: item.CourseID, SortOrder: sort,
			Now: s.now(), VenueID: venueID, ID: id,
		})
		return oneRow(n, err, ErrItemNotFound)
	})
}

// SetItemActive retires an item or brings it back. An item brought back
// into a retired category stays off the till until the category is back.
func (s *Service) SetItemActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if active {
			// An item cannot come back charging a class that is retired.
			ok, err := q.ItemTaxClassActive(ctx, gen.ItemTaxClassActiveParams{VenueID: venueID, ID: id})
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("menus: reading item %d's tax class: %w", id, err)
			}
			if err == nil && !ok {
				return &Invalid{"tax_class_id", "This item's tax class is retired. Bring the class back, or edit the item to use another."}
			}
		}
		n, err := q.SetItemActive(ctx, gen.SetItemActiveParams{Active: active, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrItemNotFound)
	})
}

// SetItemSoldOut marks an item sold out until the next trading day starts
// in the venue's timezone, or puts it back on sale now. It returns when it
// comes back, or nil. Retiring is separate: a sold-out item stays on the
// menu, shown as sold out, and returns by itself (ADR-109).
func (s *Service) SetItemSoldOut(ctx context.Context, venueID, id int64, soldOut bool) (*time.Time, error) {
	var until *time.Time
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		at := sql.NullTime{}
		if soldOut {
			venue, err := q.VenueByID(ctx, venueID)
			if err != nil {
				return fmt.Errorf("menus: reading venue %d: %w", venueID, err)
			}
			loc, err := time.LoadLocation(venue.Timezone)
			if err != nil {
				return fmt.Errorf("menus: venue %d timezone %q: %w", venueID, venue.Timezone, err)
			}
			dayStart, err := svcshift.DayStart(ctx, q, venueID)
			if err != nil {
				return fmt.Errorf("menus: %w", err)
			}
			next, err := shift.NextDayStart(s.now(), loc, dayStart)
			if err != nil {
				return fmt.Errorf("menus: the next trading day: %w", err)
			}
			next = next.UTC()
			at, until = sql.NullTime{Time: next, Valid: true}, &next
		}
		n, err := q.SetItemSoldOut(ctx, gen.SetItemSoldOutParams{Until: at, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrItemNotFound)
	})
	if err != nil {
		return nil, err
	}
	return until, nil
}

// OrderItems puts a category's items in the order given. ids must name
// every item in the category, retired ones included, once each.
func (s *Service) OrderItems(ctx context.Context, venueID, categoryID int64, ids []int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if _, err := s.category(ctx, q, venueID, categoryID); err != nil {
			return err
		}
		have, err := q.ItemIDsInCategory(ctx, gen.ItemIDsInCategoryParams{VenueID: venueID, CategoryID: categoryID})
		if err != nil {
			return fmt.Errorf("menus: reading items: %w", err)
		}
		if !samePermutation(have, ids) {
			return ErrStaleOrder
		}
		now := s.now()
		for i, id := range ids {
			n, err := q.SetItemSort(ctx, gen.SetItemSortParams{SortOrder: int64(i), Now: now, VenueID: venueID, ID: id})
			if err := oneRow(n, err, ErrItemNotFound); err != nil {
				return err
			}
		}
		return nil
	})
}

// edit runs fn as one menu edit and, once it has committed, tells the
// stream. A failed edit tells nobody.
func (s *Service) edit(ctx context.Context, venueID int64, fn func(q *gen.Queries) error) error {
	if _, err := s.db.EditMenu(ctx, venueID, fn); err != nil {
		return err
	}
	s.onChange(venueID)
	return nil
}

func (s *Service) now() time.Time { return s.clock.Now().UTC() }

// checkItem cleans an item and checks everything it names belongs to the
// venue and can be used. An HSN is required where the venue's tax profile
// prints one on every line; elsewhere it may stay, if well formed, so a
// menu written for GST (the demo menu, or a venue that changed profile)
// can still be edited (ADR-108).
func (s *Service) checkItem(ctx context.Context, q *gen.Queries, venueID int64, in ItemInput) (ItemInput, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return ItemInput{}, err
	}
	in.Name = name

	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return ItemInput{}, fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	if in.Price.Currency != venue.Currency {
		return ItemInput{}, &Invalid{"price", fmt.Sprintf("Prices at this venue are in %s.", venue.Currency)}
	}
	price, err := money.New(in.Price.Minor, in.Price.Currency)
	if err != nil || price.Minor < 0 {
		return ItemInput{}, &Invalid{"price", "Give a price of zero or more."}
	}
	in.Price = price

	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return ItemInput{}, fmt.Errorf("menus: reading the venue's tax profile: %w", err)
	}
	in.HSN = strings.TrimSpace(in.HSN)
	switch needs := tax.NeedsHSN(taxes.Profile); {
	case needs && !tax.ValidHSN(in.HSN):
		return ItemInput{}, &Invalid{"hsn", "Every item needs an HSN or SAC code of 4, 6 or 8 digits: invoices show it."}
	case !needs && in.HSN != "" && !tax.ValidHSN(in.HSN):
		return ItemInput{}, &Invalid{"hsn", "An HSN or SAC code is 4, 6 or 8 digits. It is not needed here; you can leave it empty."}
	}

	return in, s.checkRefs(ctx, q, venueID, in)
}

// checkRefs checks the category, tax class, station and course an item
// names. Each failure is about the field that named it.
func (s *Service) checkRefs(ctx context.Context, q *gen.Queries, venueID int64, in ItemInput) error {
	cat, err := s.category(ctx, q, venueID, in.CategoryID)
	if errors.Is(err, ErrCategoryNotFound) {
		return &Invalid{"category_id", "Choose a category."}
	}
	if err != nil {
		return err
	}
	if !cat.Active {
		return &Invalid{"category_id", "That category is retired. Bring it back first."}
	}

	class, err := q.TaxClassForEdit(ctx, gen.TaxClassForEditParams{VenueID: venueID, ID: in.TaxClassID})
	if err != nil {
		return refError(err, "tax_class_id", "Choose a tax class.")
	}
	if !class.Active {
		return &Invalid{"tax_class_id", "That tax class is retired. Choose another."}
	}
	station, err := q.StationForEdit(ctx, gen.StationForEditParams{VenueID: venueID, ID: in.StationID})
	if err != nil {
		return refError(err, "station_id", "Choose where it is made.")
	}
	if !station.Active {
		return &Invalid{"station_id", "That station is retired. Choose another."}
	}
	if _, err := q.CourseForEdit(ctx, gen.CourseForEditParams{VenueID: venueID, ID: in.CourseID}); err != nil {
		return refError(err, "course_id", "Choose a course.")
	}
	return nil
}

func (s *Service) category(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.CategoryForEditRow, error) {
	c, err := q.CategoryForEdit(ctx, gen.CategoryForEditParams{VenueID: venueID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("%w: %d", ErrCategoryNotFound, id)
	}
	if err != nil {
		return c, fmt.Errorf("menus: reading category %d: %w", id, err)
	}
	return c, nil
}

// refError is a lookup of something an item names: not there is the
// field's fault, anything else is the database's.
func refError(err error, field, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &Invalid{field, message}
	}
	return fmt.Errorf("menus: checking %s: %w", field, err)
}

// oneRow turns an update's row count into a result: none changed means the
// id is not this venue's.
func oneRow(n int64, err error, notFound error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return notFound
	}
	return nil
}

// cleanName trims a name and checks its length.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", &Invalid{"name", "Give it a name."}
	case utf8.RuneCountInString(name) > MaxNameLength:
		return "", &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", MaxNameLength)}
	}
	return name, nil
}

// samePermutation reports whether want names exactly the ids in have, each
// once. have is distinct (a table's ids), so equal sorted lists leave no
// room for a repeat in want.
func samePermutation(have, want []int64) bool {
	a, b := slices.Clone(have), slices.Clone(want)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
