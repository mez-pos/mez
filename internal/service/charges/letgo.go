package charges

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/byjoellobo/mez/internal/db/gen"
)

// LetGo gives up a payment page a guest at the table opened (phase 11, Task
// 4g2b, ADR-234): the charge stops being a claim on the bill, so the rest of
// the table can pay that part. It is not the charge's end. Its page can
// still be paid at the gateway, and if it is, the payment settles as any
// does: on the bill if the bill still owes it, kept for a person if not.
//
// The gateway is asked first, so a guest who paid and then gave up is told
// they paid, and nothing is let go. If the gateway cannot be asked, the
// charge is let go all the same: letting go takes no money and loses none.
//
// Only a charge on the table's own bill can be let go. A round's charge
// claims nothing, and another table's is not found. A charge already let go,
// or already closed, is answered as it is.
//
// A charge for the whole of what is left is one charge for every phone at
// the table (ADR-220), so one guest letting go of it lets go for them all.
func (s *Service) LetGo(ctx context.Context, venueID, tableID int64, publicID string) (Charge, error) {
	row, err := s.row(ctx, venueID, publicID)
	if err != nil {
		return Charge{}, err
	}
	if row.TableID != tableID || row.RoundPublicID.Valid {
		return Charge{}, fmt.Errorf("%w: %q is not a charge on the bill at table %d", ErrChargeNotFound, publicID, tableID)
	}
	if Status(row.Status) == Open && !row.LetGoAt.Valid {
		if _, err := s.Settle(ctx, venueID, publicID); err != nil {
			s.log.Warn("a charge being let go could not be settled first", "venue_id", venueID, "charge", publicID, "error", err)
		}
		at := s.clock.Now().UTC()
		err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
			// Only an open charge is marked: one that settle just closed,
			// or that someone else let go, is left as it is.
			_, err := q.LetGoGatewayCharge(ctx, gen.LetGoGatewayChargeParams{LetGoAt: sql.NullTime{Time: at, Valid: true}, ID: row.ID})
			return err
		})
		if err != nil {
			return Charge{}, fmt.Errorf("letting go of charge %s: %w", publicID, err)
		}
		s.log.Info("a charge was let go", "venue_id", venueID, "order_id", row.OrderID.Int64, "charge", publicID)
	}
	after, err := s.row(ctx, venueID, publicID)
	if err != nil {
		return Charge{}, err
	}
	return toCharge(after), nil
}
