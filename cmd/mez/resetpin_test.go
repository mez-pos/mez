package main

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

// resetPINPrinted is the PIN `mez reset-pin` printed: six digits on a line of
// their own.
func resetPINPrinted(t *testing.T, out string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s+(\d{6})\s*$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no PIN in what was printed:\n%s", out)
	}
	return m[1]
}

// TestResetPINGetsALockedOutOwnerBackIn: the one owner has forgotten the
// PIN and locked themselves trying. At the machine they are given a new
// one, which signs them in; the old one no longer does, and every session
// they had is over.
func TestResetPINGetsALockedOutOwnerBackIn(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t)
	out := runSetupIn(t, args)
	owner, oldPIN := field(t, out, "Owner"), field(t, out, "Owner PIN")
	d := opened(t, dir)

	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(ctx, field(t, out, "Pairing code"))
	if err != nil {
		t.Fatal(err)
	}
	// A session from before, and then the lock.
	if _, err := staff.SignIn(ctx, paired.Device, owner, oldPIN); err != nil {
		t.Fatal(err)
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
	if err := runResetPIN([]string{"-data-dir", dir, "-staff", owner}, &printed); err != nil {
		t.Fatalf("reset-pin: %v", err)
	}
	newPIN := resetPINPrinted(t, printed.String())
	if newPIN == oldPIN {
		t.Fatalf("the new PIN is the old one: %s", newPIN)
	}
	if !strings.Contains(printed.String(), "Priya") || !strings.Contains(printed.String(), "shown here once") {
		t.Errorf("printed %q", printed.String())
	}

	var lockedAt sql.NullTime
	var open, uncleared int
	if err := w.QueryRowContext(ctx, `SELECT locked_at FROM staff WHERE public_id = ?`, owner).Scan(&lockedAt); err != nil {
		t.Fatal(err)
	}
	if err := w.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_sessions WHERE ended_at IS NULL`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if err := w.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_lockouts WHERE cleared_at IS NULL`).Scan(&uncleared); err != nil {
		t.Fatal(err)
	}
	if lockedAt.Valid || open != 0 || uncleared != 0 {
		t.Errorf("after reset-pin: locked_at = %v, open sessions = %d, lockouts not cleared = %d", lockedAt, open, uncleared)
	}

	if _, err := staff.SignIn(ctx, paired.Device, owner, oldPIN); err == nil {
		t.Error("the old PIN still signs the owner in")
	}
	in, err := staff.SignIn(ctx, paired.Device, owner, newPIN)
	if err != nil {
		t.Fatalf("the owner cannot sign in with the printed PIN: %v", err)
	}
	if in.Session.Staff.Name != "Priya" {
		t.Errorf("signed in as %q", in.Session.Staff.Name)
	}
}

// TestResetPINWorksForSomeoneNotLocked: it is a new PIN, not only an unlock.
func TestResetPINWorksForSomeoneNotLocked(t *testing.T) {
	dir, args := setupArgs(t)
	out := runSetupIn(t, args)
	var printed strings.Builder
	if err := runResetPIN([]string{"-data-dir", dir, "-staff", field(t, out, "Owner")}, &printed); err != nil {
		t.Fatalf("reset-pin: %v", err)
	}
	// Twice gives two PINs: each is drawn.
	var again strings.Builder
	if err := runResetPIN([]string{"-data-dir", dir, "-staff", field(t, out, "Owner")}, &again); err != nil {
		t.Fatal(err)
	}
	if a, b := resetPINPrinted(t, printed.String()), resetPINPrinted(t, again.String()); len(a) < auth.MinPowerfulPINLength || len(b) < auth.MinPowerfulPINLength {
		t.Errorf("PINs %q and %q: an owner's is %d digits or more", a, b, auth.MinPowerfulPINLength)
	}
}

// TestResetPINListsWhoThereIs: the person who needs it cannot open the Staff
// screen to find their id, so without -staff it says who there is.
func TestResetPINListsWhoThereIs(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t, "-demo-menu")
	out := runSetupIn(t, args)
	owner := field(t, out, "Owner")
	d := opened(t, dir)
	if _, err := d.Writer().ExecContext(ctx, `UPDATE staff SET locked_at = ?, locked_reason = ? WHERE name = 'Kabir'`, time.Now().UTC(), auth.HardLockReason); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE name = 'Ravi'`); err != nil {
		t.Fatal(err)
	}
	before := ""
	if err := d.Writer().QueryRowContext(ctx, `SELECT pin_hash FROM staff WHERE public_id = ?`, owner).Scan(&before); err != nil {
		t.Fatal(err)
	}

	var printed strings.Builder
	if err := runResetPIN([]string{"-data-dir", dir}, &printed); err != nil {
		t.Fatalf("listing is not a failure: %v", err)
	}
	list := printed.String()
	for _, want := range []string{owner, "Priya", "owner", "Arjun", "manager", "(locked)", "(no longer works here)", "mez reset-pin -staff"} {
		if !strings.Contains(list, want) {
			t.Errorf("the list does not say %q:\n%s", want, list)
		}
	}
	if regexp.MustCompile(`\d{6}`).MatchString(list) {
		t.Errorf("the list prints something like a PIN:\n%s", list)
	}
	after := ""
	if err := d.Writer().QueryRowContext(ctx, `SELECT pin_hash FROM staff WHERE public_id = ?`, owner).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Error("listing changed a PIN")
	}
}

func TestResetPINRefusals(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t, "-demo-menu")
	runSetupIn(t, args)
	d := opened(t, dir)
	var gone string
	if err := d.Writer().QueryRowContext(ctx, `SELECT public_id FROM staff WHERE name = 'Ravi'`).Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE name = 'Ravi'`); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		says string
	}{
		{"nobody of that id", []string{"-data-dir", dir, "-staff", "stf_NOBODY00"}, "no person has the id"},
		{"someone who no longer works here", []string{"-data-dir", dir, "-staff", gone}, "no longer works here"},
		{"a stray argument", []string{"-data-dir", dir, "-staff", gone, "now"}, "unexpected argument"},
		{"no database there", []string{"-data-dir", t.TempDir(), "-staff", gone}, "there is no database"},
		// A PIN is drawn, never given: it would be in the shell's history.
		{"a PIN on the command line", []string{"-data-dir", dir, "-staff", gone, "-pin", "123456"}, "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var printed strings.Builder
			err := runResetPIN(tt.args, &printed)
			if err == nil || !strings.Contains(err.Error(), tt.says) {
				t.Errorf("err = %v, want it to say %q", err, tt.says)
			}
			if regexp.MustCompile(`(?m)^\s+\d{6}\s*$`).MatchString(printed.String()) {
				t.Errorf("a refusal printed a PIN:\n%s", printed.String())
			}
		})
	}
}
