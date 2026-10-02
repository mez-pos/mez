package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/payment"
)

// ErrSplitAfterPayment refuses a split by item or by guest once money has
// been taken: the lines sum to the bill, not to what is left of it, and
// nothing records which lines an earlier payment was for.
var ErrSplitAfterPayment = errors.New("split by item or guest before any payment")

// SplitKind is how a bill is divided.
type SplitKind string

// The four ways the till divides a bill (phase 07, Task 2).
const (
	SplitByEvenly SplitKind = "evenly"
	SplitByAmount SplitKind = "amount"
	SplitByItem   SplitKind = "item"
	SplitByGuest  SplitKind = "guest"
)

// SplitKinds is every kind, for the API's generated vocabulary.
func SplitKinds() []SplitKind {
	return []SplitKind{SplitByEvenly, SplitByAmount, SplitByItem, SplitByGuest}
}

// maxPayers caps a split: a table of more than this is a party, not a till.
const maxPayers = 20

// SplitRequest asks how a bill divides. Only the fields its Kind names are
// read:
//
//	evenly  Ways payers share the balance equally
//	amount  each of Amounts is one payer; the rest is one more payer
//	item    Ways payers; each of Items names a line and who pays for it
//	guest   Ways guests; each of Seats names a line and its seat, 0 shared
type SplitRequest struct {
	Kind    SplitKind      `json:"kind"`
	Ways    int            `json:"ways,omitempty"`
	Amounts []money.Amount `json:"amounts,omitempty"`
	Items   []SplitItem    `json:"items,omitempty"`
	Seats   []SplitSeat    `json:"seats,omitempty"`
}

// SplitItem is a line and the payers sharing it, 0-based.
type SplitItem struct {
	LineID int64 `json:"line_id"`
	Payers []int `json:"payers"`
}

// SplitSeat is a line and the seat it was ordered at: 1 upwards, or 0 for a
// line the whole table shares.
type SplitSeat struct {
	LineID int64 `json:"line_id"`
	Guest  int   `json:"guest"`
}

// Split divides what is owed on an order. It records nothing: a split is a
// plan for the tenders that follow, each of which is its own command.
func (s *Service) Split(ctx context.Context, venueID, orderID int64, req SplitRequest) (payment.Split, error) {
	res, err := s.Order(ctx, venueID, orderID)
	if err != nil {
		return payment.Split{}, err
	}
	return planSplit(res.State, req)
}

// planSplit is Split without the database. Evenly and by amount divide the
// balance, so they work part way through paying; by item and by guest
// divide the lines, which sum to the whole bill, so they need nothing paid.
func planSplit(st order.State, req SplitRequest) (payment.Split, error) {
	switch st.Status {
	case order.StatusClosed:
		return payment.Split{}, order.ErrOrderClosed
	case order.StatusHeld:
		return payment.Split{}, order.ErrOrderHeld
	}
	balance := st.Totals.BalanceDue
	if balance.Minor <= 0 {
		return payment.Split{}, fmt.Errorf("%w: nothing is owed", payment.ErrInvalidSplit)
	}
	switch req.Kind {
	case SplitByEvenly:
		if err := checkWays(req.Ways); err != nil {
			return payment.Split{}, err
		}
		return payment.Evenly(balance, req.Ways)
	case SplitByAmount:
		if len(req.Amounts) == 0 || len(req.Amounts) >= maxPayers {
			return payment.Split{}, fmt.Errorf("%w: %d amounts", payment.ErrInvalidSplit, len(req.Amounts))
		}
		return payment.ByAmount(balance, req.Amounts)
	case SplitByItem, SplitByGuest:
		if !st.Totals.NetPaid.IsZero() {
			return payment.Split{}, ErrSplitAfterPayment
		}
		if err := checkWays(req.Ways); err != nil {
			return payment.Split{}, err
		}
		if req.Kind == SplitByItem {
			return splitByItem(st, req)
		}
		return splitByGuest(st, req)
	default:
		return payment.Split{}, fmt.Errorf("%w: unknown split %q", payment.ErrInvalidSplit, req.Kind)
	}
}

func checkWays(n int) error {
	if n < 1 || n > maxPayers {
		return fmt.Errorf("%w: %d payers, 1 to %d", payment.ErrInvalidSplit, n, maxPayers)
	}
	return nil
}

func splitByItem(st order.State, req SplitRequest) (payment.Split, error) {
	payers := make(map[int64][]int, len(req.Items))
	for _, it := range req.Items {
		if _, dup := payers[it.LineID]; dup {
			return payment.Split{}, fmt.Errorf("%w: line %d is listed twice", payment.ErrUnassigned, it.LineID)
		}
		payers[it.LineID] = it.Payers
	}
	items := make([]payment.Item, 0, len(payers))
	err := eachBilledLine(st, func(l order.Line) error {
		p, ok := payers[l.ID]
		if !ok {
			return fmt.Errorf("%w: %s (line %d) has no payer", payment.ErrUnassigned, l.Name, l.ID)
		}
		delete(payers, l.ID)
		items = append(items, payment.Item{LineID: l.ID, Amount: l.Due, Payers: p})
		return nil
	})
	if err != nil {
		return payment.Split{}, err
	}
	if err := noneLeft(len(payers)); err != nil {
		return payment.Split{}, err
	}
	return payment.ByItem(st.Totals.Total, req.Ways, items)
}

func splitByGuest(st order.State, req SplitRequest) (payment.Split, error) {
	seats := make(map[int64]int, len(req.Seats))
	for _, it := range req.Seats {
		if _, dup := seats[it.LineID]; dup {
			return payment.Split{}, fmt.Errorf("%w: line %d is listed twice", payment.ErrUnassigned, it.LineID)
		}
		seats[it.LineID] = it.Guest
	}
	items := make([]payment.GuestItem, 0, len(seats))
	err := eachBilledLine(st, func(l order.Line) error {
		g, ok := seats[l.ID]
		if !ok {
			return fmt.Errorf("%w: %s (line %d) has no seat", payment.ErrUnassigned, l.Name, l.ID)
		}
		delete(seats, l.ID)
		items = append(items, payment.GuestItem{LineID: l.ID, Amount: l.Due, Guest: g})
		return nil
	})
	if err != nil {
		return payment.Split{}, err
	}
	if err := noneLeft(len(seats)); err != nil {
		return payment.Split{}, err
	}
	return payment.ByGuest(st.Totals.Total, req.Ways, items)
}

// eachBilledLine visits the lines that cost something, in ticket order. A
// voided or comped line costs nothing and needs no payer.
func eachBilledLine(st order.State, visit func(order.Line) error) error {
	for _, l := range st.Lines {
		if l.Due.IsZero() {
			continue
		}
		if err := visit(l); err != nil {
			return err
		}
	}
	return nil
}

// noneLeft refuses lines in the request that are not on the bill.
func noneLeft(extra int) error {
	if extra > 0 {
		return fmt.Errorf("%w: %d lines named are not on the bill", payment.ErrUnassigned, extra)
	}
	return nil
}
