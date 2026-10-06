package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

// TestUnlockClearsAHardLock: a person locked by too many wrong PINs can be
// unlocked at the machine, and only a locked person can be.
func TestUnlockClearsAHardLock(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t)
	out := runSetupIn(t, args)
	owner := field(t, out, "Owner")
	d := opened(t, dir)

	var open strings.Builder
	if err := runUnlock([]string{"-data-dir", dir, "-staff", owner}, &open); !errors.Is(err, ErrNotLocked) {
		t.Fatalf("unlocking a person who is not locked: err = %v, want ErrNotLocked", err)
	}

	w := d.Writer()
	if _, err := w.ExecContext(ctx, `UPDATE staff SET locked_at = ?, locked_reason = ? WHERE public_id = ?`,
		time.Now().UTC(), auth.HardLockReason, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO staff_lockouts (venue_id, staff_id, at) SELECT venue_id, id, ? FROM staff WHERE public_id = ?`,
		time.Now().UTC(), owner); err != nil {
		t.Fatal(err)
	}

	var printed strings.Builder
	if err := runUnlock([]string{"-data-dir", dir, "-staff", owner}, &printed); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if !strings.Contains(printed.String(), "is unlocked") {
		t.Errorf("printed %q", printed.String())
	}
	var lockedAt sql.NullTime
	var cleared int
	if err := w.QueryRowContext(ctx, `SELECT locked_at FROM staff WHERE public_id = ?`, owner).Scan(&lockedAt); err != nil {
		t.Fatal(err)
	}
	if err := w.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_lockouts WHERE cleared_at IS NOT NULL`).Scan(&cleared); err != nil {
		t.Fatal(err)
	}
	if lockedAt.Valid || cleared != 1 {
		t.Errorf("after unlock: locked_at = %v, cleared lockouts = %d", lockedAt, cleared)
	}

	for _, bad := range [][]string{{"-data-dir", dir}, {"-data-dir", dir, "-staff", "stf_NOBODY00"}} {
		if err := runUnlock(bad, &strings.Builder{}); err == nil {
			t.Errorf("unlock %v succeeded, want a refusal", bad)
		}
	}
}
