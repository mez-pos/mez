//go:build !windows

package announce

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// shareAddress lets this socket bind a port another has: SO_REUSEADDR, and
// where the system has it SO_REUSEPORT, which Avahi and mDNSResponder set on
// their own 5353 sockets.
func shareAddress(_, _ string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		if opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); opErr != nil {
			return
		}
		opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return opErr
}
