package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// subsystem is one thing that was started and now has to stop.
type subsystem struct {
	name  string
	close func(context.Context) error
}

// stack is the shutdown order, which is the startup order backwards.
//
// It is a stack because the dependencies run that way: the logger is opened
// first and closed last, since everything above it needs somewhere to report
// what went wrong on the way down.
type stack struct {
	subsystems []subsystem
}

// push registers a subsystem as started. Call it immediately after the thing
// starts, so that a failure three lines later still shuts down what is running.
func (s *stack) push(name string, close func(context.Context) error) {
	s.subsystems = append(s.subsystems, subsystem{name: name, close: close})
}

// shutdown closes everything in reverse order and returns whatever went wrong.
//
// It does not stop at the first error. A print queue that refuses to close must
// not strand the database handle behind it, because the next thing that happens
// on that machine is someone starting the binary again.
//
// Every close shares one deadline: ctx is already bounded by the caller, and a
// subsystem that ignores it eats the budget of the ones below. That is
// deliberate. The alternative, a timeout each, turns a single hung close into a
// shutdown that takes subsystems times the timeout, which on a venue PC is the
// difference between a slow restart and one that looks hung.
func (s *stack) shutdown(ctx context.Context, log *slog.Logger) error {
	var errs []error

	for i := len(s.subsystems) - 1; i >= 0; i-- {
		sub := s.subsystems[i]

		start := time.Now()
		err := sub.close(ctx)
		took := time.Since(start)

		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("closing %s: %w", sub.name, err))
			log.Error("subsystem did not close cleanly", "subsystem", sub.name, "error", err)
		default:
			log.Info("subsystem closed", "subsystem", sub.name, "took", took)
		}
	}

	return errors.Join(errs...)
}
