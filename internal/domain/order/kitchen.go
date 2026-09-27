package order

import (
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/kitchen"
)

// A ticket at its station (phase 08, Task 4, ADR-090). A ticket.sent puts
// it in the kitchen, fired: being made. The display bumps it to ready when
// it goes on the pass, and recalls it if the bump was a mistake. Both are
// order events, so a display that dropped off catches up from the stream
// like any other device. The kitchen works after the bill: a cafe that is
// paid at the counter still makes the coffee, so neither is refused on a
// closed or held order.

// ErrTicketNotFound means a bump or recall names a ticket the order does
// not have at that station.
var ErrTicketNotFound = errors.New("no such ticket at that station")

// TicketBumped moves a ticket to ready: it is on the pass.
type TicketBumped struct {
	TicketSeq int64  `json:"ticket_seq"`
	Station   string `json:"station"`
}

// TicketRecalled moves a bumped ticket back into the kitchen.
type TicketRecalled struct {
	TicketSeq int64  `json:"ticket_seq"`
	Station   string `json:"station"`
}

func (TicketBumped) EventType() Type   { return TypeTicketBumped }
func (TicketRecalled) EventType() Type { return TypeTicketRecalled }

// kitchenEvent reports the events a closed or held order still takes: the
// kitchen finishing what it was sent.
func kitchenEvent(t Type) bool {
	return t == TypeTicketBumped || t == TypeTicketRecalled
}

func (p *projector) bump(e Event, pl TicketBumped) error {
	t, err := p.ticket(pl.TicketSeq, pl.Station)
	if err != nil {
		return err
	}
	if t.Status, err = kitchen.Transition(t.Status, kitchen.TicketReady); err != nil {
		return err
	}
	t.ReadyAt = e.At.UTC()
	return nil
}

func (p *projector) recall(pl TicketRecalled) error {
	t, err := p.ticket(pl.TicketSeq, pl.Station)
	if err != nil {
		return err
	}
	if t.Status, err = kitchen.Transition(t.Status, kitchen.TicketFired); err != nil {
		return err
	}
	t.ReadyAt = time.Time{}
	return nil
}

func (p *projector) ticket(seq int64, station string) (*Ticket, error) {
	for i := range p.s.Tickets {
		if t := &p.s.Tickets[i]; t.Seq == seq {
			if t.Station != station {
				return nil, fmt.Errorf("%w: ticket %d is %q's, not %q's", ErrTicketNotFound, seq, t.Station, station)
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("%w: %d", ErrTicketNotFound, seq)
}
