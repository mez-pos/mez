// Package payment splits a bill between payers.
//
// docs/02-domain.md sets the one invariant: the tenders must sum to the
// order total exactly. Every strategy here returns a Split, and every Split
// is checked against that invariant before it is returned, so a split that
// loses or invents a paisa is an error, never a result.
//
// Tips are a separate field on each share and are never folded into the
// amount that settles the bill: Amount pays the order, Tip is owed to staff.
//
// The package knows money and nothing else. The caller turns an order into
// items using order.Line.Due, which is what each line costs after discounts
// and tax and which sums to the order's total.
package payment

import (
	"errors"
	"fmt"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrInvalidSplit means the split was asked for fewer than one payer,
	// a negative total, or a negative or zero amount where one is needed.
	ErrInvalidSplit = errors.New("invalid split")

	// ErrOverAllocated means fixed amounts add up to more than the bill.
	ErrOverAllocated = errors.New("amounts exceed the bill")

	// ErrUnassigned means an item has no payer, or names a payer or guest
	// that does not exist, or names one twice.
	ErrUnassigned = errors.New("item not assigned to a valid payer")

	// ErrMismatch means the items do not add up to the bill, or a split
	// does not: the invariant this package exists to hold.
	ErrMismatch = errors.New("split does not sum to the bill")
)

// Share is what one payer owes.
type Share struct {
	Payer   int          // 0-based
	Amount  money.Amount // settles the bill
	Tip     money.Amount // on top of Amount, owed to staff; zero until WithTips
	LineIDs []int64      // by item or guest: the lines this share covers, in part or whole
}

// Split is a bill divided between payers. Its shares' Amounts sum to Total
// exactly; Check proves it.
type Split struct {
	Total  money.Amount
	Shares []Share
}

// Check is the invariant: every share is in the bill's currency, no amount or
// tip is negative, and the amounts sum to exactly the total. Every strategy
// runs it before returning.
func (s Split) Check() error {
	sum := money.Amount{Minor: 0, Currency: s.Total.Currency}
	for i, sh := range s.Shares {
		if sh.Payer != i {
			return fmt.Errorf("%w: share %d belongs to payer %d", ErrMismatch, i, sh.Payer)
		}
		if sh.Amount.Minor < 0 || sh.Tip.Minor < 0 {
			return fmt.Errorf("%w: payer %d has a negative amount or tip", ErrMismatch, i)
		}
		if sh.Tip.Currency != s.Total.Currency {
			return fmt.Errorf("%w: payer %d tips in %q", ErrMismatch, i, sh.Tip.Currency)
		}
		var err error
		if sum, err = sum.Add(sh.Amount); err != nil {
			return fmt.Errorf("payer %d: %w", i, err)
		}
	}
	if sum != s.Total {
		return fmt.Errorf("%w: shares sum to %d, the bill is %d", ErrMismatch, sum.Minor, s.Total.Minor)
	}
	return nil
}

// Tips is the total of the tips across shares, kept apart from the bill.
func (s Split) Tips() (money.Amount, error) {
	tips := make([]money.Amount, len(s.Shares))
	for i, sh := range s.Shares {
		tips[i] = sh.Tip
	}
	return money.Sum(s.Total.Currency, tips...)
}

// WithTips returns a copy of the split with a tip on each share, one per
// payer, in order. The amounts that settle the bill do not change.
func (s Split) WithTips(tips []money.Amount) (Split, error) {
	if len(tips) != len(s.Shares) {
		return Split{}, fmt.Errorf("%w: %d tips for %d payers", ErrInvalidSplit, len(tips), len(s.Shares))
	}
	out := Split{Total: s.Total, Shares: make([]Share, len(s.Shares))}
	for i, sh := range s.Shares {
		if tips[i].Currency != s.Total.Currency || tips[i].Minor < 0 {
			return Split{}, fmt.Errorf("%w: tip %d is %d %s", ErrInvalidSplit, i, tips[i].Minor, tips[i].Currency)
		}
		sh.Tip = tips[i]
		sh.LineIDs = append([]int64(nil), sh.LineIDs...)
		out.Shares[i] = sh
	}
	return out, out.Check()
}

// newSplit builds the empty shares for n payers.
func newSplit(total money.Amount, n int) (Split, error) {
	if n < 1 {
		return Split{}, fmt.Errorf("%w: %d payers", ErrInvalidSplit, n)
	}
	if _, err := money.New(total.Minor, total.Currency); err != nil {
		return Split{}, err
	}
	if total.Minor < 0 {
		return Split{}, fmt.Errorf("%w: total %d is negative", ErrInvalidSplit, total.Minor)
	}
	zero := money.Amount{Minor: 0, Currency: total.Currency}
	s := Split{Total: total, Shares: make([]Share, n)}
	for i := range s.Shares {
		s.Shares[i] = Share{Payer: i, Amount: zero, Tip: zero}
	}
	return s, nil
}
