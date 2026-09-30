package stock

import (
	"strings"
	"testing"
)

func TestCleanItem(t *testing.T) {
	good := ItemDraft{Name: " Coffee beans ", Unit: Gram, LowLevel: 1000 * PerUnit}
	got, p := CleanItem(good)
	if p != nil || got.Name != "Coffee beans" || got.Unit != Gram || got.LowLevel != 1000*PerUnit {
		t.Fatalf("CleanItem = %+v, %v", got, p)
	}

	tests := []struct {
		name  string
		edit  func(*ItemDraft)
		field string
	}{
		{"no name", func(d *ItemDraft) { d.Name = " " }, "name"},
		{"a name too long", func(d *ItemDraft) { d.Name = strings.Repeat("a", MaxNameLength+1) }, "name"},
		{"no unit", func(d *ItemDraft) { d.Unit = "" }, "unit"},
		{"a unit we do not have", func(d *ItemDraft) { d.Unit = "kg" }, "unit"},
		{"a low level below zero", func(d *ItemDraft) { d.LowLevel = -1 }, "low_level"},
		{"a low level too large", func(d *ItemDraft) { d.LowLevel = MaxQty + 1 }, "low_level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := good
			tt.edit(&d)
			if _, p := CleanItem(d); p == nil || p.Field != tt.field {
				t.Errorf("problem = %v, want one on %q", p, tt.field)
			}
		})
	}
	for _, u := range Units() {
		if _, p := CleanItem(ItemDraft{Name: "x", Unit: u}); p != nil {
			t.Errorf("unit %q refused: %v", u, p)
		}
	}
}

func TestDelta(t *testing.T) {
	const onHand = 5000 * PerUnit
	tests := []struct {
		name  string
		draft MovementDraft
		delta Qty
	}{
		{"received adds", MovementDraft{Kind: Received, Qty: 1000 * PerUnit}, 1000 * PerUnit},
		{"a fraction of a unit", MovementDraft{Kind: Received, Qty: 500}, 500},
		{"wasted takes away", MovementDraft{Kind: Wasted, Qty: 200 * PerUnit, Reason: Spilled}, -200 * PerUnit},
		{"other waste with a note", MovementDraft{Kind: Wasted, Qty: 1, Reason: Other, Note: "Fell in the sink"}, -1},
		{"adjusted up", MovementDraft{Kind: Adjusted, Qty: 300, Note: "Miscounted"}, 300},
		{"adjusted down", MovementDraft{Kind: Adjusted, Qty: -300, Note: "Miscounted"}, -300},
		{"a count under the ledger", MovementDraft{Kind: Counted, Qty: 4800 * PerUnit}, -200 * PerUnit},
		{"a count over the ledger", MovementDraft{Kind: Counted, Qty: 5100 * PerUnit}, 100 * PerUnit},
		{"a count that agrees", MovementDraft{Kind: Counted, Qty: onHand}, 0},
		{"a count of nothing", MovementDraft{Kind: Counted, Qty: 0}, -onHand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, _, p := Delta(tt.draft, onHand)
			if p != nil || delta != tt.delta {
				t.Errorf("Delta = %d, %v; want %d", delta, p, tt.delta)
			}
		})
	}
}

func TestDeltaCleansTheDraft(t *testing.T) {
	_, clean, p := Delta(MovementDraft{Kind: Wasted, Qty: 1, Reason: Expired, Note: "  past its date "}, 0)
	if p != nil || clean.Note != "past its date" || clean.Reason != Expired {
		t.Errorf("clean = %+v, %v", clean, p)
	}
	_, clean, _ = Delta(MovementDraft{Kind: Received, Qty: 1, Reason: Expired}, 0)
	if clean.Reason != "" {
		t.Error("a reason means nothing on anything but waste, so it is dropped")
	}
}

func TestDeltaRefuses(t *testing.T) {
	tests := []struct {
		name  string
		draft MovementDraft
		field string
	}{
		{"nothing received", MovementDraft{Kind: Received, Qty: 0}, "qty"},
		{"a negative delivery", MovementDraft{Kind: Received, Qty: -1}, "qty"},
		{"nothing wasted", MovementDraft{Kind: Wasted, Qty: 0, Reason: Spilled}, "qty"},
		{"waste with no reason", MovementDraft{Kind: Wasted, Qty: 1}, "reason"},
		{"waste with a made-up reason", MovementDraft{Kind: Wasted, Qty: 1, Reason: "eaten"}, "reason"},
		{"other waste with no note", MovementDraft{Kind: Wasted, Qty: 1, Reason: Other, Note: " "}, "note"},
		{"an adjustment of nothing", MovementDraft{Kind: Adjusted, Qty: 0, Note: "x"}, "qty"},
		{"an adjustment with no note", MovementDraft{Kind: Adjusted, Qty: 5}, "note"},
		{"a count below zero", MovementDraft{Kind: Counted, Qty: -1}, "qty"},
		{"a sale by hand", MovementDraft{Kind: Sold, Qty: 1}, "kind"},
		{"no kind", MovementDraft{Qty: 1}, "kind"},
		{"a note too long", MovementDraft{Kind: Adjusted, Qty: 1, Note: strings.Repeat("a", MaxNoteLength+1)}, "note"},
		{"too large", MovementDraft{Kind: Received, Qty: MaxQty + 1}, "qty"},
		{"too small", MovementDraft{Kind: Adjusted, Qty: -MaxQty - 1, Note: "x"}, "qty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, p := Delta(tt.draft, 0)
			if p == nil || p.Field != tt.field {
				t.Errorf("problem = %v, want one on %q", p, tt.field)
			}
			if p != nil && p.Error() == "" {
				t.Error("a problem must say something")
			}
		})
	}
}

func TestIsLow(t *testing.T) {
	tests := []struct {
		name          string
		onHand, level Qty
		want          bool
	}{
		{"above the level", 2000, 1000, false},
		{"at the level", 1000, 1000, true},
		{"under the level", 999, 1000, true},
		{"below zero", -5, 1000, true},
		{"no level, nothing left", 0, 0, false},
		{"no level, below zero", -5, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLow(tt.onHand, tt.level); got != tt.want {
				t.Errorf("IsLow(%d, %d) = %v", tt.onHand, tt.level, got)
			}
		})
	}
}
