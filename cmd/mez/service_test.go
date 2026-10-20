package main

import (
	"errors"
	"flag"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/fsperm"
	"github.com/mez-pos/mez/internal/winsvc"
)

// fakeControl stands in for the Service Control Manager.
type fakeControl struct {
	status winsvc.Status
	fail   error
	calls  []string
}

func (f *fakeControl) do(name string) error {
	f.calls = append(f.calls, name)
	return f.fail
}

func (f *fakeControl) Install() error                 { return f.do("install") }
func (f *fakeControl) Uninstall() error               { return f.do("uninstall") }
func (f *fakeControl) Start() error                   { return f.do("start") }
func (f *fakeControl) Stop() error                    { return f.do("stop") }
func (f *fakeControl) Status() (winsvc.Status, error) { return f.status, nil }

// fakeEnv is a Windows machine with ctl as its service manager. described
// is filled with what the service was described as.
func fakeEnv(t *testing.T, ctl *fakeControl, described *winsvc.Options) serviceEnv {
	t.Helper()
	return serviceEnv{
		goos:   "windows",
		cwd:    t.TempDir(),
		lookup: func(string) (string, bool) { return "", false },
		check:  func([]string) (string, error) { return "the data directory", nil },
		grant:  func(string) error { return nil },
		control: func(o winsvc.Options) (serviceControl, error) {
			if described != nil {
				*described = o
			}
			return ctl, nil
		},
		serve: func([]string) error { return errors.New("the service was run") },
	}
}

func TestServiceActions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		status    winsvc.Status
		wantCalls []string
		wantOut   string
		wantErr   string
	}{
		{"status of a running service", []string{"status"}, winsvc.StatusRunning, nil, "The mez service is running.", ""},
		{"status with none installed", []string{"status"}, winsvc.StatusNotInstalled, nil, "The mez service is not installed.", ""},
		{"start", []string{"start"}, winsvc.StatusStopped, []string{"start"}, "is starting", ""},
		{"start when running asks nothing", []string{"start"}, winsvc.StatusRunning, nil, "already running", ""},
		{"start with none installed", []string{"start"}, winsvc.StatusNotInstalled, nil, "", "not installed"},
		{"stop", []string{"stop"}, winsvc.StatusRunning, []string{"stop"}, "is stopped", ""},
		{"stop when stopped asks nothing", []string{"stop"}, winsvc.StatusStopped, nil, "already stopped", ""},
		// A running service is only marked for removal and keeps the port.
		{"uninstall stops a running service first", []string{"uninstall"}, winsvc.StatusRunning, []string{"stop", "uninstall"}, "data is untouched", ""},
		{"uninstall a stopped service", []string{"uninstall"}, winsvc.StatusStopped, []string{"uninstall"}, "is removed", ""},
		{"uninstall with none installed", []string{"uninstall"}, winsvc.StatusNotInstalled, nil, "", "not installed"},
		{"an argument too many", []string{"stop", "now"}, winsvc.StatusRunning, nil, "", `unexpected argument "now"`},
		{"an action nobody wrote", []string{"restart-twice"}, winsvc.StatusRunning, nil, "", "unknown action"},
		{"no action", nil, winsvc.StatusRunning, nil, "", "say what to do"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctl := &fakeControl{status: tt.status}
			var out strings.Builder

			err := serviceCommand(tt.args, &out, fakeEnv(t, ctl, nil))

			if tt.wantErr == "" && err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want %q in it", err, tt.wantErr)
			}
			if !slices.Equal(ctl.calls, tt.wantCalls) {
				t.Errorf("the manager was asked %v, want %v", ctl.calls, tt.wantCalls)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("printed %q, want %q in it", out.String(), tt.wantOut)
			}
		})
	}
}

// TestServiceInstall: the service starts the server with the flags it was
// installed with, their paths absolute, under the account asked for.
func TestServiceInstall(t *testing.T) {
	ctl := &fakeControl{status: winsvc.StatusNotInstalled}
	var described winsvc.Options
	env := fakeEnv(t, ctl, &described)
	env.lookup = func(key string) (string, bool) {
		if key == servicePasswordEnv {
			return "hunter2", true
		}
		return "", false
	}
	var out strings.Builder

	err := serviceCommand([]string{"install", "-user", `.\cafe`, "-data-dir", "data", "-listen-addr", ":7780"}, &out, env)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	want := []string{"service", "run", "-data-dir", filepath.Join(env.cwd, "data"), "-listen-addr", ":7780"}
	if !slices.Equal(described.Arguments, want) {
		t.Errorf("the service starts with %q, want %q", described.Arguments, want)
	}
	if described.UserName != `.\cafe` || described.Password != "hunter2" {
		t.Errorf("account = %q with password %q, want .\\cafe and the one in the environment", described.UserName, described.Password)
	}
	if !slices.Equal(ctl.calls, []string{"install"}) {
		t.Errorf("the manager was asked %v, want only install", ctl.calls)
	}
	if strings.Contains(out.String(), "hunter2") {
		t.Errorf("printed %q: the password must not be shown", out.String())
	}
	if !strings.Contains(out.String(), `running as .\cafe`) {
		t.Errorf("printed %q, want the account named", out.String())
	}
}

// TestServiceInstallWithNoAccount: the service runs as its own account,
// which has no password (one left in the environment by an earlier install
// is not sent), and is handed the data directory the flags name.
func TestServiceInstallWithNoAccount(t *testing.T) {
	var described winsvc.Options
	env := fakeEnv(t, &fakeControl{}, &described)
	env.lookup = func(string) (string, bool) { return "left-over", true }
	var granted []string
	env.grant = func(dir string) error { granted = append(granted, dir); return nil }
	var out strings.Builder

	if err := serviceCommand([]string{"install"}, &out, env); err != nil {
		t.Fatalf("install: %v", err)
	}
	if described.UserName != "" || described.Password != "" {
		t.Errorf("account = %q, password = %q, want neither: the default is the wrapper's", described.UserName, described.Password)
	}
	if !slices.Equal(granted, []string{"the data directory"}) {
		t.Errorf("granted %q, want the data directory the server would use", granted)
	}
	if !strings.Contains(out.String(), "running as "+winsvc.Account) {
		t.Errorf("printed %q, want the service's own account named", out.String())
	}
}

// TestServiceInstallSaysWhenTheDataIsShut: a service that cannot open its
// database fails every start. The person installing it is told then.
func TestServiceInstallSaysWhenTheDataIsShut(t *testing.T) {
	env := fakeEnv(t, &fakeControl{}, nil)
	env.grant = func(string) error { return errors.New("setting the ACL: access is denied") }
	var out strings.Builder

	err := serviceCommand([]string{"install"}, &out, env)

	if err == nil || !strings.Contains(err.Error(), "may not be able to open its data") {
		t.Errorf("err = %v, want it to say the data may be shut", err)
	}
}

// TestTheDataNamesTheServicesAccount: fsperm restates the service's name
// to keep its imports to itself. If the two part, the service is shut out
// of its own database.
func TestTheDataNamesTheServicesAccount(t *testing.T) {
	if fsperm.ServiceName != winsvc.Name {
		t.Errorf("fsperm.ServiceName = %q, winsvc.Name = %q: they must be one name", fsperm.ServiceName, winsvc.Name)
	}
	if want := `NT SERVICE\` + winsvc.Name; winsvc.Account != want {
		t.Errorf("winsvc.Account = %q, want %q", winsvc.Account, want)
	}
}

// TestServiceInstallRefusesWhatTheServerWould: a service installed with a
// flag the server refuses fails every start, where nobody is looking.
func TestServiceInstallRefusesWhatTheServerWould(t *testing.T) {
	ctl := &fakeControl{}
	env := fakeEnv(t, ctl, nil)
	env.check = func([]string) (string, error) { return "", errors.New("listen_addr: no port") }
	var out strings.Builder

	err := serviceCommand([]string{"install", "-listen-addr", "nowhere"}, &out, env)

	if err == nil || !strings.Contains(err.Error(), "would not start") {
		t.Fatalf("err = %v, want the settings refused", err)
	}
	if len(ctl.calls) != 0 {
		t.Errorf("the manager was asked %v, want nothing", ctl.calls)
	}
}

// TestServiceSaysWhoCanChangeIt: Windows says "Access is denied." and no
// more, to an owner who has never opened a terminal as administrator.
func TestServiceSaysWhoCanChangeIt(t *testing.T) {
	ctl := &fakeControl{status: winsvc.StatusStopped, fail: fs.ErrPermission}
	var out strings.Builder

	err := serviceCommand([]string{"start"}, &out, fakeEnv(t, ctl, nil))

	if err == nil || !strings.Contains(err.Error(), "Run as administrator") {
		t.Errorf("err = %v, want it to say to run as administrator", err)
	}
}

// TestServiceIsForWindows: on Linux the unit file is the service.
func TestServiceIsForWindows(t *testing.T) {
	env := fakeEnv(t, &fakeControl{}, nil)
	env.goos = "linux"
	var out strings.Builder

	err := serviceCommand([]string{"install"}, &out, env)

	if err == nil || !strings.Contains(err.Error(), "systemd") {
		t.Errorf("err = %v, want it to point at systemd", err)
	}
}

// TestServiceHelpAndRun: -h prints the usage, and run is the server.
func TestServiceHelpAndRun(t *testing.T) {
	var out strings.Builder
	if err := serviceCommand([]string{"-h"}, &out, fakeEnv(t, &fakeControl{}, nil)); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h: err = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(out.String(), "mez service install") {
		t.Errorf("-h printed %q, want the usage", out.String())
	}

	err := serviceCommand([]string{"run", "-data-dir", "x"}, &out, fakeEnv(t, &fakeControl{}, nil))
	if err == nil || err.Error() != "the service was run" {
		t.Errorf("run: err = %v, want the server to have been run", err)
	}
}

func TestInstallArgs(t *testing.T) {
	cwd := t.TempDir()
	abs := filepath.Join(cwd, "elsewhere")

	tests := []struct {
		name       string
		args       []string
		wantUser   string
		wantServer []string
		wantErr    string
	}{
		{"nothing", nil, "", nil, ""},
		{"a relative data directory is the terminal's, not system32's", []string{"-data-dir", "data"}, "", []string{"-data-dir", filepath.Join(cwd, "data")}, ""},
		{"an absolute one is left alone", []string{"-data-dir", abs}, "", []string{"-data-dir", abs}, ""},
		{"the equals form", []string{"--config=mez.toml"}, "", []string{"-config", filepath.Join(cwd, "mez.toml")}, ""},
		{"the account is not the server's", []string{"-user=NT AUTHORITY\\LocalService", "-log-level", "debug"}, "NT AUTHORITY\\LocalService", []string{"-log-level", "debug"}, ""},
		{"other flags pass as typed", []string{"-listen-addr=:7780", "-timezone", "Asia/Kolkata"}, "", []string{"-listen-addr=:7780", "-timezone", "Asia/Kolkata"}, ""},
		{"a flag with its value missing", []string{"-user"}, "", nil, "-user needs a value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, server, err := installArgs(tt.args, cwd)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q in it", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if !slices.Equal(server, tt.wantServer) {
				t.Errorf("server flags = %q, want %q", server, tt.wantServer)
			}
		})
	}
}
