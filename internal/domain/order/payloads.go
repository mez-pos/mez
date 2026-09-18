package order

import (
	"encoding/json"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
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
type OrderOpened struct {
	Channel  Channel `json:"channel"`
	TableID  int64   `json:"table_id,omitempty"` // 0 for an order with no table
	Covers   int64   `json:"covers"`
	OpenedBy int64   `json:"opened_by"`
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
// Name, UnitPrice and every modifier are copies taken at the moment the line
// was added, not references to the menu: a price changed at 7pm does not
// touch a ticket opened at 6pm. See "Prices and the snapshot rule".
type LineAdded struct {
	ItemID    int64        `json:"item_id"`
	Name      string       `json:"name"`
	Qty       int64        `json:"qty"`
	UnitPrice money.Amount `json:"unit_price"`
	Modifiers []Modifier   `json:"modifiers"`
	Course    int64        `json:"course"`
	Station   string       `json:"station"`
}

// LineQtyChanged changes a line's quantity. From is recorded so the timeline
// reads without replaying the order.
type LineQtyChanged struct {
	LineID int64 `json:"line_id"`
	From   int64 `json:"from"`
	To     int64 `json:"to"`
}

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
// TaxSnapshot is everything the tax profile used to compute the invoice,
// held as raw JSON. Its shape belongs to the tax profile (phase 03, task 5),
// and this package only has to carry it unchanged.
type InvoiceIssued struct {
	InvoiceNo   string          `json:"invoice_no"`
	Series      string          `json:"series"`
	TaxSnapshot json.RawMessage `json:"tax_snapshot"`
}

// RefundIssued returns money against a captured payment.
type RefundIssued struct {
	Amount           money.Amount `json:"amount"`
	AgainstPaymentID int64        `json:"against_payment_id"`
	Reason           string       `json:"reason"`
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

// EventType implementations: each payload names its own type.

func (OrderOpened) EventType() Type     { return TypeOrderOpened }
func (LineAdded) EventType() Type       { return TypeLineAdded }
func (LineQtyChanged) EventType() Type  { return TypeLineQtyChanged }
func (LineVoided) EventType() Type      { return TypeLineVoided }
func (LineComped) EventType() Type      { return TypeLineComped }
func (DiscountApplied) EventType() Type { return TypeDiscountApplied }
func (NoteAdded) EventType() Type       { return TypeNoteAdded }
func (CourseFired) EventType() Type     { return TypeCourseFired }
func (TicketSent) EventType() Type      { return TypeTicketSent }
func (PaymentTendered) EventType() Type { return TypePaymentTendered }
func (PaymentVoided) EventType() Type   { return TypePaymentVoided }
func (InvoiceIssued) EventType() Type   { return TypeInvoiceIssued }
func (RefundIssued) EventType() Type    { return TypeRefundIssued }
func (OrderClosed) EventType() Type     { return TypeOrderClosed }
func (OrderReopened) EventType() Type   { return TypeOrderReopened }
