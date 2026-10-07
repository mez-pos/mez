package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/tax"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A guest's receipt (phase 11, Task 5d, ADR-240):
//
//	GET /order/api/t/{code}/receipts/{bill}
//
// The bill once it is settled, as the invoice the counter issued has it: the
// same number, the seller, the lines and the tax, read from the invoice's
// own snapshot. A guest asks by two things, the table's code and the bill's
// own id, which the phone was given while the bill was open. The table's
// code alone means another bill at the next sitting.

// CodeReceiptNotFound: no receipt for that bill at this table.
const CodeReceiptNotFound ErrorCode = "receipt_not_found"

// guestReceiptSellerResponse is the venue as the invoice named it.
type guestReceiptSellerResponse struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name,omitempty"`
	// Registration is the seller's tax number: a GSTIN, a VAT number.
	Registration string `json:"registration,omitempty"`
}

// guestReceiptLineResponse is one line as it was billed.
type guestReceiptLineResponse struct {
	Name    string   `json:"name"`
	Qty     int64    `json:"qty"`
	Choices []string `json:"choices"`
	// Amount is the dish and its choices, times how many, before discounts.
	Amount money.Amount `json:"amount"`
	HSN    string       `json:"hsn,omitempty"`
}

// guestReceiptPaymentResponse is one payment: how, and how much.
type guestReceiptPaymentResponse struct {
	Method order.PaymentMethod `json:"method"`
	Amount money.Amount        `json:"amount"`
}

// guestReceiptResponse is a settled bill as a guest keeps it. It has no id
// of a member of staff, a device or a table, no payment's reference, and
// nothing of any other bill.
type guestReceiptResponse struct {
	Bill      string `json:"bill"`
	InvoiceNo string `json:"invoice_no"`
	// IssuedAt is when the invoice was issued, and TimeZone the venue's, to
	// say it in.
	IssuedAt time.Time                  `json:"issued_at"`
	TimeZone string                     `json:"time_zone"`
	Seller   guestReceiptSellerResponse `json:"seller"`
	Lines    []guestReceiptLineResponse `json:"lines"`
	// Items, Discounts, Taxes, TaxIncluded and Total read as on the bill.
	Items       money.Amount                  `json:"items"`
	Discounts   money.Amount                  `json:"discounts"`
	Taxes       []guestBillTaxResponse        `json:"taxes"`
	TaxIncluded bool                          `json:"tax_included"`
	Total       money.Amount                  `json:"total"`
	Payments    []guestReceiptPaymentResponse `json:"payments"`
	// Credited is what the venue has since taken off the total by a credit
	// note, and Refunded what it has given back. Both are usually nothing.
	Credited money.Amount `json:"credited"`
	Refunded money.Amount `json:"refunded"`
}

// guestReceipt asks for no session (ADR-262): a receipt is read for weeks
// after its sitting, long after the sitting's sessions ended, and the bill's
// id, told only to the sitting's own phones, already names the one bill.
func (s *Server) guestReceipt(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	// A credit note or a refund can still change it: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	receipt, err := s.guestOrders.Receipt(r.Context(), table.VenueID, table.TableID, r.PathValue("bill"))
	if errors.Is(err, svcorder.ErrReceiptNotFound) {
		writeError(w, s.log, http.StatusNotFound, CodeReceiptNotFound, "There is no receipt for that bill here. Ask a member of staff for a copy.")
		return
	}
	if err != nil {
		s.internalError(w, r, "reading a guest's receipt failed", err, "venue_id", table.VenueID)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toGuestReceiptResponse(receipt))
}

func toGuestReceiptResponse(rc svcorder.Receipt) guestReceiptResponse {
	out := guestReceiptResponse{
		Bill: rc.Bill, InvoiceNo: rc.InvoiceNo, IssuedAt: rc.IssuedAt.UTC(), TimeZone: rc.TimeZone,
		Seller: guestReceiptSellerResponse{Name: rc.Seller.Name, LegalName: rc.Seller.LegalName, Registration: rc.Seller.Registration},
		Lines:  make([]guestReceiptLineResponse, len(rc.Lines)), Taxes: make([]guestBillTaxResponse, len(rc.Taxes)),
		Items: rc.Items, Discounts: rc.Discounts, TaxIncluded: rc.TaxIncluded, Total: rc.Total,
		Payments: make([]guestReceiptPaymentResponse, len(rc.Payments)),
		Credited: rc.Credited, Refunded: rc.Refunded,
	}
	for i, l := range rc.Lines {
		choices := make([]string, len(l.Modifiers))
		for j, m := range l.Modifiers {
			choices[j] = m.Name
		}
		out.Lines[i] = guestReceiptLineResponse{Name: l.Name, Qty: l.Qty, Choices: choices, Amount: l.Gross, HSN: l.HSN}
	}
	for i, c := range rc.Taxes {
		out.Taxes[i] = guestBillTaxResponse{Name: c.Name, Rate: tax.Rate(c.Rate), Amount: c.Amount}
	}
	for i, p := range rc.Payments {
		out.Payments[i] = guestReceiptPaymentResponse(p)
	}
	return out
}
