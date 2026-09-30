package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxNameLength is the longest group or choice name, in characters: it has
// to fit a button on the choice sheet and a line on a kitchen ticket.
const MaxNameLength = 60

// GroupDraft is a modifier group as the back office would save it: its
// bounds and its choices, in order. Prices are minor units of the venue's
// currency and may be negative ("no cheese, less 20").
type GroupDraft struct {
	Name      string
	MinSelect int64
	MaxSelect int64
	Choices   []ChoiceDraft
}

// ChoiceDraft is one choice in a GroupDraft.
type ChoiceDraft struct {
	Name      string
	Price     int64
	IsDefault bool
}

// GroupProblem is why a draft cannot be saved, and which field it is about:
// "name", "min_select", "max_select", "modifiers", or one choice's field as
// "modifiers.2.name" (zero-based), so a form can show it on that row.
type GroupProblem struct {
	Field   string
	Message string
}

func (p *GroupProblem) Error() string { return p.Field + ": " + p.Message }

// CleanGroup trims a draft's names and checks it can be offered on a till:
// a group whose bounds its own choices cannot meet would leave a choice
// sheet that can never be completed, or one that opens already invalid.
// It returns the cleaned draft, or the first problem found.
func CleanGroup(d GroupDraft) (GroupDraft, *GroupProblem) {
	name, p := cleanName("name", d.Name)
	if p != nil {
		return GroupDraft{}, p
	}
	out := GroupDraft{Name: name, MinSelect: d.MinSelect, MaxSelect: d.MaxSelect, Choices: make([]ChoiceDraft, len(d.Choices))}

	switch {
	case d.MinSelect < 0:
		return GroupDraft{}, &GroupProblem{"min_select", "The fewest to pick cannot be below zero."}
	case d.MaxSelect < 1:
		return GroupDraft{}, &GroupProblem{"max_select", "Allow at least one to be picked."}
	case d.MaxSelect < d.MinSelect:
		return GroupDraft{}, &GroupProblem{"max_select", "The most to pick cannot be fewer than the fewest."}
	case len(d.Choices) == 0:
		return GroupDraft{}, &GroupProblem{"modifiers", "Add at least one choice."}
	case int64(len(d.Choices)) < d.MinSelect:
		return GroupDraft{}, &GroupProblem{"min_select", fmt.Sprintf("It asks for %d but offers only %d.", d.MinSelect, len(d.Choices))}
	}

	seen := make(map[string]int, len(d.Choices))
	defaults := int64(0)
	for i, c := range d.Choices {
		field := fmt.Sprintf("modifiers.%d.name", i)
		cname, p := cleanName(field, c.Name)
		if p != nil {
			return GroupDraft{}, p
		}
		key := strings.ToLower(cname)
		if _, dup := seen[key]; dup {
			return GroupDraft{}, &GroupProblem{field, "Two choices have this name."}
		}
		seen[key] = i
		if c.IsDefault {
			defaults++
		}
		out.Choices[i] = ChoiceDraft{Name: cname, Price: c.Price, IsDefault: c.IsDefault}
	}
	if defaults > d.MaxSelect {
		return GroupDraft{}, &GroupProblem{"modifiers", fmt.Sprintf("%d choices are picked to start with, but at most %d may be picked.", defaults, d.MaxSelect)}
	}
	return out, nil
}

func cleanName(field, name string) (string, *GroupProblem) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", &GroupProblem{field, "Give it a name."}
	case utf8.RuneCountInString(name) > MaxNameLength:
		return "", &GroupProblem{field, fmt.Sprintf("Keep the name to %d characters.", MaxNameLength)}
	}
	return name, nil
}
