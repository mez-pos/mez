package winsvc

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// runInteractively is the service's command started by a person in a
// terminal: the server runs until Ctrl-C, and its stop is the same stop the
// Service Control Manager would ask for.
func (s *Service) runInteractively() error {
	if err := s.prog.Start(nil); err != nil {
		return fmt.Errorf("running the service: %w", err)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case <-stop:
	case <-s.prog.done:
		// The server stopped on its own; died has ended the process by now
		// unless a test stands in for it.
		return nil
	}
	if err := s.prog.Stop(nil); err != nil {
		return fmt.Errorf("running the service: %w", err)
	}
	return nil
}

// consoleLog is the wrapper's lines when a person runs the service's
// command in a terminal: to stderr, beside the server's own log.
type consoleLog struct{}

func (consoleLog) Error(v ...any) error   { return consoleWrite("error", v) }
func (consoleLog) Warning(v ...any) error { return consoleWrite("warning", v) }
func (consoleLog) Info(v ...any) error    { return consoleWrite("info", v) }

func consoleWrite(level string, v []any) error {
	_, err := fmt.Fprintf(os.Stderr, "mez service [%s] %s\n", level, fmt.Sprint(v...))
	return err
}
