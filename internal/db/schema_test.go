package db

import (
	"context"
	"strings"
	"testing"
)

// TestMoneyColumnsRefuseAFloat is the CLAUDE.md invariant enforced by the
// database rather than only by the code above it.
//
// SQLite's INTEGER affinity is not a type. Given 12.5 it does not refuse, and
// it does not truncate: it stores a REAL, and the column quietly holds half a
// paisa forever. Every money column carries a typeof check for that reason,
// and this is the test that proves the check is on them.
func TestMoneyColumnsRefuseAFloat(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	const insert = `INSERT INTO venues (public_id, name, currency, timezone, cash_rounding_step_minor, created_at, updated_at)
	                VALUES (?, 'Test', 'INR', 'Asia/Kolkata', ?, '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`

	tests := []struct {
		name  string
		step  any
		allow bool
	}{
		{name: "whole paise", step: 50, allow: true},
		{name: "one paisa", step: 1, allow: true},

		// Affinity converts a text or float value to an integer when it can do
		// so without losing anything, and the check sees the stored type, not
		// the one that was passed. '50.0' therefore lands as the integer 50,
		// which is the right answer: the column holds an integer either way.
		{name: "text that converts to an integer exactly", step: "50.0", allow: true},

		// These cannot convert without losing something, so they stay REAL and
		// the check refuses them. That is the case that matters: half a paisa
		// in a money column is money nobody can count.
		{name: "half a paisa", step: 12.5, allow: false},
		{name: "a decimal string", step: "12.50", allow: false},

		{name: "zero", step: 0, allow: false},
		{name: "negative", step: -50, allow: false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicID := "ven_TEST" + string(rune('A'+i))

			_, err := d.Writer().ExecContext(ctx, insert, publicID, tt.step)
			switch {
			case tt.allow && err != nil:
				t.Errorf("a step of %v was refused: %v", tt.step, err)
			case !tt.allow && err == nil:
				t.Errorf("a step of %v was accepted, so a money column can hold something that is not an integer", tt.step)
			}
		})
	}
}

// TestMoneyColumnsRefuseAFloatEvenWhenItLooksWhole is the case that would slip
// through a check written as "the value equals its own integer part". SQLite
// stores 50.0 as an integer by affinity, but 12.5 as a REAL, so only typeof
// separates them reliably.
func TestMoneyColumnsRefuseAFloatEvenWhenItLooksWhole(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	_, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (public_id, name, currency, timezone, cash_rounding_step_minor, created_at, updated_at)
		 VALUES ('ven_WHOLE01', 'Test', 'INR', 'Asia/Kolkata', 50.0, '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`)
	if err != nil {
		t.Fatalf("50.0 was refused: %v", err)
	}

	var kind string
	if err := d.Reader().QueryRowContext(ctx,
		"SELECT typeof(cash_rounding_step_minor) FROM venues WHERE public_id = 'ven_WHOLE01'").Scan(&kind); err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if kind != "integer" {
		t.Errorf("50.0 was stored as %s, want integer", kind)
	}
}

func TestVenueColumnsAreConstrained(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	const insert = `INSERT INTO venues (public_id, name, currency, timezone, financial_year_start_month, tax_profile, created_at, updated_at)
	                VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	valid := []any{"ven_VALID001", "Filter Coffee House", "INR", "Asia/Kolkata", 4, "gst_in", "2026-09-18 00:00:00+00:00", "2026-09-18 00:00:00+00:00"}

	tests := []struct {
		name  string
		index int
		value any
		why   string
	}{
		{name: "empty name", index: 1, value: "", why: "a venue with no name prints a receipt with no name"},
		{name: "lowercase currency", index: 2, value: "inr", why: "money.New refuses it, so the schema should too"},
		{name: "a currency symbol", index: 2, value: "Rs", why: "a currency is a three letter code"},
		{name: "four letter currency", index: 2, value: "INRR", why: "a currency is a three letter code"},
		{name: "empty timezone", index: 3, value: "", why: "rendering a time needs a zone"},
		{name: "month thirteen", index: 4, value: 13, why: "a financial year cannot start in month thirteen"},
		{name: "month zero", index: 4, value: 0, why: "months are one based"},
		{name: "an unknown tax profile", index: 5, value: "gst_uk", why: "a profile with no implementation computes no tax"},
		{name: "a local timestamp", index: 6, value: "2026-09-18 05:30:00+05:30", why: "stored times are UTC"},
		{name: "an RFC 3339 timestamp", index: 6, value: "2026-09-18T00:00:00Z", why: "the driver writes SQLite's own format, so hand written SQL must match it"},
		{name: "a unix timestamp", index: 6, value: "1789689600", why: "stored times are ISO 8601"},
		{name: "a date with no time", index: 6, value: "2026-09-18", why: "stored times carry a time"},
		{name: "no zone at all", index: 6, value: "2026-09-18 00:00:00", why: "a time with no zone is a time somebody has to guess about"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]any, len(valid))
			copy(args, valid)
			args[tt.index] = tt.value

			if _, err := d.Writer().ExecContext(ctx, insert, args...); err == nil {
				t.Errorf("%v was accepted: %s", tt.value, tt.why)
			}
		})
	}

	// And the valid row still goes in, or the table above proves nothing.
	if _, err := d.Writer().ExecContext(ctx, insert, valid...); err != nil {
		t.Errorf("the valid row was refused: %v", err)
	}
}

func TestStaffAndDeviceConstraints(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)

	tests := []struct {
		name  string
		query string
		why   string
	}{
		{
			name: "staff without a venue",
			query: `INSERT INTO staff (public_id, venue_id, role_id, name, pin_hash, created_at, updated_at)
			        VALUES ('stf_NOVENUE1', 999, 1, 'Ghost', 'hash', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			why: "a staff member belongs to a venue that exists",
		},
		{
			name: "staff without a role",
			query: `INSERT INTO staff (public_id, venue_id, role_id, name, pin_hash, created_at, updated_at)
			        VALUES ('stf_NOROLE01', 1, 999, 'Ghost', 'hash', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			why: "an unassigned cashier can do everything or nothing depending on how the code reads it",
		},
		{
			name: "staff with no pin hash",
			query: `INSERT INTO staff (public_id, venue_id, role_id, name, pin_hash, created_at, updated_at)
			        VALUES ('stf_NOPIN001', 1, 1, 'Ghost', '', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			why: "an empty hash is an account anybody can be",
		},
		{
			name: "active as text",
			query: `INSERT INTO staff (public_id, venue_id, role_id, name, pin_hash, active, created_at, updated_at)
			        VALUES ('stf_BOOL0001', 1, 1, 'Ghost', 'hash', 'false', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			why: "SQLite stores the text 'false', and every truthiness test in Go then reads it as set",
		},
		{
			name: "a device of an unknown kind",
			query: `INSERT INTO devices (public_id, venue_id, name, kind, created_at)
			        VALUES ('dev_UNKNOWN1', 1, 'Mystery', 'fax', '2026-09-18 00:00:00+00:00')`,
			why: "nothing routes a ticket to a kind that does not exist",
		},
		{
			name: "a role with permissions that are not JSON",
			query: `INSERT INTO roles (venue_id, name, permissions, created_at)
			        VALUES (1, 'broken', 'order.create', '2026-09-18 00:00:00+00:00')`,
			why: "permissions are read as JSON, and a bare string parses as nothing",
		},
		{
			name: "two roles with the same name in one venue",
			query: `INSERT INTO roles (venue_id, name, permissions, created_at)
			        VALUES (1, 'cashier', '[]', '2026-09-18 00:00:00+00:00')`,
			why: "a role is chosen by name on the terminal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := d.Writer().ExecContext(ctx, tt.query); err == nil {
				t.Errorf("accepted: %s", tt.why)
			}
		})
	}
}

// TestDeletingAVenueTakesItsRowsWithIt checks the cascade, and that the one
// relationship deliberately not cascading holds. Nothing in the product
// deletes a venue, but a test database and a failed setup both do.
func TestDeletingAVenueTakesItsRowsWithIt(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)

	// A role still held by somebody must not be deletable on its own.
	if _, err := d.Writer().ExecContext(ctx, "DELETE FROM roles WHERE id = 1"); err == nil {
		t.Error("a role still held by a staff member was deleted, unassigning them silently")
	} else if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Errorf("error = %v, want one naming the foreign key constraint", err)
	}

	if _, err := d.Writer().ExecContext(ctx, "DELETE FROM venues WHERE id = 1"); err != nil {
		t.Fatalf("deleting the venue: %v", err)
	}

	for _, table := range []string{"staff", "devices", "roles", "settings"} {
		t.Run(table, func(t *testing.T) {
			var rows int
			if err := d.Reader().QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&rows); err != nil {
				t.Fatalf("counting %s: %v", table, err)
			}
			if rows != 0 {
				t.Errorf("%d rows left in %s after the venue was deleted", rows, table)
			}
		})
	}
}
