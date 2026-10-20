package charges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
)

// Standing is how paying online stands at a venue, for its owner to read
// (phase 11, Task 4f6c, ADR-231). It names the gateway and never a key.
type Standing struct {
	// Gateway is the provider's name as it is configured: "razorpay".
	Gateway string
	// Notices is whether the gateway's own calls are taken (ADR-224).
	// Without them a payment lands when the sweep next asks.
	Notices bool
	// LastPaidAt is when a payment through this gateway last landed on a
	// bill, or the zero time when none ever has: a gateway nobody has seen
	// take a payment is one to try before it is relied on.
	LastPaidAt time.Time
}

// Standing reads how paying online stands. It asks the gateway nothing.
func (s *Service) Standing(ctx context.Context, venueID int64) (Standing, error) {
	gw, err := s.gateways(ctx, venueID)
	if err != nil {
		return Standing{}, err
	}
	out := Standing{Gateway: gw.Name(), Notices: s.notices != nil}
	at, err := s.db.Queries().LastPaidGatewayCharge(ctx, gen.LastPaidGatewayChargeParams{VenueID: venueID, Gateway: gw.Name()})
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return Standing{}, fmt.Errorf("reading the last charge paid at venue %d: %w", venueID, err)
	case at.Valid:
		out.LastPaidAt = at.Time.UTC()
	}
	return out, nil
}
