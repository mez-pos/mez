package menus

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// The menu from a spreadsheet (phase 10, Task 7b, ADR-288). menu.ReadImport
// reads the file; this reads what the venue has for it to be read against,
// and writes the plan it answers.

// MaxImportBytes is the largest file an import reads: a thousand rows of a
// menu are well under it.
const MaxImportBytes = 1 << 20

// ErrImportRefused is an import of a file with a problem in it. Nothing was
// written; the Import returned with it says what the problems are.
var ErrImportRefused = errors.New("the file has problems, so nothing was imported")

// errNothingToImport ends an import that has nothing to add without a
// write, so the menu's version stays where it is.
var errNothingToImport = errors.New("nothing to import")

// Import is what a file would add to the menu, or has added.
type Import struct {
	ToAdd []ImportItem `json:"to_add"`
	// NewCategories are the categories the file names that the menu does
	// not have, in the order the file first names them.
	NewCategories []string `json:"new_categories"`
	// Skipped are the rows left out because the menu already has that item.
	Skipped  []ImportSkipped `json:"skipped"`
	Problems []ImportProblem `json:"problems"`
	// Imported: ToAdd and NewCategories are on the menu now. False for a
	// preview, a refusal, and an import that found nothing to add.
	Imported bool `json:"imported"`
}

// ImportItem is one item a file adds.
type ImportItem struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row      int          `json:"row"`
	Category string       `json:"category"`
	Name     string       `json:"name"`
	Price    money.Amount `json:"price"`
}

// ImportSkipped is a row whose item is already on the menu.
type ImportSkipped struct {
	Row      int    `json:"row"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

// ImportProblem is why a row cannot be added. Row 0 is the file as a whole,
// and has no column.
type ImportProblem struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// PreviewImport reads a file and answers what importing it would do. It
// writes nothing.
func (s *Service) PreviewImport(ctx context.Context, venueID int64, data []byte) (Import, error) {
	plan, currency, err := s.planImport(ctx, s.db.Queries(), venueID, data)
	if err != nil {
		return Import{}, err
	}
	return importOf(plan, currency), nil
}

// ImportMenu adds a file's items to the menu, and the categories they need,
// as one edit: all of it at one new version, or none of it. A file with any
// problem is refused with ErrImportRefused and the problems.
//
// The file is read again here, inside the write, against the menu as it is
// now: a preview is not trusted, so a menu edited since cannot be imported
// over. Sending the same file twice adds nothing the second time.
func (s *Service) ImportMenu(ctx context.Context, venueID int64, data []byte) (Import, error) {
	var out Import
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		plan, currency, err := s.planImport(ctx, q, venueID, data)
		if err != nil {
			return err
		}
		out = importOf(plan, currency)
		switch {
		case len(plan.Problems) > 0:
			return ErrImportRefused
		case len(plan.Rows) == 0:
			return errNothingToImport
		}
		return s.writeImport(ctx, q, venueID, plan)
	})
	switch {
	case errors.Is(err, errNothingToImport):
		return out, nil
	case errors.Is(err, ErrImportRefused):
		return out, err
	case err != nil:
		return Import{}, err
	}
	out.Imported = true
	return out, nil
}

// planImport reads the venue through q and the file against it.
func (s *Service) planImport(ctx context.Context, q *gen.Queries, venueID int64, data []byte) (menu.ImportPlan, string, error) {
	if len(data) > MaxImportBytes {
		return menu.ImportPlan{}, "", &Invalid{"file", fmt.Sprintf("That file is over %d KB.", MaxImportBytes>>10)}
	}
	v, err := importVenue(ctx, q, venueID)
	if err != nil {
		return menu.ImportPlan{}, "", err
	}
	return menu.ReadImport(data, v), v.Currency, nil
}

// importVenue is what the venue has, as a file is read against it.
func importVenue(ctx context.Context, q *gen.Queries, venueID int64) (menu.ImportVenue, error) {
	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return menu.ImportVenue{}, fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return menu.ImportVenue{}, fmt.Errorf("menus: reading the venue's tax profile: %w", err)
	}
	out := menu.ImportVenue{Currency: venue.Currency, NeedsHSN: tax.NeedsHSN(taxes.Profile)}

	categories, err := q.EditorCategories(ctx, venueID)
	if err != nil {
		return menu.ImportVenue{}, fmt.Errorf("menus: reading categories: %w", err)
	}
	names := make(map[int64]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
		out.Categories = append(out.Categories, menu.ImportChoice{ID: c.ID, Name: c.Name, Active: c.Active})
	}
	items, err := q.EditorItems(ctx, venueID)
	if err != nil {
		return menu.ImportVenue{}, fmt.Errorf("menus: reading items: %w", err)
	}
	for _, it := range items {
		out.Items = append(out.Items, menu.ImportHave{Category: names[it.CategoryID], Name: it.Name})
	}
	return out, importChoices(ctx, q, venueID, &out)
}

// importChoices reads what a row may name besides its category. A station
// also answers to its code and a course to its number.
func importChoices(ctx context.Context, q *gen.Queries, venueID int64, out *menu.ImportVenue) error {
	classes, err := q.TaxClassesForVenue(ctx, venueID)
	if err != nil {
		return fmt.Errorf("menus: reading tax classes: %w", err)
	}
	for _, c := range classes {
		out.TaxClasses = append(out.TaxClasses, menu.ImportChoice{ID: c.ID, Name: c.Name, Active: c.Active})
	}
	stations, err := q.EditorStations(ctx, venueID)
	if err != nil {
		return fmt.Errorf("menus: reading stations: %w", err)
	}
	for _, st := range stations {
		out.Stations = append(out.Stations, menu.ImportChoice{ID: st.ID, Name: st.Name, Also: st.Code, Active: st.Active})
	}
	courses, err := q.EditorCourses(ctx, venueID)
	if err != nil {
		return fmt.Errorf("menus: reading courses: %w", err)
	}
	for _, c := range courses {
		out.Courses = append(out.Courses, menu.ImportChoice{ID: c.ID, Name: c.Name, Also: strconv.FormatInt(c.Number, 10), Active: c.Active})
	}
	return nil
}

// writeImport writes a plan with no problems: its new categories at the end
// of the list in the file's order, then each item at the end of its
// category in the file's order.
func (s *Service) writeImport(ctx context.Context, q *gen.Queries, venueID int64, plan menu.ImportPlan) error {
	now := s.now()
	made := make(map[string]int64, len(plan.NewCategories))
	for _, name := range plan.NewCategories {
		next, err := q.NextCategorySort(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: next category place: %w", err)
		}
		id, err := q.CreateCategory(ctx, gen.CreateCategoryParams{VenueID: venueID, Name: name, SortOrder: next, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return fmt.Errorf("menus: importing category %q: %w", name, err)
		}
		made[strings.ToLower(name)] = id
	}

	for _, row := range plan.Rows {
		category := row.CategoryID
		if category == 0 {
			category = made[strings.ToLower(row.Category)]
		}
		next, err := q.NextItemSort(ctx, gen.NextItemSortParams{VenueID: venueID, CategoryID: category})
		if err != nil {
			return fmt.Errorf("menus: next item place: %w", err)
		}
		if _, err := q.CreateItem(ctx, gen.CreateItemParams{
			VenueID: venueID, CategoryID: category, Name: row.Name, Price: row.Price, Hsn: row.HSN,
			TaxClassID: row.TaxClassID, StationID: row.StationID, CourseID: row.CourseID, SortOrder: next,
			CreatedAt: now, UpdatedAt: now,
			Description: row.Description, Diet: string(row.Diet), GlutenFree: row.GlutenFree, Popular: row.Popular,
		}); err != nil {
			return fmt.Errorf("menus: importing row %d (%s): %w", row.Row, row.Name, err)
		}
	}
	return nil
}

// importOf is a plan as the back office reads it. Every list is there, if
// empty, so the screen need not tell none from missing.
func importOf(plan menu.ImportPlan, currency string) Import {
	out := Import{
		ToAdd:         make([]ImportItem, len(plan.Rows)),
		NewCategories: append([]string{}, plan.NewCategories...),
		Skipped:       make([]ImportSkipped, len(plan.Skipped)),
		Problems:      make([]ImportProblem, len(plan.Problems)),
	}
	for i, r := range plan.Rows {
		out.ToAdd[i] = ImportItem{Row: r.Row, Category: r.Category, Name: r.Name, Price: money.Amount{Minor: r.Price, Currency: currency}}
	}
	for i, sk := range plan.Skipped {
		out.Skipped[i] = ImportSkipped(sk)
	}
	for i, p := range plan.Problems {
		out.Problems[i] = ImportProblem(p)
	}
	return out
}

// ImportTemplate is the example rows of the file the venue fills in, under
// menu.ImportColumns: rows that name the venue's own category, tax class,
// station and course, so the template imports as it comes.
func (s *Service) ImportTemplate(ctx context.Context, venueID int64) ([][]string, error) {
	v, err := importVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return nil, err
	}
	return menu.ImportTemplate(v), nil
}
