package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/payment"
	"github.com/byjoellobo/mez/internal/domain/shift"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A guest places an order (phase 11, Task 4a, ADR-210):
//
//	POST /order/api/t/{code}/orders
//
// The phone sends what is in its cart as ids and counts, and a key so that
// sending twice is ordering once. Every name and price in the answer is the
// server's, from the menu as it stands.

// What a guest's order is refused with. The messages are for the guest.
const (
	CodeDishUnavailable  ErrorCode = "dish_unavailable"
	CodeChoicesInvalid   ErrorCode = "choices_invalid"
	CodeTableNotOrdering ErrorCode = "table_not_ordering"
	CodeRequestsWaiting  ErrorCode = "requests_waiting"
)

// GuestOrders is what the API needs to take a guest's order.
type GuestOrders interface {
	PlaceRound(ctx context.Context, meta svcorder.Meta, cmd svcorder.PlaceRound) (svcorder.Round, error)
	TableTicket(ctx context.Context, venueID, tableID int64) (svcorder.Result, bool, error)
	UPIAccount(ctx context.Context, venueID int64) (payment.UPIAccount, error)
}

// WithGuestOrders lets a guest place an order. It needs WithGuests.
func WithGuestOrders(o GuestOrders) Option {
	return func(s *Server) { s.guestOrders = o }
}

// guestOrderLineRequest is one line of the cart: a dish, the choices made on
// it, and how many. No name and no price: those are the menu's.
type guestOrderLineRequest struct {
	ItemID      int64   `json:"item_id"`
	ModifierIDs []int64 `json:"modifier_ids"`
	Qty         int64   `json:"qty"`
}

// guestOrderRequest is the body of POST /order/api/t/{code}/orders.
type guestOrderRequest struct {
	Lines []guestOrderLineRequest `json:"lines"`
}

// guestRoundLineResponse is one line as the server priced it.
type guestRoundLineResponse struct {
	ItemID  int64    `json:"item_id"`
	Name    string   `json:"name"`
	Qty     int64    `json:"qty"`
	Choices []string `json:"choices"`
	// Total is the dish and its choices, times how many, before tax.
	Total money.Amount `json:"total"`
}

// guestRoundResponse is a guest's order as it stands: waiting for staff to
// accept it, or sent to the kitchen.
type guestRoundResponse struct {
	ID     string                   `json:"id"`
	Status svcorder.RoundStatus     `json:"status"`
	Lines  []guestRoundLineResponse `json:"lines"`
	// Subtotal is before any taxes and charges, which are on the bill.
	Subtotal money.Amount `json:"subtotal"`
}

func (s *Server) guestPlaceOrder(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	var req guestOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	cmd := svcorder.PlaceRound{TableID: table.TableID, Picks: make([]menu.Pick, len(req.Lines))}
	for i, l := range req.Lines {
		cmd.Picks[i] = menu.Pick{ItemID: l.ItemID, ModifierIDs: l.ModifierIDs, Qty: l.Qty}
	}
	// A guest is nobody: no actor and no device. The venue is the code's.
	meta := svcorder.Meta{VenueID: table.VenueID, IdempotencyKey: r.Header.Get(HeaderIdempotencyKey)}
	round, err := s.guestOrders.PlaceRound(r.Context(), meta, cmd)
	if err != nil {
		s.writeGuestOrderError(w, r, err)
		return
	}
	resp, err := toGuestRoundResponse(round)
	if err != nil {
		s.log.Error("encoding a guest's order failed", "request_id", RequestID(r.Context()), "round", round.PublicID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	status := http.StatusCreated
	if round.Replayed {
		status = http.StatusOK
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, status, resp)
}

func toGuestRoundResponse(round svcorder.Round) (guestRoundResponse, error) {
	out := guestRoundResponse{
		ID: round.PublicID, Status: round.Status, Subtotal: round.Subtotal,
		Lines: make([]guestRoundLineResponse, len(round.Lines)),
	}
	for i, l := range round.Lines {
		total, err := menu.LineValue(l)
		if err != nil {
			return guestRoundResponse{}, err
		}
		choices := make([]string, len(l.Modifiers))
		for j, m := range l.Modifiers {
			choices[j] = m.Name
		}
		out.Lines[i] = guestRoundLineResponse{ItemID: l.ItemID, Name: l.Name, Qty: l.Qty, Choices: choices, Total: total}
	}
	return out, nil
}

// writeGuestOrderError answers a refused order in words a guest can act on.
// None of them is the domain's own wording: that names seqs and ids.
func (s *Server) writeGuestOrderError(w http.ResponseWriter, r *http.Request, err error) {
	refuse := func(status int, code ErrorCode, message string) {
		writeError(w, s.log, status, code, message)
	}
	var pick *menu.PickError
	dish := "A dish in your cart"
	if errors.As(err, &pick) && pick.Item != "" {
		dish = pick.Item
	}
	switch {
	case errors.Is(err, menu.ErrNoSuchItem), errors.Is(err, menu.ErrSoldOut):
		refuse(http.StatusConflict, CodeDishUnavailable, dish+" is no longer available. Take it out of your cart to order the rest.")
	case errors.Is(err, menu.ErrChoices):
		refuse(http.StatusConflict, CodeChoicesInvalid, "The choices for "+dish+" have changed. Take it out of your cart and add it again.")
	case errors.Is(err, menu.ErrPickQty):
		refuse(http.StatusBadRequest, CodeInvalidRequest, dish+": the quantity is out of range.")
	case errors.Is(err, shift.ErrNoOpenShift):
		refuse(http.StatusConflict, CodeNoOpenShift, "Ordering is closed right now. Ask a member of staff.")
	case errors.Is(err, svcorder.ErrTableBusy):
		refuse(http.StatusConflict, CodeTableNotOrdering, "This table's bill is being settled, so it cannot take another order. Ask a member of staff.")
	case errors.Is(err, svcorder.ErrTooManyWaiting):
		refuse(http.StatusConflict, CodeRequestsWaiting, "Your table already has orders waiting to be accepted. Ask a member of staff.")
	case errors.Is(err, svcorder.ErrNoSuchTable):
		// The code led to a table a moment ago and the table is gone.
		refuse(http.StatusNotFound, CodeTableCodeUnknown, "This code does not lead to a table. Scan the code on your table again, or ask a member of staff.")
	case errors.Is(err, svcorder.ErrIdempotencyKeyReused):
		refuse(http.StatusConflict, CodeIdempotencyKeyReused, "This order was already sent with something else in it. Go back to your cart and place it again.")
	case errors.Is(err, svcorder.ErrInvalidCommand):
		refuse(http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		s.log.Error("a guest's order failed", "request_id", RequestID(r.Context()), "path", r.URL.Path, "error", err)
		refuse(http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
