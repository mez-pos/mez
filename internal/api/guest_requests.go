package api

import (
	"context"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/money"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// Guests' requests, as staff answer them (phase 11, Task 4b1, ADR-211):
//
//	GET  /api/v1/guest-requests               what is waiting, oldest first
//	POST /api/v1/guest-requests/{id}/accept   onto the table's ticket and to the kitchen
//	POST /api/v1/guest-requests/{id}/decline  turned down
//
// This is the staff API: a guest cannot reach it. A terminal reads the list
// again when the stream says floor_changed, which a request arriving or being
// answered moves.

// What answering a request is refused with.
const (
	CodeRequestNotFound ErrorCode = "request_not_found"
	CodeRequestDecided  ErrorCode = "request_decided"
	CodeRequestLapsed   ErrorCode = "request_lapsed"
)

// GuestRequests is what the API needs to answer guests' requests.
type GuestRequests interface {
	WaitingRounds(ctx context.Context, venueID int64) ([]svcorder.Request, error)
	AcceptRound(ctx context.Context, meta svcorder.Meta, publicID string) (svcorder.Decision, error)
	DeclineRound(ctx context.Context, meta svcorder.Meta, publicID string) (svcorder.Decision, error)
	PaidNotices(ctx context.Context, venueID int64) ([]svcorder.PaidNotice, error)
	SeePaidNotice(ctx context.Context, meta svcorder.Meta, publicID string) error
}

// WithGuestRequests lets staff see and answer guests' requests.
func WithGuestRequests(g GuestRequests) Option {
	return func(s *Server) { s.guestRequests = g }
}

func (s *Server) mountGuestRequests(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.guestRequests == nil {
		return
	}
	// Accepting a request opens a ticket and adds to it, so it asks what
	// opening one at the till asks.
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermOrderOpen, h))))
	}
	handle("GET /api/v1/guest-requests", s.listGuestRequests)
	mux.HandleFunc("/api/v1/guest-requests", s.methodNotAllowed)
	handle("POST /api/v1/guest-requests/{id}/accept", s.decideGuestRequest(func(g GuestRequests) decideRound { return g.AcceptRound }))
	handle("POST /api/v1/guest-requests/{id}/decline", s.decideGuestRequest(func(g GuestRequests) decideRound { return g.DeclineRound }))
}

type decideRound func(ctx context.Context, meta svcorder.Meta, publicID string) (svcorder.Decision, error)

// guestRequestResponse is one request waiting for an answer.
type guestRequestResponse struct {
	ID        string                   `json:"id"`
	TableID   int64                    `json:"table_id"`
	TableName string                   `json:"table_name"`
	PlacedAt  time.Time                `json:"placed_at"`
	Lines     []guestRoundLineResponse `json:"lines"`
	// Subtotal is before tax: the ticket works the bill out once accepted.
	Subtotal money.Amount `json:"subtotal"`
}

// guestRequestsResponse is GET /api/v1/guest-requests: what guests have
// asked for and nobody has answered, and who says they have paid and nobody
// has looked (ADR-218). Two lists, since one is answered and the other is
// checked against the venue's own phone.
type guestRequestsResponse struct {
	Requests []guestRequestResponse `json:"requests"`
	Paid     []paidNoticeResponse   `json:"paid"`
}

// guestDecisionResponse is a request once answered. OrderID is the ticket
// an accepted one went onto.
type guestDecisionResponse struct {
	ID      string               `json:"id"`
	Status  svcorder.RoundStatus `json:"status"`
	TableID int64                `json:"table_id"`
	OrderID string               `json:"order_id,omitempty"`
}

func (s *Server) listGuestRequests(w http.ResponseWriter, r *http.Request) {
	waiting, err := s.guestRequests.WaitingRounds(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	out := guestRequestsResponse{Requests: make([]guestRequestResponse, len(waiting))}
	for i, req := range waiting {
		round, err := toGuestRoundResponse(req.Round)
		if err != nil {
			s.writeOrderError(w, r, err)
			return
		}
		out.Requests[i] = guestRequestResponse{
			ID: req.PublicID, TableID: req.TableID, TableName: req.TableName, PlacedAt: req.PlacedAt,
			Lines: round.Lines, Subtotal: round.Subtotal,
		}
	}
	notices, err := s.guestRequests.PaidNotices(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	out.Paid = toPaidNoticeResponses(notices)
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) decideGuestRequest(pick func(GuestRequests) decideRound) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, err := pick(s.guestRequests)(r.Context(), Meta(r), r.PathValue("id"))
		if err != nil {
			s.writeOrderError(w, r, err)
			return
		}
		writeJSON(w, s.log, http.StatusOK, guestDecisionResponse{
			ID: got.Round.PublicID, Status: got.Round.Status, TableID: got.Round.TableID, OrderID: got.OrderPublicID,
		})
	}
}
