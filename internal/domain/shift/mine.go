package shift

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

// Mine is what one person did in a shift: the summary a waiter reads at
// the end of theirs.
//
// An order is theirs if they opened it; money is theirs if they took it.
// The two differ when one person seats a table and another settles it.
type Mine struct {
	Orders int64 // orders they opened in the shift
	Tables int64 // of those, the ones at a table
	// Sales is what their payments settled of the bills, less the payments
	// voided in the shift. Tips are apart, and a refund is not taken off:
	// it is a manager's correction, and the Z-report's.
	Sales money.Amount
	// Tips are the tips on those payments, before any pool shares them out.
	Tips money.Amount
	// ByTable is Tips by the table each order ended at, largest first.
	// Table 0 is the orders with no table. A table with no tip is left out.
	ByTable []TableTips
	// First is when they first did anything to an order in the shift.
	First *time.Time
}

// TableTips is the tips taken at one table.
type TableTips struct {
	TableID int64
	Tips    money.Amount
}

// mineTender is a payment someone took, kept to undo it if it is voided.
type mineTender struct {
	by          int64
	amount, tip money.Amount
}

// MineOf adds up what staffID did in shiftID, from the whole streams of
// the orders touched in it. A payment belongs to the shift it was taken
// in, and a void to the shift it was made in, as in CashFromOrders.
func MineOf(currency string, shiftID, staffID int64, orders [][]StampedEvent) (Mine, error) {
	zero, err := money.New(0, currency)
	if err != nil {
		return Mine{}, err
	}
	out := Mine{Sales: zero, Tips: zero}
	byTable := map[int64]money.Amount{}

	for _, events := range orders {
		var table int64
		mine := false // they opened it, in this shift
		tips := zero  // their tips on this order, in this shift
		tenders := map[int64]mineTender{}
		for _, se := range events {
			here := se.ShiftID == shiftID
			if here && se.Event.ActorID == staffID && (out.First == nil || se.Event.At.Before(*out.First)) {
				at := se.Event.At
				out.First = &at
			}
			switch p := se.Event.Payload.(type) {
			case order.OrderOpened:
				table = p.TableID
				mine = here && p.OpenedBy == staffID
			case order.OrderTableChanged:
				table = p.To
			case order.PaymentTendered:
				tenders[se.Event.Seq] = mineTender{by: p.TenderedBy, amount: p.Amount, tip: p.Tip}
				if !here || p.TenderedBy != staffID {
					continue
				}
				if out.Sales, err = out.Sales.Add(p.Amount); err != nil {
					return Mine{}, fmt.Errorf("shift summary: %w", err)
				}
				if tips, err = addTip(tips, p.Tip, 1); err != nil {
					return Mine{}, err
				}
			case order.PaymentVoided:
				t, ok := tenders[p.PaymentID]
				if !ok || !here || t.by != staffID {
					continue
				}
				if out.Sales, err = out.Sales.Sub(t.amount); err != nil {
					return Mine{}, fmt.Errorf("shift summary: %w", err)
				}
				if tips, err = addTip(tips, t.tip, -1); err != nil {
					return Mine{}, err
				}
			}
		}
		if mine {
			out.Orders++
			if table != 0 {
				out.Tables++
			}
		}
		if tips.Minor == 0 {
			continue
		}
		if out.Tips, err = out.Tips.Add(tips); err != nil {
			return Mine{}, fmt.Errorf("shift summary: %w", err)
		}
		at, ok := byTable[table]
		if !ok {
			at = zero
		}
		if byTable[table], err = at.Add(tips); err != nil {
			return Mine{}, fmt.Errorf("shift summary: %w", err)
		}
	}

	for id, tips := range byTable {
		if tips.Minor != 0 {
			out.ByTable = append(out.ByTable, TableTips{TableID: id, Tips: tips})
		}
	}
	slices.SortFunc(out.ByTable, func(a, b TableTips) int {
		return cmp.Or(cmp.Compare(b.Tips.Minor, a.Tips.Minor), cmp.Compare(a.TableID, b.TableID))
	})
	return out, nil
}

// addTip adds or takes off a tender's tip. A tender with no tip carries
// the zero amount, which has no currency and is nothing to add.
func addTip(sum, tip money.Amount, sign int) (money.Amount, error) {
	if tip == (money.Amount{}) {
		return sum, nil
	}
	var err error
	if sign < 0 {
		sum, err = sum.Sub(tip)
	} else {
		sum, err = sum.Add(tip)
	}
	if err != nil {
		return money.Amount{}, fmt.Errorf("shift summary: %w", err)
	}
	return sum, nil
}
