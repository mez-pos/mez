// Package reports reads the range reports (phase 10, Task 5a, ADR-118):
// a range of trading days added up from the order events, through the
// Z-report's builder, with the venue's timezone, taxes, menu and staff.
package reports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/report"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/zreport"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// ErrBadRange is a range the reports cannot be read over.
var ErrBadRange = report.ErrBadRange

// ErrNoSheet is a report the range does not have as a table.
var ErrNoSheet = report.ErrNoSheet

// Service reads the reports.
type Service struct {
	db *db.DB
}

// New returns a Service.
func New(d *db.DB) (*Service, error) {
	if d == nil {
		return nil, errors.New("reports: a database is required")
	}
	return &Service{db: d}, nil
}

// Query is what the owner asks for: trading days From to To inclusive,
// like "2026-10-01", and how sales over time are cut.
type Query struct {
	From  string       `json:"from"`
	To    string       `json:"to"`
	Every report.Every `json:"every"`
}

// Sales is a range's reports, with the staff they mention named.
type Sales struct {
	report.Result
	// Names are the staff the report mentions, by ID, as they are named now.
	Names map[int64]string `json:"names"`
	// Roles are the role each of them holds now, by the same ID. Someone no
	// longer on record has none.
	Roles map[int64]string `json:"roles"`
}

// Sales reads a range's reports.
func (s *Service) Sales(ctx context.Context, venueID int64, qy Query) (Sales, error) {
	q := s.db.Queries()
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return Sales{}, fmt.Errorf("reports: reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return Sales{}, fmt.Errorf("reports: venue timezone %q: %w", v.Timezone, err)
	}
	if qy.Every == "" {
		qy.Every = report.Day
	}
	dayStart, err := svcshift.DayStart(ctx, q, venueID)
	if err != nil {
		return Sales{}, fmt.Errorf("reports: %w", err)
	}
	rg := report.Range{From: qy.From, To: qy.To, Every: qy.Every, Loc: loc, DayStart: dayStart}
	periods, err := rg.Periods()
	if err != nil {
		return Sales{}, err
	}
	from, to, err := instants(rg, periods)
	if err != nil {
		return Sales{}, err
	}

	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return Sales{}, fmt.Errorf("reports: %w", err)
	}
	orders, err := ordersTouching(ctx, q, venueID, from, to)
	if err != nil {
		return Sales{}, err
	}
	categories, err := itemCategories(ctx, q, venueID)
	if err != nil {
		return Sales{}, err
	}
	res, err := report.Build(report.Input{Range: rg, Currency: v.Currency, Taxes: taxes, Orders: orders, Categories: categories})
	if err != nil {
		return Sales{}, fmt.Errorf("reports: %s to %s: %w", qy.From, qy.To, err)
	}
	staff, err := staffOf(ctx, q, venueID)
	if err != nil {
		return Sales{}, err
	}
	return named(res, staff), nil
}

// SalesTable is one of a range's reports as a table, for a CSV download
// (phase 10, Task 5b, ADR-119): the same figures Sales answers with.
func (s *Service) SalesTable(ctx context.Context, venueID int64, qy Query, sheet report.Sheet) (export.Table, error) {
	// Before the orders are read: a wrong name should not cost a year of them.
	if err := sheet.Check(); err != nil {
		return export.Table{}, err
	}
	res, err := s.Sales(ctx, venueID, qy)
	if err != nil {
		return export.Table{}, err
	}
	return res.Table(sheet)
}

// instants is the UTC span the range's trading days run over, to read the
// orders it touches. Events are placed by their trading day afterwards, so
// a span a little wide on a daylight-saving night costs nothing but reads.
func instants(rg report.Range, periods []report.Period) (time.Time, time.Time, error) {
	if len(periods) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: no days", report.ErrBadRange)
	}
	first, err := time.ParseInLocation("2006-01-02", rg.From, rg.Loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: %v", report.ErrBadRange, err)
	}
	last, err := time.ParseInLocation("2006-01-02", rg.To, rg.Loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: %v", report.ErrBadRange, err)
	}
	from := first.Add(rg.DayStart - time.Hour)
	to := last.AddDate(0, 0, 1).Add(rg.DayStart + time.Hour)
	return from.UTC(), to.UTC(), nil
}

// ordersTouching is the whole streams of the orders with any event between
// from and to.
func ordersTouching(ctx context.Context, q *gen.Queries, venueID int64, from, to time.Time) ([][]shift.StampedEvent, error) {
	rows, err := q.OrderEventsTouchingRange(ctx, gen.OrderEventsTouchingRangeParams{VenueID: venueID, FromAt: from, ToAt: to})
	if err != nil {
		return nil, fmt.Errorf("reports: reading the orders: %w", err)
	}
	var orders [][]shift.StampedEvent
	last := int64(0)
	for _, r := range rows {
		p, err := order.DecodePayload(order.Type(r.Type), json.RawMessage(r.Payload))
		if err != nil {
			return nil, fmt.Errorf("reports: order %d, seq %d: %w", r.OrderID, r.Seq, err)
		}
		if r.OrderID != last || len(orders) == 0 {
			orders = append(orders, nil)
			last = r.OrderID
		}
		e := order.Event{Seq: r.Seq, Type: order.Type(r.Type), At: r.At.UTC(), ActorID: r.ActorID, DeviceID: r.DeviceID, Payload: p}
		orders[len(orders)-1] = append(orders[len(orders)-1], shift.StampedEvent{Event: e, ShiftID: r.ShiftID})
	}
	return orders, nil
}

func itemCategories(ctx context.Context, q *gen.Queries, venueID int64) (map[int64]zreport.Category, error) {
	rows, err := q.ItemCategoriesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("reports: reading the menu's categories: %w", err)
	}
	out := make(map[int64]zreport.Category, len(rows))
	for _, r := range rows {
		out[r.ItemID] = zreport.Category{ID: r.CategoryID, Name: r.CategoryName}
	}
	return out, nil
}

// person is a staff member as a report shows them: their name and their role's.
type person struct {
	name, role string
}

// staffOf is everyone at the venue, inactive too, by ID.
func staffOf(ctx context.Context, q *gen.Queries, venueID int64) (map[int64]person, error) {
	rows, err := q.StaffForVenue(ctx, venueID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reports: reading staff: %w", err)
	}
	out := make(map[int64]person, len(rows))
	for _, r := range rows {
		out[r.ID] = person{name: r.Name, role: r.RoleName}
	}
	return out, nil
}

// named puts names to the staff the report mentions: each staff group is
// named, and Names and Roles hold everyone it mentions, removed lines included.
func named(res report.Result, all map[int64]person) Sales {
	names, roles := map[int64]string{}, map[int64]string{}
	mention := func(id int64) string {
		p, ok := all[id]
		if !ok {
			p = person{name: "Someone no longer on record"}
		}
		names[id], roles[id] = p.name, p.role
		return p.name
	}
	byStaff := slices.Clone(res.Totals.ByStaff)
	for i, g := range byStaff {
		if id, err := strconv.ParseInt(g.Key, 10, 64); err == nil && id != 0 {
			byStaff[i].Name = mention(id)
		}
	}
	for _, r := range res.Totals.Removed {
		mention(r.StaffID)
	}
	res.Totals.ByStaff = byStaff
	return Sales{Result: res, Names: names, Roles: roles}
}
