package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// lockOutOnce types wrong PINs for a staff member until the window's limit
// locks them, by sign-in or by override, then lets the window pass so the
// next lockout starts afresh.
func lockOutOnce(t *testing.T, w *staffWorld, staffRef string, override bool) {
	t.Helper()
	ctx := context.Background()
	for i := range DefaultStaffLockout.MaxFailures {
		w.clock.advance(time.Second)
		var err error
		if override {
			_, err = w.auth.Authorise(ctx, w.till, staffRef, "0000", PermLineVoid)
		} else {
			_, err = w.auth.SignIn(ctx, w.till, staffRef, "0000")
		}
		if err != ErrInvalidPIN {
			t.Fatalf("wrong PIN %d: err = %v, want ErrInvalidPIN", i+1, err)
		}
	}
	w.clock.advance(DefaultStaffLockout.Window + time.Minute)
}

// hardLocked reads a staff member's hard lock: when, and why.
func hardLocked(t *testing.T, w *staffWorld, staffRef string) (bool, string) {
	t.Helper()
	row, err := w.db.Queries().StaffByPublicID(context.Background(), staffRef)
	if err != nil {
		t.Fatal(err)
	}
	return row.LockedAt.Valid, row.LockedReason
}

// TestTheThirdLockoutInADayHardLocks: two lockouts are the window's lock and
// no more; the third within a day locks until someone clears it. Locked,
// the right PIN is refused as a wrong one, at sign-in and at an override,
// the person leaves the sign-in tiles, and their sessions are ended.
func TestTheThirdLockoutInADayHardLocks(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	lockOutOnce(t, w, "stf_ASHA0001", false)
	lockOutOnce(t, w, "stf_ASHA0001", false)
	if locked, _ := hardLocked(t, w, "stf_ASHA0001"); locked {
		t.Fatal("the second lockout hard-locked")
	}
	in, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468")
	if err != nil {
		t.Fatalf("after two lockouts, the right PIN: %v", err)
	}

	lockOutOnce(t, w, "stf_ASHA0001", false)
	locked, reason := hardLocked(t, w, "stf_ASHA0001")
	if !locked || reason != HardLockReason {
		t.Fatalf("after the third lockout: locked %v, reason %q", locked, reason)
	}
	if _, err := w.auth.Session(ctx, w.till, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the session open before the hard lock: err = %v, want ended", err)
	}
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != ErrInvalidPIN {
		t.Errorf("the right PIN, hard-locked: err = %v, want exactly ErrInvalidPIN", err)
	}
	grant(t, w, w.asha.RoleID, PermLineVoid)
	if _, err := w.auth.Authorise(ctx, w.till, "stf_ASHA0001", "2468", PermLineVoid); err != ErrInvalidPIN {
		t.Errorf("an override with the right PIN, hard-locked: err = %v, want exactly ErrInvalidPIN", err)
	}
	if got := outcomes(t, w.db); got[len(got)-1] != outcomeInactiveStaff {
		t.Errorf("a hard-locked attempt recorded as %q, want %q", got[len(got)-1], outcomeInactiveStaff)
	}
	staff, err := w.auth.ActiveStaff(ctx, 1)
	if err != nil || len(staff) != 1 || staff[0].Name != "Ravi" {
		t.Errorf("sign-in tiles = %+v, %v; want only Ravi", staff, err)
	}
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); locked {
		t.Error("another person was hard-locked too")
	}
}

// TestOverrideFailuresHardLockToo: an override runs the same gauntlet, so
// its wrong PINs count toward the hard lock as a sign-in's do.
func TestOverrideFailuresHardLockToo(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	grant(t, w, w.ravi.RoleID, PermLineVoid)
	for range HardLockAfter {
		lockOutOnce(t, w, "stf_RAVI0001", true)
	}
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); !locked {
		t.Fatal("three lockouts by override did not hard-lock")
	}
	if _, err := w.auth.Authorise(ctx, w.till, "stf_RAVI0001", "1357", PermLineVoid); err != ErrInvalidPIN {
		t.Errorf("the right PIN at an override: err = %v, want exactly ErrInvalidPIN", err)
	}
}

// TestLockoutsADayApartDoNotAddUp: the hard lock counts the lockouts of the
// last day, not of all time.
func TestLockoutsADayApartDoNotAddUp(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	lockOutOnce(t, w, "stf_ASHA0001", false)
	lockOutOnce(t, w, "stf_ASHA0001", false)
	w.clock.advance(HardLockWindow)
	lockOutOnce(t, w, "stf_ASHA0001", false)
	if locked, _ := hardLocked(t, w, "stf_ASHA0001"); locked {
		t.Fatal("lockouts more than a day apart hard-locked")
	}
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Errorf("the right PIN: %v", err)
	}
}

// TestAClearedLockLetsThemBackIn: clearing the hard lock (as the people
// service does) lets the right PIN in at once, even inside the window of
// the wrong PINs that caused it, and starts the count of lockouts again.
func TestAClearedLockLetsThemBackIn(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	for range HardLockAfter - 1 {
		lockOutOnce(t, w, "stf_ASHA0001", false)
	}
	// The third lockout, without letting its window pass.
	for range DefaultStaffLockout.MaxFailures {
		w.clock.advance(time.Second)
		_, _ = w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "0000")
	}
	if locked, _ := hardLocked(t, w, "stf_ASHA0001"); !locked {
		t.Fatal("not hard-locked")
	}

	w.clock.advance(time.Second)
	clear := w.clock.now().UTC()
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET locked_at = NULL, locked_reason = '' WHERE id = ?`, w.asha.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff_lockouts SET cleared_at = ? WHERE staff_id = ?`, clear, w.asha.ID); err != nil {
		t.Fatal(err)
	}
	w.clock.advance(time.Second)
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Fatalf("after the clear, the right PIN: %v", err)
	}
	// One more lockout is one, not the fourth.
	lockOutOnce(t, w, "stf_ASHA0001", false)
	if locked, _ := hardLocked(t, w, "stf_ASHA0001"); locked {
		t.Error("the first lockout after a clear hard-locked")
	}
}
