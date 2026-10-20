// Package kitchen is the lifecycle of a ticket at a station, from
// docs/02-domain.md:
//
//	queued -> fired -> in progress -> ready -> collected
//
// It is only the transition table. The kitchen display (phase 08, Task 4,
// ADR-090) adds the two moves a pass makes beyond the domain's line:
//
//	fired -> ready     bumped straight off the screen: a display has no
//	                   "picked up" tap, and a ticket is cooked while shown
//	ready -> fired     recalled: bumped by mistake, back on the screen
package kitchen

import "github.com/mez-pos/mez/internal/domain/fsm"

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

// machine is the domain's line, plus the display's bump and recall.
var machine = fsm.Must("ticket",
	[]TicketStatus{TicketQueued, TicketFired, TicketInProgress, TicketReady, TicketCollected},
	map[TicketStatus][]TicketStatus{
		TicketQueued:     {TicketFired},
		TicketFired:      {TicketInProgress, TicketReady},
		TicketInProgress: {TicketReady},
		TicketReady:      {TicketCollected, TicketFired},
	})

// Transition checks a ticket status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func Transition(from, to TicketStatus) (TicketStatus, error) {
	return machine.Transition(from, to)
}

// Machine is the ticket lifecycle, for listing the moves open to a ticket.
func Machine() fsm.Machine[TicketStatus] { return machine }
