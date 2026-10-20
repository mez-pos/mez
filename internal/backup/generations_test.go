package backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
)

// day is a session of the venue on a given day: the backup started with
// the clock saying so, n rows sold from from, backed up, and stopped. With
// again, the session cannot carry on from the one before, as after a
// destination has lost what it had, and begins a new generation.
func day(t *testing.T, v venue, at time.Time, again bool, from, n int) {
	t.Helper()
	ctx := context.Background()
	if again {
		// The file that says which generation to carry on in.
		if err := os.Remove(filepath.Join(v.dataDir, ".mez.db-litestream-for")); err != nil {
			t.Fatalf("making the next session begin again: %v", err)
		}
	}
	database, err := db.Open(ctx, v.dbOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{
		DBPath: v.dbPath(), Config: v.cfg, Clock: &fixedClock{at: at}, ProbeEvery: 20 * time.Millisecond, Log: testLog(t),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	sell(t, database, from, n)
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	// A few looks at the destination, which is when old generations go.
	time.Sleep(150 * time.Millisecond)
	if err := database.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func names(found []backup.Generation) []string {
	var out []string
	for _, g := range found {
		out = append(out, g.Name)
	}
	return out
}

// TestGenerationsAreListedAndRestoredByName: what `mez restore` shows an
// owner choosing what to bring back, and that the older of two can still
// be had.
func TestGenerationsAreListedAndRestoredByName(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	monday := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

	day(t, v, monday, false, 1, 30)
	day(t, v, monday.Add(24*time.Hour), true, 31, 20)

	found, err := backup.Generations(ctx, v.cfg)
	if err != nil {
		t.Fatalf("Generations: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("found %v, want two generations", names(found))
	}
	newest, oldest := found[0], found[1]
	if !newest.Began.Equal(monday.Add(24*time.Hour)) || !oldest.Began.Equal(monday) {
		t.Errorf("began %s and %s, want Tuesday first and then Monday", newest.Began, oldest.Began)
	}
	for _, g := range found {
		if g.Empty() || g.First.IsZero() || g.Last.Before(g.First) || g.Last.Location() != time.UTC {
			t.Errorf("generation %+v, want it not empty, with when it was first and last written, in UTC", g)
		}
	}

	// The older one, by name: Monday's venue.
	out := filepath.Join(t.TempDir(), db.FileName)
	if err := backup.RestoreGeneration(ctx, v.cfg, oldest.Name, out); err != nil {
		t.Fatalf("RestoreGeneration: %v", err)
	}
	if rows := rowsIn(t, out); rows != 30 {
		t.Errorf("Monday's generation restores %d rows, want the 30 sold by then", rows)
	}
	// And with no name, the newest.
	if rows, _ := restored(t, v.cfg); rows != 50 {
		t.Errorf("the newest generation restores %d rows, want all 50", rows)
	}

	t.Run("refusals", func(t *testing.T) {
		to := filepath.Join(t.TempDir(), db.FileName)
		if err := backup.RestoreGeneration(ctx, v.cfg, "20200101T000000Z-abcdef", to); !errors.Is(err, backup.ErrNothingToRestore) {
			t.Errorf("a generation that is not there: %v, want ErrNothingToRestore", err)
		}
		if err := backup.RestoreGeneration(ctx, v.cfg, "../../etc", to); !errors.Is(err, backup.ErrNoSuchGeneration) {
			t.Errorf("a name that is not a generation's: %v, want ErrNoSuchGeneration", err)
		}
		if err := backup.RestoreGeneration(ctx, v.cfg, newest.Name, out); !errors.Is(err, backup.ErrWouldOverwrite) {
			t.Errorf("over a file: %v, want ErrWouldOverwrite", err)
		}
		gone := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "unplugged")}
		if _, err := backup.Generations(ctx, gone); !errors.Is(err, backup.ErrDestinationMissing) {
			t.Errorf("a folder that is not there: %v, want ErrDestinationMissing", err)
		}
		if _, err := backup.Generations(ctx, backup.Config{}); !errors.Is(err, backup.ErrOff) {
			t.Errorf("no destination: %v, want ErrOff", err)
		}
	})

	t.Run("an empty destination has none", func(t *testing.T) {
		found, err := backup.Generations(ctx, backup.Config{Kind: backup.KindFolder, Folder: t.TempDir()})
		if err != nil || len(found) != 0 {
			t.Errorf("Generations = %v, %v; want none and no error", names(found), err)
		}
	})
}

// TestAWrongClockOnlyPutsTidyingOff: a venue PC whose battery has died
// starts in the wrong year. The generation begun that morning sorts
// before every other by its name, and must not be taken for the oldest.
func TestAWrongClockOnlyPutsTidyingOff(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	monday := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	longAgo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	day(t, v, monday, false, 1, 10)                   // the first
	day(t, v, longAgo, true, 11, 10)                  // begun with the clock in 2020
	day(t, v, monday.Add(24*time.Hour), true, 21, 10) // the clock put right: a third

	found, err := backup.Generations(ctx, v.cfg)
	if err != nil || len(found) != 3 {
		t.Fatalf("Generations = %v, %v; want three", names(found), err)
	}

	// Two days on. By their names the 2020 one is the oldest and was
	// replaced six years ago; in truth it was replaced yesterday, and the
	// first one the day before that. Nothing is a week old.
	day(t, v, monday.Add(3*24*time.Hour), false, 31, 5)
	if got, _ := backup.Generations(ctx, v.cfg); len(got) != 3 {
		t.Errorf("two days on the destination holds %v, want all three: none was replaced a week ago", names(got))
	}

	// Nine days after the third arrived, the two before it go.
	day(t, v, monday.Add(10*24*time.Hour), false, 36, 5)
	got, err := backup.Generations(ctx, v.cfg)
	if err != nil || len(got) != 1 {
		t.Fatalf("ten days on the destination holds %v, %v; want only the current one", names(got), err)
	}
	if rows, _ := restored(t, v.cfg); rows != 40 {
		t.Errorf("restored %d rows, want all 40", rows)
	}
}

func rowsIn(t *testing.T, path string) int {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	var rows int
	if err := database.Reader().QueryRowContext(ctx, "SELECT count(*) FROM backup_probe").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestOldGenerationsAreTidiedAway: a generation is kept for a week after
// it was replaced, which is as far back as a restore is promised to
// reach, and then removed. Only ever by the data directory that began it.
func TestOldGenerationsAreTidiedAway(t *testing.T) {
	ctx := context.Background()
	v := newVenue(t)
	monday := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	days := func(n int) time.Time { return monday.Add(time.Duration(n) * 24 * time.Hour) }

	// Somebody else's backup, already in the folder: last year's venue,
	// which the owner means to restore one day.
	theirs := filepath.Join(v.cfg.Folder, "db", "20250101T000000Z-0a0b0c")
	if err := os.MkdirAll(filepath.Join(theirs, "0009"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(theirs, "0009", "keep-me.ltx"), []byte("last year's venue"), 0o600); err != nil {
		t.Fatal(err)
	}

	has := func() []string {
		t.Helper()
		found, err := backup.Generations(ctx, v.cfg)
		if err != nil {
			t.Fatalf("Generations: %v", err)
		}
		return names(found)
	}

	day(t, v, days(0), false, 1, 10) // Monday: the first generation
	day(t, v, days(1), true, 11, 10) // Tuesday: it is replaced
	if got := has(); len(got) != 3 {
		t.Fatalf("after two generations the destination holds %v, want both and theirs", got)
	}

	// Six days after it was replaced, it is still there: a restore can
	// still go back to Monday.
	day(t, v, days(7), false, 21, 10)
	if got := has(); len(got) != 3 {
		t.Errorf("six days after it was replaced the destination holds %v, want all three still", got)
	}

	// A week and a day after: gone. The current one stays, and so does
	// the one this data directory did not begin.
	day(t, v, days(9), false, 31, 10)
	got := has()
	if len(got) != 2 {
		t.Fatalf("eight days after it was replaced the destination holds %v, want the current one and theirs", got)
	}
	if got[1] != "20250101T000000Z-0a0b0c" {
		t.Errorf("the generation that was not ours is gone: %v", got)
	}
	if kept, err := os.ReadFile(filepath.Join(theirs, "0009", "keep-me.ltx")); err != nil || string(kept) != "last year's venue" {
		t.Errorf("somebody else's backup was touched: %q, %v", kept, err)
	}
	if rows, _ := restored(t, v.cfg); rows != 40 {
		t.Errorf("restored %d rows after tidying, want all 40", rows)
	}

	// And years on, with nothing replaced since, nothing more goes.
	day(t, v, days(800), false, 41, 5)
	if got := has(); len(got) != 2 {
		t.Errorf("long after, the destination holds %v, want the same two", got)
	}
}
