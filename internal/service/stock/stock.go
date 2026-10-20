// Package stock is the back office's stock (phase 10, Task 3a, ADR-111):
// stock items, and the ledger of movements whose sum is what is on hand.
// The rules are internal/domain/stock's; this reads and writes them.
package stock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/stock"
)

// The refusals. The API maps them to codes; Invalid carries the field.
var (
	ErrNotFound  = errors.New("no such stock item at this venue")
	ErrRetired   = errors.New("the stock item is retired")
	ErrNameTaken = errors.New("a stock item with that name already exists")
)

// Invalid is an input that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service is the venue's stock.
type Service struct {
	db    *db.DB
	clock Clock
}

// New returns a Service.
func New(d *db.DB, clock Clock) (*Service, error) {
	if d == nil {
		return nil, errors.New("stock: a database is required")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: d, clock: clock}, nil
}

// ItemInput is a stock item as the back office sends it.
type ItemInput struct {
	Name     string     `json:"name"`
	Unit     stock.Unit `json:"unit"`
	LowLevel stock.Qty  `json:"low_level"`
	// CountAtClose puts it on the till's closing count (ADR-114).
	CountAtClose bool `json:"count_at_close"`
	// Cost is what a kilogram, a litre or one of it costs, by its unit, in
	// the venue's currency (ADR-124). Zero, or left out, is "not known".
	Cost money.Amount `json:"cost"`
}

// Item is a stock item with what is on hand.
type Item struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	Unit     stock.Unit `json:"unit"`
	LowLevel stock.Qty  `json:"low_level"`
	Active   bool       `json:"active"`
	OnHand   stock.Qty  `json:"on_hand"`
	// CountAtClose: it is counted, blind, as each shift closes.
	CountAtClose bool `json:"count_at_close"`
	// Low is on hand at or under the low-stock level (stock.IsLow).
	Low bool `json:"low"`
	// Cost is what a kilogram, a litre or one of it costs; zero is not known.
	Cost money.Amount `json:"cost"`
}

// Currency is the currency the venue's stock is costed in: its own.
func (s *Service) Currency(ctx context.Context, venueID int64) (string, error) {
	return currencyOf(ctx, s.db.Queries(), venueID)
}

func currencyOf(ctx context.Context, q *gen.Queries, venueID int64) (string, error) {
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return "", fmt.Errorf("stock: reading venue %d: %w", venueID, err)
	}
	return v.Currency, nil
}

// costOf is an input's cost in minor units, checked. A cost left out is
// zero; one in another currency than the venue's is refused, not converted.
func costOf(ctx context.Context, q *gen.Queries, venueID int64, in money.Amount) (int64, error) {
	if in.Minor == 0 && in.Currency == "" {
		return 0, nil
	}
	currency, err := currencyOf(ctx, q, venueID)
	if err != nil {
		return 0, err
	}
	if in.Currency != currency {
		return 0, &Invalid{"cost", "The cost must be in " + currency + ", the venue's currency."}
	}
	if p := stock.CleanCost(in.Minor); p != nil {
		return 0, &Invalid{p.Field, p.Message}
	}
	return in.Minor, nil
}

// Items is every stock item, retired ones too, by name.
func (s *Service) Items(ctx context.Context, venueID int64) ([]Item, error) {
	q := s.db.Queries()
	currency, err := currencyOf(ctx, q, venueID)
	if err != nil {
		return nil, err
	}
	rows, err := q.StockItemsWithOnHand(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("stock: reading the stock of venue %d: %w", venueID, err)
	}
	out := make([]Item, len(rows))
	for i, r := range rows {
		onHand, level := stock.Qty(r.OnHand), stock.Qty(r.LowLevel)
		out[i] = Item{
			ID: r.ID, Name: r.Name, Unit: stock.Unit(r.Unit), LowLevel: level, Active: r.Active,
			OnHand: onHand, Low: r.Active && stock.IsLow(onHand, level), CountAtClose: r.CountAtClose,
			Cost: money.Amount{Minor: r.Cost, Currency: currency},
		}
	}
	return out, nil
}

// CreateItem adds a stock item with nothing on hand.
func (s *Service) CreateItem(ctx context.Context, venueID int64, in ItemInput) (int64, error) {
	d, err := clean(in)
	if err != nil {
		return 0, err
	}
	now := s.now()
	var id int64
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		cost, err := costOf(ctx, q, venueID, in.Cost)
		if err != nil {
			return err
		}
		id, err = q.CreateStockItem(ctx, gen.CreateStockItemParams{
			VenueID: venueID, Name: d.Name, Unit: string(d.Unit), LowLevel: int64(d.LowLevel), CountAtClose: in.CountAtClose,
			Cost: cost, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, nameTaken(err)
}

// UpdateItem changes a stock item's name, unit, low-stock level or cost. A
// new cost is for movements from now on: those already written keep the
// cost they were written at (ADR-124). The unit
// cannot change once anything is recorded against it: 500 of it would mean
// grams in one row and millilitres in the next.
func (s *Service) UpdateItem(ctx context.Context, venueID, id int64, in ItemInput) error {
	d, err := clean(in)
	if err != nil {
		return err
	}
	return nameTaken(s.db.WriteTx(ctx, func(q *gen.Queries) error {
		current, err := item(ctx, q, venueID, id)
		if err != nil {
			return err
		}
		if stock.Unit(current.Unit) != d.Unit {
			n, err := q.StockMovementCount(ctx, gen.StockMovementCountParams{VenueID: venueID, StockItemID: id})
			if err != nil {
				return fmt.Errorf("stock: counting the movements of %d: %w", id, err)
			}
			if n > 0 {
				return &Invalid{"unit", "Its history is recorded in " + unitWord(stock.Unit(current.Unit)) + ". Add a new item for another unit."}
			}
		}
		cost, err := costOf(ctx, q, venueID, in.Cost)
		if err != nil {
			return err
		}
		n, err := q.UpdateStockItem(ctx, gen.UpdateStockItemParams{
			Name: d.Name, Unit: string(d.Unit), LowLevel: int64(d.LowLevel), CountAtClose: in.CountAtClose,
			Cost: cost, Now: s.now(), VenueID: venueID, ID: id,
		})
		return oneRow(n, err)
	}))
}

// SetItemActive retires a stock item or brings it back. Its history stays.
func (s *Service) SetItemActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.SetStockItemActive(ctx, gen.SetStockItemActiveParams{Active: active, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err)
	})
}

// MovementInput is a movement entered by hand: see stock.MovementDraft for
// what Qty means for each kind.
type MovementInput struct {
	Kind   stock.Kind        `json:"kind"`
	Qty    stock.Qty         `json:"qty"`
	Reason stock.WasteReason `json:"reason,omitempty"`
	Note   string            `json:"note,omitempty"`
}

// Recorded is a movement just written, and what is on hand after it.
type Recorded struct {
	ID     int64     `json:"id"`
	Delta  stock.Qty `json:"delta"`
	OnHand stock.Qty `json:"on_hand"`
}

// Record writes a movement by hand for the staff member staffID. On hand is
// read in the same transaction, so a count's delta is against the ledger as
// it stands, and two counts at once cannot both use a stale figure.
func (s *Service) Record(ctx context.Context, venueID, staffID, itemID int64, in MovementInput) (Recorded, error) {
	var out Recorded
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		it, err := item(ctx, q, venueID, itemID)
		if err != nil {
			return err
		}
		if !it.Active {
			return ErrRetired
		}
		onHand, err := q.StockOnHand(ctx, gen.StockOnHandParams{VenueID: venueID, StockItemID: itemID})
		if err != nil {
			return fmt.Errorf("stock: reading what is on hand of %d: %w", itemID, err)
		}
		delta, clean, p := stock.Delta(stock.MovementDraft{Kind: in.Kind, Qty: in.Qty, Reason: in.Reason, Note: in.Note}, stock.Qty(onHand))
		if p != nil {
			return &Invalid{p.Field, p.Message}
		}
		shiftID, err := openShift(ctx, q, venueID)
		if err != nil {
			return err
		}
		counted := sql.NullInt64{}
		if clean.Kind == stock.Counted {
			counted = sql.NullInt64{Int64: int64(clean.Qty), Valid: true}
		}
		id, err := q.InsertStockMovement(ctx, gen.InsertStockMovementParams{
			VenueID: venueID, StockItemID: itemID, Kind: string(clean.Kind), Delta: int64(delta), Counted: counted,
			Reason: string(clean.Reason), Note: clean.Note, StaffID: staffID, ShiftID: shiftID, CreatedAt: s.now(),
		})
		if err != nil {
			return fmt.Errorf("stock: recording a movement of %d: %w", itemID, err)
		}
		out = Recorded{ID: id, Delta: delta, OnHand: stock.Qty(onHand) + delta}
		return nil
	})
	return out, err
}

// Movement is one row of an item's history.
type Movement struct {
	ID        int64             `json:"id"`
	Kind      stock.Kind        `json:"kind"`
	Delta     stock.Qty         `json:"delta"`
	Counted   *stock.Qty        `json:"counted,omitempty"`
	Reason    stock.WasteReason `json:"reason,omitempty"`
	Note      string            `json:"note,omitempty"`
	ShiftID   int64             `json:"shift_id,omitempty"`
	OrderID   int64             `json:"order_id,omitempty"`
	StaffID   string            `json:"staff_id"`
	StaffName string            `json:"staff_name"`
	At        time.Time         `json:"at"`
}

// MaxPage is the most movements one page returns.
const MaxPage = 100

// Movements is a page of an item's history, newest first. before is the
// last id of the page before, or 0 for the first. The next page, if any,
// starts before the returned cursor; 0 means there is none.
func (s *Service) Movements(ctx context.Context, venueID, itemID, before int64, limit int) ([]Movement, int64, error) {
	if limit <= 0 || limit > MaxPage {
		limit = MaxPage
	}
	q := s.db.Queries()
	if _, err := item(ctx, q, venueID, itemID); err != nil {
		return nil, 0, err
	}
	// One more than asked, to know whether there is a next page.
	rows, err := q.StockMovementsForItem(ctx, gen.StockMovementsForItemParams{
		VenueID: venueID, StockItemID: itemID, BeforeID: before, MaxRows: int64(limit + 1),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("stock: reading the history of %d: %w", itemID, err)
	}
	next := int64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID
	}
	out := make([]Movement, len(rows))
	for i, r := range rows {
		m := Movement{
			ID: r.ID, Kind: stock.Kind(r.Kind), Delta: stock.Qty(r.Delta), Reason: stock.WasteReason(r.Reason), Note: r.Note,
			ShiftID: r.ShiftID, OrderID: r.OrderID, StaffID: r.StaffID, StaffName: r.StaffName, At: r.CreatedAt.UTC(),
		}
		if r.Counted.Valid {
			c := stock.Qty(r.Counted.Int64)
			m.Counted = &c
		}
		out[i] = m
	}
	return out, next, nil
}

func (s *Service) now() time.Time { return s.clock.Now().UTC() }

func clean(in ItemInput) (stock.ItemDraft, error) {
	d, p := stock.CleanItem(stock.ItemDraft{Name: in.Name, Unit: in.Unit, LowLevel: in.LowLevel})
	if p != nil {
		return stock.ItemDraft{}, &Invalid{p.Field, p.Message}
	}
	return d, nil
}

func item(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.StockItemForEditRow, error) {
	it, err := q.StockItemForEdit(ctx, gen.StockItemForEditParams{VenueID: venueID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return it, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if err != nil {
		return it, fmt.Errorf("stock: reading item %d: %w", id, err)
	}
	return it, nil
}

// openShift is the shift a movement belongs to, or 0 when none is open.
func openShift(ctx context.Context, q *gen.Queries, venueID int64) (int64, error) {
	id, err := q.OpenShiftIDForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stock: reading the open shift: %w", err)
	}
	return id, nil
}

func oneRow(n int64, err error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

func nameTaken(err error) error {
	if db.IsUniqueViolation(err) {
		return fmt.Errorf("%w: %w", ErrNameTaken, &Invalid{"name", "A stock item with that name already exists."})
	}
	return err
}

func unitWord(u stock.Unit) string {
	switch u {
	case stock.Gram:
		return "grams"
	case stock.Millilitre:
		return "millilitres"
	}
	return "pieces"
}
