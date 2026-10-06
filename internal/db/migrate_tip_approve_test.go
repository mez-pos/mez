package db

import (
	"context"
	"testing"
)

// A venue set up before tip.approve existed (ADR-261): the owner, the
// manager and whoever may void a payment gain it, since they are who let
// money through; the people who take the tip do not.
func TestTheTipApproveMigrationGivesItToManagersAndNotToSellers(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 52); err != nil {
		t.Fatalf("migrating to 52: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE1', 'Venue', 'INR', 'Asia/Kolkata', '2026-10-01 00:00:00+00:00', '2026-10-01 00:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{
		"owner":   `["order.open","payment.void"]`,
		"manager": `["order.open","payment.take"]`,
		"cashier": `["order.open","payment.take"]`,
		"waiter":  `["order.open","order.edit","payment.take","kitchen.view"]`,
		"kitchen": `["kitchen.view","kitchen.work"]`,
		// A manager under a name of the owner's: they void payments.
		"Floor lead": `["order.open","payment.take","payment.void"]`,
		// Already there, as a newer build's role would be: not added twice.
		"Head waiter": `["payment.void","tip.approve"]`,
	}
	for name, perms := range before {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO roles (venue_id, name, permissions, created_at) VALUES (1, ?, ?, '2026-10-01 00:00:00+00:00')`,
			name, perms); err != nil {
			t.Fatalf("role %s: %v", name, err)
		}
	}

	if _, err := provider.UpTo(ctx, 53); err != nil {
		t.Fatalf("migrating to 53: %v", err)
	}

	want := map[string]string{
		"owner":       `["order.open","payment.void","tip.approve"]`,
		"manager":     `["order.open","payment.take","tip.approve"]`,
		"cashier":     `["order.open","payment.take"]`,
		"waiter":      `["order.open","order.edit","payment.take","kitchen.view"]`,
		"kitchen":     `["kitchen.view","kitchen.work"]`,
		"Floor lead":  `["order.open","payment.take","payment.void","tip.approve"]`,
		"Head waiter": `["payment.void","tip.approve"]`,
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
