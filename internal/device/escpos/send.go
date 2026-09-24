package escpos

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// DefaultPort is the raw-print port every network receipt printer listens
// on (the "JetDirect" or "port 9100" protocol): bytes in, paper out.
const DefaultPort = "9100"

// sendTimeout bounds a whole send when the caller's context has no
// deadline. A printer on the LAN answers in milliseconds; one that is off
// should fail the send quickly so the queue can retry (Task 5).
const sendTimeout = 10 * time.Second

// Send writes data to the printer at addr, "host" or "host:port", over TCP.
// It returns once the bytes are written and the connection closed. The raw
// protocol has no acknowledgement: a written receipt can still jam, which is
// what the print queue's retry and a person looking at the printer are for.
func Send(ctx context.Context, addr string, data []byte) error {
	target := withPort(addr)
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sendTimeout)
		defer cancel()
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("escpos: connecting to the printer at %s: %w", target, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("escpos: %s: %w", target, err)
		}
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("escpos: sending to the printer at %s: %w", target, err)
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("escpos: closing the connection to %s: %w", target, err)
	}
	return nil
}

// withPort adds the raw-print port to an address that has none.
func withPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(strings.Trim(addr, "[]"), DefaultPort)
}
