package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/money"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// "I have paid" (phase 11, Task 4e4, ADR-218):
//
//	POST /order/api/t/{code}/bill/paid          a guest says they have paid
//	POST /api/v1/paid-notices/{id}/seen         staff put the notice away
//
// and the notices staff have still to deal with ride on
// GET /api/v1/guest-requests, since they reach the same people the same
// way. A notice is a message. Nothing here records a payment.

// CodeNoticeNotFound is a paid notice that is not there.
const CodeNoticeNotFound ErrorCode = "notice_not_found"

// guestPaidResponse answers a guest who said they have paid: staff have
// been told, and for what amount.
type guestPaidResponse struct {
	Told   bool         `json:"told"`
	Amount money.Amount `json:"amount"`
}

// paidNoticeResponse is one "I have paid" staff have still to deal with.
// Amount is what was left when the guest said so, which is what to look for
// on the venue's phone; Due is what is left now. OrderID is the ticket to
// record the payment on.
type paidNoticeResponse struct {
	ID        string       `json:"id"`
	TableID   int64        `json:"table_id"`
	TableName string       `json:"table_name"`
	OrderID   string       `json:"order_id"`
	Amount    money.Amount `json:"amount"`
	Due       money.Amount `json:"due"`
	ToldAt    time.Time    `json:"told_at"`
}

func (s *Server) guestTellPaid(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	notice, err := s.guestOrders.TellPaid(r.Context(), table.VenueID, table.TableID)
	switch {
	case errors.Is(err, svcorder.ErrNothingToPay):
		writeError(w, s.log, http.StatusConflict, CodeNothingToPay, "There is nothing to pay by UPI on this bill.")
		return
	case err != nil:
		s.log.Error("taking a guest's paid notice failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	status := http.StatusCreated
	if notice.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, s.log, status, guestPaidResponse{Told: true, Amount: notice.Amount})
}

// mountPaidNotices routes the staff's half. Putting a notice away is part
// of taking a payment, so it asks what taking one asks.
func (s *Server) mountPaidNotices(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.guestRequests == nil {
		return
	}
	mux.Handle("POST /api/v1/paid-notices/{id}/seen",
		s.requireDevice(s.requireStaff(s.requirePermission(auth.PermPaymentTake, http.HandlerFunc(s.seePaidNotice)))))
}

func (s *Server) seePaidNotice(w http.ResponseWriter, r *http.Request) {
	if err := s.guestRequests.SeePaidNotice(r.Context(), Meta(r), r.PathValue("id")); err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toPaidNoticeResponses(notices []svcorder.PaidNotice) []paidNoticeResponse {
	out := make([]paidNoticeResponse, len(notices))
	for i, n := range notices {
		out[i] = paidNoticeResponse{
			ID: n.PublicID, TableID: n.TableID, TableName: n.TableName, OrderID: n.OrderPublicID,
			Amount: n.Amount, Due: n.Due, ToldAt: n.ToldAt,
		}
	}
	return out
}
