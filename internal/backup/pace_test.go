package backup_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
)

// billed counts what a bucket has been asked that is billed as a write: every
// object put and every listing. Reading an object and deleting one are
// billed as reads or not at all.
func billed(f *fakeS3) int { return f.count("PUT object") + f.count("GET bucket") }

// TestAnHourAsksLittleOfABucket lives through an hour at the default pace
// and counts what the bucket was asked, selling the whole hour and then not
// selling at all (ADR-315).
//
// The ceilings are the cost of serving a venue. Two hundred an hour for the
// fourteen hours a cafe trades and twenty-five an hour for the rest of the
// day is under ninety thousand a month, about ₹35 on R2. Litestream as it
// comes asks five hundred and forty of a bucket in an hour with nothing
// sold, and three and a half thousand more while selling.
//
// Each is a ceiling and not a count: a slow machine takes fewer looks in the
// time, never more.
func TestAnHourAsksLittleOfABucket(t *testing.T) {
	// A venue's half minute is a tenth of a second, and its hour twelve.
	fast := backup.LiveAnHour(t, 300)

	for _, tt := range []struct {
		name    string
		selling bool
		ceiling int
	}{
		{"selling all hour", true, 200},
		{"with nothing sold", false, 25},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fake, srv := newFakeS3(t, testBucket, testKey)
			v := bucketVenue(t, srv.URL)
			ctx := context.Background()
			database, err := db.Open(ctx, v.dbOptions())
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			engine, err := backup.Start(ctx, backup.Options{
				DBPath: v.dbPath(), Config: v.cfg, SyncInterval: fast(backup.DefaultSyncInterval),
			})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			t.Cleanup(func() { database.Close(ctx); engine.Close(ctx) })

			// The first whole copy and the first of everything else are
			// the start's, not the hour's.
			sell(t, database, 0, 1)
			time.Sleep(fast(20 * time.Minute))
			if fake.count("PUT object") == 0 {
				t.Fatal("nothing reached the bucket before the hour began")
			}

			before := billed(fake)
			sold := 1
			for end := time.Now().Add(fast(time.Hour)); time.Now().Before(end); {
				if tt.selling {
					sell(t, database, sold, 1)
					sold++
				}
				time.Sleep(fast(2 * time.Second))
			}
			asked := billed(fake) - before
			t.Logf("%d sales, %d billed requests in the hour", sold, asked)
			if asked > tt.ceiling {
				t.Errorf("the bucket was asked %d billed things in an hour, want at most %d", asked, tt.ceiling)
			}
			if tt.selling && asked < 60 {
				t.Errorf("the bucket was asked only %d things in an hour of selling: is anything being sent?", asked)
			}
		})
	}
}

// TestACleanStopSendsEverythingWhateverThePace: at the slowest pace there
// is, with nothing sent since the first look, "back up now" sends what has
// been sold and a clean stop sends the rest. What a slow pace risks is a
// machine that dies, not one that is shut down.
func TestACleanStopSendsEverythingWhateverThePace(t *testing.T) {
	v := newVenue(t)
	ctx := context.Background()
	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, SyncInterval: backup.MaxSyncInterval})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			database.Close(ctx)
			engine.Close(ctx)
		}
	})

	sell(t, database, 1, 30)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if rows, _ := restored(t, v.cfg); rows != 30 {
		t.Errorf("restored after BackUpNow at a ten minute pace: %d rows, want 30", rows)
	}

	sell(t, database, 31, 20)
	closed = true
	if err := database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, _ := restored(t, v.cfg); rows != 50 {
		t.Errorf("restored after a clean stop at a ten minute pace: %d rows, want all 50", rows)
	}
}

// TestTheEngineSaysItsPace: the default when none is asked for, what was
// asked for when one is, and nothing when there is no backup.
func TestTheEngineSaysItsPace(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	engine, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath(), Config: v.cfg, SyncInterval: 45 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got := engine.SyncInterval(); got != 45*time.Second {
		t.Errorf("SyncInterval = %v, want the 45s asked for", got)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatal(err)
	}

	off, err := backup.Start(ctx, backup.Options{DBPath: v.dbPath()})
	if err != nil {
		t.Fatal(err)
	}
	if got := off.SyncInterval(); got != 0 {
		t.Errorf("SyncInterval with no backup = %v, want 0", got)
	}
	var none *backup.Engine
	if got := none.SyncInterval(); got != 0 {
		t.Errorf("SyncInterval of no engine = %v, want 0", got)
	}
}

func TestCheckSyncInterval(t *testing.T) {
	for _, tt := range []struct {
		d  time.Duration
		ok bool
	}{
		{backup.DefaultSyncInterval, true},
		{backup.MinSyncInterval, true},
		{backup.MaxSyncInterval, true},
		{backup.MinSyncInterval - time.Millisecond, false},
		{backup.MaxSyncInterval + time.Millisecond, false},
		{time.Second, false},
		{0, false},
		{-time.Minute, false},
	} {
		if err := backup.CheckSyncInterval(tt.d); (err == nil) != tt.ok {
			t.Errorf("CheckSyncInterval(%v) = %v, want ok %v", tt.d, err, tt.ok)
		}
	}
	if backup.DefaultSyncInterval != 30*time.Second {
		t.Errorf("the default pace is %v: deploy.md and ADR-315 say half a minute", backup.DefaultSyncInterval)
	}
}

// TestADestinationsFloorSlowsTheBackup: the pace asked for is the pace,
// unless the destination has a floor that is slower; and the floor is asked
// of each destination as it is started, so a change of destination while
// running is held to its own (ADR-315, ADR-319).
func TestADestinationsFloorSlowsTheBackup(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	slow := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "someone-elses")}
	fast := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "our-own")}

	manager, err := backup.NewManager(backup.ManagerOptions{
		DataDir: v.dataDir, DBPath: v.dbPath(), SyncInterval: 5 * time.Second,
		SyncFloor: func(dest backup.Config) time.Duration {
			if dest.Folder == slow.Folder {
				return 30 * time.Second
			}
			return 0
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	manager.SwapsLive(true)
	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { database.Close(ctx); manager.Close(ctx) })

	if got := manager.SyncInterval(); got != 0 {
		t.Errorf("with no destination: %v, want 0", got)
	}
	if _, err := manager.Set(ctx, slow); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := manager.SyncInterval(); got != 30*time.Second {
		t.Errorf("to a destination with a floor of 30s, asked for 5s: runs at %v", got)
	}
	if _, err := manager.Set(ctx, fast); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := manager.SyncInterval(); got != 5*time.Second {
		t.Errorf("to a destination with no floor, asked for 5s: runs at %v", got)
	}
}

// TestAFloorNeverSpeedsABackupUp: asked for a minute, with a floor of half
// of one, it is a minute.
func TestAFloorNeverSpeedsABackupUp(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	manager, err := backup.NewManager(backup.ManagerOptions{
		DataDir: v.dataDir, DBPath: v.dbPath(), SyncInterval: time.Minute,
		SyncFloor: func(backup.Config) time.Duration { return 30 * time.Second },
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.SwapsLive(true)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close(ctx) })
	if _, err := manager.Set(ctx, v.cfg); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := manager.SyncInterval(); got != time.Minute {
		t.Errorf("asked for a minute with a floor of 30s: runs at %v", got)
	}
}
