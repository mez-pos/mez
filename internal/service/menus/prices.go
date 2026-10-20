package menus

import (
	"context"
	"errors"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/money"
)

// Prices from a file (phase 10, Task 9b, ADR-295, ADR-296). menu.ReadPrices
// reads the file; this reads the menu for it to be read against, and writes
// the changes it answers.

// The refusals of a file of prices. Nothing was written by either.
var (
	// ErrPricesRefused is a file with a problem in it. The Prices returned
	// with it says what the problems are.
	ErrPricesRefused = errors.New("the file has problems, so no price was changed")
	// ErrPricesStale is a file checked against a menu that has changed
	// since: what the owner read is no longer what would happen.
	ErrPricesStale = errors.New("the menu changed since the file was checked")
)

// errNoPriceChanged ends an apply that changes nothing without a write, so
// the menu's version stays where it is.
var errNoPriceChanged = errors.New("no price to change")

// Prices is what a file would do to the menu's prices, or has done.
type Prices struct {
	// Version is the menu's version the file was read against. Applying
	// the file names it, and is refused if the menu has moved on.
	Version int64         `json:"version"`
	Changes []PriceChange `json:"changes"`
	// Unchanged is how many rows give their item the price it has.
	Unchanged int `json:"unchanged"`
	// Unknown are the rows that are no item on the menu. They are left out.
	Unknown  []PriceUnknown  `json:"unknown"`
	Problems []ImportProblem `json:"problems"`
	// Applied: Changes are the menu's prices now. False for a preview, a
	// refusal, and a file that changed nothing.
	Applied bool `json:"applied"`
}

// PriceChange is one item's price changing.
type PriceChange struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row      int          `json:"row"`
	ItemID   int64        `json:"item_id"`
	Category string       `json:"category"`
	Name     string       `json:"name"`
	Old      money.Amount `json:"old"`
	New      money.Amount `json:"new"`
}

// PriceUnknown is a row that is no item on the menu, as the file spells it.
type PriceUnknown struct {
	Row      int    `json:"row"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

// PreviewPrices reads a file and answers the prices it would change. It
// writes nothing.
func (s *Service) PreviewPrices(ctx context.Context, venueID int64, data []byte) (Prices, error) {
	if err := checkFileSize(data); err != nil {
		return Prices{}, err
	}
	// One read, so the version is the version of the prices it is read against.
	rows, err := s.db.MenuEditor(ctx, venueID)
	if err != nil {
		return Prices{}, err
	}
	venue, err := s.db.Queries().VenueByID(ctx, venueID)
	if err != nil {
		return Prices{}, fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	v := priceVenue(venue.Currency, rows.Categories, rows.Items)
	return pricesOf(menu.ReadPrices(data, v), v.Currency, rows.Version), nil
}

// ApplyPrices changes the prices a file changes, as one edit: all of them at
// one new version, or none. A file with any problem is refused with
// ErrPricesRefused and the problems.
//
// version is the menu version the owner's preview was read at. If the menu
// has changed since, by anyone, the file is refused with ErrPricesStale: the
// table of old and new prices they agreed to is not what would happen now.
// The file is read again here, inside the write, so what is written is what
// it says of the menu as it is.
//
// A line already rung up keeps the price it copied (docs/02-domain.md,
// "Prices and the snapshot rule"): only the next line takes the new one.
func (s *Service) ApplyPrices(ctx context.Context, venueID int64, data []byte, version int64) (Prices, error) {
	if err := checkFileSize(data); err != nil {
		return Prices{}, err
	}
	var out Prices
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		now, err := q.MenuVersion(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: reading the menu version: %w", err)
		}
		if now != version {
			return ErrPricesStale
		}
		plan, currency, err := planPrices(ctx, q, venueID, data)
		if err != nil {
			return err
		}
		out = pricesOf(plan, currency, now)
		switch {
		case len(plan.Problems) > 0:
			return ErrPricesRefused
		case len(plan.Changes) == 0:
			return errNoPriceChanged
		}
		return s.writePrices(ctx, q, venueID, plan)
	})
	switch {
	case errors.Is(err, errNoPriceChanged):
		return out, nil
	case errors.Is(err, ErrPricesRefused):
		return out, err
	case err != nil:
		return Prices{}, err
	}
	out.Applied = true
	return out, nil
}

func checkFileSize(data []byte) error {
	if len(data) > MaxImportBytes {
		return &Invalid{"file", fmt.Sprintf("That file is over %d KB.", MaxImportBytes>>10)}
	}
	return nil
}

// planPrices reads the menu through q and the file against it.
func planPrices(ctx context.Context, q *gen.Queries, venueID int64, data []byte) (menu.PricePlan, string, error) {
	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return menu.PricePlan{}, "", fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	categories, err := q.EditorCategories(ctx, venueID)
	if err != nil {
		return menu.PricePlan{}, "", fmt.Errorf("menus: reading categories: %w", err)
	}
	items, err := q.EditorItems(ctx, venueID)
	if err != nil {
		return menu.PricePlan{}, "", fmt.Errorf("menus: reading items: %w", err)
	}
	return menu.ReadPrices(data, priceVenue(venue.Currency, categories, items)), venue.Currency, nil
}

// priceVenue is the menu as a file of prices is read against it.
func priceVenue(currency string, categories []gen.EditorCategoriesRow, items []gen.EditorItemsRow) menu.PriceVenue {
	names := make(map[int64]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}
	out := menu.PriceVenue{Currency: currency, Items: make([]menu.PriceItem, len(items))}
	for i, it := range items {
		out.Items[i] = menu.PriceItem{ID: it.ID, Category: names[it.CategoryID], Name: it.Name, Price: it.Price}
	}
	return out
}

// writePrices writes a plan with no problems. Each change names the price
// it was planned against, so one that does not land on exactly its item at
// exactly that price fails the whole edit.
func (s *Service) writePrices(ctx context.Context, q *gen.Queries, venueID int64, plan menu.PricePlan) error {
	now := s.now()
	for _, c := range plan.Changes {
		n, err := q.SetItemPrice(ctx, gen.SetItemPriceParams{Price: c.New, Now: now, VenueID: venueID, ID: c.ItemID, Was: c.Old})
		if err != nil {
			return fmt.Errorf("menus: changing the price of row %d (%s): %w", c.Row, c.Name, err)
		}
		if n != 1 {
			return fmt.Errorf("menus: changing the price of row %d (%s): %d rows changed, want 1", c.Row, c.Name, n)
		}
	}
	return nil
}

// pricesOf is a plan as the back office reads it. Every list is there, if
// empty, so the screen need not tell none from missing.
func pricesOf(plan menu.PricePlan, currency string, version int64) Prices {
	out := Prices{
		Version:   version,
		Changes:   make([]PriceChange, len(plan.Changes)),
		Unchanged: plan.Unchanged,
		Unknown:   make([]PriceUnknown, len(plan.Unknown)),
		Problems:  make([]ImportProblem, len(plan.Problems)),
	}
	amount := func(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: currency} }
	for i, c := range plan.Changes {
		out.Changes[i] = PriceChange{Row: c.Row, ItemID: c.ItemID, Category: c.Category, Name: c.Name, Old: amount(c.Old), New: amount(c.New)}
	}
	for i, u := range plan.Unknown {
		out.Unknown[i] = PriceUnknown(u)
	}
	for i, p := range plan.Problems {
		out.Problems[i] = ImportProblem(p)
	}
	return out
}
