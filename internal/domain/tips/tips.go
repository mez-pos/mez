// Package tips is phase 09, Task 4a: what each staff member took in tips
// during a shift, and how the shift's pool is split among the people who
// worked it. Tips are owed to staff; nothing here touches sales.
//
// A split is exact to the paisa: money.Allocate's largest remainder gives
// every share, and the shares always add up to the pool.
package tips

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
)

// Rule is how a pool is split.
type Rule string

// The rules.
const (
	Even         Rule = "even"        // the same to everyone
	ByHours      Rule = "hours"       // in proportion to the time each worked
	ByRoleWeight Rule = "role_weight" // in proportion to each role's weight
)

// Rules is every rule, for the generated vocabulary.
func Rules() []Rule { return []Rule{Even, ByHours, ByRoleWeight} }

// ErrInvalidPool refuses a split that cannot be made.
var ErrInvalidPool = errors.New("invalid tip pool")

// MaxMinutes bounds one person's time in a shift: a day and a half.
const MaxMinutes = 36 * 60

// Member is someone in the pool, with what the rule weighs them by.
type Member struct {
	StaffID int64 `json:"staff_id"`
	Minutes int64 `json:"minutes"` // time worked, for ByHours
	Weight  int64 `json:"weight"`  // their role's weight, for ByRoleWeight
}

// Share is a member's part of the pool.
type Share struct {
	Member
	Share money.Amount `json:"share"`
}

// Split divides the pool among the members by the rule. Each member
// appears once; ByHours needs everyone's minutes and ByRoleWeight
// everyone's weight above nothing, since a member worth nothing should not
// be in the pool.
func Split(pool money.Amount, rule Rule, members []Member) ([]Share, error) {
	if pool.Minor < 0 {
		return nil, fmt.Errorf("%w: a pool of %s", ErrInvalidPool, pool)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("%w: nobody is in it", ErrInvalidPool)
	}
	weights := make([]int64, len(members))
	for i, m := range members {
		if m.StaffID < 1 {
			return nil, fmt.Errorf("%w: member %d has no staff member", ErrInvalidPool, i)
		}
		if slices.ContainsFunc(members[:i], func(o Member) bool { return o.StaffID == m.StaffID }) {
			return nil, fmt.Errorf("%w: staff %d is in it twice", ErrInvalidPool, m.StaffID)
		}
		if m.Minutes < 0 || m.Minutes > MaxMinutes {
			return nil, fmt.Errorf("%w: staff %d worked %d minutes", ErrInvalidPool, m.StaffID, m.Minutes)
		}
		w, err := weightOf(rule, m)
		if err != nil {
			return nil, err
		}
		weights[i] = w
	}
	parts, err := money.Allocate(pool, weights)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPool, err)
	}
	out := make([]Share, len(members))
	for i, m := range members {
		out[i] = Share{Member: m, Share: parts[i]}
	}
	return out, nil
}

func weightOf(rule Rule, m Member) (int64, error) {
	switch rule {
	case Even:
		return 1, nil
	case ByHours:
		if m.Minutes < 1 {
			return 0, fmt.Errorf("%w: staff %d has no hours to split by", ErrInvalidPool, m.StaffID)
		}
		return m.Minutes, nil
	case ByRoleWeight:
		if m.Weight < 1 {
			return 0, fmt.Errorf("%w: staff %d's role weighs nothing", ErrInvalidPool, m.StaffID)
		}
		return m.Weight, nil
	}
	return 0, fmt.Errorf("%w: no rule %q", ErrInvalidPool, rule)
}

// Taken is the tips each staff member took on their tenders in the shift,
// less the tips of tenders voided in it, by staff ID. A tender voided in a
// later shift takes its tip off that shift's pool, not this one's.
func Taken(currency string, shiftID int64, orders [][]shift.StampedEvent) (map[int64]money.Amount, error) {
	zero, err := money.New(0, currency)
	if err != nil {
		return nil, err
	}
	out := map[int64]money.Amount{}
	add := func(by int64, tip money.Amount, sign int64) error {
		if tip == (money.Amount{}) {
			return nil
		}
		cur, ok := out[by]
		if !ok {
			cur = zero
		}
		if sign < 0 {
			cur, err = cur.Sub(tip)
		} else {
			cur, err = cur.Add(tip)
		}
		out[by] = cur
		return err
	}
	for _, events := range orders {
		type tender struct {
			by  int64
			tip money.Amount
		}
		tenders := map[int64]tender{}
		for _, se := range events {
			switch p := se.Event.Payload.(type) {
			case order.PaymentTendered:
				tenders[se.Event.Seq] = tender{by: p.TenderedBy, tip: p.Tip}
				if se.ShiftID == shiftID {
					if err := add(p.TenderedBy, p.Tip, 1); err != nil {
						return nil, fmt.Errorf("tips: %w", err)
					}
				}
			case order.PaymentVoided:
				t, ok := tenders[p.PaymentID]
				if ok && se.ShiftID == shiftID {
					if err := add(t.by, t.tip, -1); err != nil {
						return nil, fmt.Errorf("tips: %w", err)
					}
				}
			}
		}
	}
	return out, nil
}
