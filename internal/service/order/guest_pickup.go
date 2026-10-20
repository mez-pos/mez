package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/id"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/order"
)

// An order to collect (phase 11, Task 5f1a, ADR-237, ADR-248): a guest
// orders with no table, by the venue's pickup code, pays, and is called when
// it is ready.
//
// It is a round like any other, with no table. That is all that is new about
// it here: it is priced from the menu as a table's is, held unpaid with the
// amount it asks, and found again by its key. What it is not is a request.
// Nobody is at a table for staff to see, so a pickup order is never one for
// them to accept: it is paid for first, in every venue, whatever the venue
// chose for its tables.

// ErrPickupOff means the venue takes no orders to collect: guests cannot pay
// online here, and an order to collect is always paid first.
var ErrPickupOff = errors.New("the venue takes no orders to collect")

// MaxUnpaidPickups is how many orders to collect may wait at one venue for
// their payment. A table is bounded by its own count; the venue's code is on
// a door, so this is what bounds what a stranger can leave lying. It is high
// enough for a lunch queue all paying at once.
const MaxUnpaidPickups = 30

// PlacePickup is a guest's order to collect: what they picked, and no table.
type PlacePickup struct {
	Picks []menu.Pick `json:"picks"`
}

// PlacePickup takes an order to collect and holds it until it is paid for.
// The answer is the round, unpaid, with what it asks: its lines' value with
// their tax, as a ticket of its own would bill them.
//
// Nothing is an order yet, nothing reaches the kitchen, and no terminal is
// told. Placed again with the same key it is the same round, as it now
// stands.
func (s *Service) PlacePickup(ctx context.Context, meta Meta, cmd PlacePickup) (Round, error) {
	if err := meta.validate(); err != nil {
		return Round{}, err
	}
	if meta.IdempotencyKey == "" {
		return Round{}, fmt.Errorf("%w: a guest's order needs an idempotency key", ErrInvalidCommand)
	}
	if n := len(cmd.Picks); n < 1 || n > MaxRoundLines {
		return Round{}, fmt.Errorf("%w: an order has 1 to %d lines, not %d", ErrInvalidCommand, MaxRoundLines, n)
	}
	// Refused before anything is read: with no way to pay there is no way
	// for this order ever to be sent.
	if !s.canCharge {
		return Round{}, ErrPickupOff
	}
	// A fingerprint of its own: a key used for a table's round is not this
	// order, whatever was in it.
	fp, err := fingerprint("guest_pickup", 0, cmd)
	if err != nil {
		return Round{}, err
	}
	if prior, err := s.priorRound(ctx, meta, fp); err != nil || prior != nil {
		return deref(prior), err
	}

	v, err := loadVenue(ctx, s.db.Queries(), meta.VenueID)
	if err != nil {
		return Round{}, err
	}
	m, err := s.db.Menu(ctx, meta.VenueID)
	if err != nil {
		return Round{}, fmt.Errorf("reading the menu for an order to collect: %w", err)
	}
	at := s.clock.Now().UTC()
	draft, err := priceRound(m, v.Currency, PlaceRound{Picks: cmd.Picks}, at)
	if err != nil {
		return Round{}, err
	}
	draft.key, draft.fingerprint = meta.IdempotencyKey, fp
	// What it asks is what a ticket opened for it alone would come to.
	due, err := order.Adding(nil, draft.lines, v.Taxes, v.Currency, 0, at)
	if err != nil {
		return Round{}, err
	}
	if due.Minor <= 0 {
		return Round{}, fmt.Errorf("%w: the order comes to nothing", ErrInvalidCommand)
	}

	for range maxRoundAttempts {
		publicID, err := id.NewPublic(roundPrefix)
		if err != nil {
			return Round{}, fmt.Errorf("drawing a round id: %w", err)
		}
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			if _, err := takingRequestsSince(ctx, q, meta.VenueID); err != nil {
				return err
			}
			unpaid, err := q.CountUnpaidPickupRounds(ctx, gen.CountUnpaidPickupRoundsParams{VenueID: meta.VenueID, Since: at.Add(-unpaidFor)})
			if err != nil {
				return fmt.Errorf("counting the unpaid orders to collect at venue %d: %w", meta.VenueID, err)
			}
			if unpaid >= MaxUnpaidPickups {
				return fmt.Errorf("%w: venue %d has %d orders to collect unpaid", ErrTooManyWaiting, meta.VenueID, unpaid)
			}
			return saveRound(ctx, q, meta, draft, publicID, RoundUnpaid, 0, due, 0, at)
		})
		switch {
		case err == nil:
			s.log.Info("an order to collect is waiting to be paid for", "venue_id", meta.VenueID, "round", publicID, "due_minor", due.Minor)
			return Round{PublicID: publicID, Status: RoundUnpaid, Lines: draft.lines, Subtotal: draft.subtotal, Due: due}, nil
		case db.IsUniqueViolation(err):
			// The same key twice at once, and the other won: this is its
			// answer. Otherwise the public id collided, and another is drawn.
			if prior, perr := s.priorRound(ctx, meta, fp); perr != nil || prior != nil {
				return deref(prior), perr
			}
			continue
		default:
			return Round{}, err
		}
	}
	return Round{}, fmt.Errorf("placing an order to collect at venue %d: its id kept colliding", meta.VenueID)
}

// isPickup reports whether a round is an order to collect: one with no
// table.
func isPickup(tableID int64) bool { return tableID == 0 }

// ErrPickupCodesOut means every pickup code is on an order still out. It
// passes when one is handed over and its ticket closed.
var ErrPickupCodesOut = errors.New("every pickup code is on an order still out")

// pickupCodeDraws is how many short codes are drawn before a long one is,
// and how many long ones before giving up. With a few thousand short codes
// and a counter that holds a few dozen orders, the first draw is free.
const pickupCodeDraws = 20

// sendPaidPickup puts a paid order to collect on a ticket of its own, sends
// it to the kitchen, and gives it the code it is called out by (ADR-249).
//
// The ticket is opened under a key that is the round's, so a call that
// stopped after opening it finds the same ticket the next time and does not
// open another: the caller has the money already, and this is safe to call
// again. It is on the web channel, to take away, with no table, and it is
// nobody's on staff.
func (s *Service) sendPaidPickup(ctx context.Context, row gen.GuestRoundByPublicIDRow) (Round, error) {
	publicID, venueID := row.PublicID, row.VenueID
	lines, err := decodeLines(publicID, row.Lines)
	if err != nil {
		return Round{}, err
	}
	opened, err := s.Open(ctx, Meta{VenueID: venueID, IdempotencyKey: PickupTicketKeyPrefix + publicID},
		OpenOrder{Channel: order.ChannelWeb, Service: order.ServiceTakeaway})
	if err != nil {
		return Round{}, busy(err)
	}
	guest := Meta{VenueID: venueID}
	_, err = s.run(ctx, guest, opened.OrderID, "paid_pickup", publicID, func(ctx context.Context, tx *db.OrderTx, _ venue, at time.Time) ([]order.Payload, error) {
		if tx.State.Status == order.StatusClosed {
			return nil, fmt.Errorf("%w: its ticket is closed", ErrRoundNotFound)
		}
		code, err := freePickupCode(ctx, tx.Queries(), s.random, venueID)
		if err != nil {
			return nil, err
		}
		first := tx.State.LastSeq + 1
		n, err := tx.Queries().SendPickupRound(ctx, gen.SendPickupRoundParams{
			OrderID: sql.NullInt64{Int64: tx.OrderID, Valid: true}, DecidedAt: sql.NullTime{Time: at, Valid: true},
			FirstSeq: sql.NullInt64{Int64: first, Valid: true}, PickupCode: sql.NullString{String: code, Valid: true},
			ID: row.ID, VenueID: venueID,
		})
		if err != nil {
			return nil, fmt.Errorf("sending order to collect %s: %w", publicID, err)
		}
		if n != 1 {
			return nil, errRoundRaced
		}
		return roundPayloads(lines, first), nil
	})
	if err != nil && !errors.Is(err, errRoundRaced) {
		return Round{}, busy(err)
	}
	after, err := s.heldRound(ctx, venueID, publicID)
	if err != nil {
		return Round{}, err
	}
	if RoundStatus(after.Status) != RoundSent {
		return Round{}, fmt.Errorf("sending order to collect %s: it is %s", publicID, after.Status)
	}
	s.log.Info("a paid order to collect was sent", "venue_id", venueID, "round", publicID, "order_id", after.OrderID.Int64, "pickup_code", after.PickupCode)
	return heldRoundOf(after)
}

// freePickupCode draws a code that no order still out at the venue is called
// by. It is asked inside the write that gives the code, and this process is
// the only writer, so two orders sent at once cannot be given the same one.
func freePickupCode(ctx context.Context, q *gen.Queries, random io.Reader, venueID int64) (string, error) {
	out, err := q.PickupCodesOut(ctx, venueID)
	if err != nil {
		return "", fmt.Errorf("reading the pickup codes out at venue %d: %w", venueID, err)
	}
	for _, length := range []int{order.PickupCodeLength, order.PickupCodeLong} {
		for range pickupCodeDraws {
			code, err := order.NewPickupCode(random, length)
			if err != nil {
				return "", err
			}
			if !slices.Contains(out, code) {
				return code, nil
			}
		}
	}
	return "", fmt.Errorf("%w: venue %d has %d out", ErrPickupCodesOut, venueID, len(out))
}
