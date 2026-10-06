package winsvc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLog records the wrapper's lines, by severity.
type fakeLog struct {
	mu    sync.Mutex
	lines []string
}

func (f *fakeLog) Error(v ...any) error   { return f.add("error", v) }
func (f *fakeLog) Warning(v ...any) error { return f.add("warning", v) }
func (f *fakeLog) Info(v ...any) error    { return f.add("info", v) }

func (f *fakeLog) add(kind string, v []any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, kind+": "+v[0].(string))
	return nil
}

func (f *fakeLog) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.lines, "\n")
}

// untilCancelled is a server that serves until it is told to stop.
func untilCancelled(result error) func(context.Context, SystemLog) error {
	return func(ctx context.Context, _ SystemLog) error {
		<-ctx.Done()
		return result
	}
}

// TestStopDrainsTheServer: a stop from the Service Control Manager is the
// server's own shutdown, waited for, and not the process being cut off with
// a payment half written.
func TestStopDrainsTheServer(t *testing.T) {
	tests := []struct {
		name    string
		result  error
		wantErr bool
		wantLog string
	}{
		{"a clean stop", nil, false, "info: mez stopped."},
		{"a stop that went wrong says so", errors.New("closing database: locked"), true, "error: mez stopped, and not cleanly: closing database: locked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &fakeLog{}
			var exited sync.Once
			ended := false
			p := &program{run: untilCancelled(tt.result), timeout: time.Second, log: log, exit: func(int) { exited.Do(func() { ended = true }) }}

			if err := p.Start(nil); err != nil {
				t.Fatalf("Start: %v", err)
			}
			err := p.Stop(nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("Stop = %v, want an error: %v", err, tt.wantErr)
			}
			if ended {
				t.Error("the process was ended; a stop that was asked for must report itself stopped")
			}
			if !strings.Contains(log.String(), tt.wantLog) {
				t.Errorf("log = %q, want %q in it", log.String(), tt.wantLog)
			}
		})
	}
}

// TestAServerThatDiesEndsTheProcess: Windows restarts a service only when
// its process ends without reporting a stop. A server that could not bind
// its port, or lost its listener, has to end that way or it stays down
// until somebody walks to the computer.
func TestAServerThatDiesEndsTheProcess(t *testing.T) {
	tests := []struct {
		name    string
		result  error
		wantLog string
	}{
		{"it could not start", errors.New("listening on :7777: address already in use"),
			"error: mez stopped on its own and will be started again: listening on :7777: address already in use"},
		{"it stopped without a reason", nil, "error: mez stopped on its own and will be started again."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &fakeLog{}
			code := make(chan int, 1)
			p := &program{
				run:     func(context.Context, SystemLog) error { return tt.result },
				timeout: time.Second,
				log:     log,
				exit:    func(c int) { code <- c },
			}

			if err := p.Start(nil); err != nil {
				t.Fatalf("Start: %v", err)
			}

			select {
			case got := <-code:
				if got != 1 {
					t.Errorf("exit code = %d, want 1", got)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the process was not ended")
			}
			if !strings.Contains(log.String(), tt.wantLog) {
				t.Errorf("log = %q, want %q in it", log.String(), tt.wantLog)
			}
		})
	}
}

// TestStopGivesUpOnAServerThatHangs: a stop that never returns leaves the
// manager showing "stopping" for ever, and the owner with no way to start
// the till again short of a reboot.
func TestStopGivesUpOnAServerThatHangs(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	p := &program{
		run:     func(context.Context, SystemLog) error { <-release; return nil },
		timeout: 20 * time.Millisecond,
		exit:    func(int) {},
	}

	if err := p.Start(nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	err := p.Stop(nil)

	if err == nil || !strings.Contains(err.Error(), "did not stop within") {
		t.Errorf("Stop = %v, want it to say the server did not stop in time", err)
	}
}

// TestStopBeforeStartIsNothing: the manager may stop a service whose start
// it never finished.
func TestStopBeforeStartIsNothing(t *testing.T) {
	p := &program{timeout: time.Second}
	if err := p.Stop(nil); err != nil {
		t.Errorf("Stop = %v, want nil", err)
	}
}

// TestTheServerIsHandedTheSystemLog: the server's own warnings reach the
// event log only if the wrapper passes it on.
func TestTheServerIsHandedTheSystemLog(t *testing.T) {
	system := &fakeLog{}
	got := make(chan SystemLog, 1)
	p := &program{
		run: func(ctx context.Context, s SystemLog) error {
			got <- s
			<-ctx.Done()
			return nil
		},
		timeout: time.Second,
		system:  system,
	}

	if err := p.Start(nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s := <-got; s != SystemLog(system) {
		t.Errorf("the server was handed %v, want the system log", s)
	}
	if err := p.Stop(nil); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

// TestNewDescribesTheService: the name is what the manager shows, and a
// service with no server cannot be run.
func TestNewDescribesTheService(t *testing.T) {
	s, err := New(Options{Arguments: []string{"service", "run"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.svc.String(); got != DisplayName {
		t.Errorf("the service is shown as %q, want %q", got, DisplayName)
	}
	if err := s.Run(); err == nil {
		t.Error("Run with no server = nil, want an error")
	}
}
