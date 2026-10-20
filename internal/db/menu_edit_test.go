package db

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
)

func TestEditMenuBumpsTheVersionWithTheEdit(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()

	version, err := d.EditMenu(ctx, 1, func(q *gen.Queries) error {
		_, err := q.CreateCategory(ctx, gen.CreateCategoryParams{VenueID: 1, Name: "Coffee", CreatedAt: menuNow, UpdatedAt: menuNow})
		return err
	})
	if err != nil || version != 2 {
		t.Fatalf("EditMenu = %d, %v; want version 2", version, err)
	}

	refused := errors.New("refused")
	_, err = d.EditMenu(ctx, 1, func(q *gen.Queries) error {
		if _, err := q.CreateCategory(ctx, gen.CreateCategoryParams{VenueID: 1, Name: "Tea", CreatedAt: menuNow, UpdatedAt: menuNow}); err != nil {
			return err
		}
		return refused
	})
	if !errors.Is(err, refused) {
		t.Fatalf("err = %v, want the edit's own", err)
	}

	rows, err := d.MenuEditor(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rows.Version != 2 || len(rows.Categories) != 1 || rows.Categories[0].Name != "Coffee" {
		t.Errorf("after a failed edit: version %d, categories %+v; it must write nothing and bump nothing", rows.Version, rows.Categories)
	}
}

func TestTheEditorSeesRetiredRows(t *testing.T) {
	d := menuDB(t)
	ctx := context.Background()
	if _, err := d.CreateMenu(ctx, 1, smallMenu(), menuNow); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE items SET active = 0 WHERE name = 'Water'`); err != nil {
		t.Fatal(err)
	}
	rows, err := d.MenuEditor(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Items) != 3 || len(rows.Stations) != 2 || len(rows.Courses) != 2 || len(rows.TaxClasses) != 2 || len(rows.Links) != 2 {
		t.Errorf("editor rows = %d items, %d stations, %d courses, %d classes, %d links",
			len(rows.Items), len(rows.Stations), len(rows.Courses), len(rows.TaxClasses), len(rows.Links))
	}
	m, err := d.Menu(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 2 {
		t.Errorf("the till sells %d items, want 2: water is retired", len(m.Items))
	}
}
