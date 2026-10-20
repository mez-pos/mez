package kitchen

import (
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/fsm"
	"github.com/mez-pos/mez/internal/domain/fsm/fsmtest"
)

// Restated from docs/02-domain.md, not copied from ticket.go.
func TestTicketTransitionsAreExactlyTheDomainTable(t *testing.T) {
	fsmtest.Exhaustive(t, Machine(),
		[]TicketStatus{TicketQueued, TicketFired, TicketInProgress, TicketReady, TicketCollected},
		map[TicketStatus][]TicketStatus{
			TicketQueued:     {TicketFired},
			TicketFired:      {TicketInProgress, TicketReady},
			TicketInProgress: {TicketReady},
			TicketReady:      {TicketCollected, TicketFired},
		})
}

func TestATicketFromQueueToPass(t *testing.T) {
	status := TicketQueued
	for _, next := range []TicketStatus{TicketFired, TicketInProgress, TicketReady, TicketCollected} {
		var err error
		if status, err = Transition(status, next); err != nil {
			t.Fatalf("ticket stopped: %v", err)
		}
	}
	if next := Machine().Next(TicketCollected); len(next) != 0 {
		t.Errorf("a collected ticket can still move to %v", next)
	}
}

func TestTicketShortcutsAreRefused(t *testing.T) {
	tests := []struct {
		name     string
		from, to TicketStatus
	}{
		{"cook before the course is fired", TicketQueued, TicketInProgress},
		{"collect from the queue", TicketQueued, TicketCollected},
		{"un-collect", TicketCollected, TicketReady},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Transition(tt.from, tt.to); !errors.Is(err, fsm.ErrInvalidTransition) {
				t.Errorf("Transition(%s, %s) err = %v, want ErrInvalidTransition", tt.from, tt.to, err)
			}
		})
	}
}
