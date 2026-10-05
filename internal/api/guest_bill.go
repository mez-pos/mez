package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/tax"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// The table's bill, as a guest reads it (phase 11, Task 4d, ADR-214):
//
//	GET /order/api/t/{code}/bill
//
// It is the table's open ticket with the till's parts taken out: what was
// ordered, what it comes to with its tax as the venue works it out, what has
// been paid and what is left. Every figure is the projection's; nothing is
// added up here or on the phone.

// guestBillLineResponse is one line of the bill. A voided line is not on
// the bill and is not here.
type guestBillLineResponse struct {
	Name    string   `json:"name"`
	Qty     int64    `json:"qty"`
	Choices []string `json:"choices"`
	// Amount is the dish and its choices, times how many, before discounts.
	Amount money.Amount `json:"amount"`
	// Comped is a line the venue gave away: shown, and not charged.
	Comped bool `json:"comped,omitempty"`
}

// guestBillTaxResponse is one tax on the bill, by name and rate.
type guestBillTaxResponse struct {
	Name   string       `json:"name"`
	Rate   tax.Rate     `json:"rate"`
	Amount money.Amount `json:"amount"`
}

// guestBillResponse is the table's bill. Open is false when the table has
// no ticket: nothing ordered yet, or the last bill settled and closed.
type guestBillResponse struct {
	Open  bool                    `json:"open"`
	Lines []guestBillLineResponse `json:"lines"`
	// Items is what the lines charged come to, before discounts and before
	// tax when prices exclude it.
	Items money.Amount `json:"items"`
	// Discounts is everything taken off, line and order together.
	Discounts money.Amount `json:"discounts"`
	// Taxes is each tax by name. TaxIncluded says the prices already hold
	// it, so the rows explain the total and do not add to it.
	Taxes       []guestBillTaxResponse `json:"taxes"`
	TaxIncluded bool                   `json:"tax_included"`
	Total       money.Amount           `json:"total"`
	Paid        money.Amount           `json:"paid"`
	BalanceDue  money.Amount           `json:"balance_due"`
}

func (s *Server) guestBill(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	res, open, err := s.guestOrders.TableTicket(r.Context(), table.VenueID, table.TableID)
	if err != nil {
		s.log.Error("reading a table's bill for a guest failed", "request_id", RequestID(r.Context()), "venue_id", table.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	// A bill moves with every round and every payment: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	if !open {
		zero := money.Amount{Currency: table.Currency}
		writeJSON(w, s.log, http.StatusOK, guestBillResponse{
			Lines: []guestBillLineResponse{}, Taxes: []guestBillTaxResponse{},
			Items: zero, Discounts: zero, Total: zero, Paid: zero, BalanceDue: zero,
		})
		return
	}
	bill, err := toGuestBillResponse(res)
	if err != nil {
		s.log.Error("adding up a table's bill for a guest failed", "request_id", RequestID(r.Context()), "order", res.PublicID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	writeJSON(w, s.log, http.StatusOK, bill)
}

func toGuestBillResponse(res svcorder.Result) (guestBillResponse, error) {
	st := res.State
	t := st.Totals
	discounts, err := t.LineDiscounts.Add(t.OrderDiscounts)
	if err != nil {
		return guestBillResponse{}, err
	}
	out := guestBillResponse{
		Open: true, Lines: []guestBillLineResponse{}, Taxes: []guestBillTaxResponse{},
		Items: t.Gross, Discounts: discounts,
		TaxIncluded: st.TaxDetail.Pricing == tax.Inclusive,
		Total:       t.Total, Paid: t.NetPaid, BalanceDue: t.BalanceDue,
	}
	for _, l := range st.Lines {
		if l.Status == order.LineStatusVoided {
			continue
		}
		choices := make([]string, len(l.Modifiers))
		for i, m := range l.Modifiers {
			choices[i] = m.Name
		}
		out.Lines = append(out.Lines, guestBillLineResponse{
			Name: l.Name, Qty: l.Qty, Choices: choices, Amount: l.Gross, Comped: l.Status == order.LineStatusComped,
		})
	}
	for _, c := range st.TaxDetail.Components {
		out.Taxes = append(out.Taxes, guestBillTaxResponse{Name: c.Name, Rate: c.Rate, Amount: c.Amount})
	}
	return out, nil
}
