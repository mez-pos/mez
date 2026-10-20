// Package fsmtest checks a machine against a transition table restated by
// hand in a test. The restatement is the point: the machine's table and the
// test's table are written separately, from docs/02-domain.md, so a slip in
// one shows up as a disagreement with the other.
package fsmtest

import (
	"errors"
	"slices"
	"testing"

	"github.com/mez-pos/mez/internal/domain/fsm"
)

// Exhaustive walks every pair of states, not a sample. Each move in legal
// must succeed; every other pair, staying put included, must fail with
// fsm.ErrInvalidTransition. states is the lifecycle in order, and must match
// the machine's own list exactly.
func Exhaustive[S ~string](t *testing.T, m fsm.Machine[S], states []S, legal map[S][]S) {
	t.Helper()
	if !slices.Equal(m.States(), states) {
		t.Fatalf("%s states = %v, want %v", m.Name(), m.States(), states)
	}
	checked := 0
	for _, from := range states {
		for _, to := range states {
			want := slices.Contains(legal[from], to)
			got, err := m.Transition(from, to)
			switch {
			case want && err != nil:
				t.Errorf("%s: %s -> %s should be legal: %v", m.Name(), from, to, err)
			case want && got != to:
				t.Errorf("%s: %s -> %s returned %q", m.Name(), from, to, got)
			case !want && !errors.Is(err, fsm.ErrInvalidTransition):
				t.Errorf("%s: %s -> %s should be refused, got err %v", m.Name(), from, to, err)
			}
			checked++
		}
	}
	for from, tos := range legal {
		for _, to := range tos {
			if !slices.Contains(states, from) || !slices.Contains(states, to) {
				t.Errorf("%s: the test lists %s -> %s, which names a state the machine lacks", m.Name(), from, to)
			}
		}
	}
	t.Logf("%s: checked all %d pairs of %d states", m.Name(), checked, len(states))
}
