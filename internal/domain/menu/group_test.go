package menu

import (
	"strings"
	"testing"
)

func milk() GroupDraft {
	return GroupDraft{Name: " Milk ", MinSelect: 1, MaxSelect: 1, Choices: []ChoiceDraft{
		{Name: " Dairy", IsDefault: true},
		{Name: "Oat ", Price: 4000},
		{Name: "No milk", Price: -1000},
	}}
}

func TestCleanGroupAccepts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*GroupDraft)
	}{
		{"exactly one, with a default", func(*GroupDraft) {}},
		{"optional, none picked to start", func(d *GroupDraft) { d.MinSelect = 0; d.Choices[0].IsDefault = false }},
		{"pick every one", func(d *GroupDraft) { d.MinSelect = 3; d.MaxSelect = 3 }},
		{"more allowed than offered", func(d *GroupDraft) { d.MaxSelect = 5 }},
		{"as many defaults as allowed", func(d *GroupDraft) { d.MaxSelect = 2; d.Choices[1].IsDefault = true }},
		{"a negative price", func(d *GroupDraft) { d.Choices[2].Price = -99999 }},
		{"a name of the longest length", func(d *GroupDraft) { d.Name = strings.Repeat("é", MaxNameLength) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := milk()
			tt.edit(&d)
			if _, p := CleanGroup(d); p != nil {
				t.Errorf("refused: %v", p)
			}
		})
	}
}

func TestCleanGroupTrims(t *testing.T) {
	got, p := CleanGroup(milk())
	if p != nil {
		t.Fatal(p)
	}
	if got.Name != "Milk" || got.Choices[0].Name != "Dairy" || got.Choices[1].Name != "Oat" || got.Choices[2].Price != -1000 || !got.Choices[0].IsDefault {
		t.Errorf("cleaned = %+v", got)
	}
	if orig := milk(); orig.Name != " Milk " {
		t.Error("the draft passed in must not change")
	}
}

func TestCleanGroupRefuses(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*GroupDraft)
		field string
	}{
		{"no name", func(d *GroupDraft) { d.Name = "  " }, "name"},
		{"a name too long", func(d *GroupDraft) { d.Name = strings.Repeat("a", MaxNameLength+1) }, "name"},
		{"fewest below zero", func(d *GroupDraft) { d.MinSelect = -1 }, "min_select"},
		{"most of zero", func(d *GroupDraft) { d.MinSelect = 0; d.MaxSelect = 0 }, "max_select"},
		{"most below fewest", func(d *GroupDraft) { d.MinSelect = 2; d.MaxSelect = 1 }, "max_select"},
		{"no choices", func(d *GroupDraft) { d.Choices = nil }, "modifiers"},
		{"asks for more than it offers", func(d *GroupDraft) { d.MinSelect = 4; d.MaxSelect = 4 }, "min_select"},
		{"a nameless choice", func(d *GroupDraft) { d.Choices[1].Name = "" }, "modifiers.1.name"},
		{"a choice named too long", func(d *GroupDraft) { d.Choices[2].Name = strings.Repeat("a", MaxNameLength+1) }, "modifiers.2.name"},
		{"two choices alike, whatever the case", func(d *GroupDraft) { d.Choices[2].Name = " oat" }, "modifiers.2.name"},
		{"more picked to start than allowed", func(d *GroupDraft) { d.Choices[1].IsDefault = true }, "modifiers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := milk()
			tt.edit(&d)
			_, p := CleanGroup(d)
			if p == nil || p.Field != tt.field {
				t.Errorf("problem = %v, want one on %q", p, tt.field)
			}
			if p != nil && p.Error() == "" {
				t.Error("a problem must say something")
			}
		})
	}
}
