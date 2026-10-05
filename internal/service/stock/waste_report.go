package stock

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

// WasteItem is one stock item's write-offs over a range.
type WasteItem struct {
	StockItemID int64      `json:"stock_item_id"`
	Name        string     `json:"name"`
	Unit        stock.Unit `json:"unit"`
	// Qty is how much was written off, above zero.
	Qty     stock.Qty `json:"qty"`
	Entries int64     `json:"entries"`
	// Value is the entries with a cost, each at the cost it had when it was
	// written off; Uncosted is how many had none, so a total is never read
	// as the whole of it (ADR-124).
	Value    money.Amount `json:"value"`
	Uncosted int64        `json:"uncosted"`
}

// WasteReasonTotal is the write-offs given one reason.
type WasteReasonTotal struct {
	Reason   stock.WasteReason `json:"reason"`
	Entries  int64             `json:"entries"`
	Value    money.Amount      `json:"value"`
	Uncosted int64             `json:"uncosted"`
}

// WasteReport is what was written off over a range of trading days, at cost
// (rebuild Task 16d1, ADR-186).
type WasteReport struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Currency string `json:"currency"`
	// Items and Reasons are the largest loss first.
	Items    []WasteItem        `json:"items"`
	Reasons  []WasteReasonTotal `json:"reasons"`
	Entries  int64              `json:"entries"`
	Value    money.Amount       `json:"value"`
	Uncosted int64              `json:"uncosted"`
}

// WasteReport reads the write-offs keyed on the trading days From to To,
// both inclusive, like "2026-10-01". An entry belongs to the trading day it
// was keyed in, by the venue's clock and day start: stock written off by
// hand has no shift to take a day from.
func (s *Service) WasteReport(ctx context.Context, venueID int64, from, to string) (WasteReport, error) {
	if err := checkDays(from, to); err != nil {
		return WasteReport{}, err
	}
	q := s.db.Queries()
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return WasteReport{}, fmt.Errorf("stock: reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return WasteReport{}, fmt.Errorf("stock: venue timezone %q: %w", v.Timezone, err)
	}
	dayStart, err := dayStartOf(ctx, q, venueID)
	if err != nil {
		return WasteReport{}, err
	}

	// A day either side of the range is read, then each entry is placed by
	// the same rule a shift's trading day is (shift.TradingDay), so the edge
	// of a day is decided in one place.
	first, _ := time.ParseInLocation(time.DateOnly, from, loc)
	last, _ := time.ParseInLocation(time.DateOnly, to, loc)
	rows, err := q.WastedBetween(ctx, gen.WastedBetweenParams{
		VenueID: venueID, FromAt: first.AddDate(0, 0, -1).UTC(), ToAt: last.AddDate(0, 0, 2).UTC(),
	})
	if err != nil {
		return WasteReport{}, fmt.Errorf("stock: reading the write-offs from %s to %s: %w", from, to, err)
	}
	inRange := make([]gen.WastedBetweenRow, 0, len(rows))
	for _, r := range rows {
		day, err := shift.TradingDay(r.CreatedAt, loc, dayStart)
		if err != nil {
			return WasteReport{}, fmt.Errorf("stock: placing a write-off of %s: %w", r.Name, err)
		}
		if day >= from && day <= to {
			inRange = append(inRange, r)
		}
	}
	out, err := wasteReport(inRange, v.Currency)
	if err != nil {
		return WasteReport{}, err
	}
	out.From, out.To = from, to
	return out, nil
}

// settingDayStart is the venue setting for when the trading day begins. The
// shift and order services read the same key (svcshift.SettingDayStart); the
// order service imports this package, so neither can be imported from here.
const settingDayStart = "shift.day_start"

// dayStartOf is when the venue's trading day begins, as it stands now.
func dayStartOf(ctx context.Context, q *gen.Queries, venueID int64) (time.Duration, error) {
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: settingDayStart})
	if errors.Is(err, sql.ErrNoRows) {
		return shift.DefaultDayStart, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stock: reading %s: %w", settingDayStart, err)
	}
	d, err := shift.ParseDayStart(strings.TrimSpace(row.Value))
	if err != nil {
		return 0, fmt.Errorf("stock: %s is %q, not a time of day like 04:00", settingDayStart, row.Value)
	}
	return d, nil
}

// wasteReport adds the write-offs up by item and by reason.
func wasteReport(rows []gen.WastedBetweenRow, currency string) (WasteReport, error) {
	zero := money.Amount{Currency: currency}
	out := WasteReport{Currency: currency, Items: []WasteItem{}, Reasons: []WasteReasonTotal{}, Value: zero}
	items := map[int64]*WasteItem{}
	reasons := map[stock.WasteReason]*WasteReasonTotal{}
	for _, r := range rows {
		unit, qty := stock.Unit(r.Unit), stock.Qty(-r.Delta)
		it, ok := items[r.StockItemID]
		if !ok {
			it = &WasteItem{StockItemID: r.StockItemID, Name: r.Name, Unit: unit, Value: zero}
			items[r.StockItemID] = it
		}
		why, ok := reasons[stock.WasteReason(r.Reason)]
		if !ok {
			why = &WasteReasonTotal{Reason: stock.WasteReason(r.Reason), Value: zero}
			reasons[why.Reason] = why
		}
		it.Qty += qty
		it.Entries++
		why.Entries++
		out.Entries++
		if r.UnitCost <= 0 {
			it.Uncosted++
			why.Uncosted++
			out.Uncosted++
			continue
		}
		worth, err := stock.Value(qty, unit, r.UnitCost)
		if err != nil {
			return WasteReport{}, fmt.Errorf("stock: valuing a write-off of %s: %w", r.Name, err)
		}
		it.Value.Minor += worth
		why.Value.Minor += worth
		out.Value.Minor += worth
	}
	for _, it := range items {
		out.Items = append(out.Items, *it)
	}
	slices.SortFunc(out.Items, func(a, b WasteItem) int {
		return cmp.Or(
			cmp.Compare(b.Value.Minor, a.Value.Minor), cmp.Compare(b.Entries, a.Entries),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.StockItemID, b.StockItemID),
		)
	})
	for _, why := range reasons {
		out.Reasons = append(out.Reasons, *why)
	}
	slices.SortFunc(out.Reasons, func(a, b WasteReasonTotal) int {
		return cmp.Or(cmp.Compare(b.Value.Minor, a.Value.Minor), cmp.Compare(b.Entries, a.Entries), cmp.Compare(a.Reason, b.Reason))
	})
	return out, nil
}
