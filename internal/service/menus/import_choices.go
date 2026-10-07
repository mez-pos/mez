package menus

import (
	"context"
	"errors"
	"fmt"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
)

// Groups of choices from a spreadsheet (phase 10, Task 11, ADR-302).
// menu.ReadChoices reads the file; this reads the menu's groups for it to be
// read against, and writes the groups it answers. It is the import's shape
// (ADR-289): a preview that writes nothing, and an import that reads the
// file again inside the write.

// ChoicesImport is the groups a file would add, or has added.
type ChoicesImport struct {
	ToAdd []ChoicesImportGroup `json:"to_add"`
	// Skipped are the rows left out because the menu already has a group of
	// that name.
	Skipped  []ChoicesImportSkipped `json:"skipped"`
	Problems []ImportProblem        `json:"problems"`
	// Imported: ToAdd is on the menu now, offered by no item yet. False for
	// a preview, a refusal, and a file that had nothing new.
	Imported bool `json:"imported"`
}

// ChoicesImportGroup is a group a file adds, with its choices in order.
type ChoicesImportGroup struct {
	// Row is the row that first names it; the header is row 1.
	Row       int                   `json:"row"`
	Name      string                `json:"name"`
	MinSelect int64                 `json:"min_select"`
	MaxSelect int64                 `json:"max_select"`
	Choices   []ChoicesImportChoice `json:"choices"`
}

// ChoicesImportChoice is one choice of a group a file adds. Price is what
// it adds to the line, and may be below zero.
type ChoicesImportChoice struct {
	Name      string       `json:"name"`
	Price     money.Amount `json:"price"`
	IsDefault bool         `json:"is_default"`
}

// ChoicesImportSkipped is a row whose group is already on the menu.
type ChoicesImportSkipped struct {
	Row   int    `json:"row"`
	Group string `json:"group"`
}

// PreviewChoices reads a file of choices and answers what importing it
// would do. It writes nothing.
func (s *Service) PreviewChoices(ctx context.Context, venueID int64, data []byte) (ChoicesImport, error) {
	plan, currency, err := planChoices(ctx, s.db.Queries(), venueID, data)
	if err != nil {
		return ChoicesImport{}, err
	}
	return choicesImportOf(plan, currency), nil
}

// ImportChoices adds a file's groups and their choices to the menu as one
// edit: all of them at one new version, or none. A group is added offered
// by no item: an item offers it from its editor, or from the Choices column
// of a file of items imported afterwards. A file with any problem is
// refused with ErrImportRefused and the problems. The same file twice adds
// nothing the second time.
func (s *Service) ImportChoices(ctx context.Context, venueID int64, data []byte) (ChoicesImport, error) {
	var out ChoicesImport
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		plan, currency, err := planChoices(ctx, q, venueID, data)
		if err != nil {
			return err
		}
		out = choicesImportOf(plan, currency)
		switch {
		case len(plan.Problems) > 0:
			return ErrImportRefused
		case len(plan.Groups) == 0:
			return errNothingToImport
		}
		now := s.now()
		for _, g := range plan.Groups {
			id, err := q.CreateModifierGroup(ctx, gen.CreateModifierGroupParams{
				VenueID: venueID, Name: g.Group.Name, MinSelect: g.Group.MinSelect, MaxSelect: g.Group.MaxSelect, CreatedAt: now, UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("menus: importing the group %q (row %d): %w", g.Group.Name, g.Row, err)
			}
			// Every choice is new: none has an id.
			if err := s.writeChoices(ctx, q, venueID, id, g.Group, make([]ModifierInput, len(g.Group.Choices)), nil); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errNothingToImport):
		return out, nil
	case errors.Is(err, ErrImportRefused):
		return out, err
	case err != nil:
		return ChoicesImport{}, err
	}
	out.Imported = true
	return out, nil
}

// ChoicesTemplate is the example rows of the file a venue fills in, under
// menu.ChoiceColumns.
func (s *Service) ChoicesTemplate() [][]string { return menu.ChoicesTemplate() }

// planChoices reads the menu's groups through q and the file against them.
func planChoices(ctx context.Context, q *gen.Queries, venueID int64, data []byte) (menu.ChoicesPlan, string, error) {
	if err := checkFileSize(data); err != nil {
		return menu.ChoicesPlan{}, "", err
	}
	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return menu.ChoicesPlan{}, "", fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	groups, err := q.EditorModifierGroups(ctx, venueID)
	if err != nil {
		return menu.ChoicesPlan{}, "", fmt.Errorf("menus: reading the groups of choices: %w", err)
	}
	v := menu.ChoicesVenue{Currency: venue.Currency, Groups: make([]string, len(groups))}
	for i, g := range groups {
		v.Groups[i] = g.Name
	}
	return menu.ReadChoices(data, v), venue.Currency, nil
}

// choicesImportOf is a plan as the back office reads it. Every list is
// there, if empty.
func choicesImportOf(plan menu.ChoicesPlan, currency string) ChoicesImport {
	out := ChoicesImport{
		ToAdd:    make([]ChoicesImportGroup, len(plan.Groups)),
		Skipped:  make([]ChoicesImportSkipped, len(plan.Skipped)),
		Problems: make([]ImportProblem, len(plan.Problems)),
	}
	for i, g := range plan.Groups {
		group := ChoicesImportGroup{
			Row: g.Row, Name: g.Group.Name, MinSelect: g.Group.MinSelect, MaxSelect: g.Group.MaxSelect,
			Choices: make([]ChoicesImportChoice, len(g.Group.Choices)),
		}
		for j, c := range g.Group.Choices {
			group.Choices[j] = ChoicesImportChoice{Name: c.Name, Price: money.Amount{Minor: c.Price, Currency: currency}, IsDefault: c.IsDefault}
		}
		out.ToAdd[i] = group
	}
	for i, sk := range plan.Skipped {
		out.Skipped[i] = ChoicesImportSkipped(sk)
	}
	for i, p := range plan.Problems {
		out.Problems[i] = ImportProblem(p)
	}
	return out
}
