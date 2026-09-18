// Package fsm is the transition table every lifecycle in the domain is
// checked against: orders, payments, tables and station tickets.
//
// A Machine is a fixed list of states and the moves allowed between them.
// It holds no current state. Callers own the state and ask the machine
// whether a move is legal, so there is one place per lifecycle where the
// rules are written down, and a test can walk every pair of states.
//
// An illegal move returns an error, never a panic: an order in the wrong
// state is a normal thing for a busy terminal to try, and the handler must
// be able to say so.
package fsm

import (
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidTransition means a move is not in the machine's table. Every
// refusal from every machine unwraps to this, so a handler maps it to one API
// error code, whichever lifecycle refused.
var ErrInvalidTransition = errors.New("invalid transition")

// TransitionError says which machine refused which move.
type TransitionError struct {
	Machine string // "order", "payment", "table", "ticket"
	From    string
	To      string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s: cannot go from %q to %q", e.Machine, e.From, e.To)
}

// Unwrap lets errors.Is(err, ErrInvalidTransition) succeed.
func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }

// Machine is the legal moves of one lifecycle. It is immutable once built.
type Machine[S ~string] struct {
	name   string
	states []S
	edges  map[S][]S
}

// New builds a machine from its states, in lifecycle order, and the moves out
// of each. It refuses a move that names a state not in the list, or a state
// listed twice, because a typo in a transition table is otherwise a move that
// silently never matches.
func New[S ~string](name string, states []S, edges map[S][]S) (Machine[S], error) {
	for i, s := range states {
		if slices.Contains(states[:i], s) {
			return Machine[S]{}, fmt.Errorf("fsm %s: state %q listed twice", name, s)
		}
	}
	copied := make(map[S][]S, len(edges))
	for from, tos := range edges {
		if !slices.Contains(states, from) {
			return Machine[S]{}, fmt.Errorf("fsm %s: move from unknown state %q", name, from)
		}
		for _, to := range tos {
			if !slices.Contains(states, to) {
				return Machine[S]{}, fmt.Errorf("fsm %s: move to unknown state %q", name, to)
			}
			if to == from {
				return Machine[S]{}, fmt.Errorf("fsm %s: %q moves to itself", name, from)
			}
		}
		copied[from] = slices.Clone(tos)
	}
	return Machine[S]{name: name, states: slices.Clone(states), edges: copied}, nil
}

// Must is New for tables written in source, where an error is a programming
// mistake found at startup and by the package's own tests, never at runtime.
func Must[S ~string](name string, states []S, edges map[S][]S) Machine[S] {
	m, err := New(name, states, edges)
	if err != nil {
		panic(err)
	}
	return m
}

// Transition checks the move from -> to. It returns to when the move is
// legal, so a caller can write status, err = m.Transition(status, next).
// Staying in the same state is not a move and is refused.
func (m Machine[S]) Transition(from, to S) (S, error) {
	if !m.Can(from, to) {
		return from, &TransitionError{Machine: m.name, From: string(from), To: string(to)}
	}
	return to, nil
}

// Can reports whether from -> to is a legal move.
func (m Machine[S]) Can(from, to S) bool {
	return slices.Contains(m.edges[from], to)
}

// Next lists the states reachable from from in one move, in table order. A
// terminal state has none.
func (m Machine[S]) Next(from S) []S {
	return slices.Clone(m.edges[from])
}

// States lists every state, in lifecycle order.
func (m Machine[S]) States() []S {
	return slices.Clone(m.states)
}

// Name is the machine's name, as it appears in a TransitionError.
func (m Machine[S]) Name() string { return m.name }
