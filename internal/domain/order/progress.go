package order

import (
	"slices"

	"github.com/byjoellobo/mez/internal/domain/kitchen"
)

// Progress is how far some lines of an order have come in the kitchen, as a
// guest would be told (phase 11, Task 5a, ADR-236). It is read off the
// tickets the lines were sent on, which move only when someone in the
// kitchen does something: nothing here is a guess or a timer.
type Progress string

// The steps, in the order an order passes through them. The strings are
// what is sent.
const (
	// ProgressAccepted: the lines are on the bill, and nothing of them is
	// with a station yet: held for a later course, or made at no station.
	ProgressAccepted Progress = "accepted"
	// ProgressSent: with the kitchen, and nobody has started on it.
	ProgressSent Progress = "sent"
	// ProgressPreparing: someone has started on it, or part of it is ready
	// and part is not.
	ProgressPreparing Progress = "preparing"
	// ProgressReady: all of it is on the pass or with the guest.
	ProgressReady Progress = "ready"
	// ProgressCancelled: every line was voided.
	ProgressCancelled Progress = "cancelled"
)

// Progresses is every step, for the generated vocabulary.
func Progresses() []Progress {
	return []Progress{ProgressAccepted, ProgressSent, ProgressPreparing, ProgressReady, ProgressCancelled}
}

// Progress is how far the given lines have come. Lines not on the order are
// ignored, and so is a voided line unless every one of them is voided. With
// no line of them on the order at all, the answer is empty.
//
// A ticket counts if any of the lines still to be made is on it. A ticket
// carries one send to one station, so a round sent to the bar and the
// kitchen is ready when both are.
func (s State) Progress(lineIDs []int64) Progress {
	var live []int64
	found := false
	for _, l := range s.Lines {
		if !slices.Contains(lineIDs, l.ID) {
			continue
		}
		found = true
		if l.Status != LineStatusVoided {
			live = append(live, l.ID)
		}
	}
	if !found {
		return ""
	}
	if len(live) == 0 {
		return ProgressCancelled
	}

	var fired, started, ready int
	for _, t := range s.Tickets {
		if !slices.ContainsFunc(t.LineIDs, func(id int64) bool { return slices.Contains(live, id) }) {
			continue
		}
		switch t.Status {
		case kitchen.TicketFired:
			fired++
		case kitchen.TicketInProgress:
			started++
		case kitchen.TicketReady, kitchen.TicketCollected:
			ready++
		}
	}
	switch {
	case started > 0 || (ready > 0 && fired > 0):
		return ProgressPreparing
	case ready > 0:
		return ProgressReady
	case fired > 0:
		return ProgressSent
	default:
		return ProgressAccepted
	}
}
