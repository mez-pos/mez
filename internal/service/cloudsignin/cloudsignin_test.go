package cloudsignin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

var at = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// world is a machine with two venues, the first paired: it has an owner's
// role and a cloud owner. key is what its pairing holds; priv is the
// cloud's half.
type world struct {
	db     *db.DB
	staff  *auth.StaffAuth
	svc    *Service
	priv   ed25519.PrivateKey
	key    ed25519.PublicKey
	paired bool
	now    time.Time
}

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
	everything, err := auth.NewPermissionSet(keysOf(auth.Permissions()))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := everything.JSON()
	if err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for _, publicID := range []string{"ven_PAIRED01", "ven_OTHER001"} {
			v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: publicID, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: at, UpdatedAt: at,
			})
			if err != nil {
				return err
			}
			if _, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.ID, Name: "owner", Permissions: raw, CreatedAt: at}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	w := &world{db: d, paired: true, now: at}
	if w.key, w.priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return w.now }
	if w.staff, err = auth.NewStaffAuth(auth.StaffOptions{DB: d, Now: clock}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.staff.PairCloudOwner(ctx, 1, "Priya Menon"); err != nil {
		t.Fatal(err)
	}
	w.svc, err = New(Options{
		DB: d, Staff: w.staff, Now: clock,
		Key: func() (ed25519.PublicKey, bool) { return w.key, w.paired },
	})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func keysOf(perms []auth.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

// signed is an assertion for a venue, issued at a moment, by a key.
func signed(t *testing.T, key ed25519.PrivateKey, venue string, issued time.Time) string {
	t.Helper()
	a, err := cloud.NewAssertion(venue, issued, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := cloud.SignAssertion(key, a)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (w *world) sessions(t *testing.T) int {
	t.Helper()
	var n int
	if err := w.db.Reader().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM staff_sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSignIn(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	in, err := w.svc.SignIn(ctx, signed(t, w.priv, "ven_PAIRED01", at))
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if in.Session.Staff.Name != "Priya Menon" || !in.Session.Cloud || !in.Session.Remote || in.Session.Staff.VenueID != 1 {
		t.Errorf("session = %+v", in.Session)
	}
	if in.Device.Name != auth.CloudDeviceName || in.Device.Kind != auth.KindBackOffice || in.Device.ID != in.Session.DeviceID {
		t.Errorf("device = %+v, session on device %d", in.Device, in.Session.DeviceID)
	}
	// The token opens the session on the device it names.
	if _, err := w.staff.Session(ctx, in.Device, in.Token); err != nil {
		t.Errorf("the session just opened: %v", err)
	}
}

func TestSignInRefuses(t *testing.T) {
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		setup     func(t *testing.T, w *world)
		presented func(t *testing.T, w *world) string
		want      error
	}{
		{name: "a venue that is not paired", want: ErrRefused,
			setup:     func(_ *testing.T, w *world) { w.paired = false },
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_PAIRED01", at) }},
		{name: "another key's", want: ErrRefused,
			presented: func(t *testing.T, _ *world) string { return signed(t, stranger, "ven_PAIRED01", at) }},
		{name: "for a venue that is not here", want: ErrRefused,
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_ELSEWHERE", at) }},
		{name: "not an assertion", want: ErrRefused,
			presented: func(*testing.T, *world) string { return "v1.e30.AAAA" }},
		{name: "nothing", want: ErrRefused,
			presented: func(*testing.T, *world) string { return "" }},
		{name: "issued ten minutes ago", want: ErrClock,
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_PAIRED01", at.Add(-10*time.Minute)) }},
		{name: "issued ten minutes from now", want: ErrClock,
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_PAIRED01", at.Add(10*time.Minute)) }},
		// The second venue on this machine was never paired from: the
		// assertion is rightly signed for it, and it has nobody to be.
		{name: "for a venue here with no cloud owner", want: ErrRefused,
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_OTHER001", at) }},
		{name: "another key's, and out of its time", want: ErrRefused,
			presented: func(t *testing.T, _ *world) string { return signed(t, stranger, "ven_PAIRED01", at.Add(-time.Hour)) }},
		{name: "an owner who was deactivated", want: ErrRefused,
			setup: func(t *testing.T, w *world) {
				if _, err := w.db.Writer().ExecContext(context.Background(), `UPDATE staff SET active = 0 WHERE cloud = 1`); err != nil {
					t.Fatal(err)
				}
			},
			presented: func(t *testing.T, w *world) string { return signed(t, w.priv, "ven_PAIRED01", at) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			if tt.setup != nil {
				tt.setup(t, w)
			}
			_, err := w.svc.SignIn(context.Background(), tt.presented(t, w))
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if n := w.sessions(t); n != 0 {
				t.Errorf("a refused sign-in opened %d sessions", n)
			}
		})
	}
}

func TestAnAssertionSignsInOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	presented := signed(t, w.priv, "ven_PAIRED01", at)
	if _, err := w.svc.SignIn(ctx, presented); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := w.svc.SignIn(ctx, presented); !errors.Is(err, ErrRefused) {
		t.Errorf("replayed: %v, want ErrRefused", err)
	}
	if n := w.sessions(t); n != 1 {
		t.Errorf("%d sessions, want 1", n)
	}
}

// A venue paired again has a new key from that moment: the old cloud's
// assertions stop, and the new one's start, with no restart.
func TestTheKeyIsAskedForEachTime(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	old := w.priv
	var err error
	if w.key, w.priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.SignIn(ctx, signed(t, old, "ven_PAIRED01", at)); !errors.Is(err, ErrRefused) {
		t.Errorf("the key before pairing again: %v, want ErrRefused", err)
	}
	if _, err := w.svc.SignIn(ctx, signed(t, w.priv, "ven_PAIRED01", at)); err != nil {
		t.Errorf("the key after pairing again: %v", err)
	}
}

func TestNewRefusesIncompleteOptions(t *testing.T) {
	w := newWorld(t)
	key := func() (ed25519.PublicKey, bool) { return nil, false }
	for name, o := range map[string]Options{
		"no database": {Staff: w.staff, Key: key},
		"no staff":    {DB: w.db, Key: key},
		"no key":      {DB: w.db, Staff: w.staff},
	} {
		if _, err := New(o); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
}

// learned is one call to Learn.
type learned struct{ held, next ed25519.PublicKey }

// learning rebuilds the world's service with a Learn that keeps the key as
// a pairing would, and records each call. fail makes it fail instead.
func (w *world) learning(t *testing.T, fail error) *[]learned {
	t.Helper()
	var calls []learned
	svc, err := New(Options{
		DB: w.db, Staff: w.staff, Now: func() time.Time { return w.now },
		Key: func() (ed25519.PublicKey, bool) { return w.key, w.paired },
		Learn: func(_ context.Context, held, next ed25519.PublicKey) (bool, error) {
			calls = append(calls, learned{held, next})
			if fail != nil {
				return false, fail
			}
			if !held.Equal(w.key) {
				return false, nil
			}
			w.key = next
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return &calls
}

// rotate gives the cloud a new key and returns the link that hands it on.
func (w *world) rotate(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	next, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := cloud.SignKeyLink(w.priv, next)
	if err != nil {
		t.Fatal(err)
	}
	return priv, link
}

// ADR-345: the cloud rotates, and the venue learns its next key from the
// first sign-in after, and from then on is not led there again.
func TestARotatedKeyIsLearnedOnceTheSignInIsDone(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	calls := w.learning(t, nil)
	held := w.key

	// Before the rotation: nothing to learn.
	if _, err := w.svc.SignIn(ctx, signed(t, w.priv, "ven_PAIRED01", at)); err != nil {
		t.Fatalf("before the rotation: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("Learn was called %d times with nothing new", len(*calls))
	}

	newest, link := w.rotate(t)
	if _, err := w.svc.SignIn(ctx, cloud.PresentAssertion(signed(t, newest, "ven_PAIRED01", at), link)); err != nil {
		t.Fatalf("after the rotation: %v", err)
	}
	if len(*calls) != 1 || !(*calls)[0].held.Equal(held) || !(*calls)[0].next.Equal(newest.Public().(ed25519.PublicKey)) {
		t.Fatalf("Learn calls = %d, want one from the held key to the newest", len(*calls))
	}

	// The venue now holds the newest key: it needs no link, and the key it
	// held before signs nothing here any more.
	if _, err := w.svc.SignIn(ctx, signed(t, newest, "ven_PAIRED01", at)); err != nil {
		t.Errorf("the newest key alone: %v", err)
	}
	if _, err := w.svc.SignIn(ctx, signed(t, w.priv, "ven_PAIRED01", at)); !errors.Is(err, ErrRefused) {
		t.Errorf("the key before the rotation: %v, want ErrRefused", err)
	}
	if len(*calls) != 1 {
		t.Errorf("Learn was called %d times, want once", len(*calls))
	}
}

// A sign-in that is refused learns nothing, even when its links are good.
func TestARefusedSignInLearnsNothing(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	calls := w.learning(t, nil)
	newest, link := w.rotate(t)

	// The other venue has nobody the cloud may be.
	if _, err := w.svc.SignIn(ctx, cloud.PresentAssertion(signed(t, newest, "ven_OTHER001", at), link)); !errors.Is(err, ErrRefused) {
		t.Fatalf("a venue with no cloud owner: %v, want ErrRefused", err)
	}
	// Out of its time.
	if _, err := w.svc.SignIn(ctx, cloud.PresentAssertion(signed(t, newest, "ven_PAIRED01", at.Add(-time.Hour)), link)); !errors.Is(err, ErrClock) {
		t.Fatalf("an old assertion: %v, want ErrClock", err)
	}
	if len(*calls) != 0 {
		t.Errorf("Learn was called %d times by refused sign-ins", len(*calls))
	}

	// Spent: the first sign-in learns the key and a replay of the same
	// presentation, links and all, is refused and calls nothing.
	presented := cloud.PresentAssertion(signed(t, newest, "ven_PAIRED01", at), link)
	if _, err := w.svc.SignIn(ctx, presented); err != nil {
		t.Fatalf("the first sign-in: %v", err)
	}
	w.key = w.priv.Public().(ed25519.PublicKey) // as if it had not been kept
	if _, err := w.svc.SignIn(ctx, presented); !errors.Is(err, ErrRefused) {
		t.Fatalf("a replay: %v, want ErrRefused", err)
	}
	if len(*calls) != 1 {
		t.Errorf("Learn was called %d times, want once, by the first sign-in only", len(*calls))
	}
}

// Keeping the key is not part of signing in: a venue that cannot write it
// lets the owner in, and is led there again next time.
func TestAKeyThatCannotBeKeptDoesNotStopTheSignIn(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	calls := w.learning(t, errors.New("the disk is full"))
	newest, link := w.rotate(t)

	for i := 0; i < 2; i++ {
		if _, err := w.svc.SignIn(ctx, cloud.PresentAssertion(signed(t, newest, "ven_PAIRED01", at), link)); err != nil {
			t.Fatalf("sign-in %d: %v", i+1, err)
		}
	}
	if len(*calls) != 2 {
		t.Errorf("Learn was called %d times, want each time", len(*calls))
	}
}
