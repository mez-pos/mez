package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
)

const testNonce = "nonce-0123456789abcdef"

// addRole gives a venue a role that grants these permissions.
func addRole(t *testing.T, w *staffWorld, venue int64, name string, perms []Permission) int64 {
	t.Helper()
	ctx := context.Background()
	set := PermissionSet{}
	for _, p := range perms {
		set[p] = struct{}{}
	}
	raw, err := set.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = w.db.WriteTx(ctx, func(q *gen.Queries) error {
		r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: venue, Name: name, Permissions: raw, CreatedAt: w.clock.now().UTC()})
		id = r.ID
		return err
	})
	if err != nil {
		t.Fatalf("adding the role %s: %v", name, err)
	}
	return id
}

// cloudWorld is a staffWorld whose venue 1 has an owner's role and has been
// paired: it has a cloud owner and a cloud device.
func cloudWorld(t *testing.T) (*staffWorld, Staff) {
	t.Helper()
	w := newStaffWorld(t)
	addRole(t, w, 1, "owner", Permissions())
	owner, err := w.auth.PairCloudOwner(context.Background(), 1, "Priya Menon")
	if err != nil {
		t.Fatalf("PairCloudOwner: %v", err)
	}
	return w, owner
}

// vouched is an assertion checked a moment ago, with six minutes left
// before its nonce may be forgotten.
func (w *staffWorld) vouched(nonce string) Vouched {
	return Vouched{Nonce: nonce, KeepUntil: w.clock.now().Add(6 * time.Minute)}
}

func (w *staffWorld) deactivate(t *testing.T, staffID int64) {
	t.Helper()
	ctx := context.Background()
	err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetStaffActive(ctx, gen.SetStaffActiveParams{Active: false, Now: w.clock.now().UTC(), VenueID: 1, ID: staffID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (w *staffWorld) revokeCloudDevice(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		dev, err := q.CloudDeviceForVenue(ctx, 1)
		if err != nil {
			return err
		}
		_, err = q.RevokeDevice(ctx, gen.RevokeDeviceParams{
			RevokedAt: sql.NullTime{Time: w.clock.now().UTC(), Valid: true}, ID: dev.ID, VenueID: 1,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, w *staffWorld, query string, args ...any) int {
	t.Helper()
	var n int
	if err := w.db.Reader().QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func cloudDevices(t *testing.T, w *staffWorld, venue int64) (inService, revoked int) {
	t.Helper()
	odd := count(t, w, "SELECT COUNT(*) FROM devices WHERE venue_id = ? AND cloud = 1 AND (token_hash != '' OR kind != 'back_office' OR name != ?)", venue, CloudDeviceName)
	if odd != 0 {
		t.Errorf("%d cloud devices have a token, or are not a back office called %q", odd, CloudDeviceName)
	}
	inService = count(t, w, "SELECT COUNT(*) FROM devices WHERE venue_id = ? AND cloud = 1 AND revoked_at IS NULL", venue)
	revoked = count(t, w, "SELECT COUNT(*) FROM devices WHERE venue_id = ? AND cloud = 1 AND revoked_at IS NOT NULL", venue)
	return inService, revoked
}

func TestPairingMakesTheCloudsOwner(t *testing.T) {
	w, owner := cloudWorld(t)
	ctx := context.Background()

	if owner.Name != "Priya Menon" || owner.RoleName != "owner" || owner.VenueID != 1 {
		t.Errorf("owner = %+v", owner)
	}
	row, err := w.db.Queries().StaffByPublicID(ctx, owner.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Cloud || !row.Active || row.PinHash != NoPIN {
		t.Errorf("row = cloud %v, active %v, pin hash %q", row.Cloud, row.Active, row.PinHash)
	}
	held, err := w.auth.Permissions(ctx, row.RoleID)
	if err != nil || !held.Everything() {
		t.Errorf("the cloud's owner holds %v (err %v), want everything", held.Keys(), err)
	}
	if in, gone := cloudDevices(t, w, 1); in != 1 || gone != 0 {
		t.Errorf("cloud devices: %d in service, %d revoked", in, gone)
	}

	// Nobody types a PIN for them, so they are not on the sign-in tiles.
	tiles, err := w.auth.ActiveStaff(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range tiles {
		if s.ID == owner.ID {
			t.Error("the cloud's owner is on the sign-in tiles")
		}
	}
	// Nor counted among those who can manage staff at the venue.
	if n, err := w.db.Queries().StaffManagersLeft(ctx, 1); err != nil || n != 0 {
		t.Errorf("managers left = %d (err %v), want 0: the cloud's owner is not one", n, err)
	}
	// Another venue on the same install has neither.
	if in, gone := cloudDevices(t, w, 2); in+gone != 0 {
		t.Errorf("venue 2 has %d cloud devices", in+gone)
	}
}

func TestPairingAgainRestoresTheCloudsOwner(t *testing.T) {
	w, owner := cloudWorld(t)
	ctx := context.Background()

	// Someone at the venue turns all of it off: renamed and demoted,
	// deactivated, and the device revoked.
	err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.UpdateStaff(ctx, gen.UpdateStaffParams{Name: "Old Name", RoleID: w.asha.RoleID, Now: w.clock.now().UTC(), VenueID: 1, ID: owner.ID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	w.deactivate(t, owner.ID)
	w.revokeCloudDevice(t)

	// A start leaves it as it was found.
	same, err := w.auth.EnsureCloudOwner(ctx, 1, "Priya Menon")
	if err != nil {
		t.Fatalf("EnsureCloudOwner: %v", err)
	}
	if same.ID != owner.ID || same.Name != "Old Name" {
		t.Errorf("EnsureCloudOwner = %+v, want the person untouched", same)
	}
	if _, err := w.auth.SignInCloud(ctx, 1, w.vouched("nonce-before-pairing-again")); !errors.Is(err, ErrNoCloudOwner) {
		t.Errorf("signing in while turned off: %v, want ErrNoCloudOwner", err)
	}

	// Pairing again puts it back, as the same person.
	back, err := w.auth.PairCloudOwner(ctx, 1, "Priya M.")
	if err != nil {
		t.Fatalf("PairCloudOwner again: %v", err)
	}
	if back.ID != owner.ID || back.PublicID != owner.PublicID || back.Name != "Priya M." || back.RoleName != "owner" {
		t.Errorf("after pairing again: %+v", back)
	}
	if in, gone := cloudDevices(t, w, 1); in != 1 || gone != 1 {
		t.Errorf("cloud devices: %d in service, %d revoked, want a new one beside the revoked", in, gone)
	}
	if n := count(t, w, "SELECT COUNT(*) FROM staff WHERE venue_id = 1 AND cloud = 1"); n != 1 {
		t.Errorf("%d cloud owners, want 1", n)
	}
	if _, err := w.auth.SignInCloud(ctx, 1, w.vouched("nonce-after-pairing-again")); err != nil {
		t.Errorf("signing in after pairing again: %v", err)
	}
}

func TestEnsureMakesTheCloudsOwnerOnlyOnce(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	addRole(t, w, 1, "owner", Permissions())

	first, err := w.auth.EnsureCloudOwner(ctx, 1, "Priya Menon")
	if err != nil {
		t.Fatalf("EnsureCloudOwner: %v", err)
	}
	second, err := w.auth.EnsureCloudOwner(ctx, 1, "Somebody Else")
	if err != nil {
		t.Fatalf("EnsureCloudOwner again: %v", err)
	}
	if second.ID != first.ID || second.Name != "Priya Menon" {
		t.Errorf("second = %+v, first = %+v", second, first)
	}
	if in, _ := cloudDevices(t, w, 1); in != 1 {
		t.Errorf("%d cloud devices in service, want 1", in)
	}

	// The roles are rearranged so that none grants everything. A start
	// still finds the person there and has nothing to complain of.
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE roles SET permissions = '[]' WHERE name = 'owner'`); err != nil {
		t.Fatal(err)
	}
	if third, err := w.auth.EnsureCloudOwner(ctx, 1, "Priya Menon"); err != nil || third.ID != first.ID {
		t.Errorf("with no role that grants everything: %+v, %v; want the one there, untouched", third, err)
	}
	if _, err := w.auth.PairCloudOwner(ctx, 1, "Priya Menon"); !errors.Is(err, ErrNoOwnerRole) {
		t.Errorf("pairing again with no such role: %v, want ErrNoOwnerRole", err)
	}
}

func TestTheCloudsOwnerNeedsARoleThatGrantsEverything(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	// Nearly everything is not everything.
	addRole(t, w, 1, "nearly", Permissions()[1:])

	if _, err := w.auth.PairCloudOwner(ctx, 1, "Priya Menon"); !errors.Is(err, ErrNoOwnerRole) {
		t.Fatalf("err = %v, want ErrNoOwnerRole", err)
	}
	if n := count(t, w, "SELECT COUNT(*) FROM staff WHERE cloud = 1") + count(t, w, "SELECT COUNT(*) FROM devices WHERE cloud = 1"); n != 0 {
		t.Errorf("%d rows were made for an owner who was not", n)
	}

	addRole(t, w, 1, "owner", Permissions())
	for _, name := range []string{"", "   ", strings.Repeat("a", maxCloudOwnerName+1)} {
		if _, err := w.auth.PairCloudOwner(ctx, 1, name); err == nil {
			t.Errorf("a cloud owner was made with the name %q", name)
		}
	}
}

func TestTheCloudSignsItsOwnerIn(t *testing.T) {
	w, owner := cloudWorld(t)
	ctx := context.Background()
	start := w.clock.now()

	in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
	if err != nil {
		t.Fatalf("SignInCloud: %v", err)
	}
	s := in.Session
	if s.Staff.ID != owner.ID || s.Staff.Name != "Priya Menon" || s.Staff.RoleName != "owner" {
		t.Errorf("session staff = %+v", s.Staff)
	}
	if !s.Remote || !s.Cloud {
		t.Errorf("session remote %v, cloud %v, want both", s.Remote, s.Cloud)
	}
	if want := start.Add(RemoteSessionTTL); !s.ExpiresAt.Equal(want) {
		t.Errorf("expires %s, want %s", s.ExpiresAt, want)
	}

	// The token opens that session on the cloud's device, and on no other.
	dev, err := w.db.Queries().CloudDeviceForVenue(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	cloudDev := Device{ID: dev.ID, PublicID: dev.PublicID, VenueID: 1, Name: dev.Name, Kind: DeviceKind(dev.Kind)}
	got, err := w.auth.Session(ctx, cloudDev, in.Token)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if got.ID != s.ID || !got.Remote || !got.Cloud || got.Staff.ID != owner.ID {
		t.Errorf("session read back = %+v", got)
	}
	if _, err := w.auth.Session(ctx, w.till, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the cloud's session on a till: %v, want ErrNoSession", err)
	}

	// A PIN's session is not the cloud's.
	pin, err := w.auth.SignInRemote(ctx, w.till, w.ravi.PublicID, "1357")
	if err != nil {
		t.Fatal(err)
	}
	if pin.Session.Cloud {
		t.Error("a session opened with a PIN says the cloud opened it")
	}

	// It ends at twelve hours, whatever is done with it.
	w.clock.advance(RemoteSessionTTL)
	if _, err := w.auth.Session(ctx, cloudDev, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after twelve hours: %v, want ErrNoSession", err)
	}
}

func TestAnAssertionIsSpentOnce(t *testing.T) {
	w, _ := cloudWorld(t)
	ctx := context.Background()
	v := w.vouched(testNonce)

	if _, err := w.auth.SignInCloud(ctx, 1, v); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := w.auth.SignInCloud(ctx, 1, v); !errors.Is(err, ErrAssertionSpent) {
		t.Errorf("replayed: %v, want ErrAssertionSpent", err)
	}
	if n := count(t, w, "SELECT COUNT(*) FROM staff_sessions"); n != 1 {
		t.Errorf("%d sessions, want the one the first presentation opened", n)
	}

	// Refused in the last moment it is remembered, and after that for its
	// age: there is no gap between the two to slip through.
	w.clock.advance(6*time.Minute - time.Second)
	if _, err := w.auth.SignInCloud(ctx, 1, v); !errors.Is(err, ErrAssertionSpent) {
		t.Errorf("replayed at the end: %v, want ErrAssertionSpent", err)
	}
	w.clock.advance(time.Second)
	if _, err := w.auth.SignInCloud(ctx, 1, v); !errors.Is(err, ErrAssertionSpent) {
		t.Errorf("replayed once it may be forgotten: %v, want ErrAssertionSpent", err)
	}

	// A dead assertion's nonce is forgotten, a margin later, by the next
	// sign-in.
	w.clock.advance(nonceMargin)
	if _, err := w.auth.SignInCloud(ctx, 1, w.vouched("nonce-another-one-0001")); err != nil {
		t.Fatalf("a new assertion: %v", err)
	}
	if n := count(t, w, "SELECT COUNT(*) FROM cloud_assertions"); n != 1 {
		t.Errorf("%d nonces kept, want only the live one", n)
	}
}

// TestAClockPutRightDoesNotForgetANonce: the venue's clock runs twenty
// minutes fast when someone signs in, and is then put right. An assertion
// spent before that is inside its time again, and is still refused.
func TestAClockPutRightDoesNotForgetANonce(t *testing.T) {
	w, _ := cloudWorld(t)
	ctx := context.Background()
	v := w.vouched(testNonce)
	if _, err := w.auth.SignInCloud(ctx, 1, v); err != nil {
		t.Fatal(err)
	}
	w.clock.advance(20 * time.Minute)
	if _, err := w.auth.SignInCloud(ctx, 1, w.vouched("nonce-while-running-fast")); err != nil {
		t.Fatal(err)
	}
	w.clock.advance(-20 * time.Minute)
	if _, err := w.auth.SignInCloud(ctx, 1, v); !errors.Is(err, ErrAssertionSpent) {
		t.Errorf("replayed after the clock was put right: %v, want ErrAssertionSpent", err)
	}
}

// TestRevokingTheCloudsDeviceEndsItsSessions: the device has no token to
// refuse, so the session is what must stop, and at once.
func TestRevokingTheCloudsDeviceEndsItsSessions(t *testing.T) {
	w, _ := cloudWorld(t)
	ctx := context.Background()
	in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
	if err != nil {
		t.Fatal(err)
	}
	dev, err := w.db.Queries().CloudDeviceForVenue(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	cloudDev := Device{ID: dev.ID, PublicID: dev.PublicID, VenueID: 1, Name: dev.Name, Kind: DeviceKind(dev.Kind)}
	if _, err := w.auth.Session(ctx, cloudDev, in.Token); err != nil {
		t.Fatalf("before revoking: %v", err)
	}
	w.revokeCloudDevice(t)
	if _, err := w.auth.Session(ctx, cloudDev, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after revoking: %v, want ErrNoSession", err)
	}
	// And pairing again does not bring that session back with the device.
	if _, err := w.auth.PairCloudOwner(ctx, 1, "Priya Menon"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.auth.Session(ctx, cloudDev, in.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after pairing again: %v, want ErrNoSession", err)
	}
}

// TestTheCloudsDeviceIsKnownByItsSession: the device has no token, so a
// live session of the cloud's owner is the one thing that stands for it
// (ADR-324).
func TestTheCloudsDeviceIsKnownByItsSession(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		token func(t *testing.T, w *staffWorld, cloudToken string) string
		after func(t *testing.T, w *staffWorld, owner Staff)
		want  error
	}{
		{name: "the cloud's session"},
		{name: "a PIN's session from away", want: ErrUnauthenticated,
			token: func(t *testing.T, w *staffWorld, _ string) string {
				in, err := w.auth.SignInRemote(ctx, w.till, w.ravi.PublicID, "1357")
				if err != nil {
					t.Fatal(err)
				}
				return in.Token
			}},
		{name: "nothing", want: ErrUnauthenticated, token: func(*testing.T, *staffWorld, string) string { return "" }},
		{name: "a token of another shape", want: ErrUnauthenticated,
			token: func(_ *testing.T, _ *staffWorld, cloudToken string) string {
				return deviceTokenPrefix + strings.TrimPrefix(cloudToken, sessionTokenPrefix)
			}},
		{name: "after twelve hours", want: ErrUnauthenticated,
			after: func(_ *testing.T, w *staffWorld, _ Staff) { w.clock.advance(RemoteSessionTTL) }},
		{name: "the device revoked", want: ErrUnauthenticated,
			after: func(t *testing.T, w *staffWorld, _ Staff) { w.revokeCloudDevice(t) }},
		{name: "the owner deactivated", want: ErrUnauthenticated,
			after: func(t *testing.T, w *staffWorld, owner Staff) { w.deactivate(t, owner.ID) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, owner := cloudWorld(t)
			in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
			if err != nil {
				t.Fatal(err)
			}
			token := in.Token
			if tt.token != nil {
				token = tt.token(t, w, in.Token)
			}
			if tt.after != nil {
				tt.after(t, w, owner)
			}
			dev, err := w.auth.CloudDevice(ctx, token)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if tt.want == nil && (dev.ID != in.Session.DeviceID || dev.Name != CloudDeviceName || dev.Kind != KindBackOffice || dev.VenueID != 1) {
				t.Errorf("device = %+v, session on device %d", dev, in.Session.DeviceID)
			}
		})
	}

	t.Run("signed out", func(t *testing.T) {
		w, _ := cloudWorld(t)
		in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
		if err != nil {
			t.Fatal(err)
		}
		if err := w.auth.SignOut(ctx, in.Session); err != nil {
			t.Fatal(err)
		}
		if _, err := w.auth.CloudDevice(ctx, in.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("err = %v, want ErrUnauthenticated", err)
		}
	})
	t.Run("it says when the device was last heard from", func(t *testing.T) {
		w, _ := cloudWorld(t)
		in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.auth.CloudDevice(ctx, in.Token); err != nil {
			t.Fatal(err)
		}
		if n := count(t, w, "SELECT COUNT(*) FROM devices WHERE cloud = 1 AND last_seen_at IS NOT NULL"); n != 1 {
			t.Error("the cloud's device was used and has no last-seen time")
		}
	})
}

func TestCloudSignInRefuses(t *testing.T) {
	tests := []struct {
		name    string
		venue   int64
		vouched func(w *staffWorld) Vouched
		setup   func(t *testing.T, w *staffWorld, owner Staff)
		want    error
	}{
		{name: "a venue that was never paired", venue: 2, want: ErrNoCloudOwner},
		{name: "a nonce too short to be one", venue: 1, want: ErrAssertionSpent,
			vouched: func(w *staffWorld) Vouched { return Vouched{Nonce: "short", KeepUntil: w.clock.now().Add(time.Minute)} }},
		{name: "a nonce too long to be one", venue: 1, want: ErrAssertionSpent,
			vouched: func(w *staffWorld) Vouched {
				return Vouched{Nonce: strings.Repeat("n", maxCloudNonce+1), KeepUntil: w.clock.now().Add(time.Minute)}
			}},
		{name: "an assertion already too old to remember", venue: 1, want: ErrAssertionSpent,
			vouched: func(w *staffWorld) Vouched { return Vouched{Nonce: testNonce, KeepUntil: w.clock.now()} }},
		{name: "an owner who was deactivated", venue: 1, want: ErrNoCloudOwner,
			setup: func(t *testing.T, w *staffWorld, owner Staff) { w.deactivate(t, owner.ID) }},
		{name: "a cloud device that was revoked", venue: 1, want: ErrNoCloudOwner,
			setup: func(t *testing.T, w *staffWorld, _ Staff) { w.revokeCloudDevice(t) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, owner := cloudWorld(t)
			if tt.setup != nil {
				tt.setup(t, w, owner)
			}
			v := w.vouched(testNonce)
			if tt.vouched != nil {
				v = tt.vouched(w)
			}
			if _, err := w.auth.SignInCloud(context.Background(), tt.venue, v); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			// A refusal spends nothing and opens nothing.
			if n := count(t, w, "SELECT COUNT(*) FROM cloud_assertions") + count(t, w, "SELECT COUNT(*) FROM staff_sessions"); n != 0 {
				t.Errorf("a refused sign-in left %d rows", n)
			}
		})
	}
}

// No PIN signs the cloud's owner in, at the venue or from away, and trying
// one neither locks them nor counts against them.
func TestNoPINIsTheCloudOwners(t *testing.T) {
	w, owner := cloudWorld(t)
	ctx := context.Background()

	for _, pin := range []string{"", "0000", "123456", NoPIN} {
		if _, err := w.auth.SignIn(ctx, w.till, owner.PublicID, pin); !errors.Is(err, ErrInvalidPIN) {
			t.Errorf("at the venue, PIN %q: %v, want ErrInvalidPIN", pin, err)
		}
		if _, err := w.auth.SignInRemote(ctx, w.bar, owner.PublicID, pin); !errors.Is(err, ErrInvalidPIN) {
			t.Errorf("from away, PIN %q: %v, want ErrInvalidPIN", pin, err)
		}
		if _, err := w.auth.Authorise(ctx, w.till, owner.PublicID, pin, PermLineVoid); !errors.Is(err, ErrInvalidPIN) {
			t.Errorf("as an override, PIN %q: %v, want ErrInvalidPIN", pin, err)
		}
	}
	for _, o := range outcomes(t, w.db) {
		if o != outcomeInactiveStaff {
			t.Errorf("an attempt on the cloud's owner was recorded as %q, want %q", o, outcomeInactiveStaff)
		}
	}
	row, err := w.db.Queries().StaffByPublicID(ctx, owner.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if row.LockedAt.Valid {
		t.Error("guessing at the cloud's owner hard-locked them: anyone could shut the owner out")
	}
	if _, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce)); err != nil {
		t.Errorf("the cloud's sign-in after the guesses: %v", err)
	}
}

// TestPairingAgainEndsWhatTheLastPairingOpened: a venue paired again may
// be another account's, with another key. Whoever the last one let in is
// out, though the person and the device are the same rows.
func TestPairingAgainEndsWhatTheLastPairingOpened(t *testing.T) {
	w, _ := cloudWorld(t)
	ctx := context.Background()
	in, err := w.auth.SignInCloud(ctx, 1, w.vouched(testNonce))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.auth.CloudDevice(ctx, in.Token); err != nil {
		t.Fatalf("before pairing again: %v", err)
	}
	if _, err := w.auth.PairCloudOwner(ctx, 1, "Somebody Else"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.auth.CloudDevice(ctx, in.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the last account's session after pairing again: %v, want ErrUnauthenticated", err)
	}
	// A start that finds the person there ends nothing.
	again, err := w.auth.SignInCloud(ctx, 1, w.vouched("nonce-after-pairing-again"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.auth.EnsureCloudOwner(ctx, 1, "Somebody Else"); err != nil {
		t.Fatal(err)
	}
	if dev, err := w.auth.CloudDevice(ctx, again.Token); err != nil || !dev.Cloud {
		t.Errorf("after a start: %+v, %v; want the session still good", dev, err)
	}

	// A venue that stops using its pairing ends them all, and nobody else's.
	pin, err := w.auth.SignInRemote(ctx, w.till, w.ravi.PublicID, "1357")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.auth.EndCloudSessions(ctx); err != nil {
		t.Fatalf("EndCloudSessions: %v", err)
	}
	if _, err := w.auth.CloudDevice(ctx, again.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the cloud's session after the pairing was dropped: %v, want ErrUnauthenticated", err)
	}
	if _, err := w.auth.Session(ctx, w.till, pin.Token); err != nil {
		t.Errorf("a PIN's session after the pairing was dropped: %v", err)
	}
}
