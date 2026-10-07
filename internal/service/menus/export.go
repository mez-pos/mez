package menus

import (
	"context"
	"slices"

	"github.com/byjoellobo/mez/internal/domain/menu"
)

// ExportMenu is the venue's menu as the rows of a file (phase 10, Task 8a,
// ADR-293), under menu.ExportColumns: every item, retired ones included, by
// category in the menu's order and then in each category's own. It is read
// as the editor reads it, in one read, so the rows are one version's.
func (s *Service) ExportMenu(ctx context.Context, venueID int64) ([][]string, error) {
	e, err := s.Editor(ctx, venueID)
	if err != nil {
		return nil, err
	}
	categories := make(map[int64]string, len(e.Categories))
	place := make(map[int64]int, len(e.Categories))
	for i, c := range e.Categories {
		categories[c.ID], place[c.ID] = c.Name, i
	}
	taxClasses := make(map[int64]string, len(e.TaxClasses))
	for _, c := range e.TaxClasses {
		taxClasses[c.ID] = c.Name
	}
	stations := make(map[int64]string, len(e.Stations))
	for _, st := range e.Stations {
		stations[st.ID] = st.Name
	}
	courses := make(map[int64]string, len(e.Courses))
	for _, c := range e.Courses {
		courses[c.ID] = c.Name
	}

	// A retired group is not named: a row could not offer it (ADR-302).
	groups := make(map[int64]string, len(e.Groups))
	for _, g := range e.Groups {
		if g.Active {
			groups[g.ID] = g.Name
		}
	}

	// Stable: within a category the editor's order, which is the menu's, is kept.
	items := slices.Clone(e.Items)
	slices.SortStableFunc(items, func(a, b Item) int { return place[a.CategoryID] - place[b.CategoryID] })

	out := make([]menu.ExportItem, len(items))
	for i, it := range items {
		var choices []string
		for _, id := range it.ModifierGroups {
			if name, ok := groups[id]; ok {
				choices = append(choices, name)
			}
		}
		out[i] = menu.ExportItem{
			Category: categories[it.CategoryID], Name: it.Name, Price: it.Price,
			TaxClass: taxClasses[it.TaxClassID], HSN: it.HSN, Station: stations[it.StationID], Course: courses[it.CourseID],
			Description: it.Description, Diet: it.Diet, GlutenFree: it.GlutenFree, Popular: it.Popular, Choices: choices, Active: it.Active,
		}
	}
	return menu.ExportRows(out), nil
}
