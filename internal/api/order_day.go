package api

import (
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// dayOrderItem is a line as the orders list names it: "2 × Masala dosa".
type dayOrderItem struct {
	Qty  int64  `json:"qty"`
	Name string `json:"name"`
}

// dayOrderResponse is one order on the till's orders list (rebuild Task 10a):
// enough to find it and tell what happened to it, not the bill itself.
type dayOrderResponse struct {
	ID        string             `json:"id"`
	InvoiceNo string             `json:"invoice_no,omitempty"`
	Status    svcorder.DayStatus `json:"status"`
	Service   order.ServiceType  `json:"service"`
	TableID   int64              `json:"table_id,omitempty"`
	Covers    int64              `json:"covers"`
	OpenedAt  time.Time          `json:"opened_at"`
	ClosedAt  *time.Time         `json:"closed_at,omitempty"`
	// Items are the lines that were not voided, in the order rung up.
	Items      []dayOrderItem `json:"items"`
	Total      money.Amount   `json:"total"`
	Credited   money.Amount   `json:"credited"`
	BalanceDue money.Amount   `json:"balance_due"`
	// PickupCode is what an order to collect is called out by at the
	// counter (ADR-249, ADR-253). Absent for every other order.
	PickupCode string `json:"pickup_code,omitempty"`
	// Progress is how far the kitchen has come with an order to collect,
	// read off its tickets as a guest's phone is told it (ADR-236): when
	// it is ready, it is called. Absent for every other order.
	Progress order.Progress `json:"progress,omitempty"`
}

// dayOrdersResponse is the trading day's orders, newest first. Collected is
// their payments less their refunds, tips apart.
type dayOrdersResponse struct {
	Day       string             `json:"day"`
	Collected money.Amount       `json:"collected"`
	Orders    []dayOrderResponse `json:"orders"`
}

func toDayOrdersResponse(day svcorder.Day) dayOrdersResponse {
	out := dayOrdersResponse{Day: day.Day, Collected: day.Collected, Orders: make([]dayOrderResponse, len(day.Orders))}
	for i, r := range day.Orders {
		st := r.State
		items := make([]dayOrderItem, 0, len(st.Lines))
		for _, l := range st.Lines {
			if l.Status != order.LineStatusVoided {
				items = append(items, dayOrderItem{Qty: l.Qty, Name: l.Name})
			}
		}
		o := dayOrderResponse{
			ID: r.PublicID, Status: svcorder.DayStatusOf(st), Service: st.Service, TableID: st.TableID, Covers: st.Covers,
			OpenedAt: st.OpenedAt.UTC(), Items: items,
			Total: st.Totals.Total, Credited: st.Totals.Credited, BalanceDue: st.Totals.BalanceDue,
		}
		if code := day.PickupCodes[r.OrderID]; code != "" {
			o.PickupCode = code
			// All of the ticket is the guest's to collect, whatever was
			// added to it at the counter since.
			ids := make([]int64, len(st.Lines))
			for j, l := range st.Lines {
				ids[j] = l.ID
			}
			o.Progress = st.Progress(ids)
		}
		if st.Status == order.StatusClosed {
			at := st.ClosedAt.UTC()
			o.ClosedAt = &at
		}
		if n := len(st.Invoices); n > 0 {
			o.InvoiceNo = st.Invoices[n-1].InvoiceNo
		}
		out.Orders[i] = o
	}
	return out
}

// listDayOrders answers the orders of the trading day it is now.
func (s *Server) listDayOrders(w http.ResponseWriter, r *http.Request) {
	day, err := s.orders.DayOrders(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toDayOrdersResponse(day))
}
