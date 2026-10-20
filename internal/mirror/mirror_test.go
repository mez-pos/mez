package mirror

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var start = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// world is a copy of a venue with Priya who owns it (PIN 246813), Asha a
// cashier (PIN 2468), Kabir who was deactivated, a till the venue paired,
// and a mirror's door over it.
type world struct {
	db     *db.DB
	clock  *clock
	path   string
	keeper *Keeper
	door   *Auth
	till   string // the venue's till's own token
}

const (
	priya = "stf_PRIYA001"
	asha  = "stf_ASHA0001"
	kabir = "stf_KABIR001"
)

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	seed(t, d)
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	code, err := devices.CreatePairingCode(ctx, auth.NewCode{VenueID: 1, DeviceName: "Counter 1", DeviceKind: auth.KindBackOffice})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(ctx, code.Code)
	if err != nil {
		t.Fatal(err)
	}

	w := &world{db: d, clock: &clock{t: start}, path: filepath.Join(t.TempDir(), "state", "ven_MIRROR01.json"), till: paired.Token}
	w.reopen(t)
	return w
}

// seed makes the venue newWorld describes in d: Priya, Asha and Kabir, and
// the roles they hold.
func seed(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	everything := auth.PermissionSet{}
	for _, p := range auth.Permissions() {
		everything[p] = struct{}{}
	}
	owner, err := everything.JSON()
	if err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_MIRROR01", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: start, UpdatedAt: start,
		})
		if err != nil {
			return err
		}
		for _, member := range []struct {
			id, name, pin, role, perms string
			active                     bool
		}{
			{priya, "Priya", "246813", "owner", owner, true},
			{asha, "Asha", "2468", "cashier", `["order.open"]`, true},
			{kabir, "Kabir", "135790", "manager", `["reports.view"]`, false},
		} {
			role, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.ID, Name: member.role, Permissions: member.perms, CreatedAt: start})
			if err != nil {
				return err
			}
			hash, err := auth.HashPIN(member.pin)
			if err != nil {
				return err
			}
			if _, err := q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: member.id, VenueID: v.ID, RoleID: role.ID, Name: member.name, PinHash: hash, Active: member.active,
				CreatedAt: start, UpdatedAt: start,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// reopen is the mirror started again: a new Keeper over the same file, and
// a new door over the copy.
func (w *world) reopen(t *testing.T) {
	t.Helper()
	keeper, err := OpenKeeper(KeeperOptions{Path: w.path, Now: w.clock.now})
	if err != nil {
		t.Fatalf("OpenKeeper: %v", err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: w.db, Now: w.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: w.db, Now: w.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	door, err := NewAuth(Options{Keeper: keeper, Staff: staff, Devices: devices, VenueID: 1})
	if err != nil {
		t.Fatal(err)
	}
	w.keeper, w.door = keeper, door
}

// browser pairs a browser to the mirror with a fresh code.
func (w *world) browser(t *testing.T) (auth.Device, string) {
	t.Helper()
	code, _, err := w.keeper.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	paired, err := w.door.Redeem(context.Background(), code)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	return paired.Device, paired.Token
}

// written counts the rows of every table a door at the venue writes to:
// sessions, attempts, devices, lockouts, codes, and people locked.
func (w *world) written(t *testing.T) (rows int) {
	t.Helper()
	ctx := context.Background()
	if err := w.db.Reader().QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM staff_sessions) + (SELECT COUNT(*) FROM sign_in_attempts) + (SELECT COUNT(*) FROM devices) +
		(SELECT COUNT(*) FROM staff_lockouts) + (SELECT COUNT(*) FROM pairing_codes) + (SELECT COUNT(*) FROM staff WHERE locked_at IS NOT NULL)`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestABrowserIsPairedOnceWithTheMirrorsCode(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	code, ends, err := w.keeper.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	if !ends.Equal(start.Add(CodeTTL)) || len(code) != 9 || code[4] != '-' {
		t.Errorf("code %q, good until %s", code, ends)
	}

	for _, wrong := range []string{"", "AAAA-AAAA", code + "X", "not a code"} {
		if _, err := w.door.Redeem(ctx, wrong); !errors.Is(err, auth.ErrInvalidPairingCode) {
			t.Errorf("Redeem(%q): %v, want ErrInvalidPairingCode", wrong, err)
		}
	}
	// As it is typed: lower case, no hyphen.
	paired, err := w.door.Redeem(ctx, strings.ToLower(strings.ReplaceAll(code, "-", " ")))
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if paired.Device.Kind != auth.KindBackOffice || paired.Device.Name != BrowserName || paired.Device.ID >= 0 || paired.Device.VenueID != 1 {
		t.Errorf("paired as %+v", paired.Device)
	}
	if _, err := w.door.Redeem(ctx, code); !errors.Is(err, auth.ErrInvalidPairingCode) {
		t.Errorf("the code a second time: %v, want ErrInvalidPairingCode", err)
	}

	got, err := w.door.Authenticate(ctx, paired.Token)
	if err != nil || got != paired.Device {
		t.Errorf("Authenticate: %+v, %v", got, err)
	}
	// The venue's own till opens nothing on a mirror, though the copy
	// holds it: it may have been revoked since the backup was written.
	for _, token := range []string{w.till, "", "nfm_nothing", strings.Replace(paired.Token, "nfm_", "nfd_", 1)} {
		if _, err := w.door.Authenticate(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("Authenticate(%.8q): %v, want ErrUnauthenticated", token, err)
		}
	}

	// A code runs out.
	late, _, err := w.keeper.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	w.clock.advance(CodeTTL)
	if _, err := w.door.Redeem(ctx, late); !errors.Is(err, auth.ErrInvalidPairingCode) {
		t.Errorf("a code ten minutes old: %v, want ErrInvalidPairingCode", err)
	}
	// And a new one puts the last out of use.
	first, _, _ := w.keeper.NewCode()
	second, _, _ := w.keeper.NewCode()
	if _, err := w.door.Redeem(ctx, first); !errors.Is(err, auth.ErrInvalidPairingCode) {
		t.Errorf("a code replaced by another: %v", err)
	}
	other, err := w.door.Redeem(ctx, second)
	if err != nil || other.Device.ID == paired.Device.ID || other.Token == paired.Token {
		t.Errorf("a second browser: %+v, %v", other.Device, err)
	}
}

func TestSigningInOnAMirror(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	other, _ := w.browser(t)

	in, err := w.door.SignInRemote(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatalf("SignInRemote: %v", err)
	}
	s := in.Session
	if s.Staff.PublicID != priya || s.Staff.Name != "Priya" || s.Staff.RoleName != "owner" || !s.Remote || s.Cloud || s.DeviceID != dev.ID {
		t.Errorf("session = %+v", s)
	}
	if !s.ExpiresAt.Equal(start.Add(SessionTTL)) {
		t.Errorf("expires %s, want twelve hours on", s.ExpiresAt)
	}
	held, err := w.door.Permissions(ctx, s.Staff.RoleID)
	if err != nil || !held.Everything() {
		t.Errorf("the owner's permissions on a mirror: %v, %v", held.Keys(), err)
	}

	got, err := w.door.Session(ctx, dev, in.Token)
	if err != nil || got.ID != s.ID || got.Staff.PublicID != priya {
		t.Errorf("Session: %+v, %v", got, err)
	}
	for name, try := range map[string]func() error{
		"on another browser": func() error { _, err := w.door.Session(ctx, other, in.Token); return err },
		"no token":           func() error { _, err := w.door.Session(ctx, dev, ""); return err },
		"a browser's token":  func() error { _, err := w.door.Session(ctx, dev, "nfm_abc"); return err },
	} {
		if err := try(); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("%s: %v, want ErrNoSession", name, err)
		}
	}

	// Twelve hours from the sign-in, used or not.
	w.clock.advance(SessionTTL - time.Second)
	if _, err := w.door.Session(ctx, dev, in.Token); err != nil {
		t.Errorf("in its last second: %v", err)
	}
	w.clock.advance(time.Second)
	if _, err := w.door.Session(ctx, dev, in.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("after twelve hours: %v, want ErrNoSession", err)
	}

	again, err := w.door.SignIn(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.door.SignOut(ctx, again.Session); err != nil {
		t.Fatal(err)
	}
	if _, err := w.door.Session(ctx, dev, again.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("after signing out: %v, want ErrNoSession", err)
	}
}

func TestWhoCannotSignInOnAMirror(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	// The person mez Cloud signs in as has no PIN here either.
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	cloud, err := staff.PairCloudOwner(ctx, 1, "Priya Menon")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct{ name, staff, pin string }{
		{"nobody of that id", "stf_NOBODY00", "246813"},
		{"no id", "", "246813"},
		{"someone deactivated, with their right PIN", kabir, "135790"},
		{"the cloud's owner", cloud.PublicID, "none"},
		{"an id from another table", "ven_MIRROR01", "246813"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Many times over: none of it is a guess at anyone, and none
			// of it is counted.
			for range MaxWrongPINs + 2 {
				if _, err := w.door.SignInRemote(ctx, dev, tt.staff, tt.pin); !errors.Is(err, auth.ErrInvalidPIN) {
					t.Fatalf("err = %v, want ErrInvalidPIN", err)
				}
			}
		})
	}
	if raw, err := os.ReadFile(w.path); err != nil || strings.Contains(string(raw), `"wrong"`) || strings.Contains(string(raw), "locked_until") {
		t.Errorf("someone who could never sign in was counted: %s (%v)", raw, err)
	}
	if _, err := w.door.Authorise(ctx, dev, priya, "246813", auth.PermLineVoid); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("an override on a mirror: %v, want ErrInvalidPIN", err)
	}
	if _, err := w.door.CloudDevice(ctx, "anything"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("CloudDevice on a mirror: %v", err)
	}
}

func TestTheThirdWrongPINLocksForADay(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	wrong := func() {
		t.Helper()
		if _, err := w.door.SignInRemote(ctx, dev, priya, "000000"); !errors.Is(err, auth.ErrInvalidPIN) {
			t.Fatalf("a wrong PIN: %v", err)
		}
	}
	right := func() error {
		_, err := w.door.SignInRemote(ctx, dev, priya, "246813")
		return err
	}

	// Two wrong and then the right one: forgiven, and the count begins again.
	wrong()
	wrong()
	if err := right(); err != nil {
		t.Fatalf("the right PIN after two wrong: %v", err)
	}
	open, err := w.door.SignInRemote(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatal(err)
	}
	wrong()
	wrong()
	if err := right(); err != nil {
		t.Fatalf("the right PIN after two more: %v", err)
	}

	// Three in a row.
	wrong()
	wrong()
	wrong()
	if err := right(); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("the right PIN after three wrong: %v, want it to read as a wrong one", err)
	}
	// Whoever was guessing may have been at a browser left signed in.
	if _, err := w.door.Session(ctx, dev, open.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("a session of someone just locked: %v, want ErrNoSession", err)
	}
	// Nobody else is locked with them.
	if _, err := w.door.SignInRemote(ctx, dev, asha, "2468"); err != nil {
		t.Errorf("another person while one is locked: %v", err)
	}

	// The mirror is started again: the lock is still there. Trying while
	// locked does not make it longer.
	w.reopen(t)
	w.clock.advance(LockFor - time.Minute)
	wrong()
	if err := right(); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("after a restart, a minute before the day is out: %v, want still locked", err)
	}
	w.clock.advance(time.Minute)
	if err := right(); err != nil {
		t.Errorf("a day on: %v, want the lock lifted", err)
	}
	// And a lock that ran out left a clean slate: two wrong do not lock.
	w.clock.advance(LockFor)
	wrong()
	wrong()
	if err := right(); err != nil {
		t.Errorf("two wrong after a lock ran out: %v", err)
	}
}

// TestTheCopyIsNotWritten: pairing, signing in, wrong PINs, a lock, a
// sign-out. The mirror remembers all of it, and none of it is in the copy.
func TestTheCopyIsNotWritten(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	rows := w.written(t)

	dev, token := w.browser(t)
	if _, err := w.door.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	in, err := w.door.SignInRemote(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.door.Session(ctx, dev, in.Token); err != nil {
		t.Fatal(err)
	}
	for range MaxWrongPINs {
		_, _ = w.door.SignInRemote(ctx, dev, asha, "9999")
	}
	if _, err := w.door.SignInRemote(ctx, dev, asha, "2468"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Fatalf("Asha after three wrong: %v, want locked", err)
	}
	if err := w.door.SignOut(ctx, in.Session); err != nil {
		t.Fatal(err)
	}
	if _, err := w.door.Roster(ctx, 1); err != nil {
		t.Fatal(err)
	}

	if now := w.written(t); now != rows {
		t.Errorf("the copy was written: %d rows where a venue's door writes, and there were %d", now, rows)
	}
}

func TestASessionFollowsTheCopy(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	in, err := w.door.SignInRemote(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatal(err)
	}
	// The next copy has her renamed.
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET name = 'Priya K' WHERE public_id = ?`, priya); err != nil {
		t.Fatal(err)
	}
	if s, err := w.door.Session(ctx, dev, in.Token); err != nil || s.Staff.Name != "Priya K" {
		t.Errorf("after the copy changed: %+v, %v", s.Staff, err)
	}
	// And the one after has her deactivated at the venue.
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE public_id = ?`, priya); err != nil {
		t.Fatal(err)
	}
	if _, err := w.door.Session(ctx, dev, in.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("deactivated at the venue: %v, want ErrNoSession", err)
	}
	// Brought back, the old session does not come back with her.
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET active = 1 WHERE public_id = ?`, priya); err != nil {
		t.Fatal(err)
	}
	if _, err := w.door.Session(ctx, dev, in.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("brought back: %v, want the old session still gone", err)
	}
}

func TestAMirrorRemembersItsBrowsersAndForgetsItsSessions(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, token := w.browser(t)
	in, err := w.door.SignInRemote(ctx, dev, priya, "246813")
	if err != nil {
		t.Fatal(err)
	}

	w.reopen(t)
	got, err := w.door.Authenticate(ctx, token)
	if err != nil || got != dev {
		t.Errorf("the browser after a restart: %+v, %v", got, err)
	}
	if _, err := w.door.Session(ctx, dev, in.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("a session after a restart: %v, want the PIN asked again", err)
	}
	// The file holds no token and no PIN.
	raw, err := os.ReadFile(w.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token, in.Token, "246813", strings.TrimPrefix(token, "nfm_")} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the state file holds %q", secret)
		}
	}

	// The Devices list: the venue's, then the mirror's own.
	roster, err := w.door.Roster(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster.Devices) != 2 || roster.Devices[0].Name != "Counter 1" || roster.Devices[1].ID != dev.ID || len(roster.Waiting) != 0 {
		t.Errorf("roster = %+v", roster)
	}
}

func TestAStateFileThatCannotBeReadStopsTheMirror(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ven.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenKeeper(KeeperOptions{Path: path}); err == nil || !strings.Contains(err.Error(), "remove it") {
		t.Errorf("err = %v, want it to say what to do", err)
	}
	// It holds locks: it is not written over.
	if raw, _ := os.ReadFile(path); string(raw) != "{not json" {
		t.Errorf("the file was changed to %q", raw)
	}
	if _, err := OpenKeeper(KeeperOptions{}); err == nil {
		t.Error("a keeper with no file was opened")
	}
}

func TestADoorChangesNothing(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if _, err := w.door.CreatePairingCode(ctx, auth.NewCode{}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("CreatePairingCode: %v", err)
	}
	if err := w.door.Rename(ctx, 1, "dev_X", "New", 1); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Rename: %v", err)
	}
	if err := w.door.RevokeByPublicID(ctx, 1, "dev_X", 1, 1); !errors.Is(err, ErrReadOnly) {
		t.Errorf("RevokeByPublicID: %v", err)
	}
	if _, err := NewAuth(Options{}); err == nil {
		t.Error("a door over nothing was made")
	}
}

// oneAtATime is a copy's staff that notes how many PINs were being compared
// at once.
type oneAtATime struct {
	Staff
	mu       sync.Mutex
	inFlight int
	most     int
	checked  int
}

func (o *oneAtATime) CheckPIN(ctx context.Context, venueID int64, staffRef, pin string) (auth.Staff, auth.PINVerdict, error) {
	o.mu.Lock()
	o.inFlight++
	o.checked++
	o.most = max(o.most, o.inFlight)
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.inFlight--
		o.mu.Unlock()
	}()
	return o.Staff.CheckPIN(ctx, venueID, staffRef, pin)
}

// TestGuessesSentAtOnceAreCountedOneByOne: twenty PINs for one person sent
// together, the right one among them. Each is compared only after the one
// before it was counted, so the lock falls after the third and the right
// one, unless it was among the first three, opens nothing.
func TestGuessesSentAtOnceAreCountedOneByOne(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: w.db, Now: w.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	watched := &oneAtATime{Staff: staff}
	// A second door over the same keeper, as a mirror has while it changes
	// copies: the queue is the mirror's, not one door's.
	doors := []*Auth{w.door}
	for range 2 {
		door, err := NewAuth(Options{Keeper: w.keeper, Staff: watched, Devices: devices, VenueID: 1})
		if err != nil {
			t.Fatal(err)
		}
		doors = append(doors, door)
	}
	doors[0].staff = watched

	// Three wrong ones first, so the test does not turn on which of twenty
	// goroutines runs first.
	for range MaxWrongPINs {
		if _, err := doors[0].SignInRemote(ctx, dev, priya, "000000"); !errors.Is(err, auth.ErrInvalidPIN) {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	opened := make(chan string, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pin := "111111"
			if i%4 == 0 {
				pin = "246813"
			}
			if in, err := doors[i%len(doors)].SignInRemote(ctx, dev, priya, pin); err == nil {
				opened <- in.Token
			}
		}()
	}
	wg.Wait()
	close(opened)
	if n := len(opened); n != 0 {
		t.Errorf("%d sign-ins got through a lock", n)
	}
	if watched.most != 1 {
		t.Errorf("%d PINs were being compared at once, want one at a time", watched.most)
	}
	if watched.checked != MaxWrongPINs+20 {
		t.Errorf("%d PINs compared, want every one, so that a refusal takes as long as any other", watched.checked)
	}
}

// TestAFileThatCannotBeWrittenIsNotAWayToGuess: the state file stops
// taking writes. Wrong PINs are still wrong PINs, in the same words, the
// third still locks, and the right one after it opens nothing.
func TestAFileThatCannotBeWrittenIsNotAWayToGuess(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	dev, _ := w.browser(t)
	// Where the file was there is now a directory with something in it,
	// which no rename can replace.
	if err := os.Remove(w.path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(w.path, "in the way"), 0o700); err != nil {
		t.Fatal(err)
	}

	for i := range MaxWrongPINs {
		if _, err := w.door.SignInRemote(ctx, dev, priya, "000000"); !errors.Is(err, auth.ErrInvalidPIN) {
			t.Fatalf("wrong PIN %d with the file unwritable: %v, want it to read as any wrong PIN", i+1, err)
		}
	}
	if _, err := w.door.SignInRemote(ctx, dev, priya, "246813"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("the right PIN after three uncounted on disk: %v, want locked all the same", err)
	}
	// Someone else still signs in: a right PIN needs nothing written.
	if _, err := w.door.SignInRemote(ctx, dev, asha, "2468"); err != nil {
		t.Errorf("another person with the file unwritable: %v", err)
	}
}

// TestTwoMirrorsOfOneVenueShareTheirLocks: two mirrors running at once on
// one state file, as with -generation against an older backup. A lock set
// at one holds at the other, and neither writes over what the other
// remembered.
func TestTwoMirrorsOfOneVenueShareTheirLocks(t *testing.T) {
	a := newWorld(t)
	ctx := context.Background()
	devA, tokenA := a.browser(t)

	// The second mirror: its own process's memory, the same file, started
	// before anything is locked.
	b := &world{db: a.db, clock: a.clock, path: a.path}
	b.reopen(t)
	devB, tokenB := b.browser(t)

	for range MaxWrongPINs {
		_, _ = a.door.SignInRemote(ctx, devA, priya, "000000")
	}
	if _, err := b.door.SignInRemote(ctx, devB, priya, "246813"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("locked at one mirror, the right PIN at the other: %v, want locked there too", err)
	}
	// The second counts a wrong PIN for someone else, and so writes the file.
	if _, err := b.door.SignInRemote(ctx, devB, asha, "9999"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Fatal(err)
	}
	if _, err := a.door.SignInRemote(ctx, devA, priya, "246813"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("after the other mirror wrote the file: %v, want the lock still held", err)
	}
	// Each still knows the browser paired at the other, and its own.
	for name, try := range map[string]func() error{
		"A's browser at A": func() error { _, err := a.door.Authenticate(ctx, tokenA); return err },
		"A's browser at B": func() error { _, err := b.door.Authenticate(ctx, tokenA); return err },
		"B's browser at A": func() error { _, err := a.door.Authenticate(ctx, tokenB); return err },
		"B's browser at B": func() error { _, err := b.door.Authenticate(ctx, tokenB); return err },
	} {
		if err := try(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if devA.ID == devB.ID {
		t.Errorf("two browsers were given the id %d", devA.ID)
	}
	// And a mirror started after both finds all of it.
	a.reopen(t)
	if _, err := a.door.SignInRemote(ctx, devA, priya, "246813"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("a mirror started afterwards: %v, want the lock still held", err)
	}
	if n := len(a.keeper.Browsers()); n != 2 {
		t.Errorf("%d browsers remembered, want both", n)
	}
}

// TestASignInWhoseCallerHasGoneIsNotRun: a request that gave up while
// waiting its turn compares nothing and counts nothing.
func TestASignInWhoseCallerHasGoneIsNotRun(t *testing.T) {
	w := newWorld(t)
	dev, _ := w.browser(t)
	release, ok := w.keeper.turn(nil)
	if !ok {
		t.Fatal("could not take the turn")
	}
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	for range MaxWrongPINs + 1 {
		if _, err := w.door.SignInRemote(gone, dev, priya, "000000"); !errors.Is(err, context.Canceled) {
			t.Fatalf("a sign-in whose caller has gone: %v, want it given up", err)
		}
	}
	release()
	if _, err := w.door.SignInRemote(context.Background(), dev, priya, "246813"); err != nil {
		t.Errorf("after four that were never run: %v, want nothing counted", err)
	}
}
