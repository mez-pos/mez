package db

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// A venue set up before stock existed: its owner and manager gain
// stock.manage when it upgrades, and a cashier, or a role the owner
// renamed, does not.
func TestTheStockMigrationGrantsStockToOwnersAndManagers(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 22); err != nil {
		t.Fatalf("migrating to 22: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE', 'Venue', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct{ name, perms string }{
		{"owner", `["menu.edit","settings.edit"]`},
		{"manager", `["staff.manage","reports.view"]`},
		{"cashier", `["order.open"]`},
		{"head chef", `["kitchen.view"]`},
	} {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO roles (venue_id, name, permissions, created_at) VALUES (1, ?, ?, '2026-09-18 00:00:00+00:00')`,
			r.name, r.perms); err != nil {
			t.Fatalf("role %s: %v", r.name, err)
		}
	}

	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("migrating on: %v", err)
	}

	roles, err := d.Queries().RolesForVenue(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"owner": true, "manager": true, "cashier": false, "head chef": false}
	for _, r := range roles {
		var perms []string
		if err := json.Unmarshal([]byte(r.Permissions), &perms); err != nil {
			t.Fatalf("role %s: %v", r.Name, err)
		}
		if got := slices.Contains(perms, "stock.manage"); got != want[r.Name] {
			t.Errorf("role %s has stock.manage = %v, want %v (%s)", r.Name, got, want[r.Name], r.Permissions)
		}
	}
}
