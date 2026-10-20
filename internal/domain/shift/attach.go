package shift

import (
	"errors"

	"github.com/mez-pos/mez/internal/domain/order"
)

// ErrNoOpenShift refuses an order event that must belong to a shift when
// none is open: the venue is shut, and the till takes nothing until
// someone counts a float into the drawer.
var ErrNoOpenShift = errors.New("no shift is open")

// outsideShift are the order events that may happen with no shift open.
// None of them touches money or what the Z-report counts: the kitchen
// finishing a ticket after close, or a copy of a bill already issued.
var outsideShift = map[order.Type]bool{
	order.TypeTicketBumped:     true,
	order.TypeTicketRecalled:   true,
	order.TypeTicketStarted:    true,
	order.TypeInvoiceReprinted: true,
}

// NeedsShift reports whether an order event may only be recorded while a
// shift is open. Every event is stamped with the shift open when it
// happens (ADR-094); these are the ones refused without one. A type this
// build does not list is refused too: a new event must be let out on
// purpose, not by default.
func NeedsShift(t order.Type) bool {
	return !outsideShift[t]
}
