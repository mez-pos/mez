package shift

import (
	"context"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/zreport"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// everyOrder is SQLite's LIMIT for no limit: a close counts every order it
// carries, where the tills' list of open orders is capped (ADR-074).
const everyOrder = -1

// carried is the venue's orders still open as its shift closes, in the
// caller's transaction, and what is still to pay on them (ADR-384). An
// order counts as the Orders screen's Open tab counts it: a table's ticket
// paid in full and kept for the party's next round is paid, not open
// (ADR-383). Open orders are not closed with the shift; this only says how
// many go on into the next one.
func (s *Service) carried(ctx context.Context, q *gen.Queries, venueID int64, currency string, taxes order.Taxes) (zreport.Tally, error) {
	out := zreport.Tally{Amount: money.Amount{Currency: currency}}
	rows, err := q.UnclosedOrdersForVenue(ctx, gen.UnclosedOrdersForVenueParams{VenueID: venueID, Limit: everyOrder})
	if err != nil {
		return zreport.Tally{}, fmt.Errorf("listing the open orders: %w", err)
	}
	for _, r := range rows {
		st, err := s.db.LoadOrderIn(ctx, q, r.ID, taxes)
		if err != nil {
			// One order that will not project must not keep the day from
			// closing, as it does not take the tills' list down
			// (OpenOrders). It is said, loudly, and left out of the count.
			s.log.Error("an open order could not be read and is left out of the orders carried", "venue_id", venueID, "order_id", r.ID, "error", err)
			continue
		}
		if svcorder.DayStatusOf(st) != svcorder.DayOpen {
			continue
		}
		out.Count++
		if due := st.Totals.BalanceDue; due.Minor > 0 {
			if out.Amount, err = out.Amount.Add(due); err != nil {
				return zreport.Tally{}, fmt.Errorf("adding up order %d: %w", r.ID, err)
			}
		}
	}
	return out, nil
}
