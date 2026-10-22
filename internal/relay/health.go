package relay

import (
	"context"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// HealthSink receives what a venue says of its own health (ADR-361): its
// printers, its backup, its open shift and its opening hours, sent on the
// socket it already holds, on connecting, every few minutes and on a
// change. Whoever keeps the entitlements may also be one: a relay whose
// Entitlements is a HealthSink tells each venue on its dial that it reads
// health, and hands it what arrives. A relay whose Entitlements is not,
// the static file among them, asks for none and drops any sent.
//
// It is called on the venue's own goroutine, one summary at a time, with
// the newest in place of any it has not taken yet: a sink that is slow
// loses the middle, never the latest. An error is logged and that is all;
// the venue is not told and the socket is not touched.
type HealthSink interface {
	VenueHealth(ctx context.Context, venueID string, h wire.Health) error
}

// took keeps a health frame the venue sent, for the sink, in place of one
// not yet taken. A relay that did not ask for health drops it: a venue is
// not disconnected over a frame the relay merely did not want.
func (a *agent) took(f wire.Frame) {
	if a.health == nil {
		return
	}
	h, err := wire.DecodeHealth(f)
	if err != nil {
		// News, and not worth the line: the next one comes within minutes.
		a.log.Warn("relay: a venue's health could not be read; passed over", "error", err.Error())
		return
	}
	select {
	case <-a.health:
	default:
	}
	select {
	case a.health <- h:
	default:
	}
}

// pour hands the sink each summary the venue sends, until the socket ends.
func (s *Server) pour(a *agent) {
	for {
		select {
		case <-a.ctx.Done():
			return
		case h := <-a.health:
			ctx, cancel := context.WithTimeout(a.ctx, askTimeout)
			err := s.health.VenueHealth(ctx, a.venueID, h)
			cancel()
			if err != nil && a.ctx.Err() == nil {
				a.log.Error("relay: a venue's health could not be kept", "error", err.Error())
			}
		}
	}
}
