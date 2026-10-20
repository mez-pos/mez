package tablecodes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
)

// A venue's pickup code (phase 11, Task 6e, ADR-237, ADR-247) leads to the
// venue and to no table: it is for a guest ordering to collect, from a
// poster on the door or a card on the counter. It is drawn as a table's code
// is, from the same alphabet and at the same length, and is never the same
// string as any table's, live or revoked, so the two are never mistaken.
//
// It is made here, ahead of ordering for pickup (5f). Until that is built
// nothing a guest can ask takes it: it resolves to a venue, and no route
// offers anything for a venue without a table.

// ErrNoVenue is a venue that is not there.
var ErrNoVenue = errors.New("tablecodes: no such venue")

// maxPickupDraws bounds the redraw when a code is one already used. With
// about 59 bits to a code it does not happen; the bound is so that a broken
// source of randomness is an error and not a loop.
const maxPickupDraws = 5

// Venue is what a pickup code leads to.
type Venue struct {
	VenueID   int64
	VenueName string
	Currency  string
}

// Pickup is the venue's live pickup code, or empty when it has none.
func (s *Service) Pickup(ctx context.Context, venueID int64) (string, error) {
	code, err := s.db.Queries().PickupCodeForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("tablecodes: reading venue %d's pickup code: %w", venueID, err)
	}
	return code, nil
}

// RenewPickup gives the venue a new pickup code and turns off the one it
// had, in one transaction: there is never a moment with two live codes.
func (s *Service) RenewPickup(ctx context.Context, venueID int64) (string, error) {
	at := s.now().UTC()
	for range maxPickupDraws {
		code, err := s.draw()
		if err != nil {
			return "", err
		}
		taken := false
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			if _, err := q.VenueByID(ctx, venueID); errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %d", ErrNoVenue, venueID)
			} else if err != nil {
				return err
			}
			// Never a string that is, or was, a table's or a pickup code:
			// what a revoked code said must go on leading nowhere.
			tables, err := q.CodeIsATables(ctx, code)
			if err != nil {
				return err
			}
			pickups, err := q.CodeIsAPickups(ctx, code)
			if err != nil {
				return err
			}
			if tables+pickups > 0 {
				taken = true
				return nil
			}
			if err := q.RevokePickupCodes(ctx, gen.RevokePickupCodesParams{RevokedAt: sql.NullTime{Time: at, Valid: true}, VenueID: venueID}); err != nil {
				return err
			}
			return q.CreatePickupCode(ctx, gen.CreatePickupCodeParams{VenueID: venueID, Code: code, CreatedAt: at})
		})
		if err != nil {
			return "", fmt.Errorf("tablecodes: renewing venue %d's pickup code: %w", venueID, err)
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("tablecodes: renewing venue %d's pickup code: every code drawn was already used", venueID)
}

// RevokePickup turns off the venue's pickup code and gives it no other: the
// poster leads nowhere until the venue is given a code again. A venue with
// none is left as it is.
func (s *Service) RevokePickup(ctx context.Context, venueID int64) error {
	at := s.now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.RevokePickupCodes(ctx, gen.RevokePickupCodesParams{RevokedAt: sql.NullTime{Time: at, Valid: true}, VenueID: venueID})
	})
	if err != nil {
		return fmt.Errorf("tablecodes: turning off venue %d's pickup code: %w", venueID, err)
	}
	return nil
}

// ResolvePickup reads what a pickup code leads to. A table's code is not
// one, and neither is a code turned off: each is ErrUnknown, as a table's
// code that leads nowhere is.
func (s *Service) ResolvePickup(ctx context.Context, code string) (Venue, error) {
	if !WellFormed(code) {
		return Venue{}, ErrUnknown
	}
	row, err := s.db.Queries().VenueByPickupCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return Venue{}, ErrUnknown
	}
	if err != nil {
		return Venue{}, fmt.Errorf("tablecodes: reading a pickup code: %w", err)
	}
	return Venue(row), nil
}
