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
	Open bool `json:"open"`
	// Bill is the bill's own id, while it is open: what its receipt is asked
	// for by once it is settled (ADR-240). It names the bill and nothing
	// else, and is of no use without the table's code.
	Bill  string                  `json:"bill,omitempty"`
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
	// UPI is how to pay what is left by UPI, when the venue takes it and
	// something is owed. Absent otherwise.
	UPI *guestUPIResponse `json:"upi,omitempty"`
	// PaidTold is true while a guest at the table has said they have paid
	// and staff have still to look (ADR-218). The bill is no more paid for
	// it: Paid and BalanceDue say what staff have recorded.
	PaidTold bool `json:"paid_told,omitempty"`
	// Online is true when what is left can be paid online, through the
	// venue's gateway (ADR-222): the venue has one, and something is owed.
	Online bool `json:"online,omitempty"`
	// Split is how what is left can be paid online in parts (ADR-233).
	// Present wherever Online is, unless it could not be read.
	Split *guestSplitResponse `json:"split,omitempty"`
}

// guestUPIResponse is a UPI payment of the bill's balance to the venue
// (ADR-215). Link opens the phone's UPI app with the address and the amount
// filled in. It moves no money by itself and tells the venue nothing: a
// member of staff records the payment once they have seen it arrive.
type guestUPIResponse struct {
	Link    string       `json:"link"`
	Address string       `json:"address"`
	Payee   string       `json:"payee"`
	Amount  money.Amount `json:"amount"`
}

func (s *Server) guestBill(w http.ResponseWriter, r *http.Request) {
	table, ok := s.guestTableFor(w, r)
	if !ok {
		return
	}
	phone, ok := guestPhone(r)
	if !ok {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "phone is the name this phone gave itself: 8 to 64 letters, digits, - or _.", "phone")
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
	bill.UPI = s.guestUPI(r, table.VenueID, table.TableName, bill.BalanceDue)
	bill.Online = s.charges != nil && bill.BalanceDue.Minor > 0
	if bill.Online {
		// Not worth failing the bill for: without it the phone offers the
		// whole bill, and a share that does not fit is refused when asked for.
		if bill.Split, err = s.guestSplit(r, table.VenueID, table.TableID, phone, bill.Total); err != nil {
			s.log.Error("reading how a bill splits failed", "request_id", RequestID(r.Context()), "order", res.PublicID, "error", err)
		}
	}
	if bill.BalanceDue.Minor > 0 {
		// Not worth failing the bill for: unsaid, the guest can say it again.
		if bill.PaidTold, err = s.guestOrders.PaidTold(r.Context(), res.OrderID); err != nil {
			s.log.Error("reading a bill's paid notice failed", "request_id", RequestID(r.Context()), "order", res.PublicID, "error", err)
		}
	}
	writeJSON(w, s.log, http.StatusOK, bill)
}

// guestUPI is the UPI payment of due, or nil when there is none to offer:
// the venue has no address, nothing is owed, or the bill is not in rupees.
// A failure to read the address costs the guest the link and not the bill.
func (s *Server) guestUPI(r *http.Request, venueID int64, tableName string, due money.Amount) *guestUPIResponse {
	account, err := s.guestOrders.UPIAccount(r.Context(), venueID)
	if err != nil {
		s.log.Error("reading the venue's UPI address failed", "request_id", RequestID(r.Context()), "venue_id", venueID, "error", err)
		return nil
	}
	link, err := account.Link(due, "Table "+tableName)
	if err != nil {
		return nil
	}
	return &guestUPIResponse{Link: link, Address: account.Address, Payee: account.Payee, Amount: due}
}

func toGuestBillResponse(res svcorder.Result) (guestBillResponse, error) {
	st := res.State
	t := st.Totals
	discounts, err := t.LineDiscounts.Add(t.OrderDiscounts)
	if err != nil {
		return guestBillResponse{}, err
	}
	out := guestBillResponse{
		Open: true, Bill: res.PublicID, Lines: []guestBillLineResponse{}, Taxes: []guestBillTaxResponse{},
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
