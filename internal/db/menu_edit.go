package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
)

// MenuEditorRows is everything the back office's menu editor shows, retired
// rows included, at one version.
type MenuEditorRows struct {
	Version    int64
	Stations   []gen.EditorStationsRow
	Courses    []gen.EditorCoursesRow
	TaxClasses []gen.TaxClassesForVenueRow
	Categories []gen.EditorCategoriesRow
	Items      []gen.EditorItemsRow
	Links      []gen.ItemModifierGroupsForVenueRow
	Groups     []gen.EditorModifierGroupsRow
	// Modifiers are the choices on offer; a retired one is gone from its
	// group's form, and comes back only by being added again.
	Modifiers []gen.ActiveModifiersRow
}

// MenuEditor reads the editor's rows in one read-only transaction, so the
// version is the version of the rows.
func (d *DB) MenuEditor(ctx context.Context, venueID int64) (MenuEditorRows, error) {
	tx, err := d.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return MenuEditorRows{}, fmt.Errorf("db: starting the menu editor read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := gen.New(tx)
	var out MenuEditorRows
	steps := []struct {
		what string
		run  func() error
	}{
		{"version", func() (err error) { out.Version, err = q.MenuVersion(ctx, venueID); return }},
		{"stations", func() (err error) { out.Stations, err = q.EditorStations(ctx, venueID); return }},
		{"courses", func() (err error) { out.Courses, err = q.EditorCourses(ctx, venueID); return }},
		{"tax classes", func() (err error) { out.TaxClasses, err = q.TaxClassesForVenue(ctx, venueID); return }},
		{"categories", func() (err error) { out.Categories, err = q.EditorCategories(ctx, venueID); return }},
		{"items", func() (err error) { out.Items, err = q.EditorItems(ctx, venueID); return }},
		{"modifier links", func() (err error) { out.Links, err = q.ItemModifierGroupsForVenue(ctx, venueID); return }},
		{"modifier groups", func() (err error) { out.Groups, err = q.EditorModifierGroups(ctx, venueID); return }},
		{"modifiers", func() (err error) { out.Modifiers, err = q.ActiveModifiers(ctx, venueID); return }},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return MenuEditorRows{}, fmt.Errorf("db: reading the menu editor's %s for venue %d: %w", s.what, venueID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return MenuEditorRows{}, fmt.Errorf("db: finishing the menu editor read: %w", err)
	}
	return out, nil
}

// EditMenu runs one edit to the venue's menu and bumps the menu version in
// the same transaction, returning the new version. Every write to a menu
// table from the back office goes through here, so none can leave a
// terminal selling from a menu it believes is current. If fn fails,
// nothing is written and the version stays.
func (d *DB) EditMenu(ctx context.Context, venueID int64, fn func(q *gen.Queries) error) (int64, error) {
	var version int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		if err := fn(q); err != nil {
			return err
		}
		v, err := q.BumpMenuVersion(ctx, venueID)
		if err != nil {
			return fmt.Errorf("db: bumping the menu version of venue %d: %w", venueID, err)
		}
		version = v
		return nil
	})
	if err != nil {
		return 0, err
	}
	return version, nil
}
