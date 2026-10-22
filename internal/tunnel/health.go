package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// report sends the venue's health to a relay that asked for it (ADR-361):
// on connecting, then whenever it has changed, and every healthEvery
// whether or not it has, so that a relay hearing nothing knows the venue
// is gone and not merely quiet. It reads the health every healthPoll and
// runs until the connection ends. A health that cannot be read is logged
// and read again next time; the relay is sent nothing wrong.
func (s *session) report() {
	t := time.NewTicker(s.c.healthPoll)
	defer t.Stop()
	var last []byte // what was last sent, less its clock
	var sentAt time.Time
	for {
		ctx, cancel := context.WithTimeout(s.ctx, healthTimeout)
		h, err := s.c.health(ctx)
		cancel()
		if s.ctx.Err() != nil {
			return
		}
		if err != nil {
			s.c.log.Warn("tunnel: the venue's health could not be read; the relay is sent nothing", "error", err.Error())
		} else if key := fingerprint(h); !bytes.Equal(key, last) || s.c.now().Sub(sentAt) >= s.c.healthEvery {
			msg, err := wire.EncodeHealth(h)
			if err != nil {
				// Too many printers to fit a frame, or the like: nothing
				// the next poll will change, and said once a poll.
				s.c.log.Warn("tunnel: the venue's health could not be sent", "error", err.Error())
			} else if s.send(msg) != nil {
				// The line is gone, and so is the session.
				return
			} else {
				last, sentAt = key, s.c.now()
			}
		}
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
		}
	}
}

// fingerprint is a summary less its clock, so that one made a minute
// later of the same state reads as the same. An encoding that fails
// fingerprints as nothing, and is then sent each poll, which Encode refuses
// the same way.
func fingerprint(h wire.Health) []byte {
	h.At = time.Time{}
	b, err := json.Marshal(h)
	if err != nil {
		return nil
	}
	return b
}
