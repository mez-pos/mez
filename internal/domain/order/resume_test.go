package order

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// reopenedStream settles, invoices, closes, reopens and settles again, so a
// resume point can fall on either side of every flag the fold keeps.
func reopenedStream() []Payload {
	return []Payload{
		opened(),
		coffee(2),               // 2: 440.00
		pay(PaymentCash, 44000), // 3
		invoiceNo("NF/2026-27/000001"),
		closed,
		OrderReopened{Reason: "guest added a pastry", AuthorisedBy: 3},
		item("Almond croissant", 1, 18000), // 7
		pay(PaymentUPI, 18000),             // 8
		invoiceNo("NF/2026-27/000002"),
		closed,
		NoteAdded{Scope: ScopeOrder, Text: "regular, likes it hot"},
	}
}

// TestResumeFromAnyPointEqualsAFullReplay is the property a snapshot rests
// on: stopping a projection anywhere, encoding it, decoding it and carrying
// on gives exactly what one fold over every event gives.
func TestResumeFromAnyPointEqualsAFullReplay(t *testing.T) {
	type stream_ struct {
		name     string
		payloads []Payload
		taxes    Taxes
	}
	var streams []stream_
	for _, sc := range scenarios {
		streams = append(streams, stream_{sc.name, sc.payloads, untaxed})
	}
	streams = append(streams,
		stream_{"reopened and settled twice", reopenedStream(), untaxed},
		stream_{"gst exclusive with a discount", []Payload{
			opened(),
			dish("Masala dosa", 2, 12000),
			dish("Filter coffee", 3, 4000),
			pct(ScopeOrder, 0, 1000),
			pay(PaymentCard, 34020),
			invoiceNo("NF/2026-27/000003"),
			closed,
		}, gstCafe(tax.Exclusive)},
	)

	for _, st := range streams {
		t.Run(st.name, func(t *testing.T) {
			events := stream(st.payloads...)
			want, err := Project(events, st.taxes)
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			for k := 1; k <= len(events); k++ {
				head, err := Project(events[:k], st.taxes)
				if err != nil {
					t.Fatalf("Project(events[:%d]): %v", k, err)
				}
				got, err := Resume(roundTrip(t, head), events[k:], st.taxes)
				if err != nil {
					t.Fatalf("Resume at %d: %v", k, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("resumed at %d:\n got %+v\nwant %+v", k, got, want)
				}
			}
		})
	}
}

// roundTrip passes a State through JSON, as a stored snapshot does.
func roundTrip(t *testing.T, s State) State {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("encoding state: %v", err)
	}
	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding state: %v", err)
	}
	return out
}

func TestResumeKeepsTheFoldsRules(t *testing.T) {
	events := stream(reopenedStream()...)
	tests := []struct {
		name  string
		at    int // resume after this many events
		next  Payload
		seq   int64 // 0 for the next seq
		wants error
	}{
		{"a second invoice for one settlement", 4, invoiceNo("NF/2026-27/000009"), 0, ErrAlreadyInvoiced},
		{"a line on a closed order", 5, coffee(1), 0, ErrOrderClosed},
		{"a close that is not closed", 6, OrderReopened{Reason: "x", AuthorisedBy: 3}, 0, ErrOrderNotClosed},
		{"a second open", 2, opened(), 0, ErrAlreadyOpened},
		{"a seq that skips", 2, coffee(1), 9, ErrSeqGap},
		{"a seq that repeats", 2, coffee(1), 2, ErrSeqGap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, err := Project(events[:tt.at], untaxed)
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			seq := tt.seq
			if seq == 0 {
				seq = from.LastSeq + 1
			}
			e := NewEvent(seq, testAt, 11, 2, tt.next)
			_, err = Resume(from, []Event{e}, untaxed)
			var pe *ProjectionError
			if !errors.Is(err, tt.wants) || !errors.As(err, &pe) {
				t.Fatalf("err = %v, want a *ProjectionError wrapping %v", err, tt.wants)
			}
		})
	}
}

func TestResumeFromNothing(t *testing.T) {
	if _, err := Resume(State{}, nil, untaxed); !errors.Is(err, ErrNotOpened) {
		t.Errorf("Resume of nothing: err = %v, want ErrNotOpened", err)
	}
	events := stream(opened(), coffee(1))
	got, err := Resume(State{}, events, untaxed)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if want := mustProject(t, opened(), coffee(1)); !reflect.DeepEqual(got, want) {
		t.Errorf("Resume from zero = %+v, want Project's %+v", got, want)
	}
	same, err := Resume(got, nil, untaxed)
	if err != nil || !reflect.DeepEqual(same, got) {
		t.Errorf("Resume with no events = %+v, %v; want the state unchanged", same, err)
	}
}

// TestResumeDoesNotTouchItsInput guards the aliasing clone exists for. Every
// event here edits something the earlier state already holds: a line's
// quantity, a line's status, a tender's refund, a ticket's lines.
func TestResumeDoesNotTouchItsInput(t *testing.T) {
	head := stream(
		opened(),
		coffee(2),                          // 2
		item("Almond croissant", 1, 18000), // 3
		TicketSent{Station: "bar", LineIDs: []int64{2}},
		off(ScopeLine, 2, 1000),
		pay(PaymentCard, 61000), // 6
	)
	from, err := Project(head, untaxed)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	before := roundTrip(t, from)
	tail := []Event{
		NewEvent(7, testAt, 11, 2, void(3)),
		NewEvent(8, testAt, 11, 2, RefundIssued{Amount: inr(1000), AgainstPaymentID: 6, Reason: "croissant was out"}),
		NewEvent(9, testAt, 11, 2, LineQtyChanged{LineID: 2, From: 2, To: 3}),
	}
	if _, err := Resume(from, tail, untaxed); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if !reflect.DeepEqual(from, before) {
		t.Errorf("Resume changed its input:\n got %+v\nwant %+v", from, before)
	}
}

func TestDecodePayload(t *testing.T) {
	for _, p := range samplePayloads() {
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("encoding %T: %v", p, err)
		}
		got, err := DecodePayload(p.EventType(), raw)
		if err != nil {
			t.Fatalf("DecodePayload(%s): %v", p.EventType(), err)
		}
		if !reflect.DeepEqual(got, p) {
			t.Errorf("DecodePayload(%s) = %+v, want %+v", p.EventType(), got, p)
		}
	}
	if _, err := DecodePayload("loyalty.points_earned", json.RawMessage(`{}`)); !errors.Is(err, ErrUnknownEventType) {
		t.Errorf("unknown type: err = %v, want ErrUnknownEventType", err)
	}
	if _, err := DecodePayload(TypeNoteAdded, json.RawMessage(`null`)); !errors.Is(err, ErrMalformedEvent) {
		t.Errorf("null payload: err = %v, want ErrMalformedEvent", err)
	}
}
