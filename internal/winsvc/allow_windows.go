//go:build windows

package winsvc

import (
	"fmt"
	"os/exec"
	"strings"
)

// AllowStartByUsers lets every signed-in user start the service, and ask
// after it, so that the desktop shortcut can start a stopped mez without an
// administrator (ADR-366). It needs an administrator itself: it is for the
// installer, right after Install.
//
// The descriptor is read and written through sc.exe, which every Windows
// has: the service library has no door to a service's security.
func AllowStartByUsers() error {
	shown, err := exec.Command("sc.exe", "sdshow", Name).Output()
	if err != nil {
		return fmt.Errorf("reading the service's security: %w: %s", err, strings.TrimSpace(string(shown)))
	}
	current := strings.TrimSpace(string(shown))
	wanted := withStartByUsers(current)
	if wanted == current {
		return nil
	}
	if out, err := exec.Command("sc.exe", "sdset", Name, wanted).CombinedOutput(); err != nil {
		return fmt.Errorf("letting users start the service: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
