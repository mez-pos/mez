package order

import (
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/fsm"
	"github.com/byjoellobo/mez/internal/domain/kitchen"
)

func TestBumpAndRecall(t *testing.T) {
	sent := []Payload{opened(), coffee(1), TicketSent{Station: "bar", LineIDs: []int64{2}}} // ticket 3
	tests := []struct {
		name    string
		then    []Payload
		status  kitchen.TicketStatus
		wantErr error
	}{
		{"sent is fired", nil, kitchen.TicketFired, nil},
		{"bumped", []Payload{TicketBumped{TicketSeq: 3, Station: "bar"}}, kitchen.TicketReady, nil},
		{"bumped and recalled", []Payload{TicketBumped{TicketSeq: 3, Station: "bar"}, TicketRecalled{TicketSeq: 3, Station: "bar"}}, kitchen.TicketFired, nil},
		{"bumped twice", []Payload{TicketBumped{TicketSeq: 3, Station: "bar"}, TicketBumped{TicketSeq: 3, Station: "bar"}}, "", fsm.ErrInvalidTransition},
		{"recalled unbumped", []Payload{TicketRecalled{TicketSeq: 3, Station: "bar"}}, "", fsm.ErrInvalidTransition},
		{"no such ticket", []Payload{TicketBumped{TicketSeq: 9, Station: "bar"}}, "", ErrTicketNotFound},
		{"another station's ticket", []Payload{TicketBumped{TicketSeq: 3, Station: "kitchen"}}, "", ErrTicketNotFound},
		{"bumped while held", []Payload{OrderHeld{}, TicketBumped{TicketSeq: 3, Station: "bar"}}, kitchen.TicketReady, nil},
		{"started", []Payload{TicketStarted{TicketSeq: 3, Station: "bar"}}, kitchen.TicketInProgress, nil},
		{"started and bumped", []Payload{TicketStarted{TicketSeq: 3, Station: "bar"}, TicketBumped{TicketSeq: 3, Station: "bar"}}, kitchen.TicketReady, nil},
		{"started twice", []Payload{TicketStarted{TicketSeq: 3, Station: "bar"}, TicketStarted{TicketSeq: 3, Station: "bar"}}, "", fsm.ErrInvalidTransition},
		{"started once ready", []Payload{TicketBumped{TicketSeq: 3, Station: "bar"}, TicketStarted{TicketSeq: 3, Station: "bar"}}, "", fsm.ErrInvalidTransition},
		{"recalled while in progress", []Payload{TicketStarted{TicketSeq: 3, Station: "bar"}, TicketRecalled{TicketSeq: 3, Station: "bar"}}, "", fsm.ErrInvalidTransition},
		{"started at another station", []Payload{TicketStarted{TicketSeq: 3, Station: "kitchen"}}, "", ErrTicketNotFound},
		{"started after the bill is closed", []Payload{pay(PaymentCash, 22000), OrderClosed{ClosedBy: 1}, TicketStarted{TicketSeq: 3, Station: "bar"}}, kitchen.TicketInProgress, nil},
		{
			"recalled is new again: it is started afresh",
			[]Payload{TicketStarted{TicketSeq: 3, Station: "bar"}, TicketBumped{TicketSeq: 3, Station: "bar"}, TicketRecalled{TicketSeq: 3, Station: "bar"}},
			kitchen.TicketFired, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Project(stream(append(append([]Payload{}, sent...), tt.then...)...), untaxed)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			tk := s.Tickets[0]
			if tk.Status != tt.status || (tt.status == kitchen.TicketReady) == tk.ReadyAt.IsZero() {
				t.Errorf("ticket = %s, ready at %v; want %s", tk.Status, tk.ReadyAt, tt.status)
			}
			// Fired is new on the board: nobody has started it, even after a recall.
			if (tt.status == kitchen.TicketInProgress && tk.StartedAt.IsZero()) || (tt.status == kitchen.TicketFired && !tk.StartedAt.IsZero()) {
				t.Errorf("ticket %s started at %v", tk.Status, tk.StartedAt)
			}
		})
	}
}
