package shift

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestClosingWritesTheExports(t *testing.T) {
	f := newFixture(t)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	w.sale(45000, cash(45000, 5000))
	w.sale(30000, card(30000))
	closeExact(t, f, cashier, open.ID)

	files := must(f.svc.Exports(ctx, 1, open.ID))
	wantNames := []string{
		"2026-09-27-" + open.PublicID + "-tax-register.csv",
		"2026-09-27-" + open.PublicID + "-tally.csv",
		"2026-09-27-" + open.PublicID + "-sales.csv",
	}
	if len(files) != 3 {
		t.Fatalf("files = %+v, want three", files)
	}
	for i, want := range wantNames {
		if files[i].Filename != want || files[i].Size == 0 {
			t.Errorf("file %d = %+v, want %s", i, files[i], want)
		}
	}

	sales := must(f.svc.Export(ctx, 1, files[2].ID))
	lines := strings.Split(strings.TrimSpace(sales.Text), "\r\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "Date,Time,Type,Number") || !strings.Contains(lines[1], ",450.00,450.00,,,") || !strings.Contains(lines[2], ",300.00,,300.00,,") {
		t.Errorf("sales file:\n%s", sales.Text)
	}
	tally := must(f.svc.Export(ctx, 1, files[1].ID))
	if !strings.Contains(tally.Text, "Cash,450.00,") || !strings.Contains(tally.Text, "Sales,,450.00") {
		t.Errorf("tally file:\n%s", tally.Text)
	}
	if sales.ShiftID != open.ID || sales.Kind != ExportSales {
		t.Errorf("export = %+v", sales.ExportFile)
	}

	if _, err := f.svc.Export(ctx, 2, files[0].ID); !errors.Is(err, ErrExportNotFound) {
		t.Errorf("venue 1's export from venue 2: err = %v, want ErrExportNotFound", err)
	}
	if _, err := f.svc.Export(ctx, 1, 999); !errors.Is(err, ErrExportNotFound) {
		t.Errorf("no such export: err = %v, want ErrExportNotFound", err)
	}
	if _, err := f.svc.Exports(ctx, 2, open.ID); !errors.Is(err, ErrShiftNotFound) {
		t.Errorf("venue 1's shift from venue 2: err = %v, want ErrShiftNotFound", err)
	}
	for _, stmt := range []string{"UPDATE exports SET body = ''", "DELETE FROM exports"} {
		err := f.db.Tx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, stmt)
			return err
		})
		if err == nil {
			t.Errorf("the database let %q through", stmt)
		}
	}
}

func TestAnOpenShiftHasNoExports(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	if files := must(f.svc.Exports(ctx, 1, open.ID)); len(files) != 0 {
		t.Errorf("an open shift's exports = %+v", files)
	}
}
