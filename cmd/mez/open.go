package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"runtime"
	"time"

	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/winsvc"
)

const openUsage = `mez open: open the till on this computer, starting mez if it is not running

  mez open [-fullscreen] [-wait 45s] [-config FILE]

It is what the desktop shortcut runs (ADR-366). If the server answers, the
till opens in a window of its own. If it does not, and mez is installed as
a Windows service, the service is started and the till opens once it
answers. Only when neither can be done is a message shown saying why.
`

// openWait is how long the till waits for a started server to answer. A
// cold start on a slow disk takes a few seconds; a database being migrated
// after an upgrade can take longer.
const openWait = 45 * time.Second

// openPoll is how often the server is asked while the till waits.
const openPoll = 500 * time.Millisecond

// healthTimeout bounds one question to the server. The server is on this
// computer: an answer that takes longer is no answer.
const healthTimeout = 1500 * time.Millisecond

// eventLogHint is where a Windows service says what went wrong.
const eventLogHint = `Event Viewer, under Windows Logs, Application, source "mez", says in words why.`

// openEnv is what "mez open" does to the machine, so that the tests can
// stand in for the browser, the Service Control Manager and the clock.
type openEnv struct {
	goos string
	// healthy asks the server at base whether it is up.
	healthy func(base string) bool
	// status says whether the Windows service is installed and running.
	status func() (winsvc.Status, error)
	// start asks the Service Control Manager to start it, as this user.
	start func() error
	// elevate asks the person for an administrator's leave and starts it
	// with that, for a service this user may not start.
	elevate func() error
	// browse opens the till at url: a window of its own, or full screen.
	browse func(url string, fullscreen bool) error
	// alert shows the person a box they must dismiss. From a shortcut
	// there is no terminal to read an error in.
	alert func(text string)
	sleep func(time.Duration)
	now   func() time.Time
}

// runOpen is "mez open".
func runOpen(args []string, stdout io.Writer) error {
	return openCommand(args, stdout, openEnv{
		goos:    runtime.GOOS,
		healthy: serverHealthy,
		status: func() (winsvc.Status, error) {
			svc, err := winsvc.New(winsvc.Options{})
			if err != nil {
				return "", err
			}
			return svc.Status()
		},
		start: func() error {
			svc, err := winsvc.New(winsvc.Options{})
			if err != nil {
				return err
			}
			return svc.Start()
		},
		elevate: elevateServiceStart,
		browse:  openBrowser,
		alert:   alertBox,
		sleep:   time.Sleep,
		now:     time.Now,
	})
}

// openCommand is runOpen with the machine handed in.
func openCommand(args []string, stdout io.Writer, env openEnv) error {
	var (
		fullscreen bool
		wait       time.Duration
		configPath string
	)
	set := flag.NewFlagSet("mez open", flag.ContinueOnError)
	set.SetOutput(stdout)
	set.BoolVar(&fullscreen, "fullscreen", false, "open the till full screen, with no window buttons (Alt+F4 closes it)")
	set.DurationVar(&wait, "wait", openWait, "how long to wait for a started server to answer")
	set.StringVar(&configPath, "config", "", "path to "+config.FileName+" (default: beside the binary)")
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, openUsage)
		}
		return err
	}
	if set.NArg() > 0 {
		return fmt.Errorf("open: unexpected argument %q", set.Arg(0))
	}
	var shared []string
	if configPath != "" {
		shared = append(shared, "-config", configPath)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	base, err := localURL(cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	if env.healthy(base) {
		return env.browse(base, fullscreen)
	}
	if env.goos != "windows" {
		return fmt.Errorf("open: mez is not running at %s: start it with `mez`, or as docs/deploy.md describes", base)
	}

	status, err := env.status()
	if err != nil {
		return env.complain(fmt.Sprintf("mez is not running, and whether it is installed could not be told: %v", err))
	}
	switch status {
	case winsvc.StatusNotInstalled:
		return env.complain("mez is not running on this computer, and it is not set up to start with Windows.\n\n" +
			"Run the mez installer again to set it up. Until then, mez runs only while the window it was started in stays open.")
	case winsvc.StatusStopped:
		fmt.Fprintln(stdout, "  mez is not running. Starting it...")
		if err := env.start(); err != nil {
			if !errors.Is(err, fs.ErrPermission) {
				return env.complain(fmt.Sprintf("mez is installed, but did not start:\n\n%v\n\n%s", err, eventLogHint))
			}
			// Not this user's to start: an administrator's, with their
			// leave. Saying no leaves the till closed.
			if err := env.elevate(); err != nil {
				return env.complain("mez is installed but stopped, and starting it needs an administrator's leave.\n\n" +
					"Try the shortcut again and choose Yes, or ask whoever set this computer up to start the mez service.")
			}
		}
	}

	// Started, or starting on its own after the computer was turned on:
	// give it time before saying it failed.
	deadline := env.now().Add(wait)
	for env.now().Before(deadline) {
		env.sleep(openPoll)
		if env.healthy(base) {
			return env.browse(base, fullscreen)
		}
	}
	return env.complain(fmt.Sprintf("mez was started but is not answering at %s after %s.\n\n%s\n\n"+
		"If the port was changed in %s, open the address it names instead.", base, wait.Round(time.Second), eventLogHint, config.FileName))
}

// complain shows text to the person and answers it as the error too, for a
// terminal that is reading.
func (env openEnv) complain(text string) error {
	env.alert(text)
	return errors.New("open: " + text)
}

// serverHealthy asks the server at base for its health.
func serverHealthy(base string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/health", nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close() //nolint:errcheck // nothing to do if it fails
	return res.StatusCode == http.StatusOK
}
