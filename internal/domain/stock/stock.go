// Package stock is what a venue keeps in its stores and how much of it
// there is (phase 10, Task 3, ADR-111).
//
// On hand is never stored. It is the sum of an append-only ledger of
// movements: stock received, adjusted, wasted, counted and (Task 3b) sold.
// A count does not overwrite anything; it records the difference between
// what was counted and what the ledger said, so the history still explains
// every unit.
package stock

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// Unit is what a stock item is measured in. Every quantity is a whole
// number of thousandths of its unit, so 18.5 g of coffee is 18500 and
// half a lime is 500: never a float, as money never is.
type Unit string

// The units. A weight in grams and a volume in millilitres cover a
// kitchen and a bar; anything counted by the piece is each.
const (
	Gram       Unit = "g"
	Millilitre Unit = "ml"
	Each       Unit = "each"
)

// Units is every unit, for the generated vocabulary.
func Units() []Unit { return []Unit{Gram, Millilitre, Each} }

// Qty is an amount of a stock item in thousandths of its unit.
type Qty int64

// PerUnit is one whole unit: 1 g, 1 ml, one of something.
const PerUnit Qty = 1000

// MaxQty bounds any one quantity: a thousand tonnes in grams. Far past any
// cafe's store, and far from overflowing a sum of many movements.
const MaxQty Qty = 1_000_000_000 * PerUnit

// Kind is what a movement records.
type Kind string

// The kinds of movement.
const (
	Received Kind = "received" // delivered or bought in
	Adjusted Kind = "adjusted" // corrected, with a note saying why
	Wasted   Kind = "wasted"   // thrown away, with a reason
	Counted  Kind = "counted"  // counted: the delta is counted minus the ledger
	Sold     Kind = "sold"     // used by a sale (Task 3b), never entered by hand
)

// Kinds is every kind, for the generated vocabulary.
func Kinds() []Kind { return []Kind{Received, Adjusted, Wasted, Counted, Sold} }

// WasteReason is why stock was thrown away: the reason codes a waste
// report groups by.
type WasteReason string

// The reasons.
const (
	Expired WasteReason = "expired"
	Spoiled WasteReason = "spoiled"
	Spilled WasteReason = "spilled"
	Dropped WasteReason = "dropped"
	Other   WasteReason = "other" // needs a note
)

// WasteReasons is every reason, for the generated vocabulary.
func WasteReasons() []WasteReason { return []WasteReason{Expired, Spoiled, Spilled, Dropped, Other} }

// MaxNameLength is the longest stock item name, and MaxNoteLength the
// longest note on a movement, in characters.
const (
	MaxNameLength = 60
	MaxNoteLength = 200
)

// Problem is why something cannot be saved, and the field it is about.
type Problem struct {
	Field   string
	Message string
}

func (p *Problem) Error() string { return p.Field + ": " + p.Message }

// ItemDraft is a stock item as the back office would save it. LowLevel is
// the on-hand amount at or under which it is low; zero never is.
type ItemDraft struct {
	Name     string
	Unit     Unit
	LowLevel Qty
}

// CleanItem trims a draft and checks it.
func CleanItem(d ItemDraft) (ItemDraft, *Problem) {
	name := strings.TrimSpace(d.Name)
	switch {
	case name == "":
		return ItemDraft{}, &Problem{"name", "Give it a name."}
	case utf8.RuneCountInString(name) > MaxNameLength:
		return ItemDraft{}, &Problem{"name", fmt.Sprintf("Keep the name to %d characters.", MaxNameLength)}
	case !validUnit(d.Unit):
		return ItemDraft{}, &Problem{"unit", "Choose grams, millilitres or each."}
	case d.LowLevel < 0 || d.LowLevel > MaxQty:
		return ItemDraft{}, &Problem{"low_level", "The low-stock level cannot be below zero."}
	}
	return ItemDraft{Name: name, Unit: d.Unit, LowLevel: d.LowLevel}, nil
}

func validUnit(u Unit) bool { return slices.Contains(Units(), u) }

// MovementDraft is a movement entered by hand. Qty means what the kind
// says: received and wasted are the amount, above zero; adjusted is the
// signed change; counted is what was counted, zero or more.
type MovementDraft struct {
	Kind   Kind
	Qty    Qty
	Reason WasteReason
	Note   string
}

// Delta is the change a movement makes to on hand, given what the ledger
// says is on hand now, with the draft cleaned; or the problem that stops it.
// A count that finds exactly what the ledger says is a delta of zero, and
// still worth recording: someone checked.
func Delta(d MovementDraft, onHand Qty) (Qty, MovementDraft, *Problem) {
	note := strings.TrimSpace(d.Note)
	if utf8.RuneCountInString(note) > MaxNoteLength {
		return 0, MovementDraft{}, &Problem{"note", fmt.Sprintf("Keep the note to %d characters.", MaxNoteLength)}
	}
	if d.Qty > MaxQty || d.Qty < -MaxQty {
		return 0, MovementDraft{}, &Problem{"qty", "That amount is too large."}
	}
	clean := MovementDraft{Kind: d.Kind, Qty: d.Qty, Note: note}

	switch d.Kind {
	case Received:
		if d.Qty <= 0 {
			return 0, MovementDraft{}, &Problem{"qty", "Give the amount received."}
		}
		return d.Qty, clean, nil
	case Wasted:
		if d.Qty <= 0 {
			return 0, MovementDraft{}, &Problem{"qty", "Give the amount thrown away."}
		}
		if !validReason(d.Reason) {
			return 0, MovementDraft{}, &Problem{"reason", "Choose why it was thrown away."}
		}
		if d.Reason == Other && note == "" {
			return 0, MovementDraft{}, &Problem{"note", "Say what happened."}
		}
		clean.Reason = d.Reason
		return -d.Qty, clean, nil
	case Adjusted:
		if d.Qty == 0 {
			return 0, MovementDraft{}, &Problem{"qty", "Give the change, like 500 or -500."}
		}
		if note == "" {
			return 0, MovementDraft{}, &Problem{"note", "Say why it is being corrected."}
		}
		return d.Qty, clean, nil
	case Counted:
		if d.Qty < 0 {
			return 0, MovementDraft{}, &Problem{"qty", "A count cannot be below zero."}
		}
		return d.Qty - onHand, clean, nil
	case Sold:
		return 0, MovementDraft{}, &Problem{"kind", "Sales are recorded as they are rung up, not by hand."}
	}
	return 0, MovementDraft{}, &Problem{"kind", "Choose received, adjusted, wasted or counted."}
}

func validReason(r WasteReason) bool { return slices.Contains(WasteReasons(), r) }

// IsLow reports whether on hand is at or under an item's low-stock level.
// A level of zero is no level: the item is never low, however little is
// left, because nobody asked to be told.
func IsLow(onHand, lowLevel Qty) bool {
	return lowLevel > 0 && onHand <= lowLevel
}
