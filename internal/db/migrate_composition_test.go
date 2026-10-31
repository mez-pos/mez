package db

import (
	"context"
	"strings"
	"testing"
)

// compositionVersion is 00058_tax_profile_composition.sql (ADR-401).
const compositionVersion = 58

// TestTheCompositionMigrationKeepsEveryRow: 00058 builds venues again with
// foreign keys off, since nearly every table cascades from it. Nothing that
// refers to a venue may be lost, the venue comes through whole, foreign keys
// are on again after, and the column takes the new code and still refuses a
// made-up one. Going down and up again keeps every row too.
func TestTheCompositionMigrationKeepsEveryRow(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	p, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, compositionVersion-1); err != nil {
		t.Fatalf("UpTo %d: %v", compositionVersion-1, err)
	}
	w := d.Writer()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := w.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	const at = "2026-10-31 12:00:00+00:00"
	exec(`INSERT INTO venues (id, public_id, name, legal_name, currency, timezone, financial_year_start_month,
		tax_profile, tax_registration_id, cash_rounding_step_minor, created_at, updated_at)
		VALUES (7, 'ven_TESTTEST', 'Corner Cafe', 'Corner Cafe LLP', 'INR', 'Asia/Kolkata', 4,
		'gst_in', '29ABCDE1234F1Z5', 50, ?, ?)`, at, at)
	exec(`INSERT INTO settings (venue_id, key, value, updated_at) VALUES (7, 'tax.state_code', '29', ?)`, at)
	exec(`INSERT INTO stations (venue_id, code, name, created_at, updated_at) VALUES (7, 'kitchen', 'Kitchen', ?, ?)`, at, at)

	venueRow := func() string {
		t.Helper()
		var row string
		err := w.QueryRowContext(ctx, `SELECT public_id || '|' || name || '|' || legal_name || '|' || currency || '|' ||
			timezone || '|' || financial_year_start_month || '|' || tax_profile || '|' || tax_registration_id || '|' ||
			cash_rounding_step_minor || '|' || created_at || '|' || updated_at FROM venues WHERE id = 7`).Scan(&row)
		if err != nil {
			t.Fatalf("reading the venue: %v", err)
		}
		return row
	}
	// What a cascade would have taken, and the state the rebuild must leave.
	check := func(stage string) {
		t.Helper()
		var settings, stations, fks int
		if err := w.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM settings), (SELECT count(*) FROM stations)`).Scan(&settings, &stations); err != nil {
			t.Fatal(err)
		}
		if settings != 1 || stations != 1 {
			t.Errorf("%s: %d settings and %d stations, want 1 and 1", stage, settings, stations)
		}
		if err := w.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fks); err != nil || fks != 1 {
			t.Errorf("%s: foreign keys %d (%v), want on", stage, fks, err)
		}
		rows, err := w.QueryContext(ctx, `PRAGMA foreign_key_check`)
		if err != nil {
			t.Fatal(err)
		}
		broken := rows.Next()
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if broken {
			t.Errorf("%s: a reference is broken", stage)
		}
		var leftover int
		if err := w.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE name IN ('venues_new', 'venues_old')`).Scan(&leftover); err != nil || leftover != 0 {
			t.Errorf("%s: %d copies of the table left (%v)", stage, leftover, err)
		}
	}
	before := venueRow()

	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	check("up")
	if got := venueRow(); got != before {
		t.Errorf("the venue came through as %s, want %s", got, before)
	}
	exec(`UPDATE venues SET tax_profile = 'gst_in_composition' WHERE id = 7`)
	if _, err := w.ExecContext(ctx, `UPDATE venues SET tax_profile = 'gst_in_flat' WHERE id = 7`); err == nil || !strings.Contains(err.Error(), "CHECK") {
		t.Errorf("a made-up profile: %v, want the CHECK to refuse it", err)
	}
	var unique int
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name = 'venues_public_id'`).Scan(&unique); err != nil || unique != 1 {
		t.Errorf("the public id's unique index: %d (%v)", unique, err)
	}

	// Down needs every venue on one of the four codes it knows.
	exec(`UPDATE venues SET tax_profile = 'gst_in' WHERE id = 7`)
	if _, err := p.DownTo(ctx, compositionVersion-1); err != nil {
		t.Fatalf("Down: %v", err)
	}
	check("down")
	if got := venueRow(); got != before {
		t.Errorf("after going down the venue is %s, want %s", got, before)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("Up again: %v", err)
	}
	check("up again")
}
