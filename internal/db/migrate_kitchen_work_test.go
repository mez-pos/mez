package db

import (
	"context"
	"testing"
)

// A venue set up before kitchen.work existed: the owner, the manager and
// whoever works the kitchen and does not sell gain it; someone who sells
// keeps the sight of the board and no more (ADR-180).
func TestTheKitchenWorkMigrationGivesItToTheKitchenAndNotToSellers(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 33); err != nil {
		t.Fatalf("migrating to 33: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE1', 'Venue', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{
		"owner":     `["order.open","kitchen.view"]`,
		"manager":   `["order.open","kitchen.view"]`,
		"cashier":   `["order.open","payment.take","kitchen.view"]`,
		"waiter":    `["order.open","order.edit","payment.take","kitchen.view"]`,
		"kitchen":   `["kitchen.view"]`,
		"Head chef": `["kitchen.view","stock.manage"]`,
		// Sells and sees the kitchen, under a name of the owner's: a seller.
		"Runner": `["order.open","kitchen.view"]`,
		// Neither sees the kitchen nor sells: nothing to do with it.
		"Accountant": `["reports.view"]`,
		// Already there, as a newer build's role would be: not added twice.
		"Sous chef": `["kitchen.view","kitchen.work"]`,
	}
	for name, perms := range before {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO roles (venue_id, name, permissions, created_at) VALUES (1, ?, ?, '2026-09-18 00:00:00+00:00')`,
			name, perms); err != nil {
			t.Fatalf("role %s: %v", name, err)
		}
	}

	if _, err := provider.UpTo(ctx, 34); err != nil {
		t.Fatalf("migrating to 34: %v", err)
	}

	want := map[string]string{
		"owner":      `["order.open","kitchen.view","kitchen.work"]`,
		"manager":    `["order.open","kitchen.view","kitchen.work"]`,
		"cashier":    `["order.open","payment.take","kitchen.view"]`,
		"waiter":     `["order.open","order.edit","payment.take","kitchen.view"]`,
		"kitchen":    `["kitchen.view","kitchen.work"]`,
		"Head chef":  `["kitchen.view","stock.manage","kitchen.work"]`,
		"Runner":     `["order.open","kitchen.view"]`,
		"Accountant": `["reports.view"]`,
		"Sous chef":  `["kitchen.view","kitchen.work"]`,
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
