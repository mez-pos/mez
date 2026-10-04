package api

import (
	"net/http"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/order"
)

// TestServiceTypeOverHTTP: an order is opened as dine-in, takeaway or
// delivery, reads back as one on the order and on the ticket list, and an
// order opened the old way still says what it is (ADR-146).
func TestServiceTypeOverHTTP(t *testing.T) {
	r := newRegister(t)

	_, old, _ := r.call(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter","table_id":4,"covers":2}`)
	if old.Order.Service != order.ServiceDineIn {
		t.Errorf("an order at a table, no service said: %q", old.Order.Service)
	}
	_, bare, _ := r.call(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter"}`)
	if bare.Order.Service != order.ServiceTakeaway {
		t.Errorf("an order with no table, no service said: %q", bare.Order.Service)
	}
	status, delivery, apiErr := r.call(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter","service":"delivery"}`)
	if status != http.StatusCreated || delivery.Order.Service != order.ServiceDelivery {
		t.Fatalf("open a delivery: %d %+v %+v", status, delivery.Order.Service, apiErr)
	}

	seen := map[string]order.ServiceType{}
	for _, o := range r.openTickets(t).Orders {
		seen[o.ID] = o.Service
	}
	if seen[old.Order.ID] != order.ServiceDineIn || seen[bare.Order.ID] != order.ServiceTakeaway || seen[delivery.Order.ID] != order.ServiceDelivery {
		t.Errorf("the ticket list says %v", seen)
	}

	t.Run("a takeaway seated is dine-in", func(t *testing.T) {
		status, res, _ := r.call(t, http.MethodPost, "/api/v1/orders/"+bare.Order.ID+"/table", `{"from":0,"to":6}`)
		if status != http.StatusOK || res.Order.Service != order.ServiceDineIn || res.Order.TableID != 6 {
			t.Errorf("seated: %d %q at %d", status, res.Order.Service, res.Order.TableID)
		}
	})

	refusals := []struct {
		name, path, body string
		status           int
		code             ErrorCode
	}{
		{"a service that does not exist", "/api/v1/orders", `{"channel":"counter","service":"drive_thru"}`, 422, CodeInvalidPayload},
		{"a takeaway at a table", "/api/v1/orders", `{"channel":"counter","service":"takeaway","table_id":5}`, 422, CodeInvalidPayload},
		{"a delivery seated", "/api/v1/orders/" + delivery.Order.ID + "/table", `{"from":0,"to":7}`, 409, CodeServiceNoTable},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, tt.path, tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Errorf("%d %s, want %d %s", status, apiErr.Code, tt.status, tt.code)
			}
		})
	}
}
