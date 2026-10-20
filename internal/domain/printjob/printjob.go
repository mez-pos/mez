// Package printjob is the life of a print job (phase 07, Task 5): the states
// a job moves through, how long to wait before trying a printer again, and
// when to stop trying. It is pure: the queue in internal/device/printq does
// the IO and asks this package what happens next.
//
// Printers jam mid-service, and a receipt that never printed is an invoice
// the guest never got. So a job is never dropped: it is done, still being
// tried, or dead, waiting in plain view for a person to retry it.
package printjob

import (
	"errors"
	"time"

	"github.com/mez-pos/mez/internal/domain/fsm"
)

// State is where a job is. The string is what print_jobs.state stores.
type State string

// The job states.
const (
	Queued   State = "queued"   // waiting for its printer
	Printing State = "printing" // being sent now
	Done     State = "done"     // the printer took every byte
	Failed   State = "failed"   // the last send failed; it will be tried again
	Dead     State = "dead"     // no more automatic tries; a person must retry or cancel it
)

var machine = fsm.Must("print job",
	[]State{Queued, Printing, Done, Failed, Dead},
	map[State][]State{
		Queued:   {Printing, Dead},         // dead at once: no printer to send it to
		Printing: {Done, Failed, Dead},     // the outcome of one send
		Failed:   {Printing, Queued, Dead}, // retried by the queue, by a person, or given up
		Dead:     {Queued},                 // a person's retry
	})

// Machine is the job state machine, for tests and for anything that must
// enumerate the states.
func Machine() fsm.Machine[State] { return machine }

// Transition checks a move between states.
func Transition(from, to State) (State, error) { return machine.Transition(from, to) }

// Kind is what a job prints. The string is what print_jobs.kind stores.
type Kind string

// The kinds of job.
const (
	Receipt Kind = "receipt"
	Kitchen Kind = "kitchen"
	ZReport Kind = "z_report"
	Test    Kind = "test"
)

// Kinds is every kind.
func Kinds() []Kind { return []Kind{Receipt, Kitchen, ZReport, Test} }

// The retry schedule: 2s, 4s, 8s ... doubling to a five-minute cap, and ten
// tries in all. The nine waits add up to 13.5 minutes: long enough to
// clear a jam or swap a roll mid-service, short enough that a printer that
// is truly gone leaves its jobs dead on screen before the next table's bill.
const (
	firstWait   = 2 * time.Second
	maxWait     = 5 * time.Minute
	MaxAttempts = 10
)

// Backoff is how long to wait after the attempt-th failed send, counting
// from 1.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		return firstWait
	}
	wait := firstWait
	for i := 1; i < attempt && wait < maxWait; i++ {
		wait *= 2
	}
	return min(wait, maxWait)
}

// ErrPermanent marks a send that no retry can fix: a transport this build
// does not have, a printer retired from the venue. The job goes dead at
// once instead of spending its tries.
var ErrPermanent = errors.New("will not succeed on a retry")

// Outcome is what follows one send.
type Outcome struct {
	State State
	Next  time.Time // when to try again; zero unless Failed
	Error string    // the reason, kept on the job for the screen; empty if Done
}

// After decides what follows the attempt-th send, finished at now with
// err. It never returns Printing or Queued.
func After(attempt int, err error, now time.Time) Outcome {
	switch {
	case err == nil:
		return Outcome{State: Done}
	case errors.Is(err, ErrPermanent) || attempt >= MaxAttempts:
		return Outcome{State: Dead, Error: err.Error()}
	default:
		return Outcome{State: Failed, Next: now.Add(Backoff(attempt)), Error: err.Error()}
	}
}
