//go:build windows

package announce

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// shareAddress lets this socket bind a port another has: on Windows,
// SO_REUSEADDR is what the system's own mDNS responder and Bonjour use for
// 5353, and a second binding without it is refused.
func shareAddress(_, _ string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		opErr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return opErr
}
