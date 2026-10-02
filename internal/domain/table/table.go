// Package table is the lifecycle of a table on the floor, from docs/02-domain.md:
//
//	free -> seated -> ordered -> settling -> dirty -> free
//
// table.go is the transition table; turn.go derives where a table is from
// the orders at it (ADR-086). The floor plan itself is internal/service/floor.
package table

import "github.com/byjoellobo/mez/internal/domain/fsm"

// Status is where a table is in its turn.
type Status string

// The table statuses, in the order a turn passes through them.
const (
	StatusFree     Status = "free"     // clean and empty, ready to seat
	StatusSeated   Status = "seated"   // guests sat down, nothing ordered
	StatusOrdered  Status = "ordered"  // at least one order open at the table
	StatusSettling Status = "settling" // the bill is being paid
	StatusDirty    Status = "dirty"    // guests gone, needs clearing
)

// machine is the domain's cycle plus two moves a real floor makes (ADR-047):
// seated -> free, for guests who leave before ordering, and
// settling -> ordered, for the round ordered after asking for the bill.
var machine = fsm.Must("table",
	[]Status{StatusFree, StatusSeated, StatusOrdered, StatusSettling, StatusDirty},
	map[Status][]Status{
		StatusFree:     {StatusSeated},
		StatusSeated:   {StatusOrdered, StatusFree},
		StatusOrdered:  {StatusSettling},
		StatusSettling: {StatusDirty, StatusOrdered},
		StatusDirty:    {StatusFree},
	})

// Transition checks a table status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func Transition(from, to Status) (Status, error) {
	return machine.Transition(from, to)
}

// Machine is the table lifecycle, for listing the moves open to a table.
func Machine() fsm.Machine[Status] { return machine }
