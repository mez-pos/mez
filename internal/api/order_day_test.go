package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// TestDayOrdersOverHTTP: the day's list has the open ticket and the settled
// bill, each with its status, type and items, and what was collected.
func TestDayOrdersOverHTTP(t *testing.T) {
	r := newRegister(t)
	line := `{"item_id":1,"name":"Dosa","qty":2,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`
	paidID := r.openOrder(t)
	added := r.ok(t, http.MethodPost, "/api/v1/orders/"+paidID+"/lines", line)
	due := added.Order.Totals.BalanceDue.Minor
	r.ok(t, http.MethodPost, "/api/v1/orders/"+paidID+"/payments", fmt.Sprintf(`{"method":"cash","amount":{"minor":%d,"currency":"INR"}}`, due))
	r.ok(t, http.MethodPost, "/api/v1/orders/"+paidID+"/close", ``)
	openID := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+openID+"/lines", line)

	rec := send(r.handler, http.MethodGet, "/api/v1/orders/today", "", r.asStaff(r.staffToken))
	var day dayOrdersResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &day) != nil {
		t.Fatalf("today: %d %s", rec.Code, rec.Body)
	}
	if len(day.Orders) != 2 || day.Collected.Minor != due || day.Day == "" {
		t.Fatalf("day = %+v", day)
	}
	byID := map[string]dayOrderResponse{}
	for _, o := range day.Orders {
		byID[o.ID] = o
	}
	paid, open := byID[paidID], byID[openID]
	if paid.Status != svcorder.DayPaid || paid.InvoiceNo != "INV/26-27/000001" || paid.ClosedAt == nil || paid.BalanceDue.Minor != 0 {
		t.Errorf("the paid bill = %+v", paid)
	}
	if open.Status != svcorder.DayOpen || open.ClosedAt != nil || open.InvoiceNo != "" || open.BalanceDue.Minor != due {
		t.Errorf("the open ticket = %+v", open)
	}
	if len(paid.Items) != 1 || paid.Items[0] != (dayOrderItem{Qty: 2, Name: "Dosa"}) || paid.TableID != 4 || paid.Covers != 2 || paid.Service == "" {
		t.Errorf("what the row shows = %+v", paid)
	}

	// Nobody signed in reads no list.
	if rec := send(r.handler, http.MethodGet, "/api/v1/orders/today", "", r.asStaff("")); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned: %d", rec.Code)
	}
}
