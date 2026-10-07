package api

import (
	"errors"
	"net/http"

	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A guest asks how their order stands (phase 11, Task 5b, ADR-238):
//
//	GET /order/api/t/{code}/orders/{round}
//
// The answer is the order as it was priced, its status, and once it is sent
// how far the kitchen has come with it. Every word of that is read off the
// order's own events: a cook starting a ticket and bumping it. Nothing is
// timed and nothing is guessed.

func (s *Server) guestTrackOrder(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	// It moves when the kitchen does: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	round, err := s.guestOrders.TrackRound(r.Context(), table.VenueID, table.TableID, r.PathValue("round"))
	if errors.Is(err, svcorder.ErrRoundNotFound) {
		// One answer for an order that is not there and one placed at
		// another table: a table's code says nothing of another's orders.
		writeError(w, s.log, http.StatusNotFound, CodeRequestNotFound, "That order is not one of this table's.")
		return
	}
	if err != nil {
		s.internalError(w, r, "tracking a guest's order failed", err, "venue_id", table.VenueID)
		return
	}
	resp, err := toGuestRoundResponse(round)
	if err != nil {
		s.internalError(w, r, "encoding a guest's order failed", err, "round", round.PublicID)
		return
	}
	writeJSON(w, s.log, http.StatusOK, resp)
}
