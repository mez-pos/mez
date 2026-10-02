package api

import (
	"errors"
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/fsm"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/payment"
	"github.com/byjoellobo/mez/internal/domain/shift"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// The order error codes. A client switches on these, so they change only
// with the API version.
const (
	CodeOrderNotFound        ErrorCode = "order_not_found"
	CodeLineNotFound         ErrorCode = "line_not_found"
	CodeLineNotActive        ErrorCode = "line_not_active"
	CodePaymentNotFound      ErrorCode = "payment_not_found"
	CodePaymentNotCaptured   ErrorCode = "payment_not_captured"
	CodeOrderClosed          ErrorCode = "order_closed"
	CodeOrderNotClosed       ErrorCode = "order_not_closed"
	CodeQtyMismatch          ErrorCode = "qty_mismatch"
	CodeModifiersMismatch    ErrorCode = "modifiers_mismatch"
	CodeLineInKitchen        ErrorCode = "line_in_kitchen"
	CodeNeedsApproval        ErrorCode = "needs_approval"
	CodeOrderHeld            ErrorCode = "order_held"
	CodeOrderNotHeld         ErrorCode = "order_not_held"
	CodeBalanceDue           ErrorCode = "balance_due"
	CodeOverTender           ErrorCode = "over_tender"
	CodeOverRefund           ErrorCode = "over_refund"
	CodeDiscountTooLarge     ErrorCode = "discount_too_large"
	CodeAlreadyInvoiced      ErrorCode = "already_invoiced"
	CodeCourseAlreadyFired   ErrorCode = "course_already_fired"
	CodeNothingToSend        ErrorCode = "nothing_to_send"
	CodeInvalidTransition    ErrorCode = "invalid_transition"
	CodeInvalidPayload       ErrorCode = "invalid_payload"
	CodeCurrencyMismatch     ErrorCode = "currency_mismatch"
	CodeIdempotencyKeyReused ErrorCode = "idempotency_key_reused"
	CodeVenueNotConfigured   ErrorCode = "venue_not_configured"

	// Where the party sits (ADR-088).
	CodeTableTaken     ErrorCode = "table_taken"
	CodeTableMoved     ErrorCode = "table_moved"
	CodeTableAlreadyAt ErrorCode = "table_already_at"
	CodeTableNotJoined ErrorCode = "table_not_joined"
	CodeNoTable        ErrorCode = "no_table"

	// The kitchen display (ADR-090).
	CodeTicketNotFound ErrorCode = "ticket_not_found"
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
	code   ErrorCode
}{
	// The service's own refusals.
	{svcorder.ErrOrderNotFound, http.StatusNotFound, CodeOrderNotFound},
	// The till is shut: nothing is sold until a shift opens (ADR-094).
	{shift.ErrNoOpenShift, http.StatusConflict, CodeNoOpenShift},
	{svcorder.ErrNoSuchTable, http.StatusUnprocessableEntity, CodeTableNotFound},
	{svcorder.ErrTableTaken, http.StatusConflict, CodeTableTaken},
	{svcorder.ErrIdempotencyKeyReused, http.StatusConflict, CodeIdempotencyKeyReused},
	{svcorder.ErrVenueConfig, http.StatusConflict, CodeVenueNotConfigured},
	{svcorder.ErrNothingToSend, http.StatusConflict, CodeNothingToSend},
	{svcorder.ErrNeedsApproval, http.StatusConflict, CodeNeedsApproval},
	{svcorder.ErrInvoiceNotIssued, http.StatusNotFound, CodeInvoiceNotFound},
	{svcorder.ErrRefundsMismatch, http.StatusUnprocessableEntity, CodeRefundsMismatch},
	{svcorder.ErrInvalidCommand, http.StatusBadRequest, CodeInvalidRequest},

	// How a bill divides (order_split.go).
	{svcorder.ErrSplitAfterPayment, http.StatusConflict, CodeSplitAfterPayment},
	{payment.ErrOverAllocated, http.StatusUnprocessableEntity, CodeSplitOverAllocated},
	{payment.ErrUnassigned, http.StatusUnprocessableEntity, CodeSplitUnassigned},
	{payment.ErrMismatch, http.StatusConflict, CodeSplitMismatch},
	{payment.ErrInvalidSplit, http.StatusUnprocessableEntity, CodeSplitInvalid},

	// What the order itself refuses.
	{order.ErrTicketNotFound, http.StatusNotFound, CodeTicketNotFound},
	{order.ErrTableMismatch, http.StatusConflict, CodeTableMoved},
	{order.ErrTableAlreadyAt, http.StatusConflict, CodeTableAlreadyAt},
	{order.ErrTableNotJoined, http.StatusConflict, CodeTableNotJoined},
	{order.ErrNoTable, http.StatusConflict, CodeNoTable},
	{order.ErrLineNotFound, http.StatusConflict, CodeLineNotFound},
	{order.ErrLineNotActive, http.StatusConflict, CodeLineNotActive},
	{order.ErrPaymentNotFound, http.StatusConflict, CodePaymentNotFound},
	{order.ErrInvoiceNotFound, http.StatusNotFound, CodeInvoiceNotFound},
	{order.ErrOrderInvoiced, http.StatusConflict, CodeOrderInvoiced},
	{order.ErrNotInvoiced, http.StatusConflict, CodeNotInvoiced},
	{order.ErrOverCredit, http.StatusConflict, CodeOverCredit},
	{order.ErrPaymentNotCaptured, http.StatusConflict, CodePaymentNotCaptured},
	{order.ErrOrderClosed, http.StatusConflict, CodeOrderClosed},
	{order.ErrOrderNotClosed, http.StatusConflict, CodeOrderNotClosed},
	{order.ErrAlreadyOpened, http.StatusConflict, CodeOrderClosed},
	{order.ErrQtyMismatch, http.StatusConflict, CodeQtyMismatch},
	{order.ErrModifiersMismatch, http.StatusConflict, CodeModifiersMismatch},
	{order.ErrLineInKitchen, http.StatusConflict, CodeLineInKitchen},
	{order.ErrOrderHeld, http.StatusConflict, CodeOrderHeld},
	{order.ErrOrderNotHeld, http.StatusConflict, CodeOrderNotHeld},
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
