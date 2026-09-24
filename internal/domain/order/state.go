package order

import (
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// Status is where an order is in its life, derived from its events.
type Status string

// The order statuses of docs/02-domain.md. Project produces every one but
// StatusHeld, which has no event yet; the order machine in transitions.go
// already allows it, so adding the event is all it will take.
const (
	StatusOpen     Status = "open"     // taking lines, nothing tendered
	StatusHeld     Status = "held"     // parked: kept aside, not being worked
	StatusSettling Status = "settling" // at least one live tender
	StatusClosed   Status = "closed"   // settled at a zero balance
	StatusReopened Status = "reopened" // closed once, open again for a correction
)

// LineStatus says whether a line still counts toward the bill.
type LineStatus string

// A line starts active and may end voided or comped, never both.
const (
	LineStatusActive LineStatus = "active"
	LineStatusVoided LineStatus = "voided" // taken off the bill: never made, or made in error
	LineStatusComped LineStatus = "comped" // given away: made and served, not charged
)

// PaymentStatus is where a tender is. A payment.tendered is captured when it
// is recorded, so Project never produces PaymentStatusPending: pending is the
// card terminal's state before any event exists, and belongs to phase 07.
type PaymentStatus string

// A tender is captured until it is voided or refunded in full.
const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusCaptured PaymentStatus = "captured"
	PaymentStatusVoided   PaymentStatus = "voided"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// State is an order as its events add up to. Project builds a fresh one on
// every call, and nothing in it aliases the events it came from.
type State struct {
	Channel  Channel
	Currency string
	TableID  int64
	Covers   int64
	OpenedBy int64
	OpenedAt time.Time
	ClosedBy int64     // 0 while the order is not closed
	ClosedAt time.Time // zero while the order is not closed
	Status   Status

	LastSeq   int64     // the seq of the last event projected
	UpdatedAt time.Time // the time of the last event projected
	Reopens   int64     // how many times the order has been reopened

	// Invoiced is true once an invoice has been issued for the current
	// settlement, and false again after a reopen. It is what refuses a
	// second invoice.issued, and what Resume needs to carry a projection on.
	Invoiced bool

	Lines        []Line     // in the order they were added
	Discounts    []Discount // line and order discounts, in the order applied
	Notes        []Note
	FiredCourses []int64 // in the order they were fired
	Tickets      []Ticket
	Tenders      []Tender // every tender, voided ones included
	Refunds      []Refund
	Invoices     []Invoice // every invoice issued, one per close

	Totals Totals

	// TaxDetail is the tax profile's full result for the bill as it stands:
	// every line's components and the rate-wise summary. It is what
	// invoice.issued stores as its tax_snapshot. Zero for an untaxed venue.
	TaxDetail tax.TaxResult
}

// Line is one line of the bill, with the price snapshot it was added at.
type Line struct {
	ID        int64 // the seq of its line.added
	ItemID    int64
	Name      string
	Qty       int64
	UnitPrice money.Amount
	Modifiers []Modifier
	Course    int64
	Station   string
	TaxRate   tax.Rate
	HSN       string
	TaxExempt bool

	Status       LineStatus
	ReasonCode   string // why it was voided or comped
	AuthorisedBy int64  // who voided or comped it
	CourseFired  bool   // its course has been fired
	Sent         bool   // it has gone to its station on a ticket

	// Gross is Qty × (UnitPrice + every modifier's Price), whatever the
	// line's status, so a void or comp can report what it was worth.
	Gross money.Amount
	// Discount is the sum of the line's own discounts. Zero unless active.
	Discount money.Amount
	// Net is what the line contributes to the bill before order discounts:
	// Gross less Discount while active, zero once voided or comped.
	Net money.Amount
	// OrderDiscount is the line's share of the order discounts, spread over
	// active lines in proportion to Net. Tax is charged on Net less this.
	OrderDiscount money.Amount
	// Tax is the tax on the line, from the venue's tax profile. Zero when
	// the venue is untaxed or the line is not active.
	Tax money.Amount
	// Due is what the guest pays for the line: Net less OrderDiscount, plus
	// Tax when the venue prices exclusive. The Due of the active lines sums
	// to Totals.Total exactly, which is what splitting a bill by item needs.
	Due money.Amount
}

// Discount is a discount as applied, with the value it currently takes off.
// A percentage discount's Value follows the line or order it applies to, so a
// quantity change after a 10% discount keeps it at 10%.
type Discount struct {
	Seq         int64
	Scope       Scope
	LineID      int64
	Kind        DiscountKind
	BasisPoints int32
	Amount      *money.Amount
	Reason      string
	ActorID     int64
	Value       money.Amount // zero when its line is no longer active
}

// Note is free text on a line or the order.
type Note struct {
	Seq    int64
	Scope  Scope
	LineID int64
	Text   string
}

// Ticket is one send of lines to a station.
type Ticket struct {
	Seq     int64
	Station string
	LineIDs []int64
	At      time.Time
}

// Tender is one payment.tendered and what has happened to it since.
type Tender struct {
	ID         int64 // the seq of its payment.tendered
	Method     PaymentMethod
	Amount     money.Amount
	Tip        money.Amount
	Reference  string
	TenderedBy int64
	Status     PaymentStatus
	Refunded   money.Amount // refunded against it so far
	VoidReason string
}

// Refund is money given back against a tender.
type Refund struct {
	Seq              int64
	Amount           money.Amount
	AgainstPaymentID int64
	Reason           string
}

// Invoice is an invoice issued for the order.
type Invoice struct {
	Seq       int64
	InvoiceNo string
	Series    string
	Reprints  int // copies printed since, each an invoice.reprinted
}

// Totals are the money of the order, all in its currency.
//
//	Gross          sum of Gross over active lines, as priced
//	LineDiscounts  sum of line discounts over active lines
//	OrderDiscounts order-scope discounts, taken off Gross - LineDiscounts
//	Subtotal       Gross - LineDiscounts - OrderDiscounts, as priced: it
//	               includes tax when the venue prices inclusive
//	Taxable        the bill's value excluding tax
//	Tax            the tax on it, from the venue's tax profile
//	Total          Taxable + Tax: what the guest owes. Under inclusive
//	               pricing this equals Subtotal; under exclusive pricing
//	               it is Subtotal + Tax; untaxed, Subtotal
//	Paid           tenders that are not voided
//	Refunded       refunds issued
//	NetPaid        Paid - Refunded: what the venue has kept
//	BalanceDue     Total - NetPaid; negative when the venue owes the guest
//	Tips           tips on tenders that are not voided, never in Paid
//	Voided, Comped the Gross of voided and comped lines, for reporting
type Totals struct {
	Gross          money.Amount
	LineDiscounts  money.Amount
	OrderDiscounts money.Amount
	Subtotal       money.Amount
	Taxable        money.Amount
	Tax            money.Amount
	Total          money.Amount
	Paid           money.Amount
	Refunded       money.Amount
	NetPaid        money.Amount
	BalanceDue     money.Amount
	Tips           money.Amount
	Voided         money.Amount
	Comped         money.Amount
}
