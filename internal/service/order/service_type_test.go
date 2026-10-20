package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
)

// TestOpenRecordsTheServiceType: how the order is served is written to its
// opening event, whether the client said or the table decided (ADR-146), so
// the stream on disk never depends on a default that could change later.
func TestOpenRecordsTheServiceType(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		cmd  OpenOrder
		want order.ServiceType
	}{
		{"nothing said, a table", OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}, order.ServiceDineIn},
		{"nothing said, no table", OpenOrder{Channel: order.ChannelCounter}, order.ServiceTakeaway},
		{"dine-in, the table to come", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceDineIn, Covers: 2}, order.ServiceDineIn},
		{"dine-in at a table", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceDineIn, TableID: 4}, order.ServiceDineIn},
		{"takeaway", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceTakeaway}, order.ServiceTakeaway},
		{"delivery", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceDelivery}, order.ServiceDelivery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newService(t)
			res := must(t)(s.Open(ctx, cashier(""), tt.cmd))
			if res.State.Service != tt.want {
				t.Errorf("State.Service = %q, want %q", res.State.Service, tt.want)
			}
			if pl, ok := res.Events[0].Payload.(order.OrderOpened); !ok || pl.Service != tt.want {
				t.Errorf("the opening event says %+v, want service %q", res.Events[0].Payload, tt.want)
			}
			// What a later read projects from disk agrees.
			again, err := s.Order(ctx, 1, res.OrderID)
			if err != nil || again.State.Service != tt.want {
				t.Errorf("read back: %q, %v", again.State.Service, err)
			}
		})
	}
}

func TestOpenRefusesAServiceThatCannotBe(t *testing.T) {
	tests := []struct {
		name string
		cmd  OpenOrder
	}{
		{"a service that does not exist", OpenOrder{Channel: order.ChannelCounter, Service: "drive_thru"}},
		{"a takeaway at a table", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceTakeaway, TableID: 4}},
		{"a delivery at a table", OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceDelivery, TableID: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newService(t)
			if _, err := s.Open(context.Background(), cashier(""), tt.cmd); !errors.Is(err, order.ErrInvalidPayload) {
				t.Errorf("err = %v, want ErrInvalidPayload", err)
			}
			if open, err := s.OpenOrders(context.Background(), 1); err != nil || len(open.Orders) != 0 {
				t.Errorf("a refused open left %d orders (%v)", len(open.Orders), err)
			}
		})
	}
}

// TestSeatingAndTheServiceType: a takeaway seated at a free table is dine-in
// from then on, and a delivery is not seated at all.
func TestSeatingAndTheServiceType(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newService(t)

	takeaway := must(t)(s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceTakeaway}))
	seated := must(t)(s.MoveTable(ctx, cashier(""), takeaway.OrderID, MoveTable{From: 0, To: 3}))
	if seated.State.Service != order.ServiceDineIn || seated.State.TableID != 3 {
		t.Errorf("a seated takeaway is %q at table %d", seated.State.Service, seated.State.TableID)
	}

	delivery := must(t)(s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceDelivery}))
	if _, err := s.MoveTable(ctx, cashier(""), delivery.OrderID, MoveTable{From: 0, To: 6}); !errors.Is(err, order.ErrServiceNoTable) {
		t.Errorf("seating a delivery: err = %v, want ErrServiceNoTable", err)
	}
	// Whichever table it was sent to: a taken one, or one that is not there.
	for _, to := range []int64{4, 99} {
		if _, err := s.MoveTable(ctx, cashier(""), delivery.OrderID, MoveTable{From: 0, To: to}); !errors.Is(err, order.ErrServiceNoTable) {
			t.Errorf("seating a delivery at table %d: err = %v, want ErrServiceNoTable", to, err)
		}
	}
	again, err := s.Order(ctx, 1, delivery.OrderID)
	if err != nil || again.State.TableID != 0 || again.State.Service != order.ServiceDelivery {
		t.Errorf("after the refusal: table %d, %q, %v", again.State.TableID, again.State.Service, err)
	}
}

// TestOpenKeysSeeTheServiceType: a retry of the same open is the same open; a
// key reused for another service is a different request.
func TestOpenKeysSeeTheServiceType(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newService(t)
	cmd := OpenOrder{Channel: order.ChannelCounter, Service: order.ServiceTakeaway}
	a := must(t)(s.Open(ctx, cashier("open-t"), cmd))
	b := must(t)(s.Open(ctx, cashier("open-t"), cmd))
	if !b.Replayed || a.OrderID != b.OrderID {
		t.Errorf("a retried open: %+v then %+v", a, b)
	}
	cmd.Service = order.ServiceDelivery
	if _, err := s.Open(ctx, cashier("open-t"), cmd); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("the same key for a delivery: err = %v", err)
	}

	// Saying dine-in and leaving it to the table are one request.
	left := must(t)(s.Open(ctx, cashier("open-d"), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
	said := must(t)(s.Open(ctx, cashier("open-d"), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Service: order.ServiceDineIn}))
	if !said.Replayed || said.OrderID != left.OrderID {
		t.Errorf("the explicit form of the same open was not a replay: %+v then %+v", left, said)
	}
}
