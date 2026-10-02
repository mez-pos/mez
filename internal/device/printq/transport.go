package printq

import (
	"context"

	"github.com/byjoellobo/mez/internal/device/escpos"
)

// Transport sends a job's bytes to a printer at address, whose meaning is
// the transport's: "host:port" for TCP, a Windows printer name for the
// spooler. An error wrapping printjob.ErrPermanent is not retried.
type Transport interface {
	Send(ctx context.Context, address string, data []byte) error
}

// TransportFunc adapts a function to a Transport.
type TransportFunc func(ctx context.Context, address string, data []byte) error

// Send implements Transport.
func (f TransportFunc) Send(ctx context.Context, address string, data []byte) error {
	return f(ctx, address, data)
}

// The transports, by the value printers.transport stores.
const (
	TCP     = "tcp"
	Windows = "windows"
)

// DefaultTransports are TCP 9100, the primary, and the Windows RAW spooler,
// the fallback for a USB printer. The spooler is in every build so a job
// for it fails the same way everywhere; off Windows it fails permanently.
func DefaultTransports() map[string]Transport {
	return map[string]Transport{
		TCP:     TransportFunc(escpos.Send),
		Windows: TransportFunc(sendSpooler),
	}
}
