package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/service/charges"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/tablecodes"
)

// A guest orders to collect (phase 11, Task 5f2, ADR-250):
//
//	GET  /order/api/p/{code}                        the venue, and whether it
//	                                                takes orders to collect
//	GET  /order/api/p/{code}/menu                   its menu
//	POST /order/api/p/{code}/orders                 place an order, which is
//	                                                held until it is paid for
//	GET  /order/api/p/{code}/orders/{round}         how it stands, and once it
//	                                                is sent, its pickup code
//	POST /order/api/p/{code}/orders/{round}/charge  start paying for it
//	GET  /order/api/p/{code}/charge/{id}            how that payment stands
//
// The code is the venue's pickup code (ADR-247), from a poster on the door,
// and "p" is where a table's routes have "t". It leads to the venue and to
// no table, so there is no bill here, no request for staff and no receipt:
// an order to collect is paid for first, always (ADR-248), and paid, it is a
// ticket of its own with a code to call out (ADR-249).
//
// A table's code is kept by the people at the table. This one is on a door.
// So under it an order and a payment are asked for by their own ids, which
// are drawn at random and told to the phone that placed the order and to
// nobody else: holding the venue's code says nothing of anyone's order.

// What ordering to collect is refused with.
const (
	// CodePickupCodeUnknown is a pickup code that leads nowhere.
	CodePickupCodeUnknown ErrorCode = "pickup_code_unknown"
	// CodePickupOff: the venue takes no orders to collect, since guests
	// cannot pay online here.
	CodePickupOff ErrorCode = "pickup_off"
)

// What the venue's pickup code may ask, and how often. ADR-246's limits are
// by table, and there is none here: each is counted twice, by the address
// under the code and then by the venue.
//
// The venue's count is what the code can do to the venue. But the code is
// public, and a count by the venue alone is one that a single phone could
// use up for everybody. So an address is stopped first, well short of it,
// and what it is refused is not counted against the venue.
var (
	// pickupReadLimit is what may be read by the venue's code: a lunch
	// queue of thirty, each tracking an order and asking about a payment at
	// its fastest, is about 660 a minute.
	pickupReadLimit = limit{requests: 1500, per: time.Minute}
	// pickupReadByAddr is what one address may read of it. Phones ordering
	// to collect are mostly on their own data, one to an address; this
	// leaves room for a dozen behind a shared one.
	pickupReadByAddr = limit{requests: 300, per: time.Minute}

	// pickupWriteLimit is what the venue's code may change: place an order,
	// start paying for one.
	pickupWriteLimit = limit{requests: 120, per: time.Minute}
	// pickupWriteByAddr is what one address may change, as a table may.
	pickupWriteByAddr = limit{requests: 30, per: time.Minute}

	// pickupOrderLimit is how many orders to collect the venue's code may
	// place. None reaches the kitchen unpaid, so this guards no printer: it
	// bounds what can be left lying, of which the venue holds thirty at once
	// (svcorder.MaxUnpaidPickups).
	pickupOrderLimit = limit{requests: 60, per: 10 * time.Minute}
	// pickupOrderByAddr is how many one address may place. It is what stops
	// one stranger filling the thirty: over the half hour an unpaid order
	// counts for, this is half of them.
	pickupOrderByAddr = limit{requests: 5, per: 10 * time.Minute}
)

// pickupUnknownMessage is the one answer for a pickup code never made, one
// turned off, and a table's code put where the venue's goes.
const pickupUnknownMessage = "This code does not lead anywhere. Scan the code again, or order at the counter."

// pickupOffMessage is what a venue with no way to pay online says.
const pickupOffMessage = "This venue is not taking orders to collect online. Order at the counter."

// guestPickupResponse is GET /order/api/p/{code}: where the venue's pickup
// code leads.
type guestPickupResponse struct {
	Venue guestVenueResponse `json:"venue"`
	// Ordering is whether an order to collect can be placed here. It is
	// false at a venue where guests cannot pay online, and then everything
	// else under the code answers pickup_off.
	Ordering bool `json:"ordering"`
}

// mountPickup routes /order/api/p/. Every route is there whatever the venue
// has: where orders to collect are not taken the code still leads to the
// venue, and the rest say so, which a route that was absent could not.
func (s *Server) mountPickup(handle func(pattern string, h http.HandlerFunc), base string) {
	handle("GET "+base+"p/{code}", s.guestPickupVenue)
	handle("GET "+base+"p/{code}/menu", s.guestPickupMenu)
	handle("POST "+base+"p/{code}/orders", s.guestPlacePickup)
	handle("GET "+base+"p/{code}/orders/{round}", s.guestTrackPickup)
	handle("POST "+base+"p/{code}/orders/{round}/charge", s.guestStartPickupCharge)
	handle("GET "+base+"p/{code}/charge/{id}", s.guestPickupCharge)
}

// pickupOn reports whether orders to collect are taken: they are paid
// first, so only where guests can pay online.
func (s *Server) pickupOn() bool {
	return s.menus != nil && s.guestOrders != nil && s.charges != nil
}

// pickupVenueFor reads the pickup code in the path, and answers for one
// that leads nowhere or is asking too often.
func (s *Server) pickupVenueFor(w http.ResponseWriter, r *http.Request) (tablecodes.Venue, bool) {
	// The same count as wrong table codes: an address guessing codes is
	// guessing, whichever kind it tries.
	addr := remoteHost(r)
	if !s.limiter.within("guest-bad:"+addr, guestBadCodeLimit) {
		s.tooMany(w, "guest-bad:"+addr, guestBadCodeLimit, "Too many codes tried from here. Wait a minute, then scan the code again.")
		return tablecodes.Venue{}, false
	}
	venue, err := s.guests.ResolvePickup(r.Context(), r.PathValue("code"))
	if errors.Is(err, tablecodes.ErrUnknown) {
		s.limiter.allow("guest-bad:"+addr, guestBadCodeLimit)
		writeError(w, s.log, http.StatusNotFound, CodePickupCodeUnknown, pickupUnknownMessage)
		return tablecodes.Venue{}, false
	}
	if err != nil {
		s.internalError(w, r, "reading a pickup code failed", err)
		return tablecodes.Venue{}, false
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID = venue.VenueID
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return venue, s.pickupAllow(w, r, venue.VenueID, "r", pickupReadByAddr, pickupReadLimit, "Too much is being asked at once. Wait a moment.")
	}
	return venue, s.pickupAllow(w, r, venue.VenueID, "w", pickupWriteByAddr, pickupWriteLimit, "Too much is being done at once. Wait a moment and try again, or order at the counter.")
}

// pickupOrderingFor is pickupVenueFor at a venue that takes orders to
// collect, and has answered for one that does not.
func (s *Server) pickupOrderingFor(w http.ResponseWriter, r *http.Request) (tablecodes.Venue, bool) {
	venue, ok := s.pickupVenueFor(w, r)
	if !ok {
		return tablecodes.Venue{}, false
	}
	if !s.pickupOn() {
		writeError(w, s.log, http.StatusConflict, CodePickupOff, pickupOffMessage)
		return tablecodes.Venue{}, false
	}
	return venue, true
}

// pickupAllow counts one thing done by the venue's pickup code, against the
// address's limit and then the venue's, and has answered when either is
// over. An address over its own is not counted against the venue.
func (s *Server) pickupAllow(w http.ResponseWriter, r *http.Request, venueID int64, kind string, byAddr, byVenue limit, message string) bool {
	key := "pickup-" + kind + ":" + strconv.FormatInt(venueID, 10)
	counts := []struct {
		by  string
		key string
		lim limit
	}{
		{"address", key + ":" + remoteHost(r), byAddr},
		{"venue", key, byVenue},
	}
	for _, c := range counts {
		ok, retryAfter := s.limiter.allow(c.key, c.lim)
		if ok {
			continue
		}
		// The venue at its limit is a queue being refused, or someone
		// spending the code from many addresses: either wants seeing.
		s.log.Warn("the venue's pickup code is over a limit", "venue_id", venueID, "by", c.by, "kind", kind, "limit", c.lim.requests, "per", c.lim.per.String())
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
		writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, message)
		return false
	}
	return true
}

func (s *Server) guestPickupVenue(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupVenueFor(w, r)
	if !ok {
		return
	}
	// A code can be turned off at any moment: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, guestPickupResponse{
		Venue:    guestVenueResponse{Name: venue.VenueName, Currency: venue.Currency, TaxAdded: s.guestTaxAdded(r, venue.VenueID)},
		Ordering: s.pickupOn(),
	})
}

func (s *Server) guestPickupMenu(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupOrderingFor(w, r)
	if !ok {
		return
	}
	s.writeGuestMenu(w, r, venue.VenueID)
}

func (s *Server) guestPlacePickup(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupOrderingFor(w, r)
	if !ok {
		return
	}
	// Counted before the body is read, and whether or not the order is
	// taken, as a table's is.
	if !s.pickupAllow(w, r, venue.VenueID, "o", pickupOrderByAddr, pickupOrderLimit, "A lot of orders have been placed from here. Wait a few minutes, or order at the counter.") {
		return
	}
	var req guestOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	cmd := svcorder.PlacePickup{Picks: make([]menu.Pick, len(req.Lines))}
	for i, l := range req.Lines {
		cmd.Picks[i] = menu.Pick{ItemID: l.ItemID, ModifierIDs: l.ModifierIDs, Qty: l.Qty}
	}
	meta := svcorder.Meta{VenueID: venue.VenueID, IdempotencyKey: r.Header.Get(HeaderIdempotencyKey)}
	round, err := s.guestOrders.PlacePickup(r.Context(), meta, cmd)
	switch {
	case err == nil:
	case errors.Is(err, svcorder.ErrPickupOff):
		writeError(w, s.log, http.StatusConflict, CodePickupOff, pickupOffMessage)
		return
	case errors.Is(err, svcorder.ErrTooManyWaiting):
		// The venue's own bound on what may wait unpaid, not a table's.
		writeError(w, s.log, http.StatusConflict, CodeRequestsWaiting, "A lot of orders here are waiting to be paid for. Try again in a few minutes, or order at the counter.")
		return
	default:
		s.writeGuestOrderError(w, r, err)
		return
	}
	resp, err := toGuestRoundResponse(round)
	if err != nil {
		s.internalError(w, r, "encoding an order to collect failed", err, "round", round.PublicID)
		return
	}
	status := http.StatusCreated
	if round.Replayed {
		status = http.StatusOK
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, status, resp)
}

// guestTrackPickup answers an order to collect as it stands. It is asked for
// by its own id and no table: a round placed at a table is not found here,
// as one that is not there is not.
func (s *Server) guestTrackPickup(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupOrderingFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	round, err := s.guestOrders.TrackRound(r.Context(), venue.VenueID, 0, r.PathValue("round"))
	if errors.Is(err, svcorder.ErrRoundNotFound) {
		writeError(w, s.log, http.StatusNotFound, CodeRequestNotFound, "That order is not one placed to collect here.")
		return
	}
	if err != nil {
		s.internalError(w, r, "tracking an order to collect failed", err, "venue_id", venue.VenueID)
		return
	}
	// Its ticket is not said: a receipt is asked for by a table's code
	// (ADR-240), and there is none to ask by.
	round.Bill = ""
	resp, err := toGuestRoundResponse(round)
	if err != nil {
		s.internalError(w, r, "encoding an order to collect failed", err, "round", round.PublicID)
		return
	}
	writeJSON(w, s.log, http.StatusOK, resp)
}

// guestStartPickupCharge starts paying for an order to collect: a charge
// with no table (ADR-249). Paid, the order is sent.
func (s *Server) guestStartPickupCharge(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupOrderingFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	c, err := s.charges.StartRound(r.Context(), venue.VenueID, 0, r.PathValue("round"), "Order to collect, "+venue.VenueName, s.pickupReturnURL(r))
	switch {
	case errors.Is(err, svcorder.ErrRoundNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeRequestNotFound, "That order is not one waiting to be paid for here.")
	case errors.Is(err, charges.ErrNothingToPay):
		writeError(w, s.log, http.StatusConflict, CodeNothingToPay, "That order is already paid for and with the kitchen.")
	case errors.Is(err, svcorder.ErrRoundLapsed):
		writeError(w, s.log, http.StatusConflict, CodeRequestLapsed, "That order is from an earlier day. Place it again.")
	case errors.Is(err, shift.ErrNoOpenShift):
		writeError(w, s.log, http.StatusConflict, CodeNoOpenShift, "Ordering is closed right now. Ask at the counter.")
	default:
		s.writeStartedCharge(w, r, venue.VenueID, c, err)
	}
}

// pickupReturnURL is where a gateway sends a guest back after paying for an
// order to collect: the venue's way in with no table, at the venue's own
// configured address, or nowhere. As guestReturnURL, it is never taken from
// the request.
func (s *Server) pickupReturnURL(r *http.Request) string {
	if s.public() == "" {
		return ""
	}
	return s.public() + s.guestMount + "p/" + r.PathValue("code")
}

// guestPickupCharge answers how the payment for an order to collect stands,
// asking the gateway. Only a charge with no table is one: a table's payment
// is asked about by its table's code.
func (s *Server) guestPickupCharge(w http.ResponseWriter, r *http.Request) {
	venue, ok := s.pickupOrderingFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id := r.PathValue("id")
	stored, err := s.charges.Charge(r.Context(), venue.VenueID, id)
	if errors.Is(err, charges.ErrChargeNotFound) || (err == nil && (stored.TableID != 0 || stored.Round == "")) {
		writeError(w, s.log, http.StatusNotFound, CodeChargeNotFound, "That payment is not one for an order to collect here.")
		return
	}
	if err != nil {
		s.internalError(w, r, "reading the charge of an order to collect failed", err, "venue_id", venue.VenueID)
		return
	}
	c, err := s.charges.Settle(r.Context(), venue.VenueID, id)
	if err != nil {
		// Nothing was decided, as for a table's: the charge is as it was,
		// and the phone asks again. With every pickup code out (ADR-249)
		// this is where a paid order waits, and the sweep keeps trying.
		s.log.Warn("the charge of an order to collect could not be settled just now", "request_id", RequestID(r.Context()), "venue_id", venue.VenueID, "charge", id, "error", err)
		c = stored
	}
	writeJSON(w, s.log, http.StatusOK, toGuestChargeResponse(c))
}
