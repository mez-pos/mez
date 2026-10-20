package printjob

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/fsm"
)

func TestTransitions(t *testing.T) {
	allowed := map[[2]State]bool{
		{Queued, Printing}: true, {Queued, Dead}: true,
		{Printing, Done}: true, {Printing, Failed}: true, {Printing, Dead}: true,
		{Failed, Printing}: true, {Failed, Queued}: true, {Failed, Dead}: true,
		{Dead, Queued}: true,
	}
	for _, from := range Machine().States() {
		for _, to := range Machine().States() {
			_, err := Transition(from, to)
			if allowed[[2]State{from, to}] != (err == nil) {
				t.Errorf("%s -> %s: err = %v", from, to, err)
			}
			if err != nil && !errors.Is(err, fsm.ErrInvalidTransition) {
				t.Errorf("%s -> %s: %v is not ErrInvalidTransition", from, to, err)
			}
		}
	}
	// Done is final: a printed receipt is never printed again by the queue.
	if next := Machine().Next(Done); len(next) != 0 {
		t.Errorf("done moves to %v", next)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 2 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{7, 128 * time.Second},
		{8, 256 * time.Second},
		{9, 5 * time.Minute},
		{50, 5 * time.Minute},
	}
	for _, tt := range tests {
		if got := Backoff(tt.attempt); got != tt.want {
			t.Errorf("Backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
	var total time.Duration
	for a := 1; a < MaxAttempts; a++ {
		total += Backoff(a)
	}
	if want := 13*time.Minute + 30*time.Second; total != want {
		t.Errorf("the retries span %v; the schedule promises %v", total, want)
	}
}

func TestAfter(t *testing.T) {
	now := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)
	refused := errors.New("connection refused")
	tests := []struct {
		name    string
		attempt int
		err     error
		want    Outcome
	}{
		{"printed", 1, nil, Outcome{State: Done}},
		{"printed on a retry", 4, nil, Outcome{State: Done}},
		{"first failure waits 2s", 1, refused, Outcome{State: Failed, Next: now.Add(2 * time.Second), Error: "connection refused"}},
		{"third failure waits 8s", 3, refused, Outcome{State: Failed, Next: now.Add(8 * time.Second), Error: "connection refused"}},
		{"the last try gives up", MaxAttempts, refused, Outcome{State: Dead, Error: "connection refused"}},
		{"a permanent error gives up at once", 1, fmt.Errorf("no spooler here: %w", ErrPermanent),
			Outcome{State: Dead, Error: "no spooler here: will not succeed on a retry"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := After(tt.attempt, tt.err, now); got != tt.want {
				t.Errorf("After = %+v, want %+v", got, tt.want)
			}
		})
	}
}
