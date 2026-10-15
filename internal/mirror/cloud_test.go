package mirror

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/service/cloudsignin"
)

const venuePublicID = "ven_MIRROR01"

// cloudWorld is a world whose venue was paired to mez Cloud before the
// backup was written, and a mirror started with the cloud's key.
type cloudWorld struct {
	*world
	key ed25519.PrivateKey
}

func newCloudWorld(t *testing.T) *cloudWorld {
	t.Helper()
	w := newWorld(t)
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: w.db, Now: w.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staff.PairCloudOwner(context.Background(), 1, "Priya's account"); err != nil {
		t.Fatalf("PairCloudOwner: %v", err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c := &cloudWorld{world: w, key: key}
	c.open(t, pub)
	return c
}

// open is the mirror started again with this key of the cloud's, or none.
func (c *cloudWorld) open(t *testing.T, key ed25519.PublicKey) {
	t.Helper()
	keeper, err := OpenKeeper(KeeperOptions{Path: c.path, Now: c.clock.now})
	if err != nil {
		t.Fatalf("OpenKeeper: %v", err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: c.db, Now: c.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: c.db, Now: c.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	door, err := NewAuth(Options{Keeper: keeper, Staff: staff, Devices: devices, VenueID: 1, CloudKey: key, VenuePublicID: venuePublicID, Now: c.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	c.keeper, c.door = keeper, door
}

// assertion is what mez Cloud would present now, for a venue, signed by key.
func (c *cloudWorld) assertion(t *testing.T, venue string, key ed25519.PrivateKey) string {
	t.Helper()
	a, err := cloud.NewAssertion(venue, c.clock.now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := cloud.SignAssertion(key, a)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (c *cloudWorld) public() ed25519.PublicKey { return c.key.Public().(ed25519.PublicKey) }

func TestTheCloudSignsTheOwnerInOnAMirror(t *testing.T) {
	ctx := context.Background()
	c := newCloudWorld(t)
	before := c.written(t)

	in, err := c.door.Cloud().SignIn(ctx, c.assertion(t, venuePublicID, c.key))
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if !in.Session.Cloud || !in.Session.Remote || in.Session.Staff.Name != "Priya's account" {
		t.Errorf("session = %+v, want the cloud's owner, from away, by the cloud", in.Session)
	}
	if !in.Device.Cloud || in.Device.Name != auth.CloudDeviceName || in.Device.ID <= 0 {
		t.Errorf("device = %+v, want the cloud's device as the copy has it", in.Device)
	}
	if got := in.Session.ExpiresAt.Sub(c.clock.now()); got != SessionTTL {
		t.Errorf("the session lasts %s, want %s", got, SessionTTL)
	}

	// The token is presented as both: it finds the device, then the session on it.
	dev, err := c.door.CloudDevice(ctx, in.Token)
	if err != nil || dev.ID != in.Device.ID || !dev.Cloud {
		t.Fatalf("CloudDevice = %+v, %v", dev, err)
	}
	s, err := c.door.Session(ctx, dev, in.Token)
	if err != nil || !s.Cloud || s.Staff.PublicID != in.Session.Staff.PublicID {
		t.Fatalf("Session = %+v, %v", s, err)
	}
	// The cloud's owner holds what the owner's role grants, read from the copy.
	held, err := c.door.Permissions(ctx, s.Staff.RoleID)
	if err != nil || !held.Has(auth.PermReportsView) {
		t.Errorf("the cloud's owner may not read reports: %v, %v", held, err)
	}

	if after := c.written(t); after != before {
		t.Errorf("the copy was written: %d rows before, %d after", before, after)
	}
	var nonces int
	if err := c.db.Reader().QueryRowContext(ctx, `SELECT COUNT(*) FROM cloud_assertions`).Scan(&nonces); err != nil || nonces != 0 {
		t.Errorf("the copy holds %d spent assertions (%v), want none: they are the keeper's", nonces, err)
	}

	// Signed out, the token opens nothing.
	if err := c.door.SignOut(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := c.door.CloudDevice(ctx, in.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("after signing out CloudDevice = %v, want ErrUnauthenticated", err)
	}
}

func TestWhatACloudSignInOnAMirrorRefuses(t *testing.T) {
	ctx := context.Background()
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		// arrange changes the world and returns what is presented.
		arrange func(t *testing.T, c *cloudWorld) string
		want    error
	}{
		{"a mirror started with no key", func(t *testing.T, c *cloudWorld) string {
			c.open(t, nil)
			return c.assertion(t, venuePublicID, c.key)
		}, cloudsignin.ErrRefused},
		{"signed by somebody else", func(t *testing.T, c *cloudWorld) string {
			return c.assertion(t, venuePublicID, stranger)
		}, cloudsignin.ErrRefused},
		{"for another venue", func(t *testing.T, c *cloudWorld) string {
			return c.assertion(t, "ven_SOMEONE1", c.key)
		}, cloudsignin.ErrRefused},
		{"not an assertion", func(*testing.T, *cloudWorld) string { return "v1.e30.AAAA" }, cloudsignin.ErrRefused},
		{"nothing", func(*testing.T, *cloudWorld) string { return "" }, cloudsignin.ErrRefused},
		{"held past its five minutes and the minute's grace", func(t *testing.T, c *cloudWorld) string {
			a := c.assertion(t, venuePublicID, c.key)
			c.clock.advance(cloud.AssertionLife + cloud.AssertionLeeway + time.Second)
			return a
		}, cloudsignin.ErrClock},
		{"from before this computer's clock", func(t *testing.T, c *cloudWorld) string {
			c.clock.advance(time.Hour)
			a := c.assertion(t, venuePublicID, c.key)
			c.clock.advance(-time.Hour)
			return a
		}, cloudsignin.ErrClock},
		{"a copy whose venue deactivated the cloud's owner", func(t *testing.T, c *cloudWorld) string {
			if _, err := c.db.Writer().ExecContext(ctx, `UPDATE staff SET active = 0 WHERE cloud = 1`); err != nil {
				t.Fatal(err)
			}
			return c.assertion(t, venuePublicID, c.key)
		}, cloudsignin.ErrRefused},
		{"a copy whose venue revoked the cloud's device", func(t *testing.T, c *cloudWorld) string {
			if _, err := c.db.Writer().ExecContext(ctx, `UPDATE devices SET revoked_at = ? WHERE cloud = 1`, start); err != nil {
				t.Fatal(err)
			}
			return c.assertion(t, venuePublicID, c.key)
		}, cloudsignin.ErrRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCloudWorld(t)
			presented := tt.arrange(t, c)
			in, err := c.door.Cloud().SignIn(ctx, presented)
			if !errors.Is(err, tt.want) || in.Token != "" {
				t.Errorf("SignIn = token %q, %v; want %v and no token", in.Token, err, tt.want)
			}
		})
	}
}

func TestACopyNeverPairedHasNobodyForTheCloudToBe(t *testing.T) {
	w := newWorld(t)
	c := &cloudWorld{world: w}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c.key = key
	c.open(t, pub)
	if _, err := c.door.Cloud().SignIn(context.Background(), c.assertion(t, venuePublicID, key)); !errors.Is(err, cloudsignin.ErrRefused) {
		t.Errorf("SignIn = %v, want ErrRefused", err)
	}
}

func TestAnAssertionOpensOneSessionOnAMirror(t *testing.T) {
	ctx := context.Background()
	c := newCloudWorld(t)
	presented := c.assertion(t, venuePublicID, c.key)
	if _, err := c.door.Cloud().SignIn(ctx, presented); err != nil {
		t.Fatalf("the first time: %v", err)
	}
	if _, err := c.door.Cloud().SignIn(ctx, presented); !errors.Is(err, cloudsignin.ErrRefused) {
		t.Errorf("the second time = %v, want ErrRefused", err)
	}

	// The mirror started again inside the assertion's time has forgotten
	// the session and not the assertion.
	c.open(t, c.public())
	if _, err := c.door.Cloud().SignIn(ctx, presented); !errors.Is(err, cloudsignin.ErrRefused) {
		t.Errorf("after a restart = %v, want ErrRefused", err)
	}

	// Another mirror of the same venue, over the same file, has it too.
	other, err := OpenKeeper(KeeperOptions{Path: c.path, Now: c.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	fresh := c.assertion(t, venuePublicID, c.key)
	if _, err := c.door.Cloud().SignIn(ctx, fresh); err != nil {
		t.Fatalf("a new assertion: %v", err)
	}
	a, _, err := cloud.VerifyAssertion(c.public(), fresh, venuePublicID, c.clock.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := other.spend(a.Nonce, a.KeepUntil()); !errors.Is(err, errSpent) {
		t.Errorf("the other mirror took an assertion this one had spent: %v", err)
	}

	// Long dead, an assertion's nonce is forgotten: the file does not grow.
	c.clock.advance(cloud.AssertionLife + cloud.AssertionLeeway + spentMargin + time.Minute)
	if _, err := c.door.Cloud().SignIn(ctx, c.assertion(t, venuePublicID, c.key)); err != nil {
		t.Fatalf("a later assertion: %v", err)
	}
	kept, err := os.ReadDir(c.keeper.spentDir())
	if err != nil || len(kept) != 1 {
		t.Errorf("%d spent assertions remembered (%v), want the live one alone", len(kept), err)
	}
}

func TestAnAssertionThatCannotBeWrittenDownIsNotAccepted(t *testing.T) {
	c := newCloudWorld(t)
	// Where they are remembered is a file, not a directory: nothing can be put there.
	if err := os.WriteFile(c.keeper.spentDir(), []byte("in the way"), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := c.door.Cloud().SignIn(context.Background(), c.assertion(t, venuePublicID, c.key))
	if err == nil || errors.Is(err, cloudsignin.ErrRefused) || in.Token != "" {
		t.Errorf("SignIn = token %q, %v; want a failure of this machine's and no session", in.Token, err)
	}
}

func TestTheCloudsSessionAndAPINsAreNotEachOthers(t *testing.T) {
	ctx := context.Background()
	c := newCloudWorld(t)
	browser, _ := c.browser(t)
	pin, err := c.door.SignIn(ctx, browser, priya, "246813")
	if err != nil {
		t.Fatalf("a PIN on a paired browser: %v", err)
	}
	in, err := c.door.Cloud().SignIn(ctx, c.assertion(t, venuePublicID, c.key))
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	// A PIN's session token is not the cloud's device.
	if _, err := c.door.CloudDevice(ctx, pin.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("a PIN's token as the cloud's device = %v, want ErrUnauthenticated", err)
	}
	// Nor is a browser's own token, or nothing.
	for _, token := range []string{"", "nfm_nothing", c.till} {
		if _, err := c.door.CloudDevice(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("CloudDevice(%q) = %v, want ErrUnauthenticated", token, err)
		}
	}
	// The cloud's session is no session on a paired browser.
	if _, err := c.door.Session(ctx, browser, in.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("the cloud's token on a browser = %v, want ErrNoSession", err)
	}
	// A PIN's session is none on the cloud's device, though the ids were made to match.
	onCloud := in.Device
	if _, err := c.door.Session(ctx, onCloud, pin.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("a PIN's token on the cloud's device = %v, want ErrNoSession", err)
	}
	// And no PIN signs in on the cloud's device, right or wrong.
	if _, err := c.door.SignIn(ctx, onCloud, priya, "246813"); !errors.Is(err, auth.ErrInvalidPIN) {
		t.Errorf("a right PIN on the cloud's device = %v, want ErrInvalidPIN", err)
	}
	// The cloud's owner is still on no sign-in tile.
	people, err := c.door.ActiveStaff(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range people {
		if p.PublicID == in.Session.Staff.PublicID {
			t.Error("the cloud's owner is among those who sign in with a PIN")
		}
	}
}

func TestACloudSessionFollowsTheCopy(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		// what the venue did, as a later copy shows it.
		sql string
	}{
		{"the venue deactivated the cloud's owner", `UPDATE staff SET active = 0 WHERE cloud = 1`},
		{"the venue revoked the cloud's device", `UPDATE devices SET revoked_at = '2026-10-12 09:00:00+00:00' WHERE cloud = 1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCloudWorld(t)
			in, err := c.door.Cloud().SignIn(ctx, c.assertion(t, venuePublicID, c.key))
			if err != nil {
				t.Fatalf("SignIn: %v", err)
			}
			if _, err := c.db.Writer().ExecContext(ctx, tt.sql); err != nil {
				t.Fatal(err)
			}
			if _, err := c.door.CloudDevice(ctx, in.Token); !errors.Is(err, auth.ErrUnauthenticated) {
				t.Errorf("CloudDevice = %v, want ErrUnauthenticated", err)
			}
			if _, err := c.door.Session(ctx, in.Device, in.Token); !errors.Is(err, auth.ErrNoSession) {
				t.Errorf("Session = %v, want ErrNoSession", err)
			}
		})
	}

	t.Run("it runs out after twelve hours", func(t *testing.T) {
		c := newCloudWorld(t)
		in, err := c.door.Cloud().SignIn(ctx, c.assertion(t, venuePublicID, c.key))
		if err != nil {
			t.Fatalf("SignIn: %v", err)
		}
		c.clock.advance(SessionTTL)
		if _, err := c.door.CloudDevice(ctx, in.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("CloudDevice = %v, want ErrUnauthenticated", err)
		}
	})
}

func TestADoorWithAKeyThatIsNotOne(t *testing.T) {
	w := newWorld(t)
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for name, o := range map[string]Options{
		"too short":            {CloudKey: pub[:10], VenuePublicID: venuePublicID},
		"no venue to check by": {CloudKey: pub},
	} {
		o.Keeper, o.Staff, o.Devices, o.VenueID = w.keeper, staff, devices, 1
		if _, err := NewAuth(o); err == nil {
			t.Errorf("%s: NewAuth took it", name)
		}
	}
}

// TestTwoMirrorsAskedAtOnceTakeAnAssertionOnce: the same assertion sent to
// two mirrors of one venue in the same instant. Each has its own memory and
// they share the directory; exactly one of them may open a session.
func TestTwoMirrorsAskedAtOnceTakeAnAssertionOnce(t *testing.T) {
	c := newCloudWorld(t)
	const mirrors = 8
	keepers := make([]*Keeper, mirrors)
	for i := range keepers {
		k, err := OpenKeeper(KeeperOptions{Path: c.path, Now: c.clock.now})
		if err != nil {
			t.Fatal(err)
		}
		keepers[i] = k
	}
	for round := range 20 {
		a, err := cloud.NewAssertion(venuePublicID, c.clock.now(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		var (
			wg    sync.WaitGroup
			mu    sync.Mutex
			took  int
			other []error
		)
		begin := make(chan struct{})
		for _, k := range keepers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-begin
				err := k.spend(a.Nonce, a.KeepUntil())
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					took++
				case !errors.Is(err, errSpent):
					other = append(other, err)
				}
			}()
		}
		close(begin)
		wg.Wait()
		if took != 1 || len(other) != 0 {
			t.Fatalf("round %d: %d of %d mirrors took the assertion, want one; other errors %v", round, took, mirrors, other)
		}
	}
}
