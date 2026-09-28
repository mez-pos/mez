package shift

import (
	"errors"
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// ErrInvalidThreshold refuses a variance threshold below nothing.
var ErrInvalidThreshold = errors.New("the variance threshold cannot be negative")

// StampedEvent is an order event and the shift it was recorded in
// (ADR-094). 0 is no shift.
type StampedEvent struct {
	Event   order.Event
	ShiftID int64
}

// CashSales is the cash that went through the drawer for orders in one
// shift. A tender belongs to the shift it was taken in; a void or refund
// to the shift it was given in, whichever shift took the money.
type CashSales struct {
	Taken    money.Amount `json:"taken"`    // cash tenders: the bill and the tip handed over with it
	Voided   money.Amount `json:"voided"`   // cash tenders voided: bill and tip handed back
	Refunded money.Amount `json:"refunded"` // refunds against cash tenders
}

// Net is what the sales left in the drawer: taken, less voided and
// refunded.
func (c CashSales) Net() (money.Amount, error) {
	out, err := c.Taken.Sub(c.Voided)
	if err != nil {
		return money.Amount{}, err
	}
	return out.Sub(c.Refunded)
}

// cashTender is what a tender put in the drawer.
type cashTender struct {
	amount, tip money.Amount
}

// CashFromOrders adds up the cash that went through the drawer in shiftID,
// from the whole streams of the orders touched in it. The whole stream is
// needed because a void or refund in this shift may be against a tender
// taken in an earlier one: the refund still leaves this drawer.
//
// A cash tip is counted: it was handed over at the till with the bill and
// is in the drawer until it is paid out, which is a cash movement.
func CashFromOrders(currency string, shiftID int64, orders [][]StampedEvent) (CashSales, error) {
	zero, err := money.New(0, currency)
	if err != nil {
		return CashSales{}, err
	}
	out := CashSales{Taken: zero, Voided: zero, Refunded: zero}
	for _, events := range orders {
		cash := map[int64]cashTender{} // cash tenders by payment ID, the tender's seq
		for _, se := range events {
			e := se.Event
			if p, ok := e.Payload.(order.PaymentTendered); ok && p.Method == order.PaymentCash {
				tip := p.Tip
				if tip == (money.Amount{}) {
					tip = zero
				}
				cash[e.Seq] = cashTender{amount: p.Amount, tip: tip}
			}
			if se.ShiftID != shiftID {
				continue
			}
			if err := out.add(e, cash); err != nil {
				return CashSales{}, fmt.Errorf("shift: order event %s at seq %d: %w", e.Type, e.Seq, err)
			}
		}
	}
	return out, nil
}

// add counts one of the shift's events against the drawer.
func (c *CashSales) add(e order.Event, cash map[int64]cashTender) error {
	var err error
	switch p := e.Payload.(type) {
	case order.PaymentTendered:
		t, ok := cash[e.Seq]
		if !ok {
			return nil
		}
		if c.Taken, err = c.Taken.Add(t.amount); err != nil {
			return err
		}
		c.Taken, err = c.Taken.Add(t.tip)
	case order.PaymentVoided:
		t, ok := cash[p.PaymentID]
		if !ok {
			return nil
		}
		if c.Voided, err = c.Voided.Add(t.amount); err != nil {
			return err
		}
		c.Voided, err = c.Voided.Add(t.tip)
	case order.RefundIssued:
		if _, ok := cash[p.AgainstPaymentID]; !ok {
			return nil
		}
		c.Refunded, err = c.Refunded.Add(p.Amount)
	}
	return err
}

// Expected is what the drawer should hold at close: the float, the cash
// moved in and out, and what the shift's sales left in it.
func Expected(float Count, movements []Movement, sales CashSales) (money.Amount, error) {
	drawer, err := Drawer(float, movements)
	if err != nil {
		return money.Amount{}, err
	}
	net, err := sales.Net()
	if err != nil {
		return money.Amount{}, fmt.Errorf("shift: the cash sales: %w", err)
	}
	return drawer.Add(net)
}

// Reconciliation is the blind count set against what was expected, and
// the threshold in force when it was counted. It is worked out after the
// count is in, never shown before.
type Reconciliation struct {
	Expected  money.Amount `json:"expected"`
	Counted   money.Amount `json:"counted"`
	Variance  money.Amount `json:"variance"`  // counted less expected: over is positive, short negative
	Threshold money.Amount `json:"threshold"` // a variance further from zero than this needs a reason
}

// Reconcile sets a closing count against the expected figure.
func Reconcile(expected money.Amount, counted Count, threshold money.Amount) (Reconciliation, error) {
	if counted.Kind != CountClosing {
		return Reconciliation{}, fmt.Errorf("%w: a drawer is reconciled with its closing count, not a %s count", ErrInvalidCount, counted.Kind)
	}
	if threshold.Minor < 0 {
		return Reconciliation{}, fmt.Errorf("%w: %s", ErrInvalidThreshold, threshold)
	}
	if threshold.Currency != expected.Currency {
		return Reconciliation{}, fmt.Errorf("shift: a threshold in %q for a drawer in %q", threshold.Currency, expected.Currency)
	}
	variance, err := counted.Total.Sub(expected)
	if err != nil {
		return Reconciliation{}, fmt.Errorf("shift: the variance: %w", err)
	}
	return Reconciliation{Expected: expected, Counted: counted.Total, Variance: variance, Threshold: threshold}, nil
}

// Over reports cash in the drawer beyond what was expected.
func (r Reconciliation) Over() bool { return r.Variance.Minor > 0 }

// Short reports cash missing from the drawer.
func (r Reconciliation) Short() bool { return r.Variance.Minor < 0 }

// NeedsReason reports whether the variance is further from zero than the
// threshold, and so needs a reason before the shift can close. Exactly at
// the threshold does not.
func (r Reconciliation) NeedsReason() bool {
	v := r.Variance.Minor
	if v < 0 {
		v = -v // representable: money refuses the one int64 that is not
	}
	return v > r.Threshold.Minor
}
