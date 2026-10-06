package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/winsvc"
)

// servicePasswordEnv carries the password of the account named by -user. It
// is read from the environment and not from a flag: a command line is kept
// in the shell's history and shown to every user in the task list.
const servicePasswordEnv = "MEZ_SERVICE_PASSWORD"

const serviceUsage = `mez service: run mez as a Windows service

  mez service install [-user ACCOUNT] [server flags]
        register the service: it starts with Windows and is started again
        if it dies. The server flags (-data-dir, -config, -listen-addr ...)
        are the ones mez itself takes, and the service starts with them.
        With no -user it runs as LocalSystem. With one, the account's
        password is read from ` + servicePasswordEnv + `.
  mez service uninstall   stop the service and remove it; the data stays
  mez service start       start it now
  mez service stop        stop it, letting requests in flight finish
  mez service status      say whether it is installed and running

install, uninstall, start and stop need an Administrator terminal.
`

// serviceControl is the part of a winsvc.Service the subcommands drive, so
// the tests can stand in for the Service Control Manager.
type serviceControl interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (winsvc.Status, error)
}

// serviceEnv is what `mez service` reads from the machine.
type serviceEnv struct {
	goos   string
	cwd    string
	lookup func(string) (string, bool)
	// check is the server's own reading of its flags.
	check func(args []string) error
	// control describes the service to the manager without touching it.
	control func(winsvc.Options) (serviceControl, error)
	// serve is the service itself: what the manager starts.
	serve func(args []string) error
}

// runService is `mez service`: the server registered with Windows, so that
// a venue's computer sells after a power cut with nobody signed in.
func runService(args []string, stdout io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("service: finding the current directory: %w", err)
	}
	return serviceCommand(args, stdout, serviceEnv{
		goos:   runtime.GOOS,
		cwd:    cwd,
		lookup: os.LookupEnv,
		check: func(args []string) error {
			_, err := config.LoadArgs(args)
			return err
		},
		control: func(o winsvc.Options) (serviceControl, error) { return winsvc.New(o) },
		serve:   serveAsService,
	})
}

// serviceCommand is runService with the machine handed in.
func serviceCommand(args []string, stdout io.Writer, env serviceEnv) error {
	if len(args) == 0 {
		return errors.New("service: say what to do: install, uninstall, start, stop or status (mez service -h)")
	}
	action, rest := args[0], args[1:]
	if action == "-h" || action == "--help" || action == "help" {
		fmt.Fprint(stdout, serviceUsage)
		return flag.ErrHelp
	}
	// On Linux the unit file in docs/deploy.md is the service, with the
	// account and the hardening written into it. One the library generated
	// would have neither.
	if env.goos != "windows" {
		return fmt.Errorf("service: this is for Windows; on %s run mez under systemd, as docs/deploy.md describes", env.goos)
	}

	switch action {
	case "run":
		return env.serve(rest)
	case "install":
		return installService(rest, stdout, env)
	case "uninstall", "start", "stop", "status":
		if len(rest) > 0 {
			return fmt.Errorf("service %s: unexpected argument %q", action, rest[0])
		}
		svc, err := env.control(winsvc.Options{})
		if err != nil {
			return err
		}
		return controlService(action, svc, stdout)
	default:
		return fmt.Errorf("service: unknown action %q: install, uninstall, start, stop or status", action)
	}
}

// installService registers the service to start with the flags given.
func installService(args []string, stdout io.Writer, env serviceEnv) error {
	user, server, err := installArgs(args, env.cwd)
	if err != nil {
		return err
	}
	// Flags the server would refuse are refused here, to the person typing
	// them. Installed, they would fail every start, out of sight.
	if err := env.check(server); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, serviceUsage)
			return err
		}
		return fmt.Errorf("service install: the server would not start with these settings: %w", err)
	}

	opts := winsvc.Options{Arguments: append([]string{"service", "run"}, server...), UserName: user}
	if user != "" {
		// An account with no password (LocalService, a virtual account)
		// leaves it unset.
		opts.Password, _ = env.lookup(servicePasswordEnv)
	}
	svc, err := env.control(opts)
	if err != nil {
		return err
	}
	if err := svc.Install(); err != nil {
		return needsAdmin(err)
	}

	account := "LocalSystem"
	if user != "" {
		account = user
	}
	fmt.Fprintf(stdout, "  The mez service is installed, running as %s.\n", account)
	fmt.Fprintln(stdout, "  It starts with Windows. Start it now with: mez service start")
	return nil
}

// controlService does one thing to a service already described.
func controlService(action string, svc serviceControl, stdout io.Writer) error {
	status, err := svc.Status()
	if err != nil {
		return err
	}
	if status == winsvc.StatusNotInstalled && action != "status" {
		return errors.New("service " + action + ": the mez service is not installed: run `mez service install` first")
	}

	switch action {
	case "status":
		fmt.Fprintf(stdout, "  The mez service is %s.\n", status)
	case "start":
		if status == winsvc.StatusRunning {
			fmt.Fprintln(stdout, "  The mez service is already running.")
			return nil
		}
		if err := svc.Start(); err != nil {
			return needsAdmin(err)
		}
		fmt.Fprintln(stdout, "  The mez service is starting. If the till does not open, look in the Windows event log under mez.")
	case "stop":
		if status == winsvc.StatusStopped {
			fmt.Fprintln(stdout, "  The mez service is already stopped.")
			return nil
		}
		if err := svc.Stop(); err != nil {
			return needsAdmin(err)
		}
		fmt.Fprintln(stdout, "  The mez service is stopped.")
	case "uninstall":
		// A running service is only marked for removal, and keeps the port
		// until the next restart of the computer.
		if status == winsvc.StatusRunning {
			if err := svc.Stop(); err != nil {
				return needsAdmin(err)
			}
		}
		if err := svc.Uninstall(); err != nil {
			return needsAdmin(err)
		}
		fmt.Fprintln(stdout, "  The mez service is removed. The venue's data is untouched.")
	}
	return nil
}

// needsAdmin says what to do about a refusal: every change to a service is
// an administrator's, and Windows says only "Access is denied."
func needsAdmin(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w (open the terminal with Run as administrator and try again)", err)
	}
	return err
}

// installArgs splits what `mez service install` was given into the account
// to run as and the flags the service starts the server with.
//
// The paths among them are made absolute against cwd. A service starts in
// system32, so "-data-dir data" typed in C:\mez would otherwise put the
// venue's sales in C:\Windows\System32\data.
func installArgs(args []string, cwd string) (user string, server []string, err error) {
	for i := 0; i < len(args); i++ {
		name, value, hasValue := splitFlag(args[i])
		switch name {
		case "user", "config", "data-dir":
		default:
			server = append(server, args[i])
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("service install: -%s needs a value", name)
			}
			i++
			value = args[i]
		}
		if name == "user" {
			user = value
			continue
		}
		if value != "" && !filepath.IsAbs(value) {
			value = filepath.Join(cwd, value)
		}
		server = append(server, "-"+name, value)
	}
	return user, server, nil
}

// splitFlag reads "-name", "--name" and "-name=value". An argument that is
// not a flag has no name.
func splitFlag(arg string) (name, value string, hasValue bool) {
	if !strings.HasPrefix(arg, "-") {
		return "", "", false
	}
	name = strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
	name, value, hasValue = strings.Cut(name, "=")
	return name, value, hasValue
}

// serveAsService is `mez service run`, what the Service Control Manager
// starts: the same server, stopped by the manager and not by Ctrl-C.
func serveAsService(args []string) error {
	svc, err := winsvc.New(winsvc.Options{
		Run: func(ctx context.Context, system winsvc.SystemLog) error {
			// Under the manager there is no console: a write to stdout
			// fails, and the file is written after it. Started by hand
			// there is one, and no event log.
			if system == nil {
				return run(ctx, args, runOptions{stdout: os.Stdout})
			}
			return run(ctx, args, runOptions{stdout: io.Discard, system: system})
		},
	})
	if err != nil {
		return err
	}
	return svc.Run()
}
