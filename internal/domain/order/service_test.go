package order

import (
	"encoding/json"
	"errors"
	"testing"
)

// openedAs is an order opened with a service type and, optionally, a table.
func openedAs(service ServiceType, table int64) OrderOpened {
	return OrderOpened{Channel: ChannelCounter, Currency: "INR", Service: service, TableID: table, Covers: 2, OpenedBy: 11}
}

// TestServiceTypeOfAnOrder: an order opened before the field existed reads as
// dine-in when it has a table and takeaway when it has none (ADR-146), and
// one opened with a service type keeps it.
func TestServiceTypeOfAnOrder(t *testing.T) {
	tests := []struct {
		name   string
		opened OrderOpened
		want   ServiceType
	}{
		{"an old order at a table", OrderOpened{Channel: ChannelCounter, Currency: "INR", TableID: 4, Covers: 2}, ServiceDineIn},
		{"an old order with no table", OrderOpened{Channel: ChannelCounter, Currency: "INR"}, ServiceTakeaway},
		{"dine-in at a table", openedAs(ServiceDineIn, 4), ServiceDineIn},
		{"dine-in with the table still to pick", openedAs(ServiceDineIn, 0), ServiceDineIn},
		{"takeaway", openedAs(ServiceTakeaway, 0), ServiceTakeaway},
		{"delivery", openedAs(ServiceDelivery, 0), ServiceDelivery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustProject(t, tt.opened).Service; got != tt.want {
				t.Errorf("Service = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceTypeRefusals(t *testing.T) {
	tests := []struct {
		name    string
		events  []Payload
		wantErr error
	}{
		{"a service that does not exist", []Payload{openedAs("drive_thru", 0)}, ErrInvalidPayload},
		{"a takeaway at a table", []Payload{openedAs(ServiceTakeaway, 4)}, ErrInvalidPayload},
		{"a delivery at a table", []Payload{openedAs(ServiceDelivery, 4)}, ErrInvalidPayload},
		{"a delivery seated", []Payload{openedAs(ServiceDelivery, 0), OrderTableChanged{From: 0, To: 3}}, ErrServiceNoTable},
		{"a delivery with tables joined", []Payload{openedAs(ServiceDelivery, 0), TablesJoined{Tables: []int64{3}}}, ErrNoTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(stream(tt.events...), untaxed); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestSeatingATakeawayMakesItDineIn: a ticket rung up at the counter and then
// seated is a dine-in order from there on, so the service never disagrees
// with the table. It stays dine-in when moved on.
func TestSeatingATakeawayMakesItDineIn(t *testing.T) {
	seated := mustProject(t, openedAs(ServiceTakeaway, 0), OrderTableChanged{From: 0, To: 3})
	if seated.Service != ServiceDineIn || seated.TableID != 3 {
		t.Errorf("seated: service %q at table %d", seated.Service, seated.TableID)
	}
	moved := mustProject(t, openedAs(ServiceTakeaway, 0), OrderTableChanged{From: 0, To: 3}, OrderTableChanged{From: 3, To: 5})
	if moved.Service != ServiceDineIn || moved.TableID != 5 {
		t.Errorf("moved: service %q at table %d", moved.Service, moved.TableID)
	}
	// An old counter ticket does the same: it was read as takeaway.
	old := mustProject(t, OrderOpened{Channel: ChannelCounter, Currency: "INR"}, OrderTableChanged{From: 0, To: 3})
	if old.Service != ServiceDineIn {
		t.Errorf("an old counter ticket seated reads %q", old.Service)
	}
}

// TestResumeCarriesTheServiceType: a snapshot taken before a seating and
// resumed after it agrees with a projection from scratch.
func TestResumeCarriesTheServiceType(t *testing.T) {
	events := stream(openedAs(ServiceTakeaway, 0), coffee(1), OrderTableChanged{From: 0, To: 3})
	whole, err := Project(events, untaxed)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Project(events[:2], untaxed)
	if err != nil {
		t.Fatal(err)
	}
	if before.Service != ServiceTakeaway {
		t.Fatalf("before the seating: %q", before.Service)
	}
	resumed, err := Resume(before, events[2:], untaxed)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Service != ServiceDineIn || resumed.Service != whole.Service {
		t.Errorf("resumed %q, whole %q", resumed.Service, whole.Service)
	}
}

// TestOpenedDecodesWithAndWithoutAService: the field is optional on the wire,
// so every event stored before it still decodes, and a new one round-trips.
func TestOpenedDecodesWithAndWithoutAService(t *testing.T) {
	old := `{"channel":"counter","currency":"INR","table_id":4,"covers":2,"opened_by":11}`
	var pl OrderOpened
	if err := json.Unmarshal([]byte(old), &pl); err != nil || pl.Service != "" || pl.TableID != 4 {
		t.Fatalf("an old payload decodes to %+v, %v", pl, err)
	}
	raw, err := json.Marshal(openedAs(ServiceDelivery, 0))
	if err != nil {
		t.Fatal(err)
	}
	var back OrderOpened
	if err := json.Unmarshal(raw, &back); err != nil || back.Service != ServiceDelivery {
		t.Errorf("round trip: %s -> %+v, %v", raw, back, err)
	}
	// A payload with no service writes none, so an old request stays the
	// same bytes.
	plain, _ := json.Marshal(opened())
	if json.Valid(plain) && string(plain) != `{"channel":"counter","currency":"INR","covers":1,"opened_by":11}` {
		t.Errorf("a payload with no service writes %s", plain)
	}
}
