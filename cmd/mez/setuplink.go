package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"

	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/logging"
	"github.com/mez-pos/mez/internal/service/firstrun"
)

// runSetupLink is `mez setup-link`: the address that opens mez in a browser
// on this machine, with the setup token in it while the venue is still to
// be made (ADR-270).
//
// It prints one line and nothing else, because the installer reads it and
// opens what it says. Being able to run it is the proof the token stands
// for: it reads a file only an administrator and the service's account can.
// With a venue already there, or a server that has not started yet, there
// is no token and the address is the plain one.
func runSetupLink(args []string, stdout io.Writer) error {
	var dataDir string
	set := flag.NewFlagSet("mez setup-link", flag.ContinueOnError)
	set.SetOutput(stdout)
	set.StringVar(&dataDir, "data-dir", "", "directory holding the database (default: as the server resolves it)")
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() > 0 {
		return fmt.Errorf("setup-link: unexpected argument %q", set.Arg(0))
	}
	var shared []string
	if dataDir != "" {
		shared = append(shared, "-data-dir", dataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	base, err := localURL(cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("setup-link: %w", err)
	}
	token, err := firstrun.ReadToken(cfg.DataDir)
	switch {
	case errors.Is(err, firstrun.ErrNoToken):
		fmt.Fprintln(stdout, base)
	case err != nil:
		return fmt.Errorf("setup-link: %w", err)
	default:
		fmt.Fprintln(stdout, firstrun.SetupLink(base, token))
	}
	return nil
}

// localURL is where a browser on this machine opens a server listening on
// listenAddr. It is always localhost: the wizard is refused at any other
// name (ADR-266).
func localURL(listenAddr string) (string, error) {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("listen address %q: %w", listenAddr, err)
	}
	if port == "80" {
		return "http://localhost", nil
	}
	return "http://" + net.JoinHostPort("localhost", port), nil
}

// announceSetupLink tells the person who started the server in a terminal
// where to set the venue up. Only a terminal: under a service there is
// nobody reading, and written to a file or a container's log the token
// would be kept where the data directory's permissions do not reach.
func announceSetupLink(stdout io.Writer, cfg config.Config, token *firstrun.Token) {
	if !logging.IsTerminal(stdout) {
		return
	}
	base, err := localURL(cfg.ListenAddr)
	if err != nil {
		return
	}
	value, err := firstrun.ReadToken(cfg.DataDir)
	if err != nil || !token.Valid(value) {
		return
	}
	fmt.Fprintf(stdout, "\nThis machine has no venue yet. To set it up, open this in a browser on this computer:\n\n  %s\n\n", firstrun.SetupLink(base, value))
}
