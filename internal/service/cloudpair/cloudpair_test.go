package cloudpair

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var at = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func pairing(t *testing.T) cloud.Pairing {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return cloud.Pairing{
		PairedAt: at, CloudURL: cloud.DefaultURL,
		Venue: cloud.Venue{Slug: "corner-cafe", Hostname: "corner-cafe.mezpos.com"},
		Relay: cloud.Relay{URL: "https://relay.mezpos.com", Key: "mzr_corner-cafe.0123456789ab.remote.c2lnbmF0dXJl"},
		Backup: backup.S3{
			Bucket: "mez-venues", Path: "v/corner-cafe", Endpoint: "https://acct.r2.cloudflarestorage.com",
			AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "not-a-real-bucket-secret",
		},
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		Owner:     cloud.Owner{Name: "Priya Menon"},
	}
}

// fakeCloud accepts one code, once, and remembers what it was asked.
type fakeCloud struct {
	give  cloud.Pairing
	err   error
	asked []cloud.About
	codes []string
}

func (f *fakeCloud) Pair(_ context.Context, code string, about cloud.About) (cloud.Pairing, error) {
	f.codes = append(f.codes, code)
	f.asked = append(f.asked, about)
	if f.err != nil {
		return cloud.Pairing{}, f.err
	}
	return f.give, nil
}

// fakeBackups keeps what it is set to.
type fakeBackups struct {
	cfg     backup.Config
	sets    []backup.Config
	restart bool
	err     error
}

func (f *fakeBackups) Config() backup.Config { return f.cfg }
func (f *fakeBackups) Set(_ context.Context, cfg backup.Config) (bool, error) {
	f.sets = append(f.sets, cfg)
	if f.err != nil {
		return f.restart, f.err
	}
	f.cfg = cfg
	return f.restart, nil
}

type world struct {
	dir     string
	cloud   *fakeCloud
	backups *fakeBackups
	applied []cloud.Pairing
	opts    Options
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
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_TEST0001", Name: "Corner Cafe", LegalName: "Corner Cafe Pvt Ltd", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 50, CreatedAt: at, UpdatedAt: at,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	w := &world{dir: t.TempDir(), cloud: &fakeCloud{give: pairing(t)}, backups: &fakeBackups{}}
	w.opts = Options{
		DB: d, DataDir: w.dir, Version: "v0.1.0", Cloud: w.cloud, Backups: w.backups,
		Apply: func(_ context.Context, p cloud.Pairing) (bool, error) {
			w.applied = append(w.applied, p)
			return false, nil
		},
	}
	return w
}

func (w *world) service(t *testing.T) *Service {
	t.Helper()
	s, err := New(w.opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func (w *world) kept(t *testing.T) *cloud.Pairing {
	t.Helper()
	p, err := cloud.Load(w.dir)
	if err != nil {
		t.Fatalf("reading what was kept: %v", err)
	}
	return p
}

// TestPair: the code goes to the cloud cleaned, with who the venue is; the
// pairing is kept, put to use, and the venue's backup pointed at the bucket
// it gave. Nothing needs a restart.
func TestPair(t *testing.T) {
	w := newWorld(t)
	got, err := w.service(t).Pair(context.Background(), 1, "abcd-2345")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if got.Owner != "Priya Menon" || got.Hostname != "corner-cafe.mezpos.com" || !got.PairedAt.Equal(at) ||
		got.Backup != BackupSet || got.RestartNeeded {
		t.Errorf("result %+v", got)
	}
	if len(w.cloud.codes) != 1 || w.cloud.codes[0] != "ABCD2345" ||
		w.cloud.asked[0] != (cloud.About{VenueID: "ven_TEST0001", Name: "Corner Cafe", Version: "v0.1.0"}) {
		t.Errorf("the cloud was asked %v about %+v", w.cloud.codes, w.cloud.asked)
	}
	if kept := w.kept(t); kept == nil || kept.Relay != w.cloud.give.Relay {
		t.Errorf("kept %+v", kept)
	}
	if len(w.applied) != 1 || w.applied[0].Venue.Slug != "corner-cafe" {
		t.Errorf("put to use %d times", len(w.applied))
	}
	want := backup.Config{Kind: backup.KindS3, S3: w.cloud.give.Backup}
	if len(w.backups.sets) != 1 || w.backups.sets[0] != want {
		t.Errorf("the backup was set to %+v", w.backups.sets)
	}
}

// TestNothingIsSentThatCouldBeRefusedFirst: a code is good once, so each
// thing that can refuse does so before the cloud hears of it.
func TestNothingIsSentThatCouldBeRefusedFirst(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name  string
		tune  func(*world)
		venue int64
		code  string
		want  error
	}{
		{"a relay in the settings file", func(w *world) { w.opts.RelayInSettings = true }, 1, "ABCD2345", ErrRelayInSettings},
		{"a code that could not be one", nil, 1, "ABCD-234", cloud.ErrCodeShape},
		{"a venue that is not there", nil, 99, "ABCD2345", ErrNoVenue},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			if tt.tune != nil {
				tt.tune(w)
			}
			if _, err := w.service(t).Pair(ctx, tt.venue, tt.code); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if len(w.cloud.codes) != 0 {
				t.Errorf("the code was sent: %v", w.cloud.codes)
			}
			if w.kept(t) != nil || len(w.applied) != 0 || len(w.backups.sets) != 0 {
				t.Error("something was changed by a pairing that was refused")
			}
		})
	}
}

// TestACodeTheCloudRefusesChangesNothing: its error is passed on as it is,
// for the API to word, and the venue is as it was.
func TestACodeTheCloudRefusesChangesNothing(t *testing.T) {
	for _, refusal := range []error{cloud.ErrCodeRefused, cloud.ErrBusy, cloud.ErrUnreachable, cloud.ErrInvalid} {
		w := newWorld(t)
		w.cloud.err = refusal
		if _, err := w.service(t).Pair(context.Background(), 1, "ABCD2345"); !errors.Is(err, refusal) {
			t.Errorf("err = %v, want %v", err, refusal)
		}
		if w.kept(t) != nil || len(w.applied) != 0 || len(w.backups.sets) != 0 {
			t.Errorf("%v: something was changed", refusal)
		}
	}
}

// TestOnceTheCodeIsSpentThePairingStands: whatever goes wrong after the
// cloud has accepted the code, the pairing is kept and the owner is told
// what is left to do, not to start again.
func TestOnceTheCodeIsSpentThePairingStands(t *testing.T) {
	ctx := context.Background()

	t.Run("it cannot be put to use now", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Apply = func(context.Context, cloud.Pairing) (bool, error) {
			return false, errors.New("the tunnel would not start")
		}
		got, err := w.service(t).Pair(ctx, 1, "ABCD2345")
		if err != nil || !got.RestartNeeded || got.Backup != BackupSet {
			t.Errorf("result %+v, %v; want paired, a restart needed, and the backup set", got, err)
		}
		if w.kept(t) == nil {
			t.Error("the pairing was not kept")
		}
	})
	t.Run("putting it to use needs a restart", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Apply = func(context.Context, cloud.Pairing) (bool, error) { return true, nil }
		if got, err := w.service(t).Pair(ctx, 1, "ABCD2345"); err != nil || !got.RestartNeeded {
			t.Errorf("result %+v, %v", got, err)
		}
	})
	t.Run("a server that only reads its pairing at start", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Apply = nil
		if got, err := w.service(t).Pair(ctx, 1, "ABCD2345"); err != nil || !got.RestartNeeded || w.kept(t) == nil {
			t.Errorf("result %+v, %v", got, err)
		}
	})
	t.Run("the bucket cannot be written to", func(t *testing.T) {
		w := newWorld(t)
		w.backups.err = errors.New("the bucket did not answer")
		got, err := w.service(t).Pair(ctx, 1, "ABCD2345")
		if err != nil || got.Backup != BackupFailed || got.RestartNeeded {
			t.Errorf("result %+v, %v; want paired, with the backup failed", got, err)
		}
		if w.kept(t) == nil || len(w.applied) != 1 {
			t.Error("a backup that failed undid the pairing")
		}
	})
	t.Run("the backup needs a restart", func(t *testing.T) {
		w := newWorld(t)
		w.backups.restart = true
		if got, err := w.service(t).Pair(ctx, 1, "ABCD2345"); err != nil || got.Backup != BackupSet || !got.RestartNeeded {
			t.Errorf("result %+v, %v", got, err)
		}
	})
	t.Run("the caller goes away as the cloud answers", func(t *testing.T) {
		w := newWorld(t)
		gone, cancel := context.WithCancel(ctx)
		w.opts.Apply = func(ctx context.Context, p cloud.Pairing) (bool, error) {
			cancel()
			return false, ctx.Err()
		}
		got, err := w.service(t).Pair(gone, 1, "ABCD2345")
		if err != nil || got.RestartNeeded || got.Backup != BackupSet {
			t.Errorf("result %+v, %v; a browser closed mid-pairing must not stop it", got, err)
		}
	})
	t.Run("it cannot be written down", func(t *testing.T) {
		w := newWorld(t)
		// A data directory that is not one.
		w.opts.DataDir = filepath.Join(w.dir, "not-a-directory")
		if err := os.WriteFile(w.opts.DataDir, []byte("a file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := w.service(t).Pair(ctx, 1, "ABCD2345"); !errors.Is(err, ErrNotKept) {
			t.Errorf("err = %v, want ErrNotKept", err)
		}
		if len(w.applied) != 0 || len(w.backups.sets) != 0 {
			t.Error("a pairing that was not kept was put to use")
		}
	})
}

// TestTheVenuesOwnBackupIsLeftAlone: a folder or a bucket the owner chose
// stays. A venue already on the pairing's bucket, which is one paired
// again, gets the new key. A server with no backup is said to have none.
func TestTheVenuesOwnBackupIsLeftAlone(t *testing.T) {
	ctx := context.Background()
	for name, own := range map[string]backup.Config{
		"a folder":                               {Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "usb")},
		"its own bucket":                         {Kind: backup.KindS3, S3: backup.S3{Bucket: "my-own", Region: "ap-south-1", AccessKeyID: "A", SecretAccessKey: "s"}},
		"another's folder in the cloud's bucket": {Kind: backup.KindS3, S3: backup.S3{Bucket: "mez-venues", Endpoint: "https://acct.r2.cloudflarestorage.com", Path: "v/other", AccessKeyID: "A", SecretAccessKey: "s"}},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.backups.cfg = own
			got, err := w.service(t).Pair(ctx, 1, "ABCD2345")
			if err != nil || got.Backup != BackupKept || len(w.backups.sets) != 0 || w.backups.cfg != own {
				t.Errorf("result %+v, %v; sets %+v", got, err, w.backups.sets)
			}
		})
	}

	t.Run("paired again", func(t *testing.T) {
		w := newWorld(t)
		old := w.cloud.give.Backup
		old.AccessKeyID, old.SecretAccessKey = "REVOKED", "the-old-secret"
		w.backups.cfg = backup.Config{Kind: backup.KindS3, S3: old}
		got, err := w.service(t).Pair(ctx, 1, "ABCD2345")
		if err != nil || got.Backup != BackupSet || len(w.backups.sets) != 1 || w.backups.cfg.S3.SecretAccessKey != w.cloud.give.Backup.SecretAccessKey {
			t.Errorf("result %+v, %v; the backup holds %+v", got, err, w.backups.cfg.S3.AccessKeyID)
		}
	})

	t.Run("a server with no backup", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Backups = nil
		if got, err := w.service(t).Pair(ctx, 1, "ABCD2345"); err != nil || got.Backup != BackupNone || got.RestartNeeded {
			t.Errorf("result %+v, %v", got, err)
		}
	})
}

func TestNewRefusesIncompleteOptions(t *testing.T) {
	w := newWorld(t)
	for name, spoil := range map[string]func(*Options){
		"no database":       func(o *Options) { o.DB = nil },
		"no data directory": func(o *Options) { o.DataDir = "" },
		"no cloud":          func(o *Options) { o.Cloud = nil },
	} {
		o := w.opts
		spoil(&o)
		if _, err := New(o); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
}

// TestPairingMakesWhoTheCloudSignsInAs: the person is asked for once the
// pairing is kept, for the venue that was paired and by the account's name;
// never for a code that was refused; and failing to make them unpairs
// nothing (ADR-323).
func TestPairingMakesWhoTheCloudSignsInAs(t *testing.T) {
	ctx := context.Background()
	type asked struct {
		venue int64
		name  string
		kept  bool
	}

	t.Run("made, after the pairing is kept", func(t *testing.T) {
		w := newWorld(t)
		var got []asked
		w.opts.Owner = func(_ context.Context, venueID int64, name string) error {
			got = append(got, asked{venueID, name, w.kept(t) != nil})
			return nil
		}
		if _, err := w.service(t).Pair(ctx, 1, "ABCD2345"); err != nil {
			t.Fatalf("Pair: %v", err)
		}
		if len(got) != 1 || got[0] != (asked{1, "Priya Menon", true}) {
			t.Errorf("asked for %+v, want once, for venue 1, as Priya Menon, with the pairing kept", got)
		}
	})
	t.Run("not for a code the cloud refuses", func(t *testing.T) {
		w := newWorld(t)
		w.cloud.err = cloud.ErrCodeRefused
		calls := 0
		w.opts.Owner = func(context.Context, int64, string) error { calls++; return nil }
		if _, err := w.service(t).Pair(ctx, 1, "ABCD2345"); !errors.Is(err, cloud.ErrCodeRefused) {
			t.Fatalf("err = %v", err)
		}
		if calls != 0 {
			t.Error("someone was made for a pairing that did not happen")
		}
	})
	t.Run("failing to make them unpairs nothing", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Owner = func(context.Context, int64, string) error { return errors.New("no role grants everything") }
		got, err := w.service(t).Pair(ctx, 1, "ABCD2345")
		if err != nil || got.RestartNeeded || got.Backup != BackupSet {
			t.Errorf("result %+v, %v; want paired as if nothing had gone wrong", got, err)
		}
		if w.kept(t) == nil || len(w.applied) != 1 {
			t.Error("the pairing was not kept and put to use")
		}
	})
}

// ADR-345: the cloud's next key replaces the one the kept pairing holds,
// and only that one. A pairing made since, or none, is left as it is.
func TestLearnKey(t *testing.T) {
	newKey := func(t *testing.T) ed25519.PublicKey {
		t.Helper()
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return pub
	}

	t.Run("the pairing holds the key it was led from", func(t *testing.T) {
		w := newWorld(t)
		was := w.cloud.give
		if err := cloud.Save(w.dir, was); err != nil {
			t.Fatal(err)
		}
		held, _ := was.Key()
		next := newKey(t)
		kept, err := w.service(t).LearnKey(held, next)
		if err != nil || !kept {
			t.Fatalf("LearnKey = %v, %v", kept, err)
		}
		now := w.kept(t)
		if key, _ := now.Key(); !key.Equal(next) {
			t.Error("the file does not hold the new key")
		}
		if want := was.WithKey(next); now.Relay != want.Relay || now.Backup != want.Backup || now.Venue != want.Venue ||
			now.Owner != want.Owner || !now.PairedAt.Equal(want.PairedAt) || now.CloudURL != want.CloudURL {
			t.Errorf("more than the key changed: %+v", now)
		}
	})

	t.Run("the venue was paired again since", func(t *testing.T) {
		w := newWorld(t)
		held, _ := w.cloud.give.Key()
		again := pairing(t)
		if err := cloud.Save(w.dir, again); err != nil {
			t.Fatal(err)
		}
		kept, err := w.service(t).LearnKey(held, newKey(t))
		if err != nil || kept {
			t.Fatalf("LearnKey = %v, %v; want not kept", kept, err)
		}
		if w.kept(t).PublicKey != again.PublicKey {
			t.Error("the pairing made since lost its key")
		}
	})

	t.Run("the venue is not paired", func(t *testing.T) {
		w := newWorld(t)
		held, _ := w.cloud.give.Key()
		kept, err := w.service(t).LearnKey(held, newKey(t))
		if err != nil || kept {
			t.Fatalf("LearnKey = %v, %v; want not kept", kept, err)
		}
		if p := w.kept(t); p != nil {
			t.Errorf("a pairing was written where there was none: %v", p)
		}
	})
}

// A sign-in is waiting on LearnKey, so it does not wait for a pairing:
// it keeps nothing and says so, and the key is learned next time.
func TestLearnKeyDoesNotWaitForAPairing(t *testing.T) {
	w := newWorld(t)
	if err := cloud.Save(w.dir, w.cloud.give); err != nil {
		t.Fatal(err)
	}
	held, _ := w.cloud.give.Key()
	next, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s := w.service(t)
	s.mu.Lock() // a pairing under way
	kept, err := s.LearnKey(held, next)
	s.mu.Unlock()
	if err != nil || kept {
		t.Fatalf("LearnKey during a pairing = %v, %v; want not kept", kept, err)
	}
	if key, _ := w.kept(t).Key(); !key.Equal(held) {
		t.Error("the file changed during a pairing")
	}
}

// A cloud.json that cannot be read is an error, and is left as it is.
func TestLearnKeyLeavesAFileItCannotReadAlone(t *testing.T) {
	w := newWorld(t)
	path := filepath.Join(w.dir, cloud.File)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	held, _ := w.cloud.give.Key()
	next, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if kept, err := w.service(t).LearnKey(held, next); err == nil || kept {
		t.Fatalf("LearnKey = %v, %v; want an error", kept, err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{not json" {
		t.Errorf("the file was rewritten: %q", raw)
	}
}
