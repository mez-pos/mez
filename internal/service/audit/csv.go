package audit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/domain/shift"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// kindWords is each kind as the file says it.
var kindWords = map[Kind]string{
	Void: "Void", Comp: "Comp", Discount: "Discount", Refund: "Refund", PaymentVoid: "Payment void", Reopen: "Reopen",
	TipApproved: "Tip approved",
}

// kindsWords is each kind in the plural, for a column of them.
var kindsWords = map[Kind]string{
	Void: "Voids", Comp: "Comps", Discount: "Discounts", Refund: "Refunds", PaymentVoid: "Payment voids", Reopen: "Reopens",
	TipApproved: "Tips approved",
}

// Table is everything the filter finds, newest first, as a table for a CSV
// download (phase 10, Task 5b, ADR-119). It is the whole trail, not a
// page: Before and Limit are not read. Each row has its trading day and
// its time on the venue's clock; the amount is in major units, its
// currency in the header.
func (s *Service) Table(ctx context.Context, venueID int64, f Filter) (export.Table, error) {
	q := s.db.Queries()
	f.Before, f.Limit = 0, MaxPage
	ask, err := asked(ctx, q, venueID, f)
	if err != nil {
		return export.Table{}, err
	}
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return export.Table{}, fmt.Errorf("audit: reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return export.Table{}, fmt.Errorf("audit: venue timezone %q: %w", v.Timezone, err)
	}

	t := export.Table{
		Header: []string{"Trading day", "At", "Kind", "Order", "Item", "Qty", "Amount (" + v.Currency + ")", "Reason", "By", "Authorised by"},
		Rows:   [][]string{},
	}
	dayStart, err := svcshift.DayStart(ctx, q, venueID)
	if err != nil {
		return export.Table{}, fmt.Errorf("audit: %w", err)
	}
	entries, err := s.everything(ctx, q, ask)
	if err != nil {
		return export.Table{}, err
	}
	for _, e := range entries {
		row, err := rowOf(e, loc, dayStart)
		if err != nil {
			return export.Table{}, err
		}
		t.Rows = append(t.Rows, row)
	}
	return t, nil
}

// rowOf is one correction as the file's cells.
func rowOf(e Entry, loc *time.Location, dayStart time.Duration) ([]string, error) {
	day, err := shift.TradingDay(e.At, loc, dayStart)
	if err != nil {
		return nil, fmt.Errorf("audit: the trading day of entry %d: %w", e.ID, err)
	}
	name := func(p *Person) string {
		if p == nil {
			return ""
		}
		return export.Text(p.Name)
	}
	qty, amount := "", ""
	if e.Item != "" {
		qty = strconv.FormatInt(e.Qty, 10)
	}
	if e.Amount != nil {
		amount = export.Decimal(*e.Amount)
	}
	return []string{
		day, e.At.In(loc).Format("2006-01-02 15:04:05"), kindWords[e.Kind], e.OrderID,
		export.Text(e.Item), qty, amount, export.Text(e.Reason), name(e.By), name(e.AuthorisedBy),
	}, nil
}
