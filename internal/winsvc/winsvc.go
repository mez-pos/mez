package winsvc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kardianos/service"
)

// What the service is called. Name is the key in the Service Control Manager
// and the source of its entries in the event log, so it is not to change
// once a venue has installed one: a renamed service is a second service.
const (
	Name        = "mez"
	DisplayName = "mez point of sale"
	Description = "The mez server: the till, the kitchen display and the back office for this venue."
)

// RestartDelay is how long Windows waits before starting a server that died.
// Long enough that a server failing at once (its port taken, its disk full)
// writes an event every few seconds and not a thousand a minute; short
// enough that a till notices only a reconnect.
const RestartDelay = 5 * time.Second

// StopTimeout bounds a stop. The server drains its requests and then closes
// its subsystems, each within ten seconds, and a stop that outlasts both has
// hung.
const StopTimeout = 25 * time.Second

// Status is what the Service Control Manager says of the service.
type Status string

// The states a person can act on. Starting counts as running and stopping
// as stopped, as the manager's own tools round them.
const (
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusNotInstalled Status = "not installed"
)

// SystemLog is the operating system's log for this service: on Windows, the
// event log under the service's name.
type SystemLog interface {
	Error(v ...any) error
	Warning(v ...any) error
	Info(v ...any) error
}

// Options is what a Service is made from.
type Options struct {
	// Run is the server. It serves until ctx is cancelled and returns once
	// everything it started has stopped. system is the event log, or nil
	// when a person started the service's command in a terminal. Nil when
	// the Service is only to be installed, started or stopped.
	Run func(ctx context.Context, system SystemLog) error

	// Arguments are what the Service Control Manager starts the binary
	// with. Paths in them must be absolute: a service starts in system32.
	Arguments []string

	// UserName is the account the service runs as, and Password its
	// password. Empty is LocalSystem, which needs none (ADR-263).
	UserName string
	Password string
}

// Service is the server as the Service Control Manager knows it.
type Service struct {
	svc  service.Service
	prog *program
}

// New describes the service. It touches nothing: installing, starting and
// running are the methods.
func New(o Options) (*Service, error) {
	prog := &program{run: o.Run, timeout: StopTimeout}
	cfg := &service.Config{
		Name:        Name,
		DisplayName: DisplayName,
		Description: Description,
		Arguments:   o.Arguments,
		UserName:    o.UserName,
		// The library's names for these are declared only in its Windows
		// file, so they are spelled out; other systems ignore them.
		Option: service.KeyValue{
			// Started with the computer, with nobody signed in.
			"StartType": "automatic",
			// Started again if the process dies. Windows does this only for
			// a process that ends without reporting that it stopped, which
			// is what program does when the server stops on its own.
			"OnFailure":              "restart",
			"OnFailureDelayDuration": RestartDelay.String(),
			// A day without a failure and the count starts again. With one
			// action it changes nothing; it is set so the value is ours.
			"OnFailureResetPeriod": int((24 * time.Hour).Seconds()),
		},
	}
	if o.Password != "" {
		cfg.Option["Password"] = o.Password
	}
	svc, err := service.New(prog, cfg)
	if err != nil {
		return nil, fmt.Errorf("describing the service: %w", err)
	}
	return &Service{svc: svc, prog: prog}, nil
}

// Interactive reports whether this process was started by a person and not
// by the Service Control Manager.
func Interactive() bool { return service.Interactive() }

// Run hands the process to the Service Control Manager and returns when the
// service has been stopped. Started by a person, it runs the server until
// Ctrl-C instead.
func (s *Service) Run() error {
	if s.prog.run == nil {
		return errors.New("running the service: no server was given")
	}
	// The wrapper's own lines: the event log under the manager, the
	// terminal otherwise. A server with no log to write to still serves.
	if log, err := s.svc.Logger(nil); err == nil {
		s.prog.log = log
	}
	if !Interactive() {
		s.prog.system = s.prog.log
	}
	if err := s.svc.Run(); err != nil {
		return fmt.Errorf("running the service: %w", err)
	}
	return nil
}

// Install registers the service and its event log source. It needs an
// administrator.
func (s *Service) Install() error {
	if err := s.svc.Install(); err != nil {
		return fmt.Errorf("installing the service: %w", err)
	}
	return nil
}

// Uninstall removes the service and its event log source. A service still
// running is removed when it stops.
func (s *Service) Uninstall() error {
	if err := s.svc.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling the service: %w", err)
	}
	return nil
}

// Start asks the manager to start the service and returns once it has been
// asked, not once the server is listening.
func (s *Service) Start() error {
	if err := s.svc.Start(); err != nil {
		return fmt.Errorf("starting the service: %w", err)
	}
	return nil
}

// Stop asks the manager to stop the service and waits for it.
func (s *Service) Stop() error {
	if err := s.svc.Stop(); err != nil {
		return fmt.Errorf("stopping the service: %w", err)
	}
	return nil
}

// Status reports whether the service is installed and running.
func (s *Service) Status() (Status, error) {
	st, err := s.svc.Status()
	switch {
	case errors.Is(err, service.ErrNotInstalled):
		return StatusNotInstalled, nil
	case err != nil:
		return "", fmt.Errorf("asking after the service: %w", err)
	case st == service.StatusRunning:
		return StatusRunning, nil
	default:
		return StatusStopped, nil
	}
}
