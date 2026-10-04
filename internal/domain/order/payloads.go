package order

import (
	"encoding/json"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// Channel is where an order came from. It affects routing, prepayment and
// reporting, never the shape of the order.
type Channel string

// The channels of docs/02-domain.md.
const (
	ChannelCounter    Channel = "counter"
	ChannelQRTable    Channel = "qr_table"
	ChannelWeb        Channel = "web"
	ChannelKiosk      Channel = "kiosk"
	ChannelAggregator Channel = "aggregator"
)

// ServiceType is how an order is served to the guest: at a table, to take
// away, or delivered (ADR-146). Channel is where the order came from and this
// is where it goes, so a counter order may be any of the three.
type ServiceType string

// The service types. Takeaway and delivery have no table; dine-in may have
// one, or still be waiting for it to be picked.
const (
	ServiceDineIn   ServiceType = "dine_in"
	ServiceTakeaway ServiceType = "takeaway"
	ServiceDelivery ServiceType = "delivery"
)

// DefaultService is what an order opened without a service type is: dine-in
// when it has a table and takeaway when it has none. It is how orders opened
// before the field existed read, and what the service writes when a client
// leaves it out.
func DefaultService(tableID int64) ServiceType {
	if tableID != 0 {
		return ServiceDineIn
	}
	return ServiceTakeaway
}

func validService(s ServiceType) bool {
	switch s {
	case ServiceDineIn, ServiceTakeaway, ServiceDelivery:
		return true
	}
	return false
}

// Scope says whether a discount or note applies to one line or the whole
// order.
type Scope string

// The two scopes. A line-scoped event names its line in LineID.
const (
	ScopeLine  Scope = "line"
	ScopeOrder Scope = "order"
)

// DiscountKind says how a discount's value is expressed.
type DiscountKind string

// A percentage discount is in basis points; an amount discount is money.
const (
	DiscountPercent DiscountKind = "pct"
	DiscountAmount  DiscountKind = "amount"
)

// PaymentMethod is how a tender was paid.
type PaymentMethod string

// The tender methods a venue can take.
const (
	PaymentCash  PaymentMethod = "cash"
	PaymentCard  PaymentMethod = "card"
	PaymentUPI   PaymentMethod = "upi"
	PaymentOther PaymentMethod = "other"
)

// OrderOpened starts an order. It is always the event at seq 1.
//
// Currency fixes the currency of everything that follows: every price,
// discount, tender and refund on the order must be in it. It is set at open,
// from the venue, so that an order with no lines still has totals.
type OrderOpened struct {
	Channel  Channel `json:"channel"`
	Currency string  `json:"currency"`
	// Service is how it is served. Empty on an order opened before the field
	// existed: it reads as DefaultService of the table, and the event on disk
	// is not touched.
	Service  ServiceType `json:"service,omitempty"`
	TableID  int64       `json:"table_id,omitempty"` // 0 for an order with no table
	Covers   int64       `json:"covers"`
	OpenedBy int64       `json:"opened_by"`
}

// Modifier is one modifier on a line, snapshotted when the line was added.
// Price is per unit of the line and may be zero.
type Modifier struct {
	ModifierID int64        `json:"modifier_id"`
	Name       string       `json:"name"`
	Price      money.Amount `json:"price"`
}

// LineAdded adds a line. Its line ID is the event's Seq.
//
// Name, UnitPrice, every modifier and the tax facts are copies taken at the
// moment the line was added, not references to the menu: a price or a tax
// rate changed at 7pm does not touch a ticket opened at 6pm. See "Prices and
// the snapshot rule".
type LineAdded struct {
	ItemID    int64        `json:"item_id"`
	Name      string       `json:"name"`
	Qty       int64        `json:"qty"`
	UnitPrice money.Amount `json:"unit_price"`
	Modifiers []Modifier   `json:"modifiers"`
	Course    int64        `json:"course"`
	Station   string       `json:"station"`

	TaxRate   tax.Rate `json:"tax_rate"`             // GST and VAT; US sales tax takes the venue's jurisdiction
	HSN       string   `json:"hsn,omitempty"`        // GST: the item's HSN or SAC code
	TaxExempt bool     `json:"tax_exempt,omitempty"` // no tax, whatever the rate
}

// LineQtyChanged changes a line's quantity. From is recorded so the timeline
// reads without replaying the order.
type LineQtyChanged struct {
	LineID int64 `json:"line_id"`
	From   int64 `json:"from"`
	To     int64 `json:"to"`
}

// LineModified replaces a line's modifiers: oat milk instead of dairy on a
// coffee not yet made. From is what the line had, so a terminal looking at a
// stale ticket is refused and the timeline reads without replaying.
//
// It is only for a line the kitchen has not seen: not sent, its course not
// fired. After that the item may already be made, and changing it is a void
// and a new line, which needs a manager and leaves both on the bill. ADR-071.
type LineModified struct {
	LineID int64      `json:"line_id"`
	From   []Modifier `json:"from"`
	To     []Modifier `json:"to"`
}

// ReasonEnteredInError is the reason code of a line taken off because it
// was rung up by mistake, with no manager (ADR-073). The Z-report counts
// these apart from a manager's voids.
const ReasonEnteredInError = "entered_in_error"

// LineVoided removes a line from the bill. The line stays in the stream
// forever; a void is visible to an audit and to the owner.
type LineVoided struct {
	LineID       int64  `json:"line_id"`
	ReasonCode   string `json:"reason_code"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// LineComped gives a line away. Unlike a void, the item was made and served,
// so it still counts for stock and for the kitchen.
type LineComped struct {
	LineID       int64  `json:"line_id"`
	ReasonCode   string `json:"reason_code"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// DiscountApplied takes money off a line or the order.
//
// Exactly one of BasisPoints and Amount is meaningful, chosen by Kind:
// BasisPoints for DiscountPercent (1000 = 10.00%), Amount for DiscountAmount.
// Amount is a pointer so that "no amount" is absent from the JSON rather than
// a zero Amount with no currency.
type DiscountApplied struct {
	Scope       Scope         `json:"scope"`
	LineID      int64         `json:"line_id,omitempty"` // set when Scope is ScopeLine
	Kind        DiscountKind  `json:"kind"`
	BasisPoints int32         `json:"basis_points,omitempty"`
	Amount      *money.Amount `json:"amount,omitempty"`
	Reason      string        `json:"reason"`
	// AuthorisedBy is the staff member whose permission or PIN let it
	// through (ADR-117). Zero on a discount recorded before it was kept.
	AuthorisedBy int64 `json:"authorised_by,omitempty"`
}

// NoteAdded attaches free text to a line or the order: "no onions", "birthday
// at table 4".
type NoteAdded struct {
	Scope  Scope  `json:"scope"`
	LineID int64  `json:"line_id,omitempty"` // set when Scope is ScopeLine
	Text   string `json:"text"`
}

// CourseFired tells the kitchen to start a course.
type CourseFired struct {
	Course int64 `json:"course"`
}

// TicketSent sends lines to a station's display or printer.
type TicketSent struct {
	Station string  `json:"station"`
	LineIDs []int64 `json:"line_ids"`
}

// PaymentTendered records money taken. Its payment ID is the event's Seq.
//
// Tip is separate from Amount and never folded into it: Amount settles the
// bill, Tip is owed to staff. A tender with no tip carries a zero Tip in the
// order's currency.
type PaymentTendered struct {
	Method     PaymentMethod `json:"method"`
	Amount     money.Amount  `json:"amount"`
	Tip        money.Amount  `json:"tip"`
	Reference  string        `json:"reference,omitempty"` // card slip, UPI ref
	TenderedBy int64         `json:"tendered_by"`
}

// PaymentVoided cancels a tender before the order closed, such as a card
// charged twice by mistake.
type PaymentVoided struct {
	PaymentID    int64  `json:"payment_id"`
	Reason       string `json:"reason"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// InvoiceIssued records the invoice number allocated to the order.
//
// TaxSnapshot is everything the tax profile used to compute the invoice: the
// State.TaxDetail at the moment of issue, a tax.TaxResult as JSON. It is held
// raw so the invoice keeps exactly what was printed, whatever later builds
// make of the type.
//
// Prices is the rest of the printed invoice as it stood at issue: who sold,
// each line as billed, and the totals. A reprint renders from it, never from
// today's menu, venue or projection. Invoices issued before it existed
// (phase 07, Task 3) have none.
type InvoiceIssued struct {
	InvoiceNo   string          `json:"invoice_no"`
	Series      string          `json:"series"`
	TaxSnapshot json.RawMessage `json:"tax_snapshot"`
	Prices      *InvoicePrices  `json:"prices,omitempty"`
}

// InvoicePrices is the price snapshot on an invoice.
type InvoicePrices struct {
	Seller InvoiceSeller `json:"seller"`
	Lines  []InvoiceLine `json:"lines"`
	// The bill's totals at issue, as Totals names them.
	Gross          money.Amount `json:"gross"`
	LineDiscounts  money.Amount `json:"line_discounts"`
	OrderDiscounts money.Amount `json:"order_discounts"`
	Subtotal       money.Amount `json:"subtotal"`
	Taxable        money.Amount `json:"taxable"`
	Tax            money.Amount `json:"tax"`
	Total          money.Amount `json:"total"`
}

// InvoiceSeller is the venue as the invoice names it, at issue.
type InvoiceSeller struct {
	Name         string `json:"name"`
	LegalName    string `json:"legal_name"`
	TaxProfile   string `json:"tax_profile"`
	Registration string `json:"registration,omitempty"` // GSTIN, VAT number
	StateCode    string `json:"state_code,omitempty"`   // GST place of supply
}

// InvoiceLine is one active line of the bill as it was invoiced.
type InvoiceLine struct {
	LineID        int64        `json:"line_id"`
	Name          string       `json:"name"`
	Qty           int64        `json:"qty"`
	UnitPrice     money.Amount `json:"unit_price"`
	Modifiers     []Modifier   `json:"modifiers,omitempty"`
	HSN           string       `json:"hsn,omitempty"`
	Gross         money.Amount `json:"gross"`
	Discount      money.Amount `json:"discount"`
	OrderDiscount money.Amount `json:"order_discount"`
	Tax           money.Amount `json:"tax"`
	Due           money.Amount `json:"due"`
}

// InvoiceReprinted records a copy of an issued invoice printed again. It
// allocates nothing: the copy carries the original number. It needs a
// reason and whoever allowed it, and it may follow a close, so a reprint of
// last week's bill is on that bill's own timeline.
type InvoiceReprinted struct {
	InvoiceNo    string `json:"invoice_no"`
	Reason       string `json:"reason"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// RefundIssued returns money against a captured payment.
type RefundIssued struct {
	Amount           money.Amount `json:"amount"`
	AgainstPaymentID int64        `json:"against_payment_id"`
	Reason           string       `json:"reason"`
	// AuthorisedBy is who let it through (ADR-117); zero on an older refund.
	AuthorisedBy int64 `json:"authorised_by,omitempty"`
}

// OrderClosed settles the order.
type OrderClosed struct {
	ClosedBy int64 `json:"closed_by"`
}

// OrderReopened reopens a closed order. It needs a reason and a manager,
// because it is how a settled bill gets changed.
type OrderReopened struct {
	Reason       string `json:"reason"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// OrderHeld parks an open order: the guest has stepped away, the table is
// waiting on someone. A held order takes nothing but notes until it is
// resumed, so nobody adds to a ticket they did not mean to (ADR-074). Only
// an open order can be held; one with money on it is being settled.
type OrderHeld struct{}

// OrderResumed takes an order off hold.
type OrderResumed struct{}

// EventType implementations: each payload names its own type.

func (OrderOpened) EventType() Type      { return TypeOrderOpened }
func (LineAdded) EventType() Type        { return TypeLineAdded }
func (LineQtyChanged) EventType() Type   { return TypeLineQtyChanged }
func (LineModified) EventType() Type     { return TypeLineModified }
func (LineVoided) EventType() Type       { return TypeLineVoided }
func (LineComped) EventType() Type       { return TypeLineComped }
func (DiscountApplied) EventType() Type  { return TypeDiscountApplied }
func (NoteAdded) EventType() Type        { return TypeNoteAdded }
func (CourseFired) EventType() Type      { return TypeCourseFired }
func (TicketSent) EventType() Type       { return TypeTicketSent }
func (PaymentTendered) EventType() Type  { return TypePaymentTendered }
func (PaymentVoided) EventType() Type    { return TypePaymentVoided }
func (InvoiceIssued) EventType() Type    { return TypeInvoiceIssued }
func (RefundIssued) EventType() Type     { return TypeRefundIssued }
func (OrderClosed) EventType() Type      { return TypeOrderClosed }
func (OrderReopened) EventType() Type    { return TypeOrderReopened }
func (OrderHeld) EventType() Type        { return TypeOrderHeld }
func (OrderResumed) EventType() Type     { return TypeOrderResumed }
func (InvoiceReprinted) EventType() Type { return TypeInvoiceReprinted }
func (CreditNoteIssued) EventType() Type { return TypeCreditNoteIssued }
