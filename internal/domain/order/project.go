package order

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mez-pos/mez/internal/domain/kitchen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// Project folds an order's events into its current state.
//
// It is pure: no clock, no database, and the same events always give the same
// State. It is also strict. Each event is checked against the state before
// it, and the first one that could not have happened stops the fold with a
// *ProjectionError naming it. A service appending an event projects the
// stream with the candidate on the end; if that fails, the event is refused
// and nothing is written.
//
// taxes is the venue's tax regime. Tax is recomputed after every event, so an
// event that makes the bill untaxable (a GST line with no HSN) fails at its
// own seq. Taxes{} is an untaxed venue.
//
// The rules are in docs/02-domain.md, ADR-046 and ADR-050.
func Project(events []Event, taxes Taxes) (State, error) {
	if len(events) == 0 {
		return State{}, ErrNotOpened
	}
	p := projector{taxes: taxes}
	for i, e := range events {
		if err := p.step(int64(i+1), e); err != nil {
			return State{}, &ProjectionError{Seq: e.Seq, Type: e.Type, Err: err}
		}
	}
	return p.s, nil
}

// Taxes is how a venue is taxed: a profile and the venue's side of the
// calculation. The zero value, with no profile, is an untaxed venue. It is an
// argument and not a lookup so that Project stays pure.
type Taxes struct {
	Profile tax.TaxProfile
	Venue   tax.VenueContext
}

// projector is the fold's working state. It is private to one Project call,
// which is what makes mutating it in place safe.
type projector struct {
	taxes    Taxes
	s        State
	opened   bool
	closed   bool
	reopened bool // reopened since the last close
	held     bool // parked by order.held, until order.resumed
	invoiced bool // an invoice issued since the order was last opened or reopened
}

// step checks and applies one event, then brings the totals up to date so
// the next event is checked against them.
func (p *projector) step(wantSeq int64, e Event) error {
	if e.Seq != wantSeq {
		return fmt.Errorf("%w: got seq %d, want %d", ErrSeqGap, e.Seq, wantSeq)
	}
	if e.Payload == nil {
		return invalid("event has no payload")
	}
	if e.Type != e.Payload.EventType() {
		return invalid("type %q carries a %q payload", e.Type, e.Payload.EventType())
	}
	if _, isOpen := e.Payload.(OrderOpened); !p.opened && !isOpen {
		return ErrNotOpened
	}
	if p.closed && !afterClose(e.Type) && !kitchenEvent(e.Type) {
		return ErrOrderClosed
	}
	if p.held && e.Type != TypeOrderResumed && e.Type != TypeNoteAdded && e.Type != TypeInvoiceReprinted && !kitchenEvent(e.Type) {
		return ErrOrderHeld
	}
	if p.invoiced && changesBill(e.Type) {
		return fmt.Errorf("%w: %s", ErrOrderInvoiced, e.Type)
	}
	if err := p.apply(e); err != nil {
		return err
	}
	p.s.LastSeq = e.Seq
	p.s.UpdatedAt = e.At.UTC()
	return p.recompute()
}

// apply dispatches an event to its handler.
func (p *projector) apply(e Event) error {
	switch pl := e.Payload.(type) {
	case OrderOpened:
		return p.open(e, pl)
	case LineAdded:
		return p.addLine(e, pl)
	case LineQtyChanged:
		return p.changeQty(pl)
	case LineModified:
		return p.modifyLine(pl)
	case LineVoided:
		return p.endLine(pl.LineID, LineStatusVoided, pl.ReasonCode, pl.AuthorisedBy)
	case LineComped:
		return p.endLine(pl.LineID, LineStatusComped, pl.ReasonCode, pl.AuthorisedBy)
	case DiscountApplied:
		return p.discount(e, pl)
	case DiscountRemoved:
		return p.removeDiscount(pl)
	case NoteAdded:
		return p.note(e, pl)
	case CourseFired:
		return p.fireCourse(pl)
	case TicketSent:
		return p.sendTicket(e, pl)
	case PaymentTendered:
		return p.tender(e, pl)
	case PaymentVoided:
		return p.voidPayment(pl)
	case InvoiceIssued:
		return p.invoice(e, pl)
	case InvoiceReprinted:
		return p.reprint(pl)
	case CreditNoteIssued:
		return p.credit(e, pl)
	case RefundIssued:
		return p.refund(e, pl)
	case OrderClosed:
		return p.close(e, pl)
	case OrderReopened:
		return p.reopen(pl)
	case OrderHeld:
		p.held = true // the status check refuses a hold from anything but open
		return nil
	case OrderResumed:
		if !p.held {
			return ErrOrderNotHeld
		}
		p.held = false
		return nil
	case OrderTableChanged:
		return p.changeTable(pl)
	case TablesJoined:
		return p.joinTables(pl)
	case TablesParted:
		return p.partTables(pl)
	case TicketStarted:
		return p.start(e, pl)
	case TicketBumped:
		return p.bump(e, pl)
	case TicketRecalled:
		return p.recall(pl)
	default:
		return invalid("no projection for %T", e.Payload)
	}
}

func (p *projector) open(e Event, pl OrderOpened) error {
	if p.opened {
		return ErrAlreadyOpened
	}
	if !validChannel(pl.Channel) {
		return invalid("unknown channel %q", pl.Channel)
	}
	if _, err := money.New(0, pl.Currency); err != nil {
		return invalid("currency: %v", err)
	}
	if pl.Covers < 0 || pl.TableID < 0 {
		return invalid("covers and table must not be negative")
	}
	service := pl.Service
	switch {
	case service == "":
		service = DefaultService(pl.TableID)
	case !validService(service):
		return invalid("unknown service type %q", pl.Service)
	case service != ServiceDineIn && pl.TableID != 0:
		return invalid("a %s order has no table", service)
	}
	// The taxes the order opened under are the order's from here on
	// (ADR-374); without them, the venue's as handed to Project.
	if pl.Taxes != nil {
		taxes, err := pl.Taxes.Taxes()
		if err != nil {
			return err
		}
		p.taxes = taxes
	}
	if p.taxes.Profile != nil && p.taxes.Venue.Currency != pl.Currency {
		return fmt.Errorf("%w: the venue is taxed in %q, the order is in %q", ErrTaxConfig, p.taxes.Venue.Currency, pl.Currency)
	}
	p.opened = true
	p.s = State{
		Channel:  pl.Channel,
		Currency: pl.Currency,
		Service:  service,
		TableID:  pl.TableID,
		Covers:   pl.Covers,
		OpenedBy: pl.OpenedBy,
		OpenedAt: e.At.UTC(),
		Taxes:    pl.Taxes.clone(),
	}
	return nil
}

func (p *projector) addLine(e Event, pl LineAdded) error {
	if pl.Qty < 1 {
		return invalid("quantity %d is not positive", pl.Qty)
	}
	if strings.TrimSpace(pl.Name) == "" {
		return invalid("line has no name snapshot")
	}
	if pl.Course < 0 {
		return invalid("course %d is negative", pl.Course)
	}
	if err := pl.TaxRate.Validate(); err != nil {
		return invalid("tax rate: %v", err)
	}
	if err := p.checkPrice("unit price", pl.UnitPrice); err != nil {
		return err
	}
	for _, m := range pl.Modifiers {
		if err := p.checkPrice("modifier "+m.Name, m.Price); err != nil {
			return err
		}
	}
	p.s.Lines = append(p.s.Lines, Line{
		ID:          e.Seq,
		ItemID:      pl.ItemID,
		Name:        pl.Name,
		Qty:         pl.Qty,
		UnitPrice:   pl.UnitPrice,
		Modifiers:   slices.Clone(pl.Modifiers),
		Course:      pl.Course,
		Station:     pl.Station,
		TaxRate:     pl.TaxRate,
		HSN:         pl.HSN,
		TaxExempt:   pl.TaxExempt,
		Status:      LineStatusActive,
		CourseFired: slices.Contains(p.s.FiredCourses, pl.Course),
	})
	return nil
}

// changeQty changes a line's quantity while the kitchen has not seen it.
// After that, a lower quantity would be a void with no approval, and a
// higher one would be charged but never sent: a void, or a new line, says
// what happened instead (ADR-073).
func (p *projector) changeQty(pl LineQtyChanged) error {
	l, err := p.activeLine(pl.LineID)
	if err != nil {
		return err
	}
	if l.Sent || l.CourseFired {
		return fmt.Errorf("%w: line %d", ErrLineInKitchen, l.ID)
	}
	if pl.From != l.Qty {
		return fmt.Errorf("%w: line %d has %d, event says %d", ErrQtyMismatch, l.ID, l.Qty, pl.From)
	}
	if pl.To < 1 || pl.To == pl.From {
		return invalid("quantity %d to %d is not a change to a positive quantity", pl.From, pl.To)
	}
	l.Qty = pl.To
	return nil
}

// modifyLine replaces the modifiers of a line the kitchen has not seen.
func (p *projector) modifyLine(pl LineModified) error {
	l, err := p.activeLine(pl.LineID)
	if err != nil {
		return err
	}
	if l.Sent || l.CourseFired {
		return fmt.Errorf("%w: line %d", ErrLineInKitchen, l.ID)
	}
	if !sameModifiers(pl.From, l.Modifiers) {
		return fmt.Errorf("%w: line %d", ErrModifiersMismatch, l.ID)
	}
	if sameModifiers(pl.To, pl.From) {
		return invalid("line %d: modifiers are unchanged", l.ID)
	}
	for _, m := range pl.To {
		if strings.TrimSpace(m.Name) == "" {
			return invalid("line %d: a modifier has no name snapshot", l.ID)
		}
		if err := p.checkPrice("modifier "+m.Name, m.Price); err != nil {
			return err
		}
	}
	l.Modifiers = slices.Clone(pl.To)
	return nil
}

// sameModifiers compares two modifier lists in order. Order is part of the
// snapshot: it is how the ticket and the invoice print them.
func sameModifiers(a, b []Modifier) bool {
	return slices.Equal(a, b)
}

// endLine voids or comps a line. Both need a reason and an authoriser, and
// both are final: a voided line cannot be comped, or the reverse.
func (p *projector) endLine(lineID int64, to LineStatus, reason string, authorisedBy int64) error {
	l, err := p.activeLine(lineID)
	if err != nil {
		return err
	}
	if err := needReason(reason, authorisedBy); err != nil {
		return err
	}
	l.Status = to
	l.ReasonCode = reason
	l.AuthorisedBy = authorisedBy
	return nil
}

func (p *projector) discount(e Event, pl DiscountApplied) error {
	if err := p.checkScope(pl.Scope, pl.LineID, true); err != nil {
		return err
	}
	if strings.TrimSpace(pl.Reason) == "" {
		return invalid("discount has no reason")
	}
	d := Discount{
		Seq: e.Seq, Scope: pl.Scope, LineID: pl.LineID, Kind: pl.Kind,
		BasisPoints: pl.BasisPoints, Reason: pl.Reason, ActorID: e.ActorID, AuthorisedBy: pl.AuthorisedBy,
	}
	switch pl.Kind {
	case DiscountPercent:
		if pl.BasisPoints < 1 || pl.BasisPoints > 10000 || pl.Amount != nil {
			return invalid("a percentage discount needs 1 to 10000 basis points and no amount")
		}
	case DiscountAmount:
		if pl.Amount == nil || pl.BasisPoints != 0 {
			return invalid("an amount discount needs an amount and no basis points")
		}
		if err := p.checkPositive("discount", *pl.Amount); err != nil {
			return err
		}
		amount := *pl.Amount
		d.Amount = &amount
	default:
		return invalid("unknown discount kind %q", pl.Kind)
	}
	p.s.Discounts = append(p.s.Discounts, d)
	return nil
}

// removeDiscount marks the discount at pl.Seq removed (ADR-373). It must be
// a discount of this order, not removed already, and the order must still
// take changes to what was sold, as applying one needs.
func (p *projector) removeDiscount(pl DiscountRemoved) error {
	for i := range p.s.Discounts {
		d := &p.s.Discounts[i]
		if d.Seq != pl.Seq {
			continue
		}
		if d.Removed {
			return invalid("the discount at seq %d was removed already", pl.Seq)
		}
		if err := p.checkScope(d.Scope, d.LineID, false); err != nil {
			return err
		}
		d.Removed = true
		return nil
	}
	return invalid("no discount at seq %d", pl.Seq)
}

func (p *projector) note(e Event, pl NoteAdded) error {
	if err := p.checkScope(pl.Scope, pl.LineID, false); err != nil {
		return err
	}
	if strings.TrimSpace(pl.Text) == "" {
		return invalid("note is empty")
	}
	p.s.Notes = append(p.s.Notes, Note{Seq: e.Seq, Scope: pl.Scope, LineID: pl.LineID, Text: pl.Text})
	return nil
}

func (p *projector) fireCourse(pl CourseFired) error {
	if pl.Course < 1 {
		return invalid("course %d is not positive", pl.Course)
	}
	if slices.Contains(p.s.FiredCourses, pl.Course) {
		return fmt.Errorf("%w: course %d", ErrCourseAlreadyFired, pl.Course)
	}
	p.s.FiredCourses = append(p.s.FiredCourses, pl.Course)
	for i := range p.s.Lines {
		if p.s.Lines[i].Course == pl.Course {
			p.s.Lines[i].CourseFired = true
		}
	}
	return nil
}

func (p *projector) sendTicket(e Event, pl TicketSent) error {
	if pl.Station == "" || len(pl.LineIDs) == 0 {
		return invalid("a ticket needs a station and at least one line")
	}
	lines := make([]*Line, 0, len(pl.LineIDs))
	for _, id := range pl.LineIDs {
		l, err := p.line(id)
		if err != nil {
			return err
		}
		// A comped line is still made, so it can go to the kitchen: the
		// birthday dessert is comped before it is sent. A voided one is not.
		if l.Status == LineStatusVoided {
			return fmt.Errorf("%w: line %d is voided", ErrLineNotActive, id)
		}
		if l.Station != pl.Station {
			return invalid("line %d belongs to station %q, not %q", id, l.Station, pl.Station)
		}
		lines = append(lines, l)
	}
	for _, l := range lines {
		l.Sent = true
	}
	p.s.Tickets = append(p.s.Tickets, Ticket{
		Seq: e.Seq, Station: pl.Station, LineIDs: slices.Clone(pl.LineIDs), At: e.At.UTC(), Status: kitchen.TicketFired,
	})
	return nil
}

// validChannel reports whether c is one of the channels of docs/02-domain.md.
func validChannel(c Channel) bool {
	switch c {
	case ChannelCounter, ChannelQRTable, ChannelWeb, ChannelKiosk, ChannelAggregator:
		return true
	}
	return false
}

// needReason checks the two fields every void, comp and reopen carries.
func needReason(reason string, authorisedBy int64) error {
	if strings.TrimSpace(reason) == "" {
		return invalid("no reason given")
	}
	if authorisedBy == 0 {
		return invalid("not authorised by anyone")
	}
	return nil
}

// changesBill reports the events that change what the guest owes for what.
// None may follow an invoice until a reopen: the invoice already says what
// was sold, and a sale must never close bigger or smaller than its invoice.
// Payments, refunds, notes and the kitchen's events do not change the bill.
func changesBill(t Type) bool {
	switch t {
	case TypeLineAdded, TypeLineQtyChanged, TypeLineModified, TypeLineVoided, TypeLineComped, TypeDiscountApplied, TypeDiscountRemoved:
		return true
	}
	return false
}

// afterClose reports the events a closed order still takes: a note, a
// reopen, a reprint, and a credit note with the refunds that pay it out
// (ADR-083). None of them changes what was sold.
func afterClose(t Type) bool {
	switch t {
	case TypeOrderReopened, TypeNoteAdded, TypeInvoiceReprinted, TypeCreditNoteIssued, TypeRefundIssued:
		return true
	}
	return false
}
