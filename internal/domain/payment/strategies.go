package payment

import (
	"fmt"
	"slices"

	"github.com/mez-pos/mez/internal/domain/money"
)

// Evenly splits the bill n ways. The largest remainder method decides who
// carries the odd paisa: 1000.00 three ways is 333.34, 333.33, 333.33, and
// the earliest payers carry the extra, so the answer never changes.
func Evenly(total money.Amount, n int) (Split, error) {
	s, err := newSplit(total, n)
	if err != nil {
		return Split{}, err
	}
	parts, err := money.SplitEvenly(total, n)
	if err != nil {
		return Split{}, err
	}
	for i, part := range parts {
		s.Shares[i].Amount = part
	}
	return s, s.Check()
}

// ByAmount splits by fixed amounts: "I'll put 500 on my card". Each payer in
// amounts pays exactly that. If they fall short of the bill, one more payer
// is added for the rest, so the split always settles it; if they exceed the
// bill, it is ErrOverAllocated. Each amount must be more than zero.
func ByAmount(total money.Amount, amounts []money.Amount) (Split, error) {
	if len(amounts) == 0 {
		return Split{}, fmt.Errorf("%w: no amounts", ErrInvalidSplit)
	}
	rest := total
	for i, a := range amounts {
		if a.Minor <= 0 {
			return Split{}, fmt.Errorf("%w: amount %d is %d", ErrInvalidSplit, i, a.Minor)
		}
		var err error
		if rest, err = rest.Sub(a); err != nil {
			return Split{}, fmt.Errorf("amount %d: %w", i, err)
		}
	}
	if rest.Minor < 0 {
		return Split{}, fmt.Errorf("%w: %d over", ErrOverAllocated, -rest.Minor)
	}
	n := len(amounts)
	if rest.Minor > 0 {
		n++
	}
	s, err := newSplit(total, n)
	if err != nil {
		return Split{}, err
	}
	for i, a := range amounts {
		s.Shares[i].Amount = a
	}
	if rest.Minor > 0 {
		s.Shares[n-1].Amount = rest
	}
	return s, s.Check()
}

// Item is one line of the bill for splitting: what it costs the guest (use
// order.Line.Due), and who is paying for it. An item with several payers is
// split evenly between them, the earlier-listed payers carrying any odd
// paisa.
type Item struct {
	LineID int64
	Amount money.Amount
	Payers []int // 0-based, at least one, no repeats
}

// ByItem splits by what each payer had. Every item must be assigned, and the
// items must add up to the bill exactly: a line left out would leave the bill
// unpaid, and a line counted twice would overcharge.
func ByItem(total money.Amount, payers int, items []Item) (Split, error) {
	s, err := newSplit(total, payers)
	if err != nil {
		return Split{}, err
	}
	covered := money.Amount{Minor: 0, Currency: total.Currency}
	for _, it := range items {
		if err := checkItem(it, payers); err != nil {
			return Split{}, err
		}
		if covered, err = covered.Add(it.Amount); err != nil {
			return Split{}, fmt.Errorf("line %d: %w", it.LineID, err)
		}
		parts, err := money.SplitEvenly(it.Amount, len(it.Payers))
		if err != nil {
			return Split{}, err
		}
		for k, p := range it.Payers {
			sh := &s.Shares[p]
			if sh.Amount, err = sh.Amount.Add(parts[k]); err != nil {
				return Split{}, err
			}
			sh.LineIDs = append(sh.LineIDs, it.LineID)
		}
	}
	if covered != total {
		return Split{}, fmt.Errorf("%w: the items come to %d, the bill is %d", ErrMismatch, covered.Minor, total.Minor)
	}
	return s, s.Check()
}

// checkItem refuses an item with a negative amount, no payer, a payer out of
// range, or the same payer twice.
func checkItem(it Item, payers int) error {
	if it.Amount.Minor < 0 {
		return fmt.Errorf("%w: line %d is %d", ErrInvalidSplit, it.LineID, it.Amount.Minor)
	}
	if len(it.Payers) == 0 {
		return fmt.Errorf("%w: line %d has no payer", ErrUnassigned, it.LineID)
	}
	for k, p := range it.Payers {
		if p < 0 || p >= payers {
			return fmt.Errorf("%w: line %d names payer %d of %d", ErrUnassigned, it.LineID, p, payers)
		}
		if slices.Contains(it.Payers[:k], p) {
			return fmt.Errorf("%w: line %d names payer %d twice", ErrUnassigned, it.LineID, p)
		}
	}
	return nil
}

// SharedByTable is the Guest of a line the whole table shares, such as a
// plate of starters or a bottle of wine.
const SharedByTable = 0

// GuestItem is a line of the bill by seat. Guest is 1-based, the seat number
// staff use; SharedByTable splits the line across every guest.
type GuestItem struct {
	LineID int64
	Amount money.Amount
	Guest  int
}

// ByGuest splits a table's bill by seat: each guest pays for their own lines
// and an even part of the shared ones. Payer i is guest i+1.
func ByGuest(total money.Amount, guests int, items []GuestItem) (Split, error) {
	if guests < 1 {
		return Split{}, fmt.Errorf("%w: %d guests", ErrInvalidSplit, guests)
	}
	everyone := make([]int, guests)
	for i := range everyone {
		everyone[i] = i
	}
	byItem := make([]Item, len(items))
	for i, gi := range items {
		switch {
		case gi.Guest == SharedByTable:
			byItem[i] = Item{LineID: gi.LineID, Amount: gi.Amount, Payers: everyone}
		case gi.Guest < 1 || gi.Guest > guests:
			return Split{}, fmt.Errorf("%w: line %d is at seat %d of %d", ErrUnassigned, gi.LineID, gi.Guest, guests)
		default:
			byItem[i] = Item{LineID: gi.LineID, Amount: gi.Amount, Payers: []int{gi.Guest - 1}}
		}
	}
	return ByItem(total, guests, byItem)
}
