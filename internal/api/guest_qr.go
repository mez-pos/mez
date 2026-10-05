package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/qr"
)

// The UPI payment of a table's bill as a QR code (phase 11, Task 4e3,
// ADR-217):
//
//	GET /order/api/t/{code}/bill/upi.svg
//
// It is the same link the bill carries, for the same balance, drawn so that
// another phone can scan it: a guest who would rather a friend paid, or
// whose own phone is the one showing the bill. The server draws it, so the
// guest's bundle carries no encoder.

// CodeNothingToPay is a bill with no UPI payment to offer: the venue takes
// none, or nothing is owed.
const CodeNothingToPay ErrorCode = "nothing_to_pay"

// qrQuiet is the light band around the code, in modules: the standard's.
const qrQuiet = 4

func (s *Server) guestBillQR(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	res, open, err := s.guestOrders.TableTicket(r.Context(), table.VenueID, table.TableID)
	if err != nil {
		s.log.Error("reading a table's bill for its QR failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	var upi *guestUPIResponse
	if open {
		upi = s.guestUPI(r, table.VenueID, table.TableName, res.State.Totals.BalanceDue)
	}
	if upi == nil {
		writeError(w, s.log, http.StatusNotFound, CodeNothingToPay, "There is nothing to pay by UPI on this bill.")
		return
	}
	code, err := qr.Encode([]byte(upi.Link))
	if err != nil {
		// A link is a few hundred bytes at the very most; a code holds two
		// thousand.
		s.log.Error("drawing a UPI link as a QR failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	// A picture and nothing else, whatever a browser is asked to do with it.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(code.SVG(qrQuiet))); err != nil {
		s.log.Debug("writing a QR was cut short", "request_id", RequestID(r.Context()), "error", err)
	}
}
