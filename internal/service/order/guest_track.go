package order

import (
	"context"
	"fmt"
)

// TrackRound reads a guest's round as it stands, and for one that was sent,
// how far it has come in the kitchen (phase 11, Task 5a, ADR-236).
//
// The progress is the order's own projection, read off the tickets the
// round's lines were sent on: it moves when a cook starts a ticket or bumps
// it, and at no other time. The lines are the round's own, found by where
// they begin on the ticket, so what a waiter or another guest at the table
// ordered has no part in it.
//
// A round is tracked by the table it was placed at. Asked from another table
// it is ErrRoundNotFound, as one that is not there is: a table's code says
// nothing of another table's orders. A round that is waiting, declined or
// unpaid has no progress, and neither has one sent before rounds knew their
// lines.
func (s *Service) TrackRound(ctx context.Context, venueID, tableID int64, publicID string) (Round, error) {
	row, err := s.db.Queries().GuestRoundByPublicID(ctx, publicID)
	if isNoRows(err) || (err == nil && (row.VenueID != venueID || row.TableID != tableID)) {
		return Round{}, fmt.Errorf("%w: %q at table %d", ErrRoundNotFound, publicID, tableID)
	}
	if err != nil {
		return Round{}, fmt.Errorf("reading round %q: %w", publicID, err)
	}
	round, err := heldRoundOf(row)
	if err != nil {
		return Round{}, err
	}
	if round.Status != RoundSent || !row.FirstSeq.Valid {
		return round, nil
	}
	// The ticket may be closed by now: its events are still there to read,
	// and a round that was ready stays ready.
	res, err := s.Order(ctx, venueID, round.OrderID)
	if err != nil {
		return Round{}, fmt.Errorf("reading the ticket of round %q: %w", publicID, err)
	}
	ids := make([]int64, len(round.Lines))
	for i := range ids {
		ids[i] = row.FirstSeq.Int64 + int64(i)
	}
	round.Progress = res.State.Progress(ids)
	return round, nil
}
