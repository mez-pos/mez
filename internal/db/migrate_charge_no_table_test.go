package db

import (
	"context"
	"testing"
)

// Migration 00050 builds gateway_charges again so a charge can have no
// table. Every charge there was must still be there, as it was; a round's
// charge may then go without a table, and a bill's may not.
func TestAChargeWithNoTableMigrationKeepsEveryCharge(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 49); err != nil {
		t.Fatalf("migrating to 49: %v", err)
	}
	const at = "'2026-10-06 04:00:00+00:00'"
	exec := func(stmt string) error {
		_, err := d.Writer().ExecContext(ctx, stmt)
		return err
	}
	for _, stmt := range []string{
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE1', 'Venue', 'INR', 'Asia/Kolkata', ` + at + `, ` + at + `)`,
		`INSERT INTO areas (id, venue_id, name, created_at, updated_at) VALUES (1, 1, 'Floor', ` + at + `, ` + at + `)`,
		`INSERT INTO tables (id, venue_id, area_id, name, seats, shape, x, y, created_at, updated_at)
		 VALUES (4, 1, 1, 'T4', 4, 'square', 0, 0, ` + at + `, ` + at + `)`,
		`INSERT INTO orders (id, public_id, venue_id, channel, table_id, status, last_seq, opened_at, updated_at)
		 VALUES (9, 'ord_OLDORDER1', 1, 'qr_table', 4, 'open', 3, ` + at + `, ` + at + `)`,
		`INSERT INTO guest_rounds (id, public_id, venue_id, status, lines, subtotal_minor, currency, idempotency_key, fingerprint, created_at, prepay, due_minor)
		 VALUES (1, 'rnd_PICKUP01', 1, 'waiting', '[]', 6000, 'INR', 'k1', 'f1', ` + at + `, 1, 6300)`,
		// A share that was let go, a paid bill charge, and one unapplied and dealt with.
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
		                              status, payer, let_go_at, created_at)
		 VALUES (1, 'chg_SHARE001', 1, 4, 9, 'razorpay', 'plink_1', 'https://rzp.io/1', 5000, 'INR', 'open', 'phone-aaaa', ` + at + `, ` + at + `)`,
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
		                              status, method, reference, created_at, settled_at)
		 VALUES (2, 'chg_PAID0001', 1, 4, 9, 'razorpay', 'plink_2', 'https://rzp.io/2', 46200, 'INR', 'paid', 'upi', 'pay_2', ` + at + `, ` + at + `)`,
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
		                              status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by)
		 VALUES (3, 'chg_LOST0001', 1, 4, 9, 'razorpay', 'plink_3', 'https://rzp.io/3', 900, 'INR', 'unapplied', 'card', 'pay_3', 'The bill was closed.',
		         ` + at + `, ` + at + `, 'refunded', ` + at + `, 7)`,
	} {
		if err := exec(stmt); err != nil {
			t.Fatalf("seeding: %v\n%s", err, stmt)
		}
	}

	if _, err := provider.UpTo(ctx, 50); err != nil {
		t.Fatalf("migrating to 50: %v", err)
	}

	var n, letGo, paid, resolved, withTable int64
	if err := d.Writer().QueryRowContext(ctx,
		`SELECT count(*), sum(let_go_at IS NOT NULL AND payer = 'phone-aaaa'), sum(status = 'paid' AND reference = 'pay_2'),
		        sum(resolution = 'refunded' AND resolved_by = 7 AND problem = 'The bill was closed.'), sum(table_id = 4)
		 FROM gateway_charges`).Scan(&n, &letGo, &paid, &resolved, &withTable); err != nil {
		t.Fatal(err)
	}
	if n != 3 || letGo != 1 || paid != 1 || resolved != 1 || withTable != 3 {
		t.Errorf("after the migration: %d charges, %d let go, %d paid, %d resolved, %d with their table", n, letGo, paid, resolved, withTable)
	}

	// A round's charge may have no table. A bill's may not.
	if err := exec(`INSERT INTO gateway_charges (public_id, venue_id, round_public_id, gateway, gateway_id, checkout_url, amount_minor, currency, status, created_at)
	                VALUES ('chg_PICKUP01', 1, 'rnd_PICKUP01', 'razorpay', 'plink_4', 'https://rzp.io/4', 6300, 'INR', 'open', ` + at + `)`); err != nil {
		t.Errorf("a round's charge with no table: %v", err)
	}
	if err := exec(`INSERT INTO gateway_charges (public_id, venue_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency, status, created_at)
	                VALUES ('chg_NOTABLE1', 1, 9, 'razorpay', 'plink_5', 'https://rzp.io/5', 100, 'INR', 'open', ` + at + `)`); err == nil {
		t.Error("a bill's charge with no table was taken: a bill is a table's")
	}
	// One open charge to a round, still.
	if err := exec(`INSERT INTO gateway_charges (public_id, venue_id, round_public_id, gateway, gateway_id, checkout_url, amount_minor, currency, status, created_at)
	                VALUES ('chg_PICKUP02', 1, 'rnd_PICKUP01', 'razorpay', 'plink_6', 'https://rzp.io/6', 6300, 'INR', 'open', ` + at + `)`); err == nil {
		t.Error("a second open charge for one round was taken")
	}

	// Back again: refused while a charge has no table, since it is money.
	if _, err := provider.DownTo(ctx, 49); err == nil {
		t.Fatal("the migration went back with a charge that has no table")
	}
	if err := exec(`DELETE FROM gateway_charges WHERE table_id IS NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 49); err != nil {
		t.Fatalf("going back to 49: %v", err)
	}
	if err := d.Writer().QueryRowContext(ctx, `SELECT count(*) FROM gateway_charges`).Scan(&n); err != nil || n != 3 {
		t.Errorf("%d charges after going back, %v; want 3", n, err)
	}
}
