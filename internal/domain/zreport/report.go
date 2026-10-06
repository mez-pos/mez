// Package zreport works out a shift's Z-report (phase 09, Task 3a) from the
// order events recorded in it. It only adds up: every figure is taken from
// the projection or an event's payload, never recomputed from prices, so the
// report says what the invoices and payments said.
//
// What belongs to the shift is decided by where each event was stamped
// (ADR-094):
//
//   - a sale is an invoice issued in the shift, measured as the bill stood
//     at that moment;
//   - a credit note, void, comp, payment, payment void or refund belongs to
//     the shift it was recorded in, whichever shift the bill was opened or
//     invoiced in.
//
// Build is pure; storing, numbering and printing it is internal/service.
package zreport

import (
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// Report is a shift's figures. Every money field is in the venue's currency.
type Report struct {
	Currency string `json:"currency"`

	// Sales are the bills invoiced in the shift, as invoiced.
	Sales    Sales    `json:"sales"`
	Invoices Invoices `json:"invoices"`

	ByCategory []Group `json:"by_category"`
	ByItem     []Group `json:"by_item"`
	ByChannel  []Group `json:"by_channel"`
	// ByService is the bills by how they were served: dine-in, takeaway or
	// delivery (ADR-146). A report issued before it was counted has none.
	ByService []Group `json:"by_service"`
	ByStaff   []Group `json:"by_staff"` // by who opened the bill

	Discounts   Tally        `json:"discounts"`    // on the bills invoiced
	Comps       Tally        `json:"comps"`        // lines given away, at their gross
	Voids       Tally        `json:"voids"`        // lines a manager took off, at their gross
	Removed     []StaffTally `json:"removed"`      // lines rung up by mistake, by who removed them
	CreditNotes Tally        `json:"credit_notes"` // sales reversed, tax included
	Refunds     Tally        `json:"refunds"`      // money handed back
	Reprints    int64        `json:"reprints"`     // invoice copies printed

	// Taxes is the tax invoiced less the tax credited, per component and rate.
	Taxes []TaxLine `json:"taxes"`

	Payments []Payment    `json:"payments"` // by method, net of voids and refunds; tips apart
	Tips     money.Amount `json:"tips"`     // owed to staff; never in sales or payments
	// TipsBy is Tips by who took the payment, the most first; Count is their
	// payments that carried a tip. A report issued before it was counted
	// has none.
	TipsBy []StaffTally `json:"tips_by"`
	// TipsApproved is the tips in Tips that were over the venue's ceiling
	// and let through by a manager (ADR-261). A report issued before it was
	// counted has none.
	TipsApproved Tally `json:"tips_approved"`

	Balance Balance `json:"balance"`
}

// Sales is a set of bills as invoiced. Gross less Discounts is Net; Net is
// Taxable plus any tax priced into it; Total is what the guests owed.
type Sales struct {
	Bills     int64        `json:"bills"`
	Gross     money.Amount `json:"gross"`
	Discounts money.Amount `json:"discounts"`
	Net       money.Amount `json:"net"`
	Taxable   money.Amount `json:"taxable"`
	Tax       money.Amount `json:"tax"`
	Total     money.Amount `json:"total"`
	// NetOfCredits is Total less the credit notes issued in the shift.
	NetOfCredits money.Amount `json:"net_of_credits"`
	// Covers is the guests on those bills, as counted when each was opened
	// or seated. A takeaway or a party nobody counted adds none.
	Covers int64 `json:"covers"`
}

// Invoices is the run of invoice numbers issued in the shift.
type Invoices struct {
	Count int64  `json:"count"`
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

// Group is sales under one heading: a category, an item, a channel, a way
// of serving, a staff member. Qty is units for a category or item and bills otherwise.
type Group struct {
	Key       string       `json:"key"`
	Name      string       `json:"name"`
	Qty       int64        `json:"qty"`
	Gross     money.Amount `json:"gross"`
	Discounts money.Amount `json:"discounts"`
	Net       money.Amount `json:"net"`
	Tax       money.Amount `json:"tax"`
	Total     money.Amount `json:"total"`
}

// Tally is how many of something, and what they came to.
type Tally struct {
	Count  int64        `json:"count"`
	Amount money.Amount `json:"amount"`
}

// StaffTally is a tally for one staff member.
type StaffTally struct {
	StaffID int64 `json:"staff_id"`
	Tally
}

// TaxLine is one tax component at one rate over the shift.
type TaxLine struct {
	Name    string       `json:"name"`
	Rate    tax.Rate     `json:"rate"`
	Taxable money.Amount `json:"taxable"`
	Amount  money.Amount `json:"amount"`
}

// Payment is one method over the shift.
type Payment struct {
	Method   order.PaymentMethod `json:"method"`
	Count    int64               `json:"count"` // tenders taken
	Taken    money.Amount        `json:"taken"`
	Voided   money.Amount        `json:"voided"`
	Refunded money.Amount        `json:"refunded"`
	Net      money.Amount        `json:"net"` // taken less voided and refunded
}

// Balance sets what was paid against what was sold. Difference is zero on a
// day whose bills were all paid and invoiced within it; otherwise it is
// money taken for bills invoiced in another shift, or not yet.
type Balance struct {
	Payments   money.Amount `json:"payments"`   // every method's net
	Sales      money.Amount `json:"sales"`      // Sales.NetOfCredits
	Difference money.Amount `json:"difference"` // Payments less Sales
}
