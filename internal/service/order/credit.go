package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/domain/invoice"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

// ErrRefundsMismatch means the refunds asked for do not pay out exactly the
// credit: a credit note is money going back, all of it, to named payments.
var ErrRefundsMismatch = errors.New("the refunds do not add up to the credit")

// RefundPart is how much of a credit goes back against one payment.
type RefundPart struct {
	PaymentID int64        `json:"payment_id"`
	Amount    money.Amount `json:"amount"`
}

// IssueCredit is a refund on an invoiced bill (ADR-083): what is credited,
// the payments it goes back to, why, and who allowed it. The caller checks
// the authoriser's refund.give.
type IssueCredit struct {
	Credit       order.CreditRequest `json:"credit"`
	Refunds      []RefundPart        `json:"refunds"`
	Reason       string              `json:"reason"`
	AuthorisedBy int64               `json:"authorised_by"`
}

// PlanCredit works out a credit note for the order without issuing it: the
// figures the till shows before a manager approves (no arithmetic in the
// UI). Nothing is written.
func (s *Service) PlanCredit(ctx context.Context, venueID, orderID int64, req order.CreditRequest) (order.CreditPlan, error) {
	res, err := s.Order(ctx, venueID, orderID)
	if err != nil {
		return order.CreditPlan{}, err
	}
	return order.PlanCredit(res.State, req)
}

// Credit issues a credit note and the refunds that pay it out, in one
// transaction: the number is allocated from the credit note series, written
// to the register, and put on the event, or none of it happens. It works on
// a closed bill; the invoice it corrects is never touched.
func (s *Service) Credit(ctx context.Context, meta Meta, orderID int64, cmd IssueCredit) (Result, error) {
	return s.run(ctx, meta, orderID, "issue_credit", cmd, func(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) ([]order.Payload, error) {
		p, err := order.PlanCredit(tx.State, cmd.Credit)
		if err != nil {
			return nil, err
		}
		if err := checkRefunds(cmd.Refunds, p.Total); err != nil {
			return nil, err
		}
		if strings.TrimSpace(cmd.Reason) == "" {
			return nil, fmt.Errorf("%w: a refund needs a reason", ErrInvalidCommand)
		}
		against := tx.State.Invoices[len(tx.State.Invoices)-1].InvoiceNo
		issued, err := invoice.Issue(ctx, tx.Invoices(), v.Credits, tx.VenueID, at, v.Location)
		if err != nil {
			return nil, err
		}
		if err := tx.RecordCreditNote(ctx, issued, against, p.Total.Minor, at); err != nil {
			return nil, err
		}
		seller := v.Seller
		payloads := []order.Payload{order.CreditNoteIssued{
			CreditNo: issued.InvoiceNo, Series: v.Credits.Code, AgainstInvoiceNo: against,
			Reason: cmd.Reason, AuthorisedBy: cmd.AuthorisedBy,
			Lines: p.Lines, Taxable: p.Taxable, Tax: p.Tax, Total: p.Total, Components: p.Components, Seller: &seller,
		}}
		for _, r := range cmd.Refunds {
			payloads = append(payloads, order.RefundIssued{Amount: r.Amount, AgainstPaymentID: r.PaymentID, Reason: cmd.Reason, AuthorisedBy: cmd.AuthorisedBy})
		}
		return payloads, nil
	})
}

// checkRefunds refuses refunds that do not pay out the credit exactly, or
// that name a payment twice. Whether each payment has that much left is the
// projection's to check (ErrOverRefund).
func checkRefunds(parts []RefundPart, credit money.Amount) error {
	if len(parts) == 0 {
		return fmt.Errorf("%w: no payment to refund to", ErrRefundsMismatch)
	}
	seen := map[int64]bool{}
	var sum int64
	for _, r := range parts {
		if seen[r.PaymentID] {
			return fmt.Errorf("%w: payment %d is named twice", ErrRefundsMismatch, r.PaymentID)
		}
		seen[r.PaymentID] = true
		if r.Amount.Currency != credit.Currency {
			return fmt.Errorf("%w: a refund in %s on a credit in %s", money.ErrCurrencyMismatch, r.Amount.Currency, credit.Currency)
		}
		sum += r.Amount.Minor
	}
	if sum != credit.Minor {
		return fmt.Errorf("%w: refunds of %d for a credit of %d", ErrRefundsMismatch, sum, credit.Minor)
	}
	return nil
}
