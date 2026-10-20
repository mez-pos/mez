package charges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
)

// Unapplied charges, in front of a person (phase 11, Task 4f4, ADR-223).
//
// A charge is unapplied when a gateway has a guest's money and the bill
// could not take it (ADR-220). Nothing in mez can put that right: the money
// is at the gateway, and either it goes back to the guest or it is recorded
// on a bill by someone who knows which. So each one is listed for staff
// until a person says what they did with it.

// ErrNotUnapplied means a charge named to be resolved is not one waiting
// for a person: it was never unapplied, or someone has dealt with it.
var ErrNotUnapplied = errors.New("the charge is not waiting to be dealt with")

// ErrResolution means how a charge was dealt with was not said, or is not
// one of the ways there are.
var ErrResolution = errors.New("say how the charge was dealt with: refunded or recorded")

// Resolution is what a person did about an unapplied charge.
type Resolution string

// The resolutions. The strings are what is stored and sent.
const (
	// Refunded: the money went back to the guest, at the gateway.
	Refunded Resolution = "refunded"
	// Recorded: the money was kept, and recorded on a bill by hand.
	Recorded Resolution = "recorded"
)

// Resolutions is every resolution, for the generated vocabulary.
func Resolutions() []Resolution { return []Resolution{Refunded, Recorded} }

// UnappliedCharge is money a gateway took that no bill could, as staff
// need to see it to deal with it.
type UnappliedCharge struct {
	PublicID      string
	TableID       int64
	TableName     string
	OrderPublicID string
	// Gateway and GatewayID are the provider and its id for the charge;
	// Reference is its id for the payment, the one a refund names.
	Gateway   string
	GatewayID string
	Reference string
	Method    string
	Amount    money.Amount
	// Problem is why the bill could not take it, in words.
	Problem string
	// At is when the gateway's answer was acted on.
	At time.Time
}

// ToCollect stands where a table's name would for the charge of an order to
// collect (ADR-249).
const ToCollect = "To collect"

// Waiting is the unapplied charges nobody has dealt with, oldest first.
func (s *Service) Waiting(ctx context.Context, venueID int64) ([]UnappliedCharge, error) {
	rows, err := s.db.Queries().UnresolvedGatewayCharges(ctx, gen.UnresolvedGatewayChargesParams{VenueID: venueID, Limit: maxSweep})
	if err != nil {
		return nil, fmt.Errorf("listing the unapplied charges of venue %d: %w", venueID, err)
	}
	out := make([]UnappliedCharge, len(rows))
	for i, r := range rows {
		// An order to collect has no table: staff are told what it was
		// for, where a table's name would be.
		if r.TableID == 0 {
			r.TableName = ToCollect
		}
		out[i] = UnappliedCharge{
			PublicID: r.PublicID, TableID: r.TableID, TableName: r.TableName, OrderPublicID: r.OrderPublicID,
			Gateway: r.Gateway, GatewayID: r.GatewayID, Reference: r.Reference.String, Method: r.Method.String,
			Amount:  money.Amount{Minor: r.AmountMinor, Currency: r.Currency},
			Problem: r.Problem.String, At: r.SettledAt.Time.UTC(),
		}
	}
	return out, nil
}

// Resolve records what a member of staff did about an unapplied charge,
// with who and when, and takes it off every terminal. It moves no money and
// touches no bill: the refund is made at the gateway, and a payment kept is
// recorded on a bill as any other, by the person who knows which bill.
//
// Done twice, the second is refused: the first is the record.
func (s *Service) Resolve(ctx context.Context, venueID, staffID int64, publicID string, how Resolution) error {
	if how != Refunded && how != Recorded {
		return fmt.Errorf("%w: %q", ErrResolution, how)
	}
	if staffID < 1 {
		return fmt.Errorf("%w: it is said by a member of staff", ErrResolution)
	}
	row, err := s.row(ctx, venueID, publicID)
	if err != nil {
		return err
	}
	if Status(row.Status) != Unapplied || row.ResolvedAt.Valid {
		return fmt.Errorf("%w: %s", ErrNotUnapplied, publicID)
	}
	at := s.clock.Now().UTC()
	var changed int64
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.ResolveGatewayCharge(ctx, gen.ResolveGatewayChargeParams{
			Resolution: sql.NullString{String: string(how), Valid: true},
			ResolvedAt: sql.NullTime{Time: at, Valid: true}, ResolvedBy: sql.NullInt64{Int64: staffID, Valid: true},
			PublicID: publicID, VenueID: venueID,
		})
		if err != nil {
			return err
		}
		if changed = n; n != 1 {
			return nil
		}
		return q.BumpFloorVersion(ctx, venueID)
	})
	if err != nil {
		return fmt.Errorf("resolving charge %s: %w", publicID, err)
	}
	if changed != 1 {
		return fmt.Errorf("%w: %s", ErrNotUnapplied, publicID)
	}
	s.log.Info("an unapplied charge was dealt with", "venue_id", venueID, "charge", publicID, "resolution", string(how), "staff_id", staffID)
	s.onChange(venueID)
	return nil
}
