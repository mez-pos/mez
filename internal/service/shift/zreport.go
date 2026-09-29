package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/zreport"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// The Z-report's refusals (ADR-098).
var (
	ErrZReportNotFound = errors.New("no such Z-report at this venue")
	ErrShiftStillOpen  = errors.New("the shift is still open: its Z-report comes with the close")
)

// ZDocument is a Z-report as issued: everything it prints, frozen. It is
// never worked out again from the orders once written.
type ZDocument struct {
	Number   int64  `json:"number"`
	Corrects int64  `json:"corrects,omitempty"` // the number this one corrects
	Reason   string `json:"reason,omitempty"`   // why it was corrected

	Shift  ZShift         `json:"shift"`
	Report zreport.Report `json:"report"`
	Drawer ZDrawer        `json:"drawer"`
	// Names are the staff the report mentions, by ID, as they were named
	// when it was issued.
	Names map[int64]string `json:"names"`

	GeneratedBy int64     `json:"generated_by"`
	GeneratedAt time.Time `json:"generated_at"`
}

// ZShift is the shift a report is of.
type ZShift struct {
	PublicID   string    `json:"public_id"`
	TradingDay string    `json:"trading_day"`
	OpenedBy   int64     `json:"opened_by"`
	OpenedAt   time.Time `json:"opened_at"`
	ClosedBy   int64     `json:"closed_by"`
	ClosedAt   time.Time `json:"closed_at"`
}

// ZDrawer is the cash reconciliation: how the expected figure is made up,
// and the blind count against it.
type ZDrawer struct {
	Float     money.Amount    `json:"float"`
	CashIn    money.Amount    `json:"cash_in"`
	CashOut   money.Amount    `json:"cash_out"`
	Sales     shift.CashSales `json:"sales"`
	Expected  money.Amount    `json:"expected"`
	Counted   money.Amount    `json:"counted"`
	Variance  money.Amount    `json:"variance"`
	Threshold money.Amount    `json:"threshold"`
	Reason    string          `json:"reason,omitempty"`
	CountedBy int64           `json:"counted_by"`
}

// ZReport is a stored Z-report.
type ZReport struct {
	ID       int64     `json:"id"`
	ShiftID  int64     `json:"shift_id"`
	Document ZDocument `json:"document"`
}

// ZSummary is a report in the list.
type ZSummary struct {
	Number        int64        `json:"number"`
	ShiftID       int64        `json:"shift_id"`
	ShiftPublicID string       `json:"shift_public_id"`
	TradingDay    string       `json:"trading_day"`
	Correction    bool         `json:"correction"`
	Reason        string       `json:"reason,omitempty"`
	GeneratedAt   time.Time    `json:"generated_at"`
	SalesTotal    money.Amount `json:"sales_total"`
}

// document builds a closed shift's report from its orders, now.
func document(ctx context.Context, q *gen.Queries, venueID int64, sh Shift, currency string, by int64, at time.Time) (ZDocument, error) {
	if sh.Closing == nil {
		return ZDocument{}, fmt.Errorf("%w: shift %s", ErrNotCounted, sh.PublicID)
	}
	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return ZDocument{}, err
	}
	orders, err := stampedOrders(ctx, q, sh.ID)
	if err != nil {
		return ZDocument{}, err
	}
	categories, err := itemCategories(ctx, q, venueID)
	if err != nil {
		return ZDocument{}, err
	}
	report, err := zreport.Build(zreport.Input{ShiftID: sh.ID, Currency: currency, Taxes: taxes, Orders: orders, Categories: categories})
	if err != nil {
		return ZDocument{}, fmt.Errorf("the Z-report for %s: %w", sh.PublicID, err)
	}
	drawer, err := drawerOf(sh, currency)
	if err != nil {
		return ZDocument{}, err
	}
	doc := ZDocument{
		Shift: ZShift{
			PublicID: sh.PublicID, TradingDay: sh.TradingDay, OpenedBy: sh.OpenedBy, OpenedAt: sh.OpenedAt,
			ClosedBy: sh.ClosedBy, ClosedAt: sh.ClosedAt,
		},
		Report: report, Drawer: drawer, GeneratedBy: by, GeneratedAt: at,
	}
	if err := name(ctx, q, venueID, &doc); err != nil {
		return ZDocument{}, err
	}
	return doc, nil
}

// drawerOf is the shift's cash reconciliation as the report prints it.
func drawerOf(sh Shift, currency string) (ZDrawer, error) {
	zero, err := money.New(0, currency)
	if err != nil {
		return ZDrawer{}, err
	}
	rec := sh.Closing.Reconciliation
	d := ZDrawer{
		Float: sh.Float.Total, CashIn: zero, CashOut: zero, Sales: sh.Sales,
		Expected: rec.Expected, Counted: rec.Counted, Variance: rec.Variance, Threshold: rec.Threshold,
		CountedBy: sh.Closing.CountedBy,
	}
	for _, e := range sh.Events {
		switch e.Type {
		case EventCashIn:
			d.CashIn, err = d.CashIn.Add(e.Amount)
		case EventCashOut:
			d.CashOut, err = d.CashOut.Add(e.Amount)
		}
		if err != nil {
			return ZDrawer{}, err
		}
	}
	if sh.Reason != nil {
		d.Reason = sh.Reason.Reason
	}
	return d, nil
}

// name gives the report's staff their names: the by-staff headings, and
// every ID it mentions.
func name(ctx context.Context, q *gen.Queries, venueID int64, doc *ZDocument) error {
	rows, err := q.StaffNamesForVenue(ctx, venueID)
	if err != nil {
		return fmt.Errorf("reading the staff names: %w", err)
	}
	all := make(map[int64]string, len(rows))
	for _, r := range rows {
		all[r.ID] = r.Name
	}
	doc.Names = map[int64]string{}
	mention := func(id int64) string {
		if n, ok := all[id]; ok {
			doc.Names[id] = n
			return n
		}
		return fmt.Sprintf("Staff %d", id)
	}
	for i, g := range doc.Report.ByStaff {
		var id int64
		if _, err := fmt.Sscan(g.Key, &id); err == nil {
			doc.Report.ByStaff[i].Name = mention(id)
		}
	}
	for _, t := range doc.Report.Removed {
		mention(t.StaffID)
	}
	for _, id := range []int64{doc.Shift.OpenedBy, doc.Shift.ClosedBy, doc.Drawer.CountedBy, doc.GeneratedBy} {
		mention(id)
	}
	return nil
}

func itemCategories(ctx context.Context, q *gen.Queries, venueID int64) (map[int64]zreport.Category, error) {
	rows, err := q.ItemCategoriesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("reading the menu's categories: %w", err)
	}
	out := make(map[int64]zreport.Category, len(rows))
	for _, r := range rows {
		out[r.ItemID] = zreport.Category{ID: r.CategoryID, Name: r.CategoryName}
	}
	return out, nil
}

// issue numbers a document and stores it, in the caller's transaction.
// The number, and for a correction what it corrects, are set on doc.
func issue(ctx context.Context, q *gen.Queries, venueID, shiftID int64, corrects *ZReport, reason string, doc *ZDocument) (int64, error) {
	number, err := q.NextZNumber(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("numbering the Z-report: %w", err)
	}
	doc.Number = number
	var correctsID sql.NullInt64
	if corrects != nil {
		correctsID = sql.NullInt64{Int64: corrects.ID, Valid: true}
		doc.Corrects, doc.Reason = corrects.Document.Number, reason
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return 0, fmt.Errorf("encoding Z-report %d: %w", number, err)
	}
	_, err = q.CreateZReport(ctx, gen.CreateZReportParams{
		VenueID: venueID, Number: number, ShiftID: shiftID, CorrectsID: correctsID, Reason: reason,
		Body: string(body), GeneratedBy: doc.GeneratedBy, GeneratedAt: doc.GeneratedAt,
	})
	if err != nil {
		return 0, fmt.Errorf("storing Z-report %d: %w", number, err)
	}
	return number, nil
}
