package order

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

var testAt = time.Date(2026, 9, 18, 13, 45, 7, 123456789, time.UTC)

// samplePayloads holds one fully populated payload per event type. Every field
// is set to a non-zero value, so a field that fails to round-trip shows up.
func samplePayloads() []Payload {
	fiftyOff := inr(5000)
	return []Payload{
		OrderOpened{Channel: ChannelQRTable, Currency: "INR", TableID: 4, Covers: 3, OpenedBy: 11},
		LineAdded{
			ItemID: 42, Name: "Cold brew", Qty: 2, UnitPrice: inr(22000),
			Modifiers: []Modifier{
				{ModifierID: 7, Name: "Oat milk", Price: inr(4000)},
				{ModifierID: 8, Name: "No ice", Price: inr(0)},
			},
			Course: 1, Station: "bar",
		},
		LineQtyChanged{LineID: 2, From: 2, To: 3},
		LineVoided{LineID: 2, ReasonCode: "wrong_item", AuthorisedBy: 3},
		LineComped{LineID: 2, ReasonCode: "long_wait", AuthorisedBy: 3},
		DiscountApplied{Scope: ScopeLine, LineID: 2, Kind: DiscountPercent, BasisPoints: 1000, Reason: "staff"},
		NoteAdded{Scope: ScopeLine, LineID: 2, Text: "extra hot"},
		CourseFired{Course: 2},
		TicketSent{Station: "kitchen", LineIDs: []int64{2, 5, 6}},
		PaymentTendered{Method: PaymentCard, Amount: inr(52000), Tip: inr(5000), Reference: "AUTH 0042", TenderedBy: 11},
		PaymentVoided{PaymentID: 9, Reason: "double charge", AuthorisedBy: 3},
		InvoiceIssued{InvoiceNo: "NF/2026-27/000123", Series: "NF", TaxSnapshot: json.RawMessage(`{"profile":"gst","cgst":1300}`)},
		RefundIssued{Amount: inr(22000), AgainstPaymentID: 9, Reason: "cold food"},
		OrderClosed{ClosedBy: 11},
		OrderReopened{Reason: "tip entered wrong", AuthorisedBy: 3},
		DiscountApplied{Scope: ScopeOrder, Kind: DiscountAmount, Amount: &fiftyOff, Reason: "loyalty"},
	}
}

func TestEveryEventTypeRoundTrips(t *testing.T) {
	for i, p := range samplePayloads() {
		t.Run(string(p.EventType()), func(t *testing.T) {
			in := NewEvent(int64(i+1), testAt, 11, 2, p)

			b, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var out Event
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal %s: %v", b, err)
			}
			if !reflect.DeepEqual(in, out) {
				t.Errorf("round trip changed the event\n in: %#v\nout: %#v\njson: %s", in, out, b)
			}
		})
	}
}

// TestSamplesCoverEveryType stops a new event type from being added without a
// round-trip case: the registry and the samples must list the same types.
func TestSamplesCoverEveryType(t *testing.T) {
	covered := map[Type]bool{}
	for _, p := range samplePayloads() {
		covered[p.EventType()] = true
	}
	for _, typ := range Types() {
		if !covered[typ] {
			t.Errorf("no round-trip sample for %q", typ)
		}
	}
	if got, want := len(Types()), 15; got != want {
		t.Errorf("Types() has %d types, docs/02-domain.md lists %d", got, want)
	}
}

// TestEveryPayloadIsDecodedAsItsOwnType checks the registry wiring: a type
// string must decode into the payload that claims it, not a neighbour with
// the same field names, such as line.voided and line.comped.
func TestEveryPayloadIsDecodedAsItsOwnType(t *testing.T) {
	for _, typ := range Types() {
		p, err := decoders[typ](json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("%q: decoding an empty object: %v", typ, err)
		}
		if p.EventType() != typ {
			t.Errorf("%q decodes into a %q payload", typ, p.EventType())
		}
	}
}

func TestWireFormat(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	e := NewEvent(3, time.Date(2026, 9, 18, 19, 15, 0, 0, ist), 11, 2,
		LineVoided{LineID: 2, ReasonCode: "wrong_item", AuthorisedBy: 3})

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"seq":3,"type":"line.voided","at":"2026-09-18T13:45:00Z","actor_id":11,"device_id":2,` +
		`"payload":{"line_id":2,"reason_code":"wrong_item","authorised_by":3}}`
	if string(b) != want {
		t.Errorf("wire format\n got: %s\nwant: %s", b, want)
	}
}

func TestMarshalNormalisesTimeToUTC(t *testing.T) {
	// An Event built by hand, not through NewEvent, may carry a local time.
	ist := time.FixedZone("IST", 5*3600+1800)
	e := Event{Seq: 1, Type: TypeOrderClosed, At: time.Date(2026, 4, 1, 0, 30, 0, 0, ist), Payload: OrderClosed{ClosedBy: 1}}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"at":"2026-03-31T19:00:00Z"`) {
		t.Errorf("time not written as UTC: %s", b)
	}
}

func TestNewEvent(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	e := NewEvent(1, time.Date(2026, 9, 18, 19, 15, 0, 0, ist), 11, 2, CourseFired{Course: 1})

	if e.Type != TypeCourseFired {
		t.Errorf("Type = %q, want %q", e.Type, TypeCourseFired)
	}
	if e.At.Location() != time.UTC {
		t.Errorf("At is in %v, want UTC", e.At.Location())
	}
	if nilPayload := NewEvent(1, testAt, 0, 0, nil); nilPayload.Type != "" {
		t.Errorf("an event with no payload got type %q", nilPayload.Type)
	}
}

func TestUnknownEventTypeIsAnError(t *testing.T) {
	data := `{"seq":7,"type":"loyalty.points_earned","at":"2026-09-18T13:45:00Z","actor_id":1,"device_id":1,"payload":{"points":40}}`

	var e Event
	err := json.Unmarshal([]byte(data), &e)
	if !errors.Is(err, ErrUnknownEventType) {
		t.Fatalf("err = %v, want ErrUnknownEventType", err)
	}
	var unknown *UnknownEventTypeError
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %T, want *UnknownEventTypeError", err)
	}
	if unknown.Type != "loyalty.points_earned" || unknown.Seq != 7 {
		t.Errorf("error carries type %q seq %d, want loyalty.points_earned seq 7", unknown.Type, unknown.Seq)
	}
	if !strings.Contains(err.Error(), "seq 7") {
		t.Errorf("message %q does not say where the event is", err.Error())
	}
}

func TestUnknownEventTypeFailsAStream(t *testing.T) {
	// A stream is decoded as a whole. One unknown event must fail it, not
	// leave a shorter list that projects to a wrong total.
	data := `[
		{"seq":1,"type":"order.opened","at":"2026-09-18T13:45:00Z","actor_id":1,"device_id":1,"payload":{"channel":"counter","covers":1,"opened_by":1}},
		{"seq":2,"type":"payment.split_v2","at":"2026-09-18T13:46:00Z","actor_id":1,"device_id":1,"payload":{}}
	]`

	var events []Event
	if err := json.Unmarshal([]byte(data), &events); !errors.Is(err, ErrUnknownEventType) {
		t.Fatalf("err = %v, want ErrUnknownEventType", err)
	}
}

func TestUnknownFieldsAreIgnored(t *testing.T) {
	data := `{"seq":1,"type":"order.closed","at":"2026-09-18T13:45:00Z","actor_id":1,"device_id":1,"shard":"b",` +
		`"payload":{"closed_by":5,"closed_reason_v2":"end of shift"}}`

	var e Event
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if want := (OrderClosed{ClosedBy: 5}); e.Payload != want {
		t.Errorf("payload = %#v, want %#v", e.Payload, want)
	}
}

func TestMalformedEventsAreRejected(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "not json", data: `{"seq":1,`},
		{name: "not an object", data: `[1,2]`},
		{name: "no type", data: `{"seq":1,"payload":{}}`},
		{name: "empty type", data: `{"seq":1,"type":"","payload":{}}`},
		{name: "no payload", data: `{"seq":1,"type":"order.closed"}`},
		{name: "null payload", data: `{"seq":1,"type":"order.closed","payload":null}`},
		{name: "array payload", data: `{"seq":1,"type":"ticket.sent","payload":[1]}`},
		{name: "string payload", data: `{"seq":1,"type":"order.closed","payload":"closed"}`},
		{name: "wrong field kind", data: `{"seq":1,"type":"line.qty_changed","payload":{"line_id":"two"}}`},
		{name: "fractional money", data: `{"seq":1,"type":"refund.issued","payload":{"amount":{"minor":12.5,"currency":"INR"}}}`},
		{name: "bad time", data: `{"seq":1,"type":"order.closed","at":"yesterday","payload":{}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Called directly: json.Unmarshal rejects invalid syntax with
			// its own *json.SyntaxError before our decoder ever runs.
			var e Event
			err := e.UnmarshalJSON([]byte(tt.data))
			if !errors.Is(err, ErrMalformedEvent) {
				t.Fatalf("err = %v, want ErrMalformedEvent", err)
			}
			if !reflect.DeepEqual(e, Event{}) {
				t.Errorf("a failed decode left a partial event: %#v", e)
			}
		})
	}
}

func TestMarshalRefusesAnInconsistentEvent(t *testing.T) {
	tests := []struct {
		name string
		in   Event
	}{
		{name: "no payload", in: Event{Seq: 1, Type: TypeOrderClosed, At: testAt}},
		{name: "type disagrees with payload", in: Event{Seq: 1, Type: TypeLineVoided, At: testAt, Payload: LineComped{LineID: 2}}},
		{name: "no type", in: Event{Seq: 1, At: testAt, Payload: OrderClosed{ClosedBy: 1}}},
		{name: "payload cannot be encoded", in: NewEvent(1, testAt, 1, 1, InvoiceIssued{TaxSnapshot: json.RawMessage(`{not json`)})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := json.Marshal(tt.in); !errors.Is(err, ErrMalformedEvent) {
				t.Errorf("err = %v, want ErrMalformedEvent", err)
			}
		})
	}
}

func TestTypesIsSortedAndStable(t *testing.T) {
	got := Types()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("Types() not strictly sorted at %d: %q then %q", i, got[i-1], got[i])
		}
	}
	if !reflect.DeepEqual(got, Types()) {
		t.Error("Types() returned a different order on a second call")
	}
}
