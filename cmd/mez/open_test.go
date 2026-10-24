package main

import (
	"errors"
	"flag"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/winsvc"
)

// fakeMachine is a Windows computer as "mez open" sees it: a server that
// answers after so many questions, a service in some state, and a browser.
type fakeMachine struct {
	goos string
	// upAfter is how many health questions go unanswered before the
	// server is up; -1 is never.
	upAfter  int
	asked    int
	status   winsvc.Status
	startErr error
	elevErr  error

	started, elevated int
	opened            []string
	fullscreen        bool
	alerts            []string
	slept             time.Duration
	clock             time.Time
}

func (m *fakeMachine) env() openEnv {
	m.clock = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	return openEnv{
		goos: m.goos,
		healthy: func(string) bool {
			m.asked++
			return m.upAfter >= 0 && m.asked > m.upAfter
		},
		status: func() (winsvc.Status, error) { return m.status, nil },
		start: func() error {
			m.started++
			return m.startErr
		},
		elevate: func() error {
			m.elevated++
			return m.elevErr
		},
		browse: func(url string, fullscreen bool) error {
			m.opened = append(m.opened, url)
			m.fullscreen = fullscreen
			return nil
		},
		alert: func(text string) { m.alerts = append(m.alerts, text) },
		sleep: func(d time.Duration) {
			m.slept += d
			m.clock = m.clock.Add(d)
		},
		now: func() time.Time { return m.clock },
	}
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		machine     fakeMachine
		wantOpened  int
		wantStarts  int
		wantElevate int
		wantAlert   string
		wantErr     string
	}{
		{
			name:       "a server that answers is opened at once, in a window of its own",
			machine:    fakeMachine{goos: "windows", upAfter: 0, status: winsvc.StatusRunning},
			wantOpened: 1,
		},
		{
			name:       "full screen is asked for",
			args:       []string{"-fullscreen"},
			machine:    fakeMachine{goos: "windows", upAfter: 0, status: winsvc.StatusRunning},
			wantOpened: 1,
		},
		{
			name:       "a stopped service is started and the till waits for it",
			machine:    fakeMachine{goos: "windows", upAfter: 4, status: winsvc.StatusStopped},
			wantOpened: 1,
			wantStarts: 1,
		},
		{
			name:       "a service still starting after the computer was turned on is waited for, not started again",
			machine:    fakeMachine{goos: "windows", upAfter: 6, status: winsvc.StatusRunning},
			wantOpened: 1,
		},
		{
			name:        "a service this user may not start is started with an administrator's leave",
			machine:     fakeMachine{goos: "windows", upAfter: 3, status: winsvc.StatusStopped, startErr: fs.ErrPermission},
			wantOpened:  1,
			wantStarts:  1,
			wantElevate: 1,
		},
		{
			name:        "leave refused leaves the till closed, and says so",
			machine:     fakeMachine{goos: "windows", upAfter: -1, status: winsvc.StatusStopped, startErr: fs.ErrPermission, elevErr: errors.New("cancelled")},
			wantStarts:  1,
			wantElevate: 1,
			wantAlert:   "needs an administrator's leave",
			wantErr:     "needs an administrator's leave",
		},
		{
			name:       "a service that will not start says why, and where Windows says more",
			machine:    fakeMachine{goos: "windows", upAfter: -1, status: winsvc.StatusStopped, startErr: errors.New("the account cannot log on")},
			wantStarts: 1,
			wantAlert:  "did not start:\n\nthe account cannot log on\n\nEvent Viewer",
			wantErr:    "did not start",
		},
		{
			name:      "no service at all is a message, not a browser on an error page",
			machine:   fakeMachine{goos: "windows", upAfter: -1, status: winsvc.StatusNotInstalled},
			wantAlert: "not set up to start with Windows",
			wantErr:   "not set up to start with Windows",
		},
		{
			name:       "a started server that never answers is given up on after the wait",
			args:       []string{"-wait", "3s"},
			machine:    fakeMachine{goos: "windows", upAfter: -1, status: winsvc.StatusStopped},
			wantStarts: 1,
			wantAlert:  "not answering at http://localhost:7777 after 3s",
			wantErr:    "not answering",
		},
		{
			name:    "on another system a server that is down is an error, with no service to start",
			machine: fakeMachine{goos: "linux", upAfter: -1, status: winsvc.StatusNotInstalled},
			wantErr: "mez is not running at http://localhost:7777",
		},
		{
			name:    "a stray argument is refused",
			args:    []string{"now"},
			machine: fakeMachine{goos: "windows", upAfter: 0},
			wantErr: `unexpected argument "now"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.machine
			err := openCommand(tt.args, io.Discard, m.env())
			if tt.wantErr == "" && err != nil {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if len(m.opened) != tt.wantOpened {
				t.Errorf("opened %v, want %d", m.opened, tt.wantOpened)
			}
			for _, url := range m.opened {
				if url != "http://localhost:7777" {
					t.Errorf("opened %q", url)
				}
			}
			if m.started != tt.wantStarts || m.elevated != tt.wantElevate {
				t.Errorf("started %d times and elevated %d, want %d and %d", m.started, m.elevated, tt.wantStarts, tt.wantElevate)
			}
			switch {
			case tt.wantAlert == "" && len(m.alerts) > 0:
				t.Errorf("alerted: %q", m.alerts)
			case tt.wantAlert != "" && (len(m.alerts) != 1 || !strings.Contains(m.alerts[0], tt.wantAlert)):
				t.Errorf("alerts = %q, want one with %q", m.alerts, tt.wantAlert)
			}
			wantFull := len(tt.args) > 0 && tt.args[0] == "-fullscreen"
			if len(m.opened) > 0 && m.fullscreen != wantFull {
				t.Errorf("fullscreen = %v", m.fullscreen)
			}
		})
	}
}

// TestOpenWaitsNoLongerThanAsked: the wait is the flag's, and the polls are
// half a second apart, so a server up in two seconds is opened in two.
func TestOpenWaitsNoLongerThanAsked(t *testing.T) {
	m := fakeMachine{goos: "windows", upAfter: 4, status: winsvc.StatusRunning}
	if err := openCommand([]string{"-wait", "10s"}, io.Discard, m.env()); err != nil {
		t.Fatal(err)
	}
	if m.slept != 2*time.Second {
		t.Errorf("slept %s, want 2s", m.slept)
	}
	m = fakeMachine{goos: "windows", upAfter: -1, status: winsvc.StatusRunning}
	_ = openCommand([]string{"-wait", "2s"}, io.Discard, m.env())
	if m.slept != 2*time.Second {
		t.Errorf("slept %s for a 2s wait", m.slept)
	}
}

func TestOpenHelp(t *testing.T) {
	var out strings.Builder
	m := fakeMachine{}
	err := openCommand([]string{"-h"}, &out, m.env())
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(out.String(), "mez open") {
		t.Fatalf("err = %v, out = %q", err, out.String())
	}
}
