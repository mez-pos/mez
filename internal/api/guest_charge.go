package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/service/charges"
)

// A guest pays online (phase 11, Task 4f3, ADR-222):
//
//	POST /order/api/t/{code}/bill/charge        start paying what is owed
//	GET  /order/api/t/{code}/bill/charge/{id}   how that payment stands
//
// The phone is given the gateway's page to go to, and afterwards asks how
// the charge stands. The answer is the gateway's, asked by this server: the
// phone is believed about nothing.

// What paying online is refused with.
const (
	CodeGatewayUnavailable ErrorCode = "gateway_unavailable"
	CodeChargeNotFound     ErrorCode = "charge_not_found"
)

// Charges is what the API needs to take a guest's payment online.
// *charges.Service is it.
type Charges interface {
	Start(ctx context.Context, venueID, tableID int64, description, returnURL string) (charges.Charge, error)
	Settle(ctx context.Context, venueID int64, publicID string) (charges.Charge, error)
	Charge(ctx context.Context, venueID int64, publicID string) (charges.Charge, error)
}

// WithCharges lets guests pay online. c is nil at a venue with no gateway,
// and then nothing here is mounted and no bill offers it.
//
// publicURL is the address guests reach this venue at, from the venue's own
// configuration, or empty. It is where the gateway sends a guest back. It
// is never taken from a request: a charge is shared by a table, and a
// return address one phone could choose is one the next phone at the table
// would be sent to after paying.
func WithCharges(c Charges, publicURL string) Option {
	return func(s *Server) {
		s.charges = c
		s.publicURL = strings.TrimRight(publicURL, "/")
	}
}

// guestChargeResponse is a guest's online payment as it stands.
//
//	open       not paid yet: url is the gateway's page to pay it at
//	paid       paid, and on the bill
//	failed     not paid, and it will not be: nothing was taken
//	unapplied  the gateway took it and the bill could not: staff must sort
//	           it out, and the guest is told to show them
type guestChargeResponse struct {
	ID     string         `json:"id"`
	Status charges.Status `json:"status"`
	Amount money.Amount   `json:"amount"`
	URL    string         `json:"url,omitempty"`
}

func toGuestChargeResponse(c charges.Charge) guestChargeResponse {
	out := guestChargeResponse{ID: c.PublicID, Status: c.Status, Amount: c.Amount}
	if c.Status == charges.Open {
		out.URL = c.CheckoutURL
	}
	return out
}

func (s *Server) guestStartCharge(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	back := ""
	if s.publicURL != "" {
		// The guest's own table, with no fragment: a gateway adds its own
		// words to the end of the address, and the app finds its way to the
		// bill by itself.
		back = s.publicURL + s.guestMount + "t/" + r.PathValue("code")
	}
	c, err := s.charges.Start(r.Context(), table.VenueID, table.TableID, "Table "+table.TableName+", "+table.VenueName, back)
	switch {
	case err == nil:
	case errors.Is(err, charges.ErrNothingToPay):
		writeError(w, s.log, http.StatusConflict, CodeNothingToPay, "There is nothing left to pay on this bill.")
		return
	case errors.Is(err, gateway.ErrRefused):
		// The venue's keys, or its account: the owner's to fix, and loud.
		s.log.Error("the payment gateway refused to start a charge: check the venue's gateway keys",
			"request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusBadGateway, CodeGatewayUnavailable, "Paying online is not working right now. Pay a member of staff, or at the counter.")
		return
	case errors.Is(err, gateway.ErrUnavailable):
		s.log.Warn("the payment gateway could not be reached to start a charge", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusServiceUnavailable, CodeGatewayUnavailable, "Paying online can't be reached right now. Try again, or pay a member of staff.")
		return
	default:
		s.log.Error("starting a guest's charge failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	status := http.StatusCreated
	if c.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, s.log, status, toGuestChargeResponse(c))
}

func (s *Server) guestCharge(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id := r.PathValue("id")
	notFound := func() {
		writeError(w, s.log, http.StatusNotFound, CodeChargeNotFound, "That payment is not one of this table's.")
	}
	// A charge is asked about by the table it was started at, and no other:
	// a table's code says nothing of another table's payments.
	stored, err := s.charges.Charge(r.Context(), table.VenueID, id)
	if errors.Is(err, charges.ErrChargeNotFound) || (err == nil && stored.TableID != table.TableID) {
		notFound()
		return
	}
	if err != nil {
		s.log.Error("reading a guest's charge failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	c, err := s.charges.Settle(r.Context(), table.VenueID, id)
	if err != nil {
		// Nothing was decided: the gateway was not reached, or the bill
		// could not take the payment just now. The charge is as it was,
		// which is what the phone is told, and it asks again.
		s.log.Warn("a guest's charge could not be settled just now", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "charge", id, "error", err)
		c = stored
	}
	writeJSON(w, s.log, http.StatusOK, toGuestChargeResponse(c))
}
