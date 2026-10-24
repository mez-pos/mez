//go:build !windows

package winsvc

import "time"

// PreshutdownTimeout is a Windows matter; named here so the install code
// compiles everywhere.
const PreshutdownTimeout = 60 * time.Second

// interactive is always true where there is no Service Control Manager.
func interactive() bool { return true }

// run runs the server until Ctrl-C: there is no manager to hand it to.
func (s *Service) run() error {
	s.prog.log = consoleLog{}
	return s.runInteractively()
}

// setPreshutdown is a Windows matter.
func setPreshutdown(string, time.Duration) error { return nil }
