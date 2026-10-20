package stock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/stock"
)

// The closing count's refusals (ADR-114).
var (
	ErrShiftNotFound  = errors.New("no such shift at this venue")
	ErrShiftClosed    = errors.New("the shift is closed")
	ErrAlreadyCounted = errors.New("the closing stock is already counted")
	// ErrStaleCount is a closing count that is not exactly the items on
	// the list: one was flagged or retired since the till read it.
	ErrStaleCount = errors.New("the closing count does not name exactly the items to count")
)

// ClosingItem is a stock item to count as the shift closes. Nothing about
// what the ledger expects: the count is blind, as the drawer's is.
type ClosingItem struct {
	ID   int64      `json:"id"`
	Name string     `json:"name"`
	Unit stock.Unit `json:"unit"`
}

// ClosingResult is one item's closing count, revealed once it is in.
type ClosingResult struct {
	StockItemID int64      `json:"stock_item_id"`
	Name        string     `json:"name"`
	Unit        stock.Unit `json:"unit"`
	Counted     stock.Qty  `json:"counted"`
	// Delta is what the count moved: counted less what the ledger said.
	Delta stock.Qty `json:"delta"`
}

// ClosingSheet is a shift's closing count: the items to count while it is
// not done, and what it found once it is.
type ClosingSheet struct {
	Done    bool            `json:"done"`
	Items   []ClosingItem   `json:"items"`
	Results []ClosingResult `json:"results"`
}

// ClosingCountInput is one item's count, in thousandths of its unit.
type ClosingCountInput struct {
	StockItemID int64     `json:"stock_item_id"`
	Qty         stock.Qty `json:"qty"`
}

// ClosingCount reads a shift's closing count.
func (s *Service) ClosingCount(ctx context.Context, venueID, shiftID int64) (ClosingSheet, error) {
	q := s.db.Queries()
	if _, err := shiftOf(ctx, q, venueID, shiftID); err != nil {
		return ClosingSheet{}, err
	}
	return sheet(ctx, q, venueID, shiftID)
}

// RecordClosingCount records the closing count of a shift, once, by
// staffID: a counted movement per item, of that shift and marked closing,
// in one transaction. Every item on the list must be counted; a list that
// moved since the till read it is refused whole.
func (s *Service) RecordClosingCount(ctx context.Context, venueID, staffID, shiftID int64, counts []ClosingCountInput) (ClosingSheet, error) {
	var out ClosingSheet
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		sh, err := shiftOf(ctx, q, venueID, shiftID)
		if err != nil {
			return err
		}
		if sh.Status != "open" {
			return ErrShiftClosed
		}
		done, err := q.ClosingCountForShift(ctx, gen.ClosingCountForShiftParams{VenueID: venueID, ShiftID: shiftID})
		if err != nil {
			return fmt.Errorf("stock: reading the closing count of shift %d: %w", shiftID, err)
		}
		if len(done) > 0 {
			return ErrAlreadyCounted
		}
		list, err := q.ItemsToCountAtClose(ctx, venueID)
		if err != nil {
			return fmt.Errorf("stock: reading the items to count: %w", err)
		}
		want := make([]int64, len(list))
		for i, it := range list {
			want[i] = it.ID
		}
		got := make([]int64, len(counts))
		for i, c := range counts {
			got[i] = c.StockItemID
		}
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(want, got) {
			return ErrStaleCount
		}
		now := s.now()
		for i, c := range counts {
			onHand, err := q.StockOnHand(ctx, gen.StockOnHandParams{VenueID: venueID, StockItemID: c.StockItemID})
			if err != nil {
				return fmt.Errorf("stock: reading what is on hand of %d: %w", c.StockItemID, err)
			}
			delta, _, p := stock.Delta(stock.MovementDraft{Kind: stock.Counted, Qty: c.Qty}, stock.Qty(onHand))
			if p != nil {
				return &Invalid{fmt.Sprintf("counts.%d.%s", i, p.Field), p.Message}
			}
			if _, err := q.InsertStockMovement(ctx, gen.InsertStockMovementParams{
				VenueID: venueID, StockItemID: c.StockItemID, Kind: string(stock.Counted), Delta: int64(delta),
				Counted: sql.NullInt64{Int64: int64(c.Qty), Valid: true}, StaffID: staffID, ShiftID: shiftID,
				Closing: true, CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("stock: recording the closing count of %d: %w", c.StockItemID, err)
			}
		}
		out, err = sheet(ctx, q, venueID, shiftID)
		return err
	})
	return out, err
}

// sheet is a shift's closing count as it stands.
func sheet(ctx context.Context, q *gen.Queries, venueID, shiftID int64) (ClosingSheet, error) {
	done, err := q.ClosingCountForShift(ctx, gen.ClosingCountForShiftParams{VenueID: venueID, ShiftID: shiftID})
	if err != nil {
		return ClosingSheet{}, fmt.Errorf("stock: reading the closing count of shift %d: %w", shiftID, err)
	}
	out := ClosingSheet{Items: []ClosingItem{}, Results: []ClosingResult{}}
	if len(done) > 0 {
		out.Done = true
		for _, r := range done {
			out.Results = append(out.Results, ClosingResult{
				StockItemID: r.StockItemID, Name: r.Name, Unit: stock.Unit(r.Unit), Counted: stock.Qty(r.Counted.Int64), Delta: stock.Qty(r.Delta),
			})
		}
		return out, nil
	}
	list, err := q.ItemsToCountAtClose(ctx, venueID)
	if err != nil {
		return ClosingSheet{}, fmt.Errorf("stock: reading the items to count: %w", err)
	}
	for _, it := range list {
		out.Items = append(out.Items, ClosingItem{ID: it.ID, Name: it.Name, Unit: stock.Unit(it.Unit)})
	}
	return out, nil
}

func shiftOf(ctx context.Context, q *gen.Queries, venueID, shiftID int64) (gen.Shift, error) {
	sh, err := q.ShiftByID(ctx, shiftID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && sh.VenueID != venueID) {
		return gen.Shift{}, fmt.Errorf("%w: %d", ErrShiftNotFound, shiftID)
	}
	if err != nil {
		return gen.Shift{}, fmt.Errorf("stock: reading shift %d: %w", shiftID, err)
	}
	return sh, nil
}
