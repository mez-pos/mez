package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/id"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/invoice"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// ErrNothingToSend means a ticket was asked for with no lines given and none
// waiting for that station.
var ErrNothingToSend = errors.New("nothing to send to that station")

// orderPrefix is the public ID prefix of an order: ord_7K3M9QX2.
const orderPrefix = "ord"

// maxPublicIDDraws bounds the redraw on a public ID collision. Forty bits
// makes a second collision in a row a sign of a broken generator, not luck.
const maxPublicIDDraws = 5

// errReplay unwinds an open whose idempotency key was already applied.
var errReplay = errors.New("idempotency key already applied")

// The commands whose body is exactly their event's payload. The domain's
// projection checks every field, so the service passes them through.
type (
	AddLine       order.LineAdded
	ChangeQty     order.LineQtyChanged
	ModifyLine    order.LineModified
	VoidLine      order.LineVoided
	CompLine      order.LineComped
	ApplyDiscount order.DiscountApplied
	AddNote       order.NoteAdded
	FireCourse    order.CourseFired
	VoidPayment   order.PaymentVoided
	Refund        order.RefundIssued
	Reopen        order.OrderReopened
)

// OpenOrder opens an order. The currency is the venue's and the opener is
// the caller, so neither is the client's to say.
type OpenOrder struct {
	Channel order.Channel `json:"channel"`
	TableID int64         `json:"table_id,omitempty"`
	Covers  int64         `json:"covers"`
}

// SendTicket sends lines to a station. With no LineIDs it sends every line
// for that station not yet sent, not voided, and not held back by an unfired
// course.
type SendTicket struct {
	Station string  `json:"station"`
	LineIDs []int64 `json:"line_ids,omitempty"`
}

// TenderPayment records a payment. The tenderer is the caller. A tip left
// out is no tip.
type TenderPayment struct {
	Method    order.PaymentMethod `json:"method"`
	Amount    money.Amount        `json:"amount"`
	Tip       money.Amount        `json:"tip"`
	Reference string              `json:"reference,omitempty"`
}

// Open opens a new order at the caller's venue.
func (s *Service) Open(ctx context.Context, meta Meta, cmd OpenOrder) (Result, error) {
	if err := meta.validate(); err != nil {
		return Result{}, err
	}
	v, err := loadVenue(ctx, s.db.Queries(), meta.VenueID)
	if err != nil {
		return Result{}, err
	}
	fp, err := fingerprint("open", 0, cmd)
	if err != nil {
		return Result{}, err
	}
	at := s.clock.Now().UTC()
	opened := order.NewEvent(1, at, meta.ActorID, meta.DeviceID, order.OrderOpened{
		Channel: cmd.Channel, Currency: v.Currency, TableID: cmd.TableID, Covers: cmd.Covers, OpenedBy: meta.ActorID,
	})

	for range maxPublicIDDraws {
		publicID, err := id.NewPublic(orderPrefix)
		if err != nil {
			return Result{}, fmt.Errorf("drawing an order id: %w", err)
		}
		var prior *priorResult
		orderID, state, err := s.db.OpenOrder(ctx, db.NewOrder{
			PublicID: publicID,
			VenueID:  meta.VenueID,
			Opened:   opened,
			Within: func(ctx context.Context, tx *db.OrderTx) error {
				p, err := findPrior(ctx, tx.Queries(), meta, fp)
				if err != nil {
					return err
				}
				if p != nil {
					prior = p
					return errReplay
				}
				if err := tableInService(ctx, tx.Queries(), meta.VenueID, cmd.TableID); err != nil {
					return err
				}
				return saveKey(ctx, tx.Queries(), meta, fp, tx.OrderID, 1, 1, at)
			},
		}, v.Taxes)
		switch {
		case errors.Is(err, errReplay):
			return s.replay(ctx, v, *prior)
		case errors.Is(err, db.ErrPublicIDTaken):
			s.log.Warn("order public id collided, redrawing", "public_id", publicID)
			continue
		case err != nil:
			return Result{}, err
		}
		res := Result{OrderID: orderID, PublicID: publicID, State: state, Events: []order.Event{opened}}
		s.publish(ctx, meta.VenueID, res)
		return res, nil
	}
	return Result{}, fmt.Errorf("drawing an order id: %d collisions in a row", maxPublicIDDraws)
}

// one is a decider that always appends the same single payload.
func one(p order.Payload) decider {
	return func(context.Context, *db.OrderTx, venue, time.Time) ([]order.Payload, error) {
		return []order.Payload{p}, nil
	}
}

// AddLine adds a line, with the price and tax facts as they are now.
func (s *Service) AddLine(ctx context.Context, meta Meta, orderID int64, cmd AddLine) (Result, error) {
	return s.run(ctx, meta, orderID, "add_line", cmd, one(order.LineAdded(cmd)))
}

// ChangeQty changes a line's quantity. From must be what the line holds, so
// two terminals editing one line cannot overwrite each other unseen.
func (s *Service) ChangeQty(ctx context.Context, meta Meta, orderID int64, cmd ChangeQty) (Result, error) {
	return s.run(ctx, meta, orderID, "change_qty", cmd, one(order.LineQtyChanged(cmd)))
}

// ModifyLine replaces a line's modifiers before the kitchen has it. From
// must be what the line holds, as with ChangeQty.
func (s *Service) ModifyLine(ctx context.Context, meta Meta, orderID int64, cmd ModifyLine) (Result, error) {
	return s.run(ctx, meta, orderID, "modify_line", cmd, one(order.LineModified(cmd)))
}

// Hold parks an open order; Resume takes it off hold (ADR-074).
func (s *Service) Hold(ctx context.Context, meta Meta, orderID int64) (Result, error) {
	return s.run(ctx, meta, orderID, "hold", struct{}{}, one(order.OrderHeld{}))
}

// Resume takes a held order off hold.
func (s *Service) Resume(ctx context.Context, meta Meta, orderID int64) (Result, error) {
	return s.run(ctx, meta, orderID, "resume", struct{}{}, one(order.OrderResumed{}))
}

// ErrNeedsApproval means a line cannot be removed without a manager: it
// has gone to the kitchen, or money has been taken on the order.
var ErrNeedsApproval = errors.New("this needs a manager's approval")

// ReasonEnteredInError is the reason RemoveLine records.
const ReasonEnteredInError = "entered_in_error"

// RemoveLine takes off a line rung up by mistake, with no manager: the
// caller authorises it themselves, and the void is on the order forever
// like any other. It is only for a line nothing has happened to yet: not
// sent, its course not fired, and no payment on the order, so it cannot
// take back an item whose money was taken (ADR-073).
func (s *Service) RemoveLine(ctx context.Context, meta Meta, orderID, lineID int64) (Result, error) {
	cmd := VoidLine{LineID: lineID, ReasonCode: ReasonEnteredInError, AuthorisedBy: meta.ActorID}
	return s.run(ctx, meta, orderID, "remove_line", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		for _, t := range tx.State.Tenders {
			if t.Status != order.PaymentStatusVoided {
				return nil, fmt.Errorf("%w: the order has a payment", ErrNeedsApproval)
			}
		}
		for _, l := range tx.State.Lines {
			if l.ID == lineID && (l.Sent || l.CourseFired) {
				return nil, fmt.Errorf("%w: line %d has gone to the kitchen", ErrNeedsApproval, lineID)
			}
		}
		// A line that does not exist, or is already voided, is the
		// projection's to refuse, with its own error.
		return []order.Payload{order.LineVoided(cmd)}, nil
	})
}

// VoidLine takes a line off the bill.
func (s *Service) VoidLine(ctx context.Context, meta Meta, orderID int64, cmd VoidLine) (Result, error) {
	return s.run(ctx, meta, orderID, "void_line", cmd, one(order.LineVoided(cmd)))
}

// CompLine gives a line away.
func (s *Service) CompLine(ctx context.Context, meta Meta, orderID int64, cmd CompLine) (Result, error) {
	return s.run(ctx, meta, orderID, "comp_line", cmd, one(order.LineComped(cmd)))
}

// ApplyDiscount discounts a line or the order.
func (s *Service) ApplyDiscount(ctx context.Context, meta Meta, orderID int64, cmd ApplyDiscount) (Result, error) {
	return s.run(ctx, meta, orderID, "apply_discount", cmd, one(order.DiscountApplied(cmd)))
}

// AddNote adds a note to a line or the order.
func (s *Service) AddNote(ctx context.Context, meta Meta, orderID int64, cmd AddNote) (Result, error) {
	return s.run(ctx, meta, orderID, "add_note", cmd, one(order.NoteAdded(cmd)))
}

// FireCourse fires a course.
func (s *Service) FireCourse(ctx context.Context, meta Meta, orderID int64, cmd FireCourse) (Result, error) {
	return s.run(ctx, meta, orderID, "fire_course", cmd, one(order.CourseFired(cmd)))
}

// SendTicket sends lines to a station.
func (s *Service) SendTicket(ctx context.Context, meta Meta, orderID int64, cmd SendTicket) (Result, error) {
	return s.run(ctx, meta, orderID, "send_ticket", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		lines := cmd.LineIDs
		if len(lines) == 0 {
			lines = waiting(tx.State, cmd.Station)
			if len(lines) == 0 {
				return nil, fmt.Errorf("%w: %q", ErrNothingToSend, cmd.Station)
			}
		}
		return []order.Payload{order.TicketSent{Station: cmd.Station, LineIDs: lines}}, nil
	})
}

// waiting is the lines a station has not had yet and may have now.
func waiting(s order.State, station string) []int64 {
	var ids []int64
	for _, l := range s.Lines {
		if l.Station == station && !l.Sent && l.Status != order.LineStatusVoided && (l.Course == 0 || l.CourseFired) {
			ids = append(ids, l.ID)
		}
	}
	return ids
}

// TenderPayment records a payment against the order. The tender that
// settles the bill also issues its invoice, in the same transaction: the
// number is allocated with the payment that earned it, or not at all.
func (s *Service) TenderPayment(ctx context.Context, meta Meta, orderID int64, cmd TenderPayment) (Result, error) {
	return s.run(ctx, meta, orderID, "tender_payment", cmd, func(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) ([]order.Payload, error) {
		tip := cmd.Tip
		if tip == (money.Amount{}) {
			tip = money.Amount{Currency: v.Currency}
		}
		tendered := order.PaymentTendered{
			Method: cmd.Method, Amount: cmd.Amount, Tip: tip, Reference: cmd.Reference, TenderedBy: meta.ActorID,
		}
		if !settles(tx.State, cmd.Amount) {
			return []order.Payload{tendered}, nil
		}
		inv, err := issueInvoice(ctx, tx, v, at)
		if err != nil {
			return nil, err
		}
		return []order.Payload{tendered, inv}, nil
	})
}

// settles reports whether a tender of amount brings the bill to exactly
// zero with no invoice yet for this settlement. A tender the projection will
// refuse (over the balance, the wrong currency) does not settle: it fails
// on its own, before any number is allocated.
func settles(st order.State, amount money.Amount) bool {
	due := st.Totals.BalanceDue
	return !st.Invoiced && !st.Totals.Total.IsZero() && amount.Currency == due.Currency && amount.Minor == due.Minor
}

// VoidPayment voids a tender.
//
// On a closed bill it reopens it first, in the same command and under the
// same approval (ADR-084): the money has to be taken again, and a closed
// bill takes no payment. The invoice keeps its number (ADR-083).
func (s *Service) VoidPayment(ctx context.Context, meta Meta, orderID int64, cmd VoidPayment) (Result, error) {
	return s.run(ctx, meta, orderID, "void_payment", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		voided := order.PaymentVoided(cmd)
		if tx.State.Status != order.StatusClosed {
			return []order.Payload{voided}, nil
		}
		return []order.Payload{order.OrderReopened{Reason: cmd.Reason, AuthorisedBy: cmd.AuthorisedBy}, voided}, nil
	})
}

// Refund gives money back against a tender.
func (s *Service) Refund(ctx context.Context, meta Meta, orderID int64, cmd Refund) (Result, error) {
	return s.run(ctx, meta, orderID, "refund", cmd, one(order.RefundIssued(cmd)))
}

// IssueInvoice issues the next number in the venue's series for a settled
// order. The number is allocated, written to the register and put on the
// event in the one transaction, with the bill's tax detail as its snapshot.
func (s *Service) IssueInvoice(ctx context.Context, meta Meta, orderID int64) (Result, error) {
	return s.run(ctx, meta, orderID, "issue_invoice", struct{}{}, func(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) ([]order.Payload, error) {
		inv, err := issueInvoice(ctx, tx, v, at)
		if err != nil {
			return nil, err
		}
		return []order.Payload{inv}, nil
	})
}

// Close closes a settled order. If this settlement has no invoice yet and
// the bill is not zero, one is issued first, in the same transaction: a sale
// never closes without its number.
func (s *Service) Close(ctx context.Context, meta Meta, orderID int64) (Result, error) {
	return s.run(ctx, meta, orderID, "close", struct{}{}, func(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) ([]order.Payload, error) {
		var payloads []order.Payload
		if !tx.State.Invoiced && !tx.State.Totals.Total.IsZero() {
			inv, err := issueInvoice(ctx, tx, v, at)
			if err != nil {
				return nil, err
			}
			payloads = append(payloads, inv)
		}
		return append(payloads, order.OrderClosed{ClosedBy: meta.ActorID}), nil
	})
}

// Reopen reopens a closed order for a correction.
func (s *Service) Reopen(ctx context.Context, meta Meta, orderID int64, cmd Reopen) (Result, error) {
	return s.run(ctx, meta, orderID, "reopen", cmd, one(order.OrderReopened(cmd)))
}

// issueInvoice allocates a number and records it, inside tx. The domain
// refuses the event if the bill is not settled or already invoiced, and the
// rollback then returns the number: no gap either way.
func issueInvoice(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) (order.InvoiceIssued, error) {
	issued, err := invoice.Issue(ctx, tx.Invoices(), v.Series, tx.VenueID, at, v.Location)
	if err != nil {
		return order.InvoiceIssued{}, err
	}
	if err := tx.RecordInvoice(ctx, issued, at); err != nil {
		return order.InvoiceIssued{}, err
	}
	snapshot, err := json.Marshal(tx.State.TaxDetail)
	if err != nil {
		return order.InvoiceIssued{}, fmt.Errorf("encoding the tax snapshot: %w", err)
	}
	return order.InvoiceIssued{
		InvoiceNo: issued.InvoiceNo, Series: v.Series.Code, TaxSnapshot: snapshot, Prices: pricesOf(tx.State, v.Seller),
	}, nil
}
