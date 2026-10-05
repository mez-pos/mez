package stock

import (
	"context"
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

// DishCost is what a dish cost to make over a range, from the ledger.
type DishCost struct {
	// Cost is the stock its sales took, less what went back, each movement
	// at the cost the stock had then (ADR-124). Never today's cost.
	Cost money.Amount
	// Movements is how many sold movements it is made of; Uncosted is how
	// many of them had no cost, and so are not in Cost.
	Movements int64
	Uncosted  int64
	// Units is how many of the dish the ledger accounted for over the range:
	// what Cost covers. A dish sold before it had a recipe adds none.
	Units int64
	// PutBack is how many of the movements put stock back: a line voided, or
	// a recipe changed, between two closes of an order. Each is valued at the
	// cost of the day it went back, which need not be the cost it was taken
	// at, so a dish with any is not taken as wholly costed.
	PutBack int64
}

// DishCosts is what the dishes sold over a range cost to make (rebuild Task
// 16d3, ADR-189).
type DishCosts struct {
	// Dishes is by menu item id. A dish with no recipe took no stock and is
	// not here.
	Dishes map[int64]DishCost
	// NoDish is how many of the range's sold movements are under no dish:
	// sales from before the ledger kept it. While there are any, a dish's
	// cost over the range may be short.
	NoDish int64
	// KeptFrom is the trading day the ledger first kept a dish on a sale,
	// or "" when it never has.
	KeptFrom string
}

// DishRange is the range CostByDish reads, with what places a movement in a
// trading day.
type DishRange struct {
	VenueID  int64
	From, To string // trading days, both inclusive, like "2026-10-01"
	Loc      *time.Location
	DayStart time.Duration
	Currency string
}

// CostByDish adds up the stock each dish's sales took over a range of trading
// days. A movement belongs to the trading day it was written in, which is
// the day its order closed. It takes q so the sales report can read it in
// its own read (the reports service calls it).
func CostByDish(ctx context.Context, q *gen.Queries, r DishRange) (DishCosts, error) {
	if err := checkDays(r.From, r.To); err != nil {
		return DishCosts{}, err
	}
	out := DishCosts{Dishes: map[int64]DishCost{}}
	first, err := q.FirstSoldByDish(ctx, r.VenueID)
	if err != nil {
		return DishCosts{}, fmt.Errorf("stock: reading when dishes were first kept: %w", err)
	}
	if len(first) > 0 {
		day, err := shift.TradingDay(first[0], r.Loc, r.DayStart)
		if err != nil {
			return DishCosts{}, fmt.Errorf("stock: placing the first sale kept by dish: %w", err)
		}
		out.KeptFrom = day
	}

	// A day either side is read, then each movement is placed by the rule a
	// shift's trading day is, as the wastage report does (ADR-186).
	from, _ := time.ParseInLocation(time.DateOnly, r.From, r.Loc)
	to, _ := time.ParseInLocation(time.DateOnly, r.To, r.Loc)
	rows, err := q.SoldBetween(ctx, gen.SoldBetweenParams{
		VenueID: r.VenueID, FromAt: from.AddDate(0, 0, -1).UTC(), ToAt: to.AddDate(0, 0, 2).UTC(),
	})
	if err != nil {
		return DishCosts{}, fmt.Errorf("stock: reading what sales took from %s to %s: %w", r.From, r.To, err)
	}
	for _, row := range rows {
		day, err := shift.TradingDay(row.CreatedAt, r.Loc, r.DayStart)
		if err != nil {
			return DishCosts{}, fmt.Errorf("stock: placing a sale's stock: %w", err)
		}
		if day < r.From || day > r.To {
			continue
		}
		if row.MenuItemID == 0 {
			out.NoDish++
			continue
		}
		d, ok := out.Dishes[row.MenuItemID]
		if !ok {
			d.Cost = money.Amount{Currency: r.Currency}
		}
		d.Movements++
		if row.Delta > 0 {
			d.PutBack++
		}
		if row.UnitCost <= 0 {
			d.Uncosted++
		} else {
			// The delta is below zero for stock taken, so its negation is
			// what the dish used; stock put back comes off.
			worth, err := stock.Value(stock.Qty(-row.Delta), stock.Unit(row.Unit), row.UnitCost)
			if err != nil {
				return DishCosts{}, fmt.Errorf("stock: valuing what dish %d used: %w", row.MenuItemID, err)
			}
			d.Cost.Minor += worth
		}
		out.Dishes[row.MenuItemID] = d
	}
	units, err := q.DishUnitsBetween(ctx, gen.DishUnitsBetweenParams{
		VenueID: r.VenueID, FromAt: from.AddDate(0, 0, -1).UTC(), ToAt: to.AddDate(0, 0, 2).UTC(),
	})
	if err != nil {
		return DishCosts{}, fmt.Errorf("stock: reading the dish units from %s to %s: %w", r.From, r.To, err)
	}
	for _, row := range units {
		day, err := shift.TradingDay(row.CreatedAt, r.Loc, r.DayStart)
		if err != nil {
			return DishCosts{}, fmt.Errorf("stock: placing a dish's units: %w", err)
		}
		if day < r.From || day > r.To {
			continue
		}
		d, ok := out.Dishes[row.MenuItemID]
		if !ok {
			d.Cost = money.Amount{Currency: r.Currency}
		}
		d.Units += row.Qty
		out.Dishes[row.MenuItemID] = d
	}
	return out, nil
}
