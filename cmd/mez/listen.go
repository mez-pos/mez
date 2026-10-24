package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"syscall"
	"time"
)

// How long the server waits for an address of this computer's that it does
// not have yet, and how often it tries. A Windows service starts before the
// network is up, and a listen_addr naming the wifi's own address cannot be
// bound until it is (ADR-366); two minutes outlasts any DHCP lease that is
// coming at all.
const (
	listenPatience = 2 * time.Minute
	listenPause    = 2 * time.Second
)

// bindPatiently binds addr. An address this computer does not have yet is waited
// for, up to patience; anything else, a port already taken above all, is
// refused at once, since waiting would not mend it.
func bindPatiently(ctx context.Context, log *slog.Logger, addr string, patience, pause time.Duration) (net.Listener, error) {
	deadline := time.Now().Add(patience)
	said := false
	for {
		l, err := net.Listen("tcp", addr)
		if err == nil {
			return l, nil
		}
		if !addrNotAvailable(err) || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("listening on %s: %w", addr, err)
		}
		if !said {
			// Once at Warn, which reaches the event log, and no more:
			// a line every two seconds for two minutes says nothing new.
			log.Warn("the listen address is not on this computer yet: waiting for the network", "listen_addr", addr, "error", err)
			said = true
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("listening on %s: %w", addr, ctx.Err())
		case <-time.After(pause):
		}
	}
}

// wsaAddrNotAvailable is Winsock's EADDRNOTAVAIL, "the requested address is
// not valid in its context". Go's syscall names EADDRNOTAVAIL on Windows
// too, but as a number of its own that no bind ever answers with.
const wsaAddrNotAvailable syscall.Errno = 10049

// addrNotAvailable reports whether a bind failed because the address is not
// one of this computer's.
func addrNotAvailable(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == syscall.EADDRNOTAVAIL || errno == wsaAddrNotAvailable)
}
