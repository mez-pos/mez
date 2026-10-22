package mirror

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/zreport"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// liveVenue is the seeded venue, still open and backing up to a folder, so
// a test can write to it and back it up again as a venue trading does.
type liveVenue struct {
	db     *db.DB
	engine *backup.Engine
	to     backup.Config
}

func newLiveVenue(t *testing.T) *liveVenue {
	t.Helper()
	ctx := context.Background()
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, db.FileName)
	d, err := db.Open(ctx, db.Options{Path: path, LogKeptBy: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	seed(t, d)
	to := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "backups")}
	photos := filepath.Join(dataDir, "photos")
	if err := os.MkdirAll(photos, 0o750); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: path, Config: to, PhotosDir: photos})
	if err != nil {
		t.Fatal(err)
	}
	v := &liveVenue{db: d, engine: engine, to: to}
	v.backUp(t)
	t.Cleanup(func() {
		_ = d.Close(context.Background())
		_ = engine.Close(context.Background())
	})
	return v
}

func (v *liveVenue) backUp(t *testing.T) {
	t.Helper()
	if err := v.engine.BackUpNow(context.Background()); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
}

// closeADay records a closed shift's Z-report, as a close does.
func (v *liveVenue) closeADay(t *testing.T, number int64) {
	t.Helper()
	ctx := context.Background()
	at := start.Add(time.Duration(number) * 24 * time.Hour)
	doc := svcshift.ZDocument{
		Shift:       svcshift.ZShift{PublicID: "sh_X", TradingDay: at.Format("2006-01-02"), OpenedAt: at, ClosedBy: 1, ClosedAt: at.Add(12 * time.Hour)},
		Report:      zreport.Report{Currency: "INR", Sales: zreport.Sales{Bills: 3, Total: money.Amount{Minor: 45000, Currency: "INR"}}},
		Names:       map[int64]string{1: "Priya"},
		GeneratedBy: 1, GeneratedAt: at.Add(12 * time.Hour),
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	err = v.db.WriteTx(ctx, func(q *gen.Queries) error {
		venues, err := q.ListVenues(ctx)
		if err != nil {
			return err
		}
		shiftID, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "sh_X" + at.Format("0102"), VenueID: venues[0].ID, TradingDay: doc.Shift.TradingDay, OpenedBy: 1, OpenedAt: at})
		if err != nil {
			return err
		}
		if _, err := q.CloseShift(ctx, gen.CloseShiftParams{ClosedBy: 1, ClosedAt: sql.NullTime{Time: at.Add(12 * time.Hour), Valid: true}, ID: shiftID}); err != nil {
			return err
		}
		_, err = q.CreateZReport(ctx, gen.CreateZReportParams{VenueID: venues[0].ID, Number: number, ShiftID: shiftID, Body: string(body), GeneratedBy: 1, GeneratedAt: doc.GeneratedAt})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// refreshed refreshes until the replica takes a newer copy, within reason:
// the backup's clock is the folder's, and a write in the same instant as
// the one before is told apart only once the clock has moved.
func refreshed(t *testing.T, r *Replica) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		changed, err := r.Refresh(context.Background())
		if err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if changed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the replica never took the newer copy")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAReplicaFollowsItsBackupAndIsPutAway(t *testing.T) {
	v := newLiveVenue(t)
	copies := t.TempDir()
	ctx := context.Background()

	r, err := OpenReplica(ctx, ReplicaOptions{From: v.to, Venue: "ven_MIRROR01", CopyDir: copies, Log: quiet()})
	if err != nil {
		t.Fatalf("OpenReplica: %v", err)
	}
	if got := r.Venue(); got.PublicID != "ven_MIRROR01" || got.Name != "Cafe" {
		t.Errorf("Venue = %+v", got)
	}
	if r.AsOf().IsZero() {
		t.Error("AsOf is zero")
	}
	if closes, err := r.Closes(ctx, 0, 10); err != nil || len(closes) != 0 {
		t.Fatalf("Closes of a venue that never closed = %v, %v", closes, err)
	}
	if changed, err := r.Refresh(ctx); err != nil || changed {
		t.Fatalf("Refresh with nothing new = %v, %v", changed, err)
	}
	// Photos are not what a replica is for, and are not fetched.
	if _, err := os.Stat(filepath.Join(r.current.dir, "photos")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the copy has photos beside it: %v", err)
	}

	// The venue trades, closes, and backs up; the replica catches up.
	v.closeADay(t, 1)
	v.backUp(t)
	refreshed(t, r)
	closes, err := r.Closes(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(closes) != 1 || closes[0].Number != 1 || closes[0].ClosedBy != "Priya" || closes[0].SalesTotal != 45000 {
		t.Fatalf("Closes after a close = %+v", closes)
	}
	if later, err := r.Closes(ctx, closes[0].ID, 10); err != nil || len(later) != 0 {
		t.Errorf("Closes past the cursor = %v, %v", later, err)
	}

	// Again: the cursor moves, the old copy went.
	v.closeADay(t, 2)
	v.backUp(t)
	refreshed(t, r)
	if later, err := r.Closes(ctx, closes[0].ID, 10); err != nil || len(later) != 1 || later[0].Number != 2 {
		t.Errorf("Closes past the cursor after a second close = %+v, %v", later, err)
	}
	if got := entries(t, copies); len(got) != 1 {
		t.Errorf("copies on disk: %v, want the one being read", got)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := entries(t, copies); len(got) != 0 {
		t.Errorf("copies left behind: %v", got)
	}
	if _, err := r.Refresh(ctx); !errors.Is(err, ErrReplicaClosed) {
		t.Errorf("Refresh after Close = %v", err)
	}
	if _, err := r.Closes(ctx, 0, 1); !errors.Is(err, ErrReplicaClosed) {
		t.Errorf("Closes after Close = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("a second Close = %v", err)
	}
}

func TestOpenReplicaRefusesAnotherVenuesBackup(t *testing.T) {
	v := newLiveVenue(t)
	copies := t.TempDir()
	if _, err := OpenReplica(context.Background(), ReplicaOptions{From: v.to, Venue: "ven_OTHER0001", CopyDir: copies, Log: quiet()}); err == nil {
		t.Fatal("a replica was opened on another venue's backup")
	}
	if got := entries(t, copies); len(got) != 0 {
		t.Errorf("copies left behind by a refused open: %v", got)
	}
}
