package api

import (
	"net/http"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/domain/order"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// The invoice error codes (phase 07, Task 3).
const (
	CodeInvoiceNotFound ErrorCode = "invoice_not_found" // an invoice this order was never issued
	CodeOrderInvoiced   ErrorCode = "order_invoiced"    // the bill changed after its invoice
)

// issuedInvoiceResponse is an invoice as it was issued (phase 07, Task 3):
// the seller, lines and totals from its price snapshot and the tax from its
// tax snapshot, never recomputed. Prices is absent on an invoice issued
// before snapshots kept them. Tenders are the order's payments now.
type issuedInvoiceResponse struct {
	Seq       int64                `json:"seq"`
	InvoiceNo string               `json:"invoice_no"`
	Series    string               `json:"series"`
	IssuedAt  time.Time            `json:"issued_at"`
	TimeZone  string               `json:"time_zone"`
	Tax       *taxResponse         `json:"tax,omitempty"`
	Prices    *order.InvoicePrices `json:"prices,omitempty"`
	Reprints  int                  `json:"reprints"`
	Tenders   []tenderResponse     `json:"tenders"`
}

// reprintRequest names nothing but why and who allowed it: the invoice is
// in the path, and the copy carries its number.
type reprintRequest struct {
	Override
	Reason string `json:"reason"`
}

// readInvoice answers one of an order's invoices, for the preview.
func (s *Server) readInvoice(w http.ResponseWriter, r *http.Request) {
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	seq, ok := s.pathID(w, r, "seq")
	if !ok {
		return
	}
	inv, err := s.orders.Invoice(r.Context(), Meta(r).VenueID, orderID, seq)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, issuedInvoiceResponse{
		Seq: inv.Seq, InvoiceNo: inv.InvoiceNo, Series: inv.Series, IssuedAt: inv.IssuedAt,
		TimeZone: inv.TimeZone, Tax: toTaxResponse(inv.Tax), Prices: inv.Prices,
		Reprints: inv.Reprints, Tenders: toTenderResponses(inv.Tenders),
	})
}

// reprintInvoice logs a copy of an invoice. The caller only has to be working
// the register; the authoriser needs invoice.reprint and their PIN.
func (s *Server) reprintInvoice(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req reprintRequest
		seq, ok := s.pathID(w, r, "seq")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermInvoiceReprint)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.Reprint(r.Context(), meta, orderID, svcorder.ReprintInvoice{
			InvoiceSeq: seq, Reason: req.Reason, AuthorisedBy: by.ID,
		})
	})
}
