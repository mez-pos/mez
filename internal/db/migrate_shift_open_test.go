package db

import (
	"context"
	"testing"
)

// A venue set up before shift.open existed: whoever could open the shift
// still can, the cashier gains it, and nobody else does (ADR-152).
func TestTheShiftOpenMigrationKeepsWhoCouldOpenAndAddsTheCashier(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 32); err != nil {
		t.Fatalf("migrating to 32: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE1', 'Venue', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{
		"owner":      `["order.open","shift.manage","cash.move"]`,
		"cashier":    `["order.open","payment.take"]`,
		"waiter":     `["order.open","payment.take"]`,
		"supervisor": `["shift.manage"]`,
		// Already there, as a newer build's role would be: not added twice.
		"opener": `["order.open","shift.open"]`,
	}
	for name, perms := range before {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO roles (venue_id, name, permissions, created_at) VALUES (1, ?, ?, '2026-09-18 00:00:00+00:00')`,
			name, perms); err != nil {
			t.Fatalf("role %s: %v", name, err)
		}
	}

	// To 33 and no further: later migrations give their own permissions.
	if _, err := provider.UpTo(ctx, 33); err != nil {
		t.Fatalf("migrating to 33: %v", err)
	}

	want := map[string]string{
		"owner":      `["order.open","shift.manage","cash.move","shift.open"]`,
		"cashier":    `["order.open","payment.take","shift.open"]`,
		"waiter":     `["order.open","payment.take"]`,
		"supervisor": `["shift.manage","shift.open"]`,
		"opener":     `["order.open","shift.open"]`,
	}
	rows, err := d.Queries().RolesForVenue(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("the venue has %d roles, want %d", len(rows), len(want))
	}
	for _, r := range rows {
		if r.Permissions != want[r.Name] {
			t.Errorf("role %s = %s, want %s", r.Name, r.Permissions, want[r.Name])
		}
	}
}
