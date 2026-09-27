package order

import (
	"slices"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// ProjectionVersion names the rules Project folds by and the shape of State.
// A stored snapshot records the version it was projected under, and one from
// any other version is ignored and rebuilt from events.
//
// Bump it in the same commit as any change that makes an old State wrong for
// the same events: a new field, a changed rule, a changed rounding. Leaving it
// alone after such a change serves stale snapshots until they happen to be
// retaken, which is the one way a snapshot can disagree with its events.
const ProjectionVersion = 4

// Resume carries a projection on from a State that Project or Resume
// returned, folding in the events after it. It is the same fold with the same
// checks: the first event must be from.LastSeq+1, and an event that could not
// have happened is a *ProjectionError.
//
// Resume(Project(events[:k]), events[k:]) equals Project(events) for every
// k, under the same taxes. That is what makes a snapshot an optimisation and
// never a second source of truth. A zero from is a projection from scratch.
//
// from is not modified, and the result does not alias it.
func Resume(from State, events []Event, taxes Taxes) (State, error) {
	if from.LastSeq == 0 {
		return Project(events, taxes)
	}
	p := projector{
		taxes:    taxes,
		s:        from.clone(),
		opened:   true,
		closed:   from.Status == StatusClosed,
		reopened: from.Status == StatusReopened,
		held:     from.Status == StatusHeld,
		invoiced: from.Invoiced,
	}
	for i, e := range events {
		if err := p.step(from.LastSeq+int64(i+1), e); err != nil {
			return State{}, &ProjectionError{Seq: e.Seq, Type: e.Type, Err: err}
		}
	}
	return p.s, nil
}

// clone is a deep copy. The projector edits lines, tenders and tickets in
// place, so resuming from a State that shared their backing arrays would
// rewrite the caller's copy. Nil slices stay nil, so a clone compares equal
// to its original.
func (s State) clone() State {
	c := s
	c.Lines = slices.Clone(s.Lines)
	for i := range c.Lines {
		c.Lines[i].Modifiers = slices.Clone(s.Lines[i].Modifiers)
	}
	c.Discounts = slices.Clone(s.Discounts)
	for i, d := range c.Discounts {
		if d.Amount != nil {
			amount := *d.Amount
			c.Discounts[i].Amount = &amount
		}
	}
	c.Joined = slices.Clone(s.Joined)
	c.Notes = slices.Clone(s.Notes)
	c.FiredCourses = slices.Clone(s.FiredCourses)
	c.Tickets = slices.Clone(s.Tickets)
	for i := range c.Tickets {
		c.Tickets[i].LineIDs = slices.Clone(s.Tickets[i].LineIDs)
	}
	c.Tenders = slices.Clone(s.Tenders)
	c.Refunds = slices.Clone(s.Refunds)
	c.Invoices = slices.Clone(s.Invoices)
	c.Credits = slices.Clone(s.Credits)
	for i := range c.Credits {
		c.Credits[i].Lines = cloneCreditLines(s.Credits[i].Lines)
	}
	c.TaxDetail = cloneTaxResult(s.TaxDetail)
	return c
}

func cloneTaxResult(r tax.TaxResult) tax.TaxResult {
	c := r
	c.Lines = slices.Clone(r.Lines)
	for i := range c.Lines {
		c.Lines[i].Components = slices.Clone(r.Lines[i].Components)
	}
	c.Components = slices.Clone(r.Components)
	return c
}
