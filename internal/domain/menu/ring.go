package menu

import (
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

// Ringing up from the menu (phase 11, Task 4a, ADR-210; ADR-256). Nothing
// that asks for a line is trusted with money: not a guest's phone, and not a
// till either, which is worked by the least privileged staff. Each says which
// dish, which choices and how many, and the server works out the name, the
// prices, the station and the tax facts from the menu as it stands. The API
// rings every till line through Ring before the order service sees it.

// MaxPickQty is the most of one dish a pick may ask for. It is the guest
// cart's own bound (ADR-208): more than this is a conversation with staff.
const MaxPickQty = 20

// Pick is what someone chose from the menu: a dish, the choices made on it,
// and how many. It carries no name and no price.
type Pick struct {
	ItemID      int64   `json:"item_id"`
	ModifierIDs []int64 `json:"modifier_ids"`
	Qty         int64   `json:"qty"`
}

// Why a pick cannot be rung up.
var (
	// ErrNoSuchItem is a dish the menu does not have: never there, or taken
	// off since the phone read it.
	ErrNoSuchItem = errors.New("the menu has no such item")
	// ErrSoldOut is a dish that cannot be ordered until its next trading day.
	ErrSoldOut = errors.New("the item is sold out")
	// ErrChoices is choices that do not fit the dish: one it does not offer,
	// one given twice, or a group left short of or over its bounds.
	ErrChoices = errors.New("the choices do not fit the item")
	// ErrPickQty is a count below one or above MaxPickQty.
	ErrPickQty = errors.New("the quantity is out of range")
)

// PickError is a pick that was refused, and the dish it was for. Item is
// empty when the menu has no such dish.
type PickError struct {
	ItemID int64
	Item   string
	Err    error
}

func (e *PickError) Error() string {
	if e.Item == "" {
		return fmt.Sprintf("item %d: %v", e.ItemID, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Item, e.Err)
}

func (e *PickError) Unwrap() error { return e.Err }

// Ring turns a pick into the line a till would have sent for it: the name,
// the price, each choice and the tax facts, copied from the menu as it is at
// now. The choices come out in group order and then the menu's, whatever
// order they were asked in, so the same choice is always the same line.
func (m Menu) Ring(p Pick, now time.Time) (order.LineAdded, error) {
	item, ok := m.item(p.ItemID)
	if !ok {
		return order.LineAdded{}, &PickError{ItemID: p.ItemID, Err: ErrNoSuchItem}
	}
	refuse := func(err error) (order.LineAdded, error) {
		return order.LineAdded{}, &PickError{ItemID: item.ID, Item: item.Name, Err: err}
	}
	if p.Qty < 1 || p.Qty > MaxPickQty {
		return refuse(ErrPickQty)
	}
	if item.SoldOutUntil != nil && now.Before(*item.SoldOutUntil) {
		return refuse(ErrSoldOut)
	}

	asked := make(map[int64]bool, len(p.ModifierIDs))
	for _, id := range p.ModifierIDs {
		if asked[id] {
			return refuse(ErrChoices)
		}
		asked[id] = true
	}
	modifiers := []order.Modifier{}
	for _, groupID := range item.Groups {
		group, ok := m.group(groupID)
		if !ok {
			continue
		}
		var chosen int64
		for _, mod := range group.Modifiers {
			if !asked[mod.ID] {
				continue
			}
			chosen++
			delete(asked, mod.ID)
			modifiers = append(modifiers, order.Modifier{ModifierID: mod.ID, Name: mod.Name, Price: mod.Price})
		}
		if chosen < group.MinSelect || chosen > group.MaxSelect {
			return refuse(ErrChoices)
		}
	}
	// Whatever is left was asked for and is in none of the dish's groups.
	if len(asked) > 0 {
		return refuse(ErrChoices)
	}

	return order.LineAdded{
		ItemID: item.ID, Name: item.Name, Qty: p.Qty, UnitPrice: item.Price, Modifiers: modifiers,
		Course: item.Course, Station: item.Station,
		TaxRate: item.TaxRate, HSN: item.HSN, TaxExempt: item.TaxExempt,
	}, nil
}

// LineValue is what a line comes to before discounts and tax: the dish and
// its choices, times how many.
func LineValue(l order.LineAdded) (money.Amount, error) {
	prices := make([]money.Amount, 0, len(l.Modifiers)+1)
	prices = append(prices, l.UnitPrice)
	for _, mod := range l.Modifiers {
		prices = append(prices, mod.Price)
	}
	each, err := money.Sum(l.UnitPrice.Currency, prices...)
	if err != nil {
		return money.Amount{}, err
	}
	return each.Times(l.Qty)
}

func (m Menu) item(id int64) (Item, bool) {
	for _, it := range m.Items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

func (m Menu) group(id int64) (ModifierGroup, bool) {
	for _, g := range m.Groups {
		if g.ID == id {
			return g, true
		}
	}
	return ModifierGroup{}, false
}
