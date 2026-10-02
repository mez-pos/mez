package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// TestShutdownRunsInReverse is the whole contract. The logger is opened first
// and has to close last, and a database handle opened after it must be gone
// before it.
func TestShutdownRunsInReverse(t *testing.T) {
	var order []string

	var s stack
	for _, name := range []string{"logging", "database", "print queue"} {
		s.push(name, func(context.Context) error {
			order = append(order, name)
			return nil
		})
	}

	if err := s.shutdown(context.Background(), quietLogger()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	want := []string{"print queue", "database", "logging"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("closed %v, want %v", order, want)
	}
}

// TestShutdownContinuesPastFailure covers the case that actually costs
// something: one subsystem refusing to close must not strand the ones below it,
// because the next thing that happens on that machine is the operator starting
// the binary again and finding the port or the database still held.
func TestShutdownContinuesPastFailure(t *testing.T) {
	broken := errors.New("printer is wedged")
	closed := map[string]bool{}

	var s stack
	s.push("logging", func(context.Context) error {
		closed["logging"] = true
		return nil
	})
	s.push("print queue", func(context.Context) error { return broken })

	err := s.shutdown(context.Background(), quietLogger())

	if !errors.Is(err, broken) {
		t.Errorf("error = %v, want it to wrap %v", err, broken)
	}
	if !strings.Contains(err.Error(), "closing print queue") {
		t.Errorf("error = %q, want it to name the subsystem", err)
	}
	if !closed["logging"] {
		t.Error("logging was not closed: one failure stranded the subsystem below it")
	}
}

// TestShutdownJoinsEveryFailure keeps a second broken subsystem from hiding
// behind the first in the log a support call will be reading.
func TestShutdownJoinsEveryFailure(t *testing.T) {
	first, second := errors.New("db busy"), errors.New("printer wedged")

	var s stack
	s.push("database", func(context.Context) error { return first })
	s.push("print queue", func(context.Context) error { return second })

	err := s.shutdown(context.Background(), quietLogger())

	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Errorf("error = %v, want it to wrap both %v and %v", err, first, second)
	}
}

// TestShutdownOfNothingSucceeds: a startup that fails before anything starts
// still unwinds through here.
func TestShutdownOfNothingSucceeds(t *testing.T) {
	var s stack
	if err := s.shutdown(context.Background(), quietLogger()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

// TestShutdownPassesItsDeadline proves the deadline reaches the subsystems,
// which is what lets a close that talks to hardware give up rather than hang
// the restart.
func TestShutdownPassesItsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var seen error
	var s stack
	s.push("print queue", func(ctx context.Context) error {
		seen = ctx.Err()
		return nil
	})

	if err := s.shutdown(ctx, quietLogger()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !errors.Is(seen, context.Canceled) {
		t.Errorf("subsystem saw ctx.Err() = %v, want context.Canceled", seen)
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
