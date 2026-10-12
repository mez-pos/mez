package main

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/restore"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
	"github.com/byjoellobo/mez/internal/service/cloudrecover"
)

// errStartAgain is run returning because it was asked to start again, and
// not because it was asked to stop or could not go on.
var errStartAgain = errors.New("the server is to start again")

// runUntilStopped is the server for as long as the process is wanted: run,
// and run again each time it asks to be (ADR-334). A venue brought back by
// "I had mez before" is put in place by a start, so the server gives itself
// one; whoever started mez, a person at a terminal or the Service Control
// Manager, sees one server that never stopped.
//
// Every start is a whole one, from the settings file down: run unwinds all
// it began before it returns, and is entered afresh. Only if it unwound
// cleanly: a start that could not close its database is not followed by
// another in the same process, which would find that database still held.
// The process ends with what went wrong, and whatever starts mez next
// adopts the venue that is waiting.
func runUntilStopped(ctx context.Context, args []string, opts runOptions) error {
	for {
		err := run(ctx, args, opts)
		if !errors.Is(err, errStartAgain) {
			return err
		}
		// Asked to stop as it was starting again: stopping wins.
		if rest := besidesStartAgain(err); rest != nil || ctx.Err() != nil {
			return rest
		}
	}
}

// besidesStartAgain is what else an error from run says, when it says the
// server is to start again: the errors its shutdown joined to that, or nil.
func besidesStartAgain(err error) error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		if err == errStartAgain {
			return nil
		}
		return err
	}
	var rest []error
	for _, e := range joined.Unwrap() {
		if r := besidesStartAgain(e); r != nil {
			rest = append(rest, r)
		}
	}
	return errors.Join(rest...)
}

// served is what one start of the server serves under.
type served struct {
	// ctx ends when the process is asked to stop, or this start is asked
	// to start again.
	ctx   context.Context
	stop  context.CancelFunc
	asked *atomic.Bool
}

// again reports whether this start ended by being asked to start again,
// with the process itself (whose context is given) still wanted.
func (s served) again(process context.Context) bool {
	return s.asked.Load() && process.Err() == nil
}

// restartable makes what a start serves under, and the way to ask it to
// start again. Asking returns at once and may be done from inside a request:
// the server finishes what it is answering before it stops (serve).
func restartable(process context.Context) (served, func()) {
	ctx, stop := context.WithCancel(process)
	s := served{ctx: ctx, stop: stop, asked: new(atomic.Bool)}
	return s, func() {
		s.asked.Store(true)
		stop()
	}
}

// arrive begins what follows from this start having adopted a venue: its
// pairing, and the word for the page that asked (cloudrecover.Arrival). Nil
// is every other start. Nothing here stops a start: the venue is in place,
// and what cannot be done about it is said in the log.
func arrive(ctx context.Context, database *db.DB, adopted *restore.Pending,
	pair func(context.Context, int64, string) (cloudpair.Result, error), devices *auth.Devices, log *slog.Logger) *cloudrecover.Arrival {
	if adopted == nil {
		return nil
	}
	venue, err := database.Queries().VenueByPublicID(ctx, adopted.VenueID)
	if err != nil {
		log.Warn("a restored venue was put in place, and is not in the database that was: it is not paired, and the page that asked is told nothing",
			"restore", *adopted, "error", err)
		return nil
	}
	arrival, err := cloudrecover.Arrive(cloudrecover.ArrivalOptions{Adopted: *adopted, VenueID: venue.ID, Pair: pair, Codes: devices, Log: log})
	if err != nil {
		log.Warn("a restored venue was put in place, and what follows could not be begun", "restore", *adopted, "error", err)
		return nil
	}
	return arrival
}
