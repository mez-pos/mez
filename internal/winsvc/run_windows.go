//go:build windows

package winsvc

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// stopWaitHint is what the Service Control Manager is told a stop will take,
// and PreshutdownTimeout how long it waits for one at shutdown before
// killing the process (ADR-375). The server's own stop fits ten seconds;
// the hint is longer so that a checkpoint a second apart is never late, and
// the preshutdown time is what Windows would otherwise cut to five.
const (
	stopWaitHint       = 30 * time.Second
	stopCheckpointTick = time.Second
	PreshutdownTimeout = 60 * time.Second
)

// interactive reports whether this process was started by a person and not
// by the Service Control Manager.
func interactive() bool {
	isService, err := svc.IsWindowsService()
	return err != nil || !isService
}

// run hands the process to the Service Control Manager, or, started by a
// person, runs the server until Ctrl-C.
func (s *Service) run() error {
	if interactive() {
		s.prog.log = consoleLog{}
		return s.runInteractively()
	}
	if log, err := eventlog.Open(Name); err == nil {
		s.prog.log = &systemEventLog{log: log}
		s.prog.system = s.prog.log
		defer log.Close() //nolint:errcheck // nothing to do if it fails
	}
	if err := svc.Run(Name, &handler{prog: s.prog}); err != nil {
		return fmt.Errorf("running the service: %w", err)
	}
	return nil
}

// handler is the service as the Service Control Manager drives it: started
// once, then told to stop, by a stop, a shutdown or a pre-shutdown. While
// the server stops, the manager hears a checkpoint a second, so it knows
// the stop is going on and does not kill it (ADR-375).
type handler struct {
	prog *program
	// tick is the checkpoint interval; the tests shorten it.
	tick time.Duration
}

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecific bool, exitCode uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown
	changes <- svc.Status{State: svc.StartPending}
	if err := h.prog.Start(nil); err != nil {
		h.prog.complain(fmt.Sprintf("mez could not start: %v", err))
		return true, 1
	}
	changes <- svc.Status{State: svc.Running, Accepts: accepts}

	for req := range requests {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown, svc.PreShutdown:
			if err := h.stop(changes); err != nil {
				return true, 2
			}
			return false, 0
		}
	}
	return false, 0
}

// stop ends the server, telling the manager every second that it is still
// stopping.
func (h *handler) stop(changes chan<- svc.Status) error {
	tick := h.tick
	if tick <= 0 {
		tick = stopCheckpointTick
	}
	hint := uint32(stopWaitHint / time.Millisecond)
	checkpoint := uint32(1)
	changes <- svc.Status{State: svc.StopPending, CheckPoint: checkpoint, WaitHint: hint}

	done := make(chan error, 1)
	go func() { done <- h.prog.Stop(nil) }()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ticker.C:
			checkpoint++
			changes <- svc.Status{State: svc.StopPending, CheckPoint: checkpoint, WaitHint: hint}
		}
	}
}

// systemEventLog is the Windows event log under the service's name, as the
// wrapper and the server write to it.
type systemEventLog struct{ log *eventlog.Log }

// One event id for each severity: the message is the text, and Event Viewer
// shows it in words.
const (
	eventError   = 1
	eventWarning = 2
	eventInfo    = 3
)

func (l *systemEventLog) Error(v ...any) error { return l.log.Error(eventError, fmt.Sprint(v...)) }
func (l *systemEventLog) Warning(v ...any) error {
	return l.log.Warning(eventWarning, fmt.Sprint(v...))
}
func (l *systemEventLog) Info(v ...any) error { return l.log.Info(eventInfo, fmt.Sprint(v...)) }

// setPreshutdown tells the Service Control Manager how long to wait for
// this service at shutdown before killing it: the default is a few seconds,
// and the server's stop is ten (ADR-375). The service library has no door
// for it, so this goes to the manager directly, as the restarts do.
func setPreshutdown(name string, timeout time.Duration) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("reaching the service manager: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck // nothing to do if it fails

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening the service: %w", err)
	}
	defer s.Close() //nolint:errcheck // nothing to do if it fails

	// SERVICE_CONFIG_PRESHUTDOWN_INFO and SERVICE_PRESHUTDOWN_INFO, which
	// x/sys/windows does not name.
	const serviceConfigPreshutdownInfo = 7
	info := struct{ PreshutdownTimeout uint32 }{uint32(timeout / time.Millisecond)}
	if err := windows.ChangeServiceConfig2(s.Handle, serviceConfigPreshutdownInfo, (*byte)(unsafe.Pointer(&info))); err != nil {
		return fmt.Errorf("setting the shutdown time: %w", err)
	}
	return nil
}
