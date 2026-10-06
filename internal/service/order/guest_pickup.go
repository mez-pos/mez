package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/order"
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

// maxUnpaidPickups is how many orders to collect may wait at one venue for
// their payment. A table is bounded by its own count; the venue's code is on
// a door, so this is what bounds what a stranger can leave lying. It is high
// enough for a lunch queue all paying at once.
const maxUnpaidPickups = 30

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
			if unpaid >= maxUnpaidPickups {
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

// ErrPickupNotSent means a paid order to collect could not be put on a
// ticket: sending one is not built yet (Task 5f1b). It is an error and not a
// guess at a ticket, so that money taken for one would stay in front of a
// person and never be put on a table's bill by mistake.
var ErrPickupNotSent = errors.New("an order to collect cannot be sent to the kitchen yet")
