package table

import (
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/fsm"
	"github.com/mez-pos/mez/internal/domain/fsm/fsmtest"
)

// Restated from docs/02-domain.md and ADR-047, not copied from table.go.
func TestTableTransitionsAreExactlyTheDomainTable(t *testing.T) {
	fsmtest.Exhaustive(t, Machine(),
		[]Status{StatusFree, StatusSeated, StatusOrdered, StatusSettling, StatusDirty},
		map[Status][]Status{
			StatusFree:     {StatusSeated},
			StatusSeated:   {StatusOrdered, StatusFree},
			StatusOrdered:  {StatusSettling},
			StatusSettling: {StatusDirty, StatusOrdered},
			StatusDirty:    {StatusFree},
		})
}

func TestAFullTurn(t *testing.T) {
	turn := []Status{StatusFree, StatusSeated, StatusOrdered, StatusSettling, StatusOrdered, StatusSettling, StatusDirty, StatusFree}
	status := turn[0]
	for _, next := range turn[1:] {
		var err error
		if status, err = Transition(status, next); err != nil {
			t.Fatalf("turn stopped: %v", err)
		}
	}
}

func TestShortcutsAreRefused(t *testing.T) {
	tests := []struct {
		name     string
		from, to Status
	}{
		{"seat a dirty table", StatusDirty, StatusSeated},
		{"free a table mid-meal", StatusOrdered, StatusFree},
		{"free a table without clearing it", StatusSettling, StatusFree},
		{"order at an empty table", StatusFree, StatusOrdered},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transition(tt.from, tt.to)
			if !errors.Is(err, fsm.ErrInvalidTransition) || got != tt.from {
				t.Errorf("Transition(%s, %s) = %s, %v; want %s, ErrInvalidTransition", tt.from, tt.to, got, err, tt.from)
			}
		})
	}
}
