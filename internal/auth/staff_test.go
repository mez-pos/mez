package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// staffWorld is a database with venues 1 and 2, a paired terminal at each,
// Asha (PIN 2468) and Ravi (PIN 1357) at venue 1, and Meera (PIN 2468) at
// venue 2.
type staffWorld struct {
	auth  *StaffAuth
	db    *db.DB
	clock *clock
	till  Device // venue 1
	bar   Device // venue 1
	other Device // venue 2
	asha  gen.Staff
	ravi  gen.Staff
	meera gen.Staff
}

func newStaffWorld(t *testing.T) *staffWorld {
	t.Helper()
	devs, d, c := newDevices(t)
	w := &staffWorld{db: d, clock: c}
	w.till = pair(t, devs, 1).Device
	w.bar = pair(t, devs, 1).Device
	w.other = pair(t, devs, 2).Device
	w.asha = addStaff(t, d, 1, "stf_ASHA0001", "Asha", "2468", "cashier")
	w.ravi = addStaff(t, d, 1, "stf_RAVI0001", "Ravi", "1357", "manager")
	w.meera = addStaff(t, d, 2, "stf_MEERA001", "Meera", "2468", "cashier")
	a, err := NewStaffAuth(StaffOptions{DB: d, Now: c.now})
	if err != nil {
		t.Fatalf("NewStaffAuth: %v", err)
	}
	w.auth = a
	return w
}

// addStaff creates a staff member, and the role they hold if the venue does
// not have it yet. Roles start with no permissions: a test that needs one
// grants it, so what is being relied on is written down in the test.
func addStaff(t *testing.T, d *db.DB, venue int64, publicID, name, pin, roleName string) gen.Staff {
	t.Helper()
	ctx := context.Background()
	hash, err := HashPIN(pin)
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	var s gen.Staff
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		roles, err := q.RolesForVenue(ctx, venue)
		if err != nil {
			return err
		}
		roleID := int64(0)
		for _, r := range roles {
			if r.Name == roleName {
				roleID = r.ID
			}
		}
		if roleID == 0 {
			r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: venue, Name: roleName, Permissions: "[]", CreatedAt: start})
			if err != nil {
				return err
			}
			roleID = r.ID
		}
		s, err = q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: publicID, VenueID: venue, RoleID: roleID, Name: name, PinHash: hash, Active: true,
			CreatedAt: start, UpdatedAt: start,
		})
		return err
	})
	if err != nil {
		t.Fatalf("adding staff %s: %v", name, err)
	}
	return s
}

// outcomes is every recorded attempt's outcome, oldest first.
func outcomes(t *testing.T, d *db.DB) []string {
	t.Helper()
	rows, err := d.Reader().QueryContext(context.Background(), `SELECT outcome FROM sign_in_attempts ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			t.Fatal(err)
		}
		out = append(out, o)
	}
	return out
}

func TestSignInAndUseTheSession(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	in, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if !strings.HasPrefix(in.Token, "nfs_") || in.Session.Staff.Name != "Asha" || !in.Session.ExpiresAt.Equal(start.Add(DefaultSessionTTL)) {
		t.Fatalf("signed in = %+v", in)
	}
	got, err := w.auth.Session(ctx, w.till, in.Token)
	if err != nil || got != in.Session {
		t.Fatalf("Session = %+v, %v; want %+v", got, err, in.Session)
	}

	// The same token from the bar terminal, or the venue next door, is nothing.
	for _, dev := range []Device{w.bar, w.other} {
		if _, err := w.auth.Session(ctx, dev, in.Token); !errors.Is(err, ErrNoSession) {
			t.Errorf("session from device %d: err = %v", dev.ID, err)
		}
	}

	if err := w.auth.SignOut(ctx, got); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if _, err := w.auth.Session(ctx, w.till, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after sign out: err = %v", err)
	}
	if err := w.auth.SignOut(ctx, got); err != nil {
		t.Errorf("signing out twice: %v", err)
	}
	if got := outcomes(t, w.db); len(got) != 1 || got[0] != "success" {
		t.Errorf("attempts = %v, want one success", got)
	}
}

func TestSessionsEnd(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	in, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468")
	if err != nil {
		t.Fatal(err)
	}
	w.clock.advance(DefaultSessionTTL - time.Second)
	if _, err := w.auth.Session(ctx, w.till, in.Token); err != nil {
		t.Fatalf("a second before expiry: %v", err)
	}
	w.clock.advance(time.Second)
	if _, err := w.auth.Session(ctx, w.till, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("at expiry: err = %v", err)
	}

	in, err = w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE id = ?`, w.ravi.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.auth.Session(ctx, w.till, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("deactivated staff: err = %v", err)
	}

	for _, token := range []string{"", "nfd_" + strings.TrimPrefix(in.Token, "nfs_"), "nfs_" + strings.Repeat("A", 43)} {
		if _, err := w.auth.Session(ctx, w.till, token); !errors.Is(err, ErrNoSession) {
			t.Errorf("token %q: err = %v", token, err)
		}
	}
}

// TestWrongAnswersAllLookTheSame covers who can fail, and how it is recorded.
func TestWrongAnswersAllLookTheSame(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE id = ?`, w.ravi.ID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		staff   string
		pin     string
		outcome string
	}{
		{"a wrong PIN", "stf_ASHA0001", "1111", "invalid_pin"},
		{"a deactivated staff member's right PIN", "stf_RAVI0001", "1357", "inactive_staff"},
		{"a staff member of another venue", "stf_MEERA001", "2468", "unknown_staff"},
		{"nobody", "stf_NOBODY00", "2468", "unknown_staff"},
		{"no staff at all", "", "2468", "unknown_staff"},
		{"a PIN far too long", "stf_ASHA0001", strings.Repeat("2", 10_000), "invalid_pin"},
		{"a staff ID far too long", strings.Repeat("x", 500), "2468", "unknown_staff"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.auth.SignIn(ctx, w.till, tt.staff, tt.pin)
			if err != ErrInvalidPIN {
				t.Fatalf("err = %v, want exactly ErrInvalidPIN", err)
			}
			if got := outcomes(t, w.db); len(got) != i+1 || got[i] != tt.outcome {
				t.Errorf("recorded %v, want %q last", got, tt.outcome)
			}
		})
	}
}

func TestAStaffMemberLocksOutAndComesBack(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	p := DefaultStaffLockout

	for i := range p.MaxFailures {
		w.clock.advance(time.Minute)
		if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "0000"); err != ErrInvalidPIN {
			t.Fatalf("failure %d: err = %v", i+1, err)
		}
	}
	firstFailure := start.Add(time.Minute)

	// Now even the right PIN is refused, without being compared, and the
	// refusal is recorded but does not extend the lock.
	var locked *LockedError
	_, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468")
	if !errors.As(err, &locked) || !errors.Is(err, ErrLockedOut) || locked.Scope != "staff" {
		t.Fatalf("right PIN while locked: err = %v", err)
	}
	if want := firstFailure.Add(p.Window).Sub(w.clock.now()); locked.RetryAfter != want {
		t.Errorf("retry after %v, want %v", locked.RetryAfter, want)
	}
	for range 30 {
		_, _ = w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468")
	}

	// Ravi, on the same terminal, is not locked.
	if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357"); err != nil {
		t.Errorf("another staff member was locked out too: %v", err)
	}

	// When the oldest failure leaves the window, the lock lifts.
	w.clock.advance(firstFailure.Add(p.Window).Sub(w.clock.now()))
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Fatalf("after the window: %v", err)
	}

	// A success resets the count: four more failures do not lock.
	for range p.MaxFailures - 1 {
		_, _ = w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "0000")
	}
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Errorf("four failures after a success locked: %v", err)
	}

	count := map[string]int{}
	for _, o := range outcomes(t, w.db) {
		count[o]++
	}
	if count["locked_out"] != 31 || count["invalid_pin"] != p.MaxFailures+p.MaxFailures-1 || count["success"] != 3 {
		t.Errorf("recorded %v", count)
	}
}

// TestATabletCannotCycleThroughEveryone is the device limit: failures across
// many staff IDs lock the device, even when no one staff member is locked.
func TestATabletCannotCycleThroughEveryone(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	p := DefaultDeviceLockout

	for i := range p.MaxFailures {
		// Made-up IDs, so no staff member ever reaches their own limit.
		if _, err := w.auth.SignIn(ctx, w.till, "stf_GUESS"+string(rune('A'+i%26)), "1234"); err != ErrInvalidPIN {
			t.Fatalf("failure %d: err = %v", i+1, err)
		}
	}
	var locked *LockedError
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); !errors.As(err, &locked) || locked.Scope != "device" {
		t.Fatalf("after %d failures on one device: err = %v", p.MaxFailures, err)
	}
	if _, err := w.auth.SignIn(ctx, w.bar, "stf_ASHA0001", "2468"); err != nil {
		t.Errorf("the bar terminal was locked by the till's failures: %v", err)
	}
	w.clock.advance(p.Window)
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Errorf("after the window: %v", err)
	}
}

// TestParallelGuessesCannotOutrunTheLock fires many wrong PINs at once. The
// per-staff lock serialises them, so no more than the limit are compared.
func TestParallelGuessesCannotOutrunTheLock(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "9999")
		}()
	}
	wg.Wait()
	count := map[string]int{}
	for _, o := range outcomes(t, w.db) {
		count[o]++
	}
	if count["invalid_pin"] != DefaultStaffLockout.MaxFailures || count["locked_out"] != 20-DefaultStaffLockout.MaxFailures {
		t.Errorf("recorded %v: more PINs were compared than the limit allows", count)
	}
}

func TestActiveStaffAndOptions(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	staff, err := w.auth.ActiveStaff(ctx, 1)
	if err != nil || len(staff) != 2 || staff[0].Name != "Asha" || staff[1].Name != "Ravi" {
		t.Errorf("ActiveStaff = %+v, %v", staff, err)
	}
	if _, err := NewStaffAuth(StaffOptions{}); err == nil {
		t.Error("NewStaffAuth with no database succeeded")
	}
	custom, err := NewStaffAuth(StaffOptions{DB: w.db, SessionTTL: time.Hour, StaffLockout: LockoutPolicy{MaxFailures: 2, Window: time.Minute}})
	if err != nil || custom.ttl != time.Hour || custom.staffLockout.MaxFailures != 2 || custom.deviceLockout != DefaultDeviceLockout {
		t.Errorf("options not applied: %+v, %v", custom, err)
	}
}

func TestLockRemaining(t *testing.T) {
	p := LockoutPolicy{MaxFailures: 3, Window: 10 * time.Minute}
	at := func(m int) time.Time { return start.Add(time.Duration(m) * time.Minute) }
	tests := []struct {
		name     string
		failures []time.Time // newest first
		now      time.Time
		want     time.Duration
	}{
		{"fewer than the limit", []time.Time{at(2), at(1)}, at(3), 0},
		{"the limit, inside the window", []time.Time{at(3), at(2), at(1)}, at(4), 7 * time.Minute},
		{"the limit, the oldest just leaving", []time.Time{at(3), at(2), at(1)}, at(11), 0},
		{"older failures beyond the limit do not matter", []time.Time{at(9), at(8), at(7), at(1)}, at(10), 7 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lockRemaining(tt.failures, p, tt.now); got != tt.want {
				t.Errorf("lockRemaining = %v, want %v", got, tt.want)
			}
		})
	}
}
