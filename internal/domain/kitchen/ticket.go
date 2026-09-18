// Package kitchen is the lifecycle of a ticket at a station, from
// docs/02-domain.md:
//
//	queued -> fired -> in progress -> ready -> collected
//
// It is only the transition table. Station routing and the kitchen display
// arrive with phase 08, and with them any recall or cancel moves the display
// turns out to need; this table deliberately has none yet.
package kitchen

import "github.com/byjoellobo/nofiltr-pos/internal/domain/fsm"

// TicketStatus is where a ticket is at its station.
type TicketStatus string

// The ticket statuses, in the order a ticket passes through them.
const (
	TicketQueued     TicketStatus = "queued"      // at the station, waiting for its course
	TicketFired      TicketStatus = "fired"       // its course was fired: start it
	TicketInProgress TicketStatus = "in_progress" // a cook has picked it up
	TicketReady      TicketStatus = "ready"       // on the pass
	TicketCollected  TicketStatus = "collected"   // taken to the guest
)

// machine is a straight line: each state moves only to the next.
var machine = fsm.Must("ticket",
	[]TicketStatus{TicketQueued, TicketFired, TicketInProgress, TicketReady, TicketCollected},
	map[TicketStatus][]TicketStatus{
		TicketQueued:     {TicketFired},
		TicketFired:      {TicketInProgress},
		TicketInProgress: {TicketReady},
		TicketReady:      {TicketCollected},
	})

// Transition checks a ticket status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func Transition(from, to TicketStatus) (TicketStatus, error) {
	return machine.Transition(from, to)
}

// Machine is the ticket lifecycle, for listing the moves open to a ticket.
func Machine() fsm.Machine[TicketStatus] { return machine }
