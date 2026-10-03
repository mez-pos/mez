package db

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// A venue set up before the waiter existed gains the role when it upgrades,
// unless the owner already made one by that name, in any case: a second
// "Waiter" beside theirs would only confuse the people screen.
func TestTheWaiterMigrationAddsTheRoleOnce(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 30); err != nil {
		t.Fatalf("migrating to 30: %v", err)
	}
	for _, id := range []int{1, 2} {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
			 VALUES (?, ?, 'Venue', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
			id, "ven_OLDVENUE"+string(rune('0'+id))); err != nil {
			t.Fatal(err)
		}
	}
	// Venue 1 has the default four. Venue 2 made its own "Waiter".
	for _, r := range []struct {
		venue       int
		name, perms string
	}{
		{1, "owner", `["menu.edit"]`}, {1, "cashier", `["order.open"]`},
		{2, "owner", `["menu.edit"]`}, {2, "Waiter", `["order.open","line.void"]`},
	} {
		if _, err := d.Writer().ExecContext(ctx,
			`INSERT INTO roles (venue_id, name, permissions, created_at) VALUES (?, ?, ?, '2026-09-18 00:00:00+00:00')`,
			r.venue, r.name, r.perms); err != nil {
			t.Fatalf("role %s: %v", r.name, err)
		}
	}

	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("migrating on: %v", err)
	}

	roles := func(venue int64) map[string]string {
		rows, err := d.Queries().RolesForVenue(ctx, venue)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, r := range rows {
			out[r.Name] = r.Permissions
		}
		return out
	}

	one := roles(1)
	var perms []string
	if raw, ok := one["waiter"]; !ok {
		t.Fatalf("venue 1 has no waiter: %v", one)
	} else if err := json.Unmarshal([]byte(raw), &perms); err != nil {
		t.Fatal(err)
	}
	// The same list as auth.DefaultRoles (db cannot import auth to check it;
	// TestDefaultRoles holds the other copy).
	if want := []string{"order.open", "order.edit", "payment.take", "kitchen.view"}; !slices.Equal(perms, want) {
		t.Errorf("the migrated waiter has %v, want %v", perms, want)
	}
	if len(one) != 3 {
		t.Errorf("venue 1 has %v: the migration touched other roles", one)
	}

	two := roles(2)
	if _, added := two["waiter"]; added || len(two) != 2 || two["Waiter"] != `["order.open","line.void"]` {
		t.Errorf("venue 2 has %v: its own Waiter was duplicated or changed", two)
	}
}
