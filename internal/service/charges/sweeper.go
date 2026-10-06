package charges

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefaultSweepEvery is how often open charges are asked about. A guest who
// comes back to their bill asks at once (4f3); this is for the one who paid
// and closed the page.
const DefaultSweepEvery = 20 * time.Second

// SweepAll settles the open charges of every venue that has any. With none
// open it reads one small query and asks the gateway nothing, so a venue
// with no guest paying online makes no call to the internet. It answers how
// many charges it closed and the first error, having tried them all.
func (s *Service) SweepAll(ctx context.Context) (int, error) {
	venues, err := s.db.Queries().VenuesWithOpenGatewayCharges(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing the venues with open charges: %w", err)
	}
	var first error
	closed := 0
	for _, venueID := range venues {
		n, err := s.Sweep(ctx, venueID)
		closed += n
		if err != nil && first == nil {
			first = err
		}
	}
	return closed, first
}

// Sweeper runs SweepAll on a timer until it is closed. It owns its
// goroutine.
type Sweeper struct {
	s      *Service
	every  time.Duration
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// StartSweeper starts sweeping every so often. A sweep that fails is said
// once in the log, at a level that will not wake anyone: the gateway being
// out of reach is the internet being down, which is not news to a venue,
// and the next sweep tries again. An unapplied charge is the service's own
// error, and loud.
func (s *Service) StartSweeper(every time.Duration) *Sweeper {
	if every <= 0 {
		every = DefaultSweepEvery
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Sweeper{s: s, every: every, cancel: cancel, done: make(chan struct{})}
	go w.loop(ctx)
	return w
}

func (w *Sweeper) loop(ctx context.Context) {
	defer close(w.done)
	tick := time.NewTicker(w.every)
	defer tick.Stop()
	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		// One sweep is bounded, so a gateway that hangs cannot hold the
		// next one off for ever.
		sweep, stop := context.WithTimeout(ctx, 2*w.every+time.Minute)
		closed, err := w.s.SweepAll(sweep)
		stop()
		switch {
		case err != nil && !errors.Is(err, context.Canceled) && !failing:
			failing = true
			w.s.log.Warn("open charges could not all be settled; trying again", "error", err)
		case err == nil && failing:
			failing = false
			w.s.log.Info("open charges are being settled again")
		}
		if closed > 0 {
			w.s.log.Info("the sweep closed charges", "closed", closed)
		}
	}
}

// Close stops the sweeper and waits for a sweep in flight, or for ctx.
func (w *Sweeper) Close(ctx context.Context) error {
	w.once.Do(w.cancel)
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
