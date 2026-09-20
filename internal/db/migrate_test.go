package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// migratedDB is a database with the schema applied, which is what every test
// below the first few actually wants.
func migratedDB(t *testing.T) *DB {
	t.Helper()

	d := testDB(t)
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return d
}

// TestMigrationsAreEmbedded guards a failure that is invisible until runtime.
// An embed directive that matches nothing still compiles, and the binary then
// ships with no schema and creates an empty database on a venue machine.
func TestMigrationsAreEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(migrationFS, migrationDir)
	if err != nil {
		t.Fatalf("reading the embedded migrations: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no migrations are embedded, so the binary ships without a schema")
	}
	t.Logf("%d migration(s) embedded", len(entries))
}

// TestMigrationsAreWellFormed checks the things goose will not: that the
// numbering is sequential with no gaps and no repeats, and that every file has
// the markers goose needs. A migration numbered by hand is a migration
// numbered wrong eventually.
func TestMigrationsAreWellFormed(t *testing.T) {
	entries, err := fs.ReadDir(migrationFS, migrationDir)
	if err != nil {
		t.Fatalf("reading the embedded migrations: %v", err)
	}

	naming := regexp.MustCompile(`^(\d{5})_[a-z0-9_]+\.sql$`)
	versions := make([]int, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		match := naming.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("%s is not named NNNNN_lower_snake_case.sql", name)
			continue
		}

		version, err := strconv.Atoi(match[1])
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		versions = append(versions, version)

		body, err := fs.ReadFile(migrationFS, filepath.ToSlash(filepath.Join(migrationDir, name)))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for _, marker := range []string{"-- +goose Up", "-- +goose Down"} {
			if !strings.Contains(string(body), marker) {
				t.Errorf("%s has no %q marker", name, marker)
			}
		}
	}

	slices.Sort(versions)
	for i, version := range versions {
		if version != i+1 {
			t.Errorf("migration versions are %v, want 1..%d with no gaps", versions, len(versions))
			break
		}
	}
}

func TestMigrateCreatesTheSchema(t *testing.T) {
	d := migratedDB(t)

	want := []string{
		"devices", "roles", "settings", "staff", "venues",
		"orders", "order_events", "order_snapshots", "invoice_series", "invoices",
		"idempotency_keys", "pairing_codes", "staff_sessions", "sign_in_attempts",
	}

	for _, table := range want {
		t.Run(table, func(t *testing.T) {
			var name string
			err := d.Reader().QueryRowContext(context.Background(),
				"SELECT name FROM sqlite_schema WHERE type = 'table' AND name = ?", table).Scan(&name)
			if err != nil {
				t.Fatalf("table %s: %v", table, err)
			}
		})
	}
}

// TestMigrateIsIdempotent is what makes running migrations on every startup
// safe. A venue machine restarts several times a day.
func TestMigrateIsIdempotent(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	before, err := d.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if before < 1 {
		t.Fatalf("Version() = %d after migrating, want at least 1", before)
	}

	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	after, err := d.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if after != before {
		t.Errorf("version moved from %d to %d on a second run", before, after)
	}
}

// TestMigrateRunsOnTheSingleWriterConnection is here because it is the one way
// this could deadlock rather than fail. The writer pool is capped at one
// connection, so a migration runner that wants two while holding one would
// hang for the whole busy timeout and then some.
func TestMigrateRunsOnTheSingleWriterConnection(t *testing.T) {
	d := testDB(t)

	done := make(chan error, 1)
	go func() { done <- d.Migrate(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Migrate did not finish: it is probably waiting for a second writer connection")
	}
}

func TestMigrateFailsLoudly(t *testing.T) {
	d := testDB(t)

	if err := d.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := d.Migrate(context.Background()); err == nil {
		t.Error("Migrate on a closed database returned nil, so a broken startup would look like a clean one")
	}
}

// TestPublicIDColumnsAreUnique is ADR-034 landing in the schema. The generator
// draws forty bits and cannot promise uniqueness; this constraint is what
// turns a repeat into an error the caller can redraw past, instead of two
// orders answering to the same code.
func TestPublicIDColumnsAreUnique(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)

	tests := []struct {
		table  string
		insert string
		args   []any
	}{
		{
			table: "venues",
			insert: `INSERT INTO venues (public_id, name, currency, timezone, created_at, updated_at)
			         VALUES (?, 'Second', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			args: []any{"ven_7K3M9QX2"},
		},
		{
			table: "staff",
			insert: `INSERT INTO staff (public_id, venue_id, role_id, name, pin_hash, created_at, updated_at)
			         VALUES (?, 1, 1, 'Second', 'hash', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			args: []any{"stf_A1B2C3D4"},
		},
		{
			table: "devices",
			insert: `INSERT INTO devices (public_id, venue_id, name, kind, created_at)
			         VALUES (?, 1, 'Second', 'terminal', '2026-09-18 00:00:00+00:00')`,
			args: []any{"dev_Z9Y8X7W6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			// The first insert is the one seedVenue already made, so this is
			// the second use of the same public id.
			_, err := d.Writer().ExecContext(ctx, tt.insert, tt.args...)
			if err == nil {
				t.Fatalf("%s accepted a repeated public_id, so ADR-034 has nothing to redraw against", tt.table)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unique") {
				t.Errorf("%s: error = %v, want one naming the unique constraint", tt.table, err)
			}
		})
	}
}

// seedVenue inserts one venue, one role, one staff member and one device, so
// the constraint tests have something to collide with.
func seedVenue(t *testing.T, d *DB) {
	t.Helper()
	ctx := context.Background()

	err := d.Tx(ctx, func(tx *sql.Tx) error {
		const at = "2026-09-18 00:00:00+00:00"

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
			 VALUES (1, 'ven_7K3M9QX2', 'Filter Coffee House', 'INR', 'Asia/Kolkata', ?, ?)`, at, at); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO roles (id, venue_id, name, permissions, created_at)
			 VALUES (1, 1, 'cashier', '["order.create"]', ?)`, at); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO staff (id, public_id, venue_id, role_id, name, pin_hash, created_at, updated_at)
			 VALUES (1, 'stf_A1B2C3D4', 1, 1, 'Asha', 'argon2id$fake', ?, ?)`, at, at); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO devices (id, public_id, venue_id, name, kind, created_at)
			 VALUES (1, 'dev_Z9Y8X7W6', 1, 'Counter 1', 'terminal', ?)`, at); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
}
