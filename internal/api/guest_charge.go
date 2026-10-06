package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
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
	CodeChargeNotWaiting   ErrorCode = "charge_not_waiting"
)

// Charges is what the API needs to take a guest's payment online.
// *charges.Service is it.
type Charges interface {
	Start(ctx context.Context, venueID, tableID int64, description, returnURL string) (charges.Charge, error)
	Settle(ctx context.Context, venueID int64, publicID string) (charges.Charge, error)
	Charge(ctx context.Context, venueID int64, publicID string) (charges.Charge, error)
	Waiting(ctx context.Context, venueID int64) ([]charges.UnappliedCharge, error)
	Resolve(ctx context.Context, venueID, staffID int64, publicID string, how charges.Resolution) error
	Notice(ctx context.Context, gateway string, header http.Header, body []byte) error
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

// Unapplied charges, for staff (phase 11, Task 4f4, ADR-223):
//
//	POST /api/v1/charges/{id}/resolve    say how one was dealt with
//
// and the ones nobody has dealt with ride on GET /api/v1/guest-requests, as
// the requests and the paid notices do.

// unappliedChargeResponse is money a gateway took that no bill could, for
// a person to refund at the gateway or record on a bill by hand. reference
// is the gateway's id for the payment, which a refund there names.
type unappliedChargeResponse struct {
	ID        string       `json:"id"`
	TableID   int64        `json:"table_id"`
	TableName string       `json:"table_name"`
	OrderID   string       `json:"order_id"`
	Gateway   string       `json:"gateway"`
	GatewayID string       `json:"gateway_id"`
	Reference string       `json:"reference"`
	Method    string       `json:"method"`
	Amount    money.Amount `json:"amount"`
	Problem   string       `json:"problem"`
	At        time.Time    `json:"at"`
}

// resolveChargeRequest says what was done about an unapplied charge:
// refunded at the gateway, or recorded on a bill by hand.
type resolveChargeRequest struct {
	How charges.Resolution `json:"how"`
}

func toUnappliedResponses(list []charges.UnappliedCharge) []unappliedChargeResponse {
	out := make([]unappliedChargeResponse, len(list))
	for i, c := range list {
		out[i] = unappliedChargeResponse{
			ID: c.PublicID, TableID: c.TableID, TableName: c.TableName, OrderID: c.OrderPublicID,
			Gateway: c.Gateway, GatewayID: c.GatewayID, Reference: c.Reference, Method: c.Method,
			Amount: c.Amount, Problem: c.Problem, At: c.At,
		}
	}
	return out
}

// mountChargeResolution routes the staff's half. Saying a payment was
// refunded or recorded is part of handling payments, and asks what taking
// one asks.
func (s *Server) mountChargeResolution(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.charges == nil {
		return
	}
	mux.Handle("POST /api/v1/charges/{id}/resolve",
		s.requireDevice(s.requireStaff(s.requirePermission(auth.PermPaymentTake, http.HandlerFunc(s.resolveCharge)))))
}

func (s *Server) resolveCharge(w http.ResponseWriter, r *http.Request) {
	var req resolveChargeRequest
	if !s.decode(w, r, &req) {
		return
	}
	meta := Meta(r)
	err := s.charges.Resolve(r.Context(), meta.VenueID, meta.ActorID, r.PathValue("id"), req.How)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, charges.ErrResolution):
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "Say how it was dealt with: refunded or recorded.", "how")
	case errors.Is(err, charges.ErrChargeNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeChargeNotFound, "That payment is not one of this venue's.")
	case errors.Is(err, charges.ErrNotUnapplied):
		writeError(w, s.log, http.StatusConflict, CodeChargeNotWaiting, "That payment is not waiting to be dealt with: someone may have done it already.")
	default:
		s.log.Error("resolving a charge failed", "request_id", RequestID(r.Context()), "venue_id", meta.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}

// A gateway calls the venue (phase 11, Task 4f5, ADR-224):
//
//	POST /order/api/hook/{gateway}
//
// It is under the guest's mount because that is all the relay forwards, and
// a gateway reaches the venue the way a guest does. It needs no table code
// and no session: the gateway's signature over the body is what is checked.

// maxHookBytes bounds a gateway's notice. Razorpay's are a few kilobytes.
const maxHookBytes = 256 << 10

func (s *Server) gatewayHook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxHookBytes))
	if err != nil {
		writeError(w, s.log, http.StatusRequestEntityTooLarge, CodeInvalidRequest, "the notice is too large")
		return
	}
	err = s.charges.Notice(r.Context(), r.PathValue("gateway"), r.Header, body)
	switch {
	case err == nil:
		// Taken, whether or not it was about a charge of ours: a gateway
		// told no keeps calling, and stops calling a venue that says no
		// too often.
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, charges.ErrBadNotice):
		s.log.Warn("a call to the gateway's hook was not the gateway's", "request_id", RequestID(r.Context()), "gateway", r.PathValue("gateway"))
		writeError(w, s.log, http.StatusUnauthorized, CodeUnauthenticated, "not signed by the gateway")
	case errors.Is(err, charges.ErrNoNotices):
		writeError(w, s.log, http.StatusNotFound, CodeNotFound, "not found")
	default:
		s.log.Error("taking a gateway's notice failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
