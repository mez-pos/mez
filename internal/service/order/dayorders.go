package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// SettingDayStart is the venue setting for when the trading day begins, local
// time, written "04:00". Unset is shift.DefaultDayStart. The shift service
// owns what it means; the key lives here because that package imports this one.
const SettingDayStart = "shift.day_start"

// maxDayOrders caps the day's list: a very busy counter's whole day.
const maxDayOrders = 500

// DayStatus is what the orders list calls an order: one word a cashier
// filters by, where order.Status is the five states the machine moves through.
type DayStatus string

const (
	DayOpen     DayStatus = "open"     // not closed: open, held, being paid, reopened
	DayPaid     DayStatus = "paid"     // closed, and nothing given back
	DayRefunded DayStatus = "refunded" // closed, with a credit note against it
	DayVoided   DayStatus = "voided"   // closed with every line voided: nothing was sold
)

// DayStatuses is every status, for the API's generated vocabulary.
func DayStatuses() []DayStatus {
	return []DayStatus{DayOpen, DayPaid, DayRefunded, DayVoided}
}

// DayStatusOf reads an order's place on the list from its projection.
func DayStatusOf(st order.State) DayStatus {
	if st.Status != order.StatusClosed {
		return DayOpen
	}
	if st.Totals.Credited.Minor > 0 {
		return DayRefunded
	}
	sold := false
	for _, l := range st.Lines {
		// A comped line was served; a voided one never was.
		if l.Status != order.LineStatusVoided {
			sold = true
			break
		}
	}
	if !sold {
		return DayVoided
	}
	return DayPaid
}

// Day is the orders of one trading day, newest first, and what they have
// collected: payments less refunds, tips apart.
type Day struct {
	Day       string // the trading day, 2026-10-05
	Orders    []Result
	Collected money.Amount
}

// DayOrders lists the venue's orders for the trading day it is now: every
// order opened or closed since the day began, and any still open from before
// it, each projected as a read of it would be.
func (s *Service) DayOrders(ctx context.Context, venueID int64) (Day, error) {
	q := s.db.Queries()
	v, err := loadVenue(ctx, q, venueID)
	if err != nil {
		return Day{}, err
	}
	dayStart, err := dayStartOf(ctx, q, venueID)
	if err != nil {
		return Day{}, err
	}
	at := s.clock.Now()
	day, err := shift.TradingDay(at, v.Location, dayStart)
	if err != nil {
		return Day{}, fmt.Errorf("the trading day of venue %d: %w", venueID, err)
	}
	since, err := shift.DayStartAt(at, v.Location, dayStart)
	if err != nil {
		return Day{}, fmt.Errorf("the trading day of venue %d: %w", venueID, err)
	}
	// Timestamps are stored in UTC and compared as text.
	since = since.UTC()
	rows, err := q.OrdersOfDay(ctx, gen.OrdersOfDayParams{VenueID: venueID, Since: since, Max: maxDayOrders})
	if err != nil {
		return Day{}, fmt.Errorf("listing the day's orders of venue %d: %w", venueID, err)
	}
	out := Day{Day: day, Orders: make([]Result, 0, len(rows)), Collected: money.Amount{Currency: v.Currency}}
	for _, r := range rows {
		state, err := s.db.LoadOrder(ctx, r.ID, v.Taxes)
		if err != nil {
			return Day{}, err
		}
		if out.Collected, err = out.Collected.Add(state.Totals.NetPaid); err != nil {
			return Day{}, fmt.Errorf("adding up order %d: %w", r.ID, err)
		}
		out.Orders = append(out.Orders, Result{OrderID: r.ID, PublicID: r.PublicID, State: state})
	}
	return out, nil
}

// dayStartOf is when the venue's trading day begins. A value that is not a
// time of day is ErrVenueConfig: a trading day cannot be guessed at.
func dayStartOf(ctx context.Context, q *gen.Queries, venueID int64) (d time.Duration, err error) {
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingDayStart})
	if errors.Is(err, sql.ErrNoRows) {
		return shift.DefaultDayStart, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", SettingDayStart, err)
	}
	d, err = shift.ParseDayStart(strings.TrimSpace(row.Value))
	if err != nil {
		return 0, fmt.Errorf("%w: %s is %q, not a time of day like 04:00", ErrVenueConfig, SettingDayStart, row.Value)
	}
	return d, nil
}
