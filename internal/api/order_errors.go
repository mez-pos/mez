package api

import (
	"errors"
	"net/http"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/fsm"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// The order error codes. A client switches on these, so they change only
// with the API version.
const (
	CodeOrderNotFound        = "order_not_found"
	CodeLineNotFound         = "line_not_found"
	CodeLineNotActive        = "line_not_active"
	CodePaymentNotFound      = "payment_not_found"
	CodePaymentNotCaptured   = "payment_not_captured"
	CodeOrderClosed          = "order_closed"
	CodeOrderNotClosed       = "order_not_closed"
	CodeQtyMismatch          = "qty_mismatch"
	CodeBalanceDue           = "balance_due"
	CodeOverTender           = "over_tender"
	CodeOverRefund           = "over_refund"
	CodeDiscountTooLarge     = "discount_too_large"
	CodeAlreadyInvoiced      = "already_invoiced"
	CodeCourseAlreadyFired   = "course_already_fired"
	CodeNothingToSend        = "nothing_to_send"
	CodeInvalidTransition    = "invalid_transition"
	CodeInvalidPayload       = "invalid_payload"
	CodeCurrencyMismatch     = "currency_mismatch"
	CodeIdempotencyKeyReused = "idempotency_key_reused"
	CodeVenueNotConfigured   = "venue_not_configured"
)

// orderErrors maps every refusal the order path can produce to a status and
// a code. It is a list rather than a map because the first match wins and
// the specific errors have to come before the general ones: a currency
// mismatch is also an invalid payload, and the client wants to be told
// which.
//
// Anything not here is a bug rather than a refusal, and becomes a 500 with
// the detail in the log. That is the line this table exists to draw: a
// cashier who does something the domain forbids gets a code their terminal
// can act on, never an internal error.
var orderErrors = []struct {
	err    error
	status int
	code   string
}{
	// The service's own refusals.
	{svcorder.ErrOrderNotFound, http.StatusNotFound, CodeOrderNotFound},
	{svcorder.ErrIdempotencyKeyReused, http.StatusConflict, CodeIdempotencyKeyReused},
	{svcorder.ErrVenueConfig, http.StatusConflict, CodeVenueNotConfigured},
	{svcorder.ErrNothingToSend, http.StatusConflict, CodeNothingToSend},
	{svcorder.ErrInvalidCommand, http.StatusBadRequest, CodeInvalidRequest},

	// What the order itself refuses.
	{order.ErrLineNotFound, http.StatusConflict, CodeLineNotFound},
	{order.ErrLineNotActive, http.StatusConflict, CodeLineNotActive},
	{order.ErrPaymentNotFound, http.StatusConflict, CodePaymentNotFound},
	{order.ErrPaymentNotCaptured, http.StatusConflict, CodePaymentNotCaptured},
	{order.ErrOrderClosed, http.StatusConflict, CodeOrderClosed},
	{order.ErrOrderNotClosed, http.StatusConflict, CodeOrderNotClosed},
	{order.ErrAlreadyOpened, http.StatusConflict, CodeOrderClosed},
	{order.ErrQtyMismatch, http.StatusConflict, CodeQtyMismatch},
	{order.ErrBalanceDue, http.StatusConflict, CodeBalanceDue},
	{order.ErrOverTender, http.StatusConflict, CodeOverTender},
	{order.ErrOverRefund, http.StatusConflict, CodeOverRefund},
	{order.ErrDiscountExceedsValue, http.StatusConflict, CodeDiscountTooLarge},
	{order.ErrAlreadyInvoiced, http.StatusConflict, CodeAlreadyInvoiced},
	{order.ErrCourseAlreadyFired, http.StatusConflict, CodeCourseAlreadyFired},
	{order.ErrTaxConfig, http.StatusConflict, CodeVenueNotConfigured},
	{fsm.ErrInvalidTransition, http.StatusConflict, CodeInvalidTransition},

	// The general ones, last. A currency mismatch usually reaches here as
	// the projection's own invalid_payload, with the two currencies named
	// in the message; this entry catches the arithmetic that does not go
	// through an event.
	{money.ErrCurrencyMismatch, http.StatusUnprocessableEntity, CodeCurrencyMismatch},
	{money.ErrOverflow, http.StatusUnprocessableEntity, CodeInvalidPayload},
	{order.ErrInvalidPayload, http.StatusUnprocessableEntity, CodeInvalidPayload},
}

// writeOrderError answers a refusal from the order path.
//
// The message is the error's own: the domain's wording says what is wrong
// with the bill in terms the till can show ("tender exceeds the balance
// due: tendered 44000, balance due 22000"), and none of it is a secret.
func (s *Server) writeOrderError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range orderErrors {
		if errors.Is(err, m.err) {
			writeFieldError(w, s.log, m.status, m.code, err.Error(), fieldOf(err))
			return
		}
	}
	s.log.Error("an order command failed",
		"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
}

// fieldOf names the event a projection refused, so a client can say which
// step of a multi-event command failed. It is empty for everything else.
func fieldOf(err error) string {
	var pe *order.ProjectionError
	if errors.As(err, &pe) {
		return string(pe.Type)
	}
	return ""
}
