package order

import (
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
)

// Adding is what a ticket's total would rise by if lines were added to it:
// the lines, with their tax as the venue's profile works it out on the whole
// bill, and nothing rounded twice (phase 11, Task 4e5a, ADR-225).
//
// It is how a guest is told what to pay before their order is one. It asks
// the projection, the same function that will bill the lines when they are
// added, so the figure asked for and the figure billed are one calculation
// and not two that are meant to agree.
//
// prior is the ticket's stream as it stands. With none, the lines are
// priced as the first on a ticket of their own, opened now in currency at
// the table. A ticket that would refuse the lines (invoiced, on hold,
// closed) refuses here with the projection's own error.
func Adding(prior []Event, lines []LineAdded, taxes Taxes, currency string, tableID int64, at time.Time) (money.Amount, error) {
	if len(lines) == 0 {
		return money.Amount{}, invalid("nothing is being added")
	}
	events := make([]Event, 0, len(prior)+len(lines)+1)
	before := money.Amount{Currency: currency}
	if len(prior) == 0 {
		events = append(events, NewEvent(1, at, 0, 0, OrderOpened{
			Channel: ChannelQRTable, Currency: currency, Service: DefaultService(tableID), TableID: tableID,
		}))
	} else {
		events = append(events, prior...)
		was, err := Project(prior, taxes)
		if err != nil {
			return money.Amount{}, err
		}
		before = was.Totals.Total
	}
	seq := int64(len(events))
	for _, l := range lines {
		seq++
		events = append(events, NewEvent(seq, at, 0, 0, l))
	}
	with, err := Project(events, taxes)
	if err != nil {
		return money.Amount{}, err
	}
	rise, err := with.Totals.Total.Sub(before)
	if err != nil {
		return money.Amount{}, fmt.Errorf("working out what the lines add: %w", err)
	}
	return rise, nil
}
