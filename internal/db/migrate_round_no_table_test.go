package db

import (
	"context"
	"testing"
)

// Migration 00049 builds guest_rounds again so a round can have no table.
// Charges refer to a round and are deleted with it, and dropping the table
// deletes its rows: a venue that has taken a guest's payment for a round
// must come out of the migration with that charge still there, alongside the
// charges that name no round, and every round as it was.
func TestARoundWithNoTableMigrationKeepsEveryRoundAndCharge(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	provider, err := d.migrationProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 48); err != nil {
		t.Fatalf("migrating to 48: %v", err)
	}
	const at = "'2026-10-06 04:00:00+00:00'"
	for _, stmt := range []string{
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (1, 'ven_OLDVENUE1', 'Venue', 'INR', 'Asia/Kolkata', ` + at + `, ` + at + `)`,
		`INSERT INTO areas (id, venue_id, name, created_at, updated_at) VALUES (1, 1, 'Floor', ` + at + `, ` + at + `)`,
		`INSERT INTO tables (id, venue_id, area_id, name, seats, shape, x, y, created_at, updated_at)
		 VALUES (4, 1, 1, 'T4', 4, 'square', 0, 0, ` + at + `, ` + at + `)`,
		`INSERT INTO orders (id, public_id, venue_id, channel, table_id, status, last_seq, opened_at, updated_at)
		 VALUES (9, 'ord_OLDORDER1', 1, 'qr_table', 4, 'open', 3, ` + at + `, ` + at + `)`,
		// A round paid for and sent, one still unpaid, and a request.
		`INSERT INTO guest_rounds (id, public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
		                           idempotency_key, fingerprint, created_at, decided_at, prepay, due_minor, first_seq)
		 VALUES (1, 'rnd_PAID0001', 1, 4, 'sent', 9, '[]', 44000, 'INR', 'k1', 'f1', ` + at + `, ` + at + `, 1, NULL, 2)`,
		`INSERT INTO guest_rounds (id, public_id, venue_id, table_id, status, lines, subtotal_minor, currency,
		                           idempotency_key, fingerprint, created_at, prepay, due_minor)
		 VALUES (2, 'rnd_OWED0001', 1, 4, 'waiting', '[]', 18000, 'INR', 'k2', 'f2', ` + at + `, 1, 18900)`,
		`INSERT INTO guest_rounds (id, public_id, venue_id, table_id, status, lines, subtotal_minor, currency,
		                           idempotency_key, fingerprint, created_at)
		 VALUES (3, 'rnd_ASKED001', 1, 4, 'waiting', '[]', 6000, 'INR', 'k3', 'f3', ` + at + `)`,
		// The paid round's charge, an open one for the unpaid round, and a
		// bill's charge that names no round.
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, round_public_id, gateway, gateway_id, checkout_url,
		                              amount_minor, currency, status, method, reference, created_at, settled_at)
		 VALUES (1, 'chg_PAID0001', 1, 4, 9, 'rnd_PAID0001', 'razorpay', 'plink_1', 'https://rzp.io/1', 46200, 'INR', 'paid', 'upi', 'pay_1', ` + at + `, ` + at + `)`,
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, round_public_id, gateway, gateway_id, checkout_url,
		                              amount_minor, currency, status, created_at)
		 VALUES (2, 'chg_OPEN0001', 1, 4, 'rnd_OWED0001', 'razorpay', 'plink_2', 'https://rzp.io/2', 18900, 'INR', 'open', ` + at + `)`,
		`INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
		                              amount_minor, currency, status, payer, created_at)
		 VALUES (3, 'chg_BILL0001', 1, 4, 9, 'razorpay', 'plink_3', 'https://rzp.io/3', 5000, 'INR', 'open', 'phone-aaaa', ` + at + `)`,
	} {
		if _, err := d.Writer().ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seeding: %v\n%s", err, stmt)
		}
	}

	if _, err := provider.UpTo(ctx, 49); err != nil {
		t.Fatalf("migrating to 49: %v", err)
	}

	type charge struct {
		publicID, status, round, payer string
		amount                         int64
	}
	rows, err := d.Writer().QueryContext(ctx,
		`SELECT public_id, status, coalesce(round_public_id, ''), coalesce(payer, ''), amount_minor FROM gateway_charges ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []charge
	for rows.Next() {
		var c charge
		if err := rows.Scan(&c.publicID, &c.status, &c.round, &c.payer, &c.amount); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []charge{
		{"chg_PAID0001", "paid", "rnd_PAID0001", "", 46200},
		{"chg_OPEN0001", "open", "rnd_OWED0001", "", 18900},
		{"chg_BILL0001", "open", "", "phone-aaaa", 5000},
	}
	if len(got) != len(want) {
		t.Fatalf("%d charges after the migration, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("charge %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}

	var rounds, sent, first, due int64
	if err := d.Writer().QueryRowContext(ctx,
		`SELECT count(*), sum(status = 'sent'), max(coalesce(first_seq, 0)), max(coalesce(due_minor, 0)) FROM guest_rounds WHERE table_id = 4`,
	).Scan(&rounds, &sent, &first, &due); err != nil {
		t.Fatal(err)
	}
	if rounds != 3 || sent != 1 || first != 2 || due != 18900 {
		t.Errorf("rounds after the migration: %d, %d sent, first_seq %d, due %d", rounds, sent, first, due)
	}

	// A round may now have no table, if it is one that is paid first, and a
	// charge still belongs to its round: deleting the round deletes it.
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO guest_rounds (public_id, venue_id, status, lines, subtotal_minor, currency, idempotency_key, fingerprint, created_at, prepay, due_minor)
		 VALUES ('rnd_PICKUP01', 1, 'waiting', '[]', 6000, 'INR', 'k4', 'f4', `+at+`, 1, 6300)`); err != nil {
		t.Errorf("a round with no table: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx,
		`INSERT INTO guest_rounds (public_id, venue_id, status, lines, subtotal_minor, currency, idempotency_key, fingerprint, created_at)
		 VALUES ('rnd_PICKUP02', 1, 'waiting', '[]', 6000, 'INR', 'k5', 'f5', `+at+`)`); err == nil {
		t.Error("a round with no table that is not paid first was taken: it would wait for staff who cannot see a guest")
	}
	if _, err := d.Writer().ExecContext(ctx,
		`UPDATE guest_rounds SET pickup_code = 'A7K' WHERE public_id = 'rnd_PAID0001'`); err == nil {
		t.Error("a round at a table was given a pickup code")
	}

	// Back again: refused while a round with no table is there, and clean
	// once it is not, with every charge still in place.
	if _, err := provider.DownTo(ctx, 48); err == nil {
		t.Fatal("the migration went back with a pickup round in the table: that round was a guest's order")
	}
	if _, err := d.Writer().ExecContext(ctx, `DELETE FROM guest_rounds WHERE table_id IS NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 48); err != nil {
		t.Fatalf("going back to 48: %v", err)
	}
	var charges int64
	if err := d.Writer().QueryRowContext(ctx, `SELECT count(*) FROM gateway_charges`).Scan(&charges); err != nil || charges != 3 {
		t.Errorf("%d charges after going back, %v; want 3", charges, err)
	}
}
