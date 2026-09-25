package api

import (
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// The credit note error codes (phase 07, Task 6a, ADR-083).
const (
	CodeNotInvoiced     ErrorCode = "not_invoiced"
	CodeOverCredit      ErrorCode = "over_credit"
	CodeRefundsMismatch ErrorCode = "refunds_mismatch"
)

// creditPreviewRequest is what to credit, for the figures before approval.
type creditPreviewRequest = order.CreditRequest

// creditRequest issues a credit note: what it credits, the payments the
// money goes back to, why, and the manager who allowed it.
type creditRequest struct {
	Override
	Credit  order.CreditRequest   `json:"credit"`
	Refunds []svcorder.RefundPart `json:"refunds"`
	Reason  string                `json:"reason"`
}

// creditPlanResponse is a credit note's figures before it has a number.
type creditPlanResponse struct {
	Lines      []order.CreditLine   `json:"lines"`
	Taxable    money.Amount         `json:"taxable"`
	Tax        money.Amount         `json:"tax"`
	Total      money.Amount         `json:"total"`
	Components []tax.ComponentTotal `json:"components"`
}

// creditResponse is a credit note on the order.
type creditResponse struct {
	Seq              int64        `json:"seq"`
	CreditNo         string       `json:"credit_no"`
	AgainstInvoiceNo string       `json:"against_invoice_no"`
	Reason           string       `json:"reason"`
	Taxable          money.Amount `json:"taxable"`
	Tax              money.Amount `json:"tax"`
	Total            money.Amount `json:"total"`
}

func (s *Server) previewCredit(w http.ResponseWriter, r *http.Request) {
	var req creditPreviewRequest
	if !s.decode(w, r, &req) {
		return
	}
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	p, err := s.orders.PlanCredit(r.Context(), Meta(r).VenueID, orderID, req)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	out := creditPlanResponse{Lines: p.Lines, Taxable: p.Taxable, Tax: p.Tax, Total: p.Total, Components: p.Components}
	if out.Components == nil {
		out.Components = []tax.ComponentTotal{}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

// issueCredit needs only a signed-in cashier to ask; the authoriser needs
// refund.give and their PIN, as a plain refund does.
func (s *Server) issueCredit(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req creditRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermRefundGive)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.Credit(r.Context(), meta, orderID, svcorder.IssueCredit{
			Credit: req.Credit, Refunds: req.Refunds, Reason: req.Reason, AuthorisedBy: by.ID,
		})
	})
}

func toCreditResponses(credits []order.Credit) []creditResponse {
	out := make([]creditResponse, len(credits))
	for i, c := range credits {
		out[i] = creditResponse{
			Seq: c.Seq, CreditNo: c.CreditNo, AgainstInvoiceNo: c.AgainstInvoiceNo, Reason: c.Reason,
			Taxable: c.Taxable, Tax: c.Tax, Total: c.Total,
		}
	}
	return out
}

// pastBillResponse is a closed bill on the till's past bills list.
type pastBillResponse struct {
	ID        string       `json:"id"`
	InvoiceNo string       `json:"invoice_no,omitempty"`
	TableID   int64        `json:"table_id,omitempty"`
	Covers    int64        `json:"covers"`
	ClosedAt  time.Time    `json:"closed_at"`
	Total     money.Amount `json:"total"`
	Credited  money.Amount `json:"credited"`
}

type pastBillsResponse struct {
	Bills []pastBillResponse `json:"bills"`
}

// listPastBills answers the latest closed bills, or those a number finds.
func (s *Server) listPastBills(w http.ResponseWriter, r *http.Request) {
	bills, err := s.orders.PastBills(r.Context(), Meta(r).VenueID, r.URL.Query().Get("q"))
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	out := pastBillsResponse{Bills: make([]pastBillResponse, len(bills))}
	for i, b := range bills {
		st := b.State
		out.Bills[i] = pastBillResponse{
			ID: b.PublicID, TableID: st.TableID, Covers: st.Covers, ClosedAt: st.ClosedAt.UTC(),
			Total: st.Totals.Total, Credited: st.Totals.Credited,
		}
		if n := len(st.Invoices); n > 0 {
			out.Bills[i].InvoiceNo = st.Invoices[n-1].InvoiceNo
		}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}
