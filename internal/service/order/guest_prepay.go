package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// A round held until it is paid, from the side of its payment (phase 11,
// Task 4e5b, ADR-226). The charge is asked and settled in
// internal/service/charges; what is here is what it needs of the round: to
// read one that can still be paid for, and to put a paid one on a ticket.

// ErrRoundPaid means a round named to be paid for is already on a ticket.
var ErrRoundPaid = errors.New("the round is already paid for and sent")

// PayableRound reads a round a guest may be asked to pay for now: one held
// unpaid at that table, placed in the shift that is open. Its Due is what
// to ask.
//
// It is refused with no shift open, as placing it was, and once the shift
// it was placed in has closed: nobody should pay tonight for an order from
// last night.
func (s *Service) PayableRound(ctx context.Context, venueID, tableID int64, publicID string) (Round, error) {
	row, err := s.heldRound(ctx, venueID, publicID)
	if err != nil {
		return Round{}, err
	}
	if row.TableID != tableID {
		return Round{}, fmt.Errorf("%w: %q", ErrRoundNotFound, publicID)
	}
	if RoundStatus(row.Status) != RoundWaiting {
		return Round{}, fmt.Errorf("%w: %s", ErrRoundPaid, publicID)
	}
	if err := s.inThisShift(ctx, venueID, row.CreatedAt, publicID); err != nil {
		return Round{}, err
	}
	return heldRoundOf(row)
}

// SendPaidRound puts a paid round's lines on its table's ticket, opening one
// if there is none, and sends them to their stations: what accepting a
// request does, with the payment in place of the acceptance (ADR-209). The
// lines are the ones priced when the guest asked.
//
// The caller has the money already, so this is safe to call again: a round
// already sent is answered as it is, with its ticket. If the ticket is
// opened and the lines never appended, the next call finishes on that
// ticket.
//
// The events are nobody's: no member of staff let this order in.
func (s *Service) SendPaidRound(ctx context.Context, venueID int64, publicID string) (Round, error) {
	// One at a time with accepting: the look for the table's ticket and the
	// opening of one are two steps.
	s.accepting.Lock()
	defer s.accepting.Unlock()

	row, err := s.heldRound(ctx, venueID, publicID)
	if err != nil {
		return Round{}, err
	}
	if RoundStatus(row.Status) == RoundSent {
		return heldRoundOf(row)
	}
	if err := s.inThisShift(ctx, venueID, row.CreatedAt, publicID); err != nil {
		return Round{}, err
	}
	// An order to collect has no table to find a ticket by: it is sent on a
	// ticket of its own.
	if isPickup(row.TableID) {
		return s.sendPaidPickup(ctx, row)
	}
	lines, err := decodeLines(row.PublicID, row.Lines)
	if err != nil {
		return Round{}, err
	}

	guest := Meta{VenueID: venueID}
	for range maxRoundAttempts {
		orderID, err := s.ticketAt(ctx, guest, row.TableID)
		if err != nil {
			return Round{}, busy(err)
		}
		_, err = s.run(ctx, guest, orderID, "paid_round", publicID, func(ctx context.Context, tx *db.OrderTx, _ venue, at time.Time) ([]order.Payload, error) {
			if tx.State.Status == order.StatusClosed || !slices.Contains(tx.State.Tables(), row.TableID) {
				return nil, errTableChanged
			}
			n, err := tx.Queries().DecideGuestRound(ctx, gen.DecideGuestRoundParams{
				Status: string(RoundSent), OrderID: sql.NullInt64{Int64: tx.OrderID, Valid: true},
				DecidedAt: sql.NullTime{Time: at, Valid: true}, FirstSeq: sql.NullInt64{Int64: tx.State.LastSeq + 1, Valid: true},
				ID: row.ID, VenueID: venueID,
			})
			if err != nil {
				return nil, fmt.Errorf("sending paid round %s: %w", publicID, err)
			}
			if n != 1 {
				return nil, errRoundRaced
			}
			return roundPayloads(lines, tx.State.LastSeq+1), nil
		})
		switch {
		case errors.Is(err, errTableChanged):
			continue
		case err != nil && !errors.Is(err, errRoundRaced):
			return Round{}, busy(err)
		}
		after, err := s.heldRound(ctx, venueID, publicID)
		if err != nil {
			return Round{}, err
		}
		if RoundStatus(after.Status) != RoundSent {
			return Round{}, fmt.Errorf("sending paid round %s: it is %s", publicID, after.Status)
		}
		s.log.Info("a guest's paid order was sent", "venue_id", venueID, "table_id", row.TableID, "round", publicID, "order_id", after.OrderID.Int64)
		return heldRoundOf(after)
	}
	return Round{}, fmt.Errorf("sending paid round %s: table %d kept changing", publicID, row.TableID)
}

// heldRound reads a round that was held for its payment, paid since or not,
// if it is at the caller's venue. A request is not one.
func (s *Service) heldRound(ctx context.Context, venueID int64, publicID string) (gen.GuestRoundByPublicIDRow, error) {
	row, err := s.db.Queries().GuestRoundByPublicID(ctx, publicID)
	if isNoRows(err) || (err == nil && (row.VenueID != venueID || row.Prepay != 1)) {
		return gen.GuestRoundByPublicIDRow{}, fmt.Errorf("%w: %q", ErrRoundNotFound, publicID)
	}
	if err != nil {
		return gen.GuestRoundByPublicIDRow{}, fmt.Errorf("reading round %q: %w", publicID, err)
	}
	return row, nil
}

// inThisShift refuses a round with no shift open, or placed before the one
// that is.
func (s *Service) inThisShift(ctx context.Context, venueID int64, placed time.Time, publicID string) error {
	since, err := takingRequestsSince(ctx, s.db.Queries(), venueID)
	if err != nil {
		return err
	}
	if placed.Before(since) {
		return fmt.Errorf("%w: %s", ErrRoundLapsed, publicID)
	}
	return nil
}

func heldRoundOf(row gen.GuestRoundByPublicIDRow) (Round, error) {
	lines, err := decodeLines(row.PublicID, row.Lines)
	if err != nil {
		return Round{}, err
	}
	return Round{
		PublicID: row.PublicID, Status: statusOf(row.Status, row.Prepay), TableID: row.TableID, OrderID: row.OrderID.Int64,
		Lines: lines, Subtotal: money.Amount{Minor: row.SubtotalMinor, Currency: row.Currency},
		Due:        dueOf(row.Status, row.Prepay, row.DueMinor, row.Currency),
		PickupCode: row.PickupCode,
	}, nil
}
