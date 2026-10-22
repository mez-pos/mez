package health

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

var now = time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)

func newDB(t *testing.T) *db.DB {
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
	return d
}

func addVenue(t *testing.T, d *db.DB, publicID string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: publicID, Name: "Cafe", LegalName: "Cafe Pvt Ltd", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 50, CreatedAt: now, UpdatedAt: now,
		})
		id = v.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func write(t *testing.T, d *db.DB, fn func(q *gen.Queries) error) {
	t.Helper()
	if err := d.WriteTx(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
}

func addPrinter(t *testing.T, d *db.DB, venueID int64, name string) int64 {
	t.Helper()
	var id int64
	write(t, d, func(q *gen.Queries) error {
		var err error
		id, err = q.CreatePrinter(context.Background(), gen.CreatePrinterParams{
			VenueID: venueID, Name: name, Transport: "tcp", Address: "192.168.1.50", PaperMm: 80, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id
}

// finishJob queues a job on the printer and finishes it as state at when:
// the path a real job takes, so done_at and updated_at are what the queue
// writes.
func finishJob(t *testing.T, d *db.DB, venueID, printerID int64, state, lastError string, when time.Time) {
	t.Helper()
	ctx := context.Background()
	write(t, d, func(q *gen.Queries) error {
		id, err := q.CreatePrintJob(ctx, gen.CreatePrintJobParams{
			VenueID: venueID, PrinterID: sql.NullInt64{Int64: printerID, Valid: true}, Kind: "receipt", Label: "Receipt",
			Payload: []byte{0x1b}, State: "printing", NextAttemptAt: when, CreatedAt: when, UpdatedAt: when,
		})
		if err != nil {
			return err
		}
		params := gen.FinishPrintJobParams{State: state, NextAttemptAt: when, LastError: lastError, Now: when, ID: id}
		if state == "done" {
			params.DoneAt = sql.NullTime{Time: when, Valid: true}
		}
		_, err = q.FinishPrintJob(ctx, params)
		return err
	})
}

type fakeBackups struct{ st backup.Status }

func (f fakeBackups) Status() backup.Status { return f.st }

func newService(t *testing.T, d *db.DB, b Backups) *Service {
	t.Helper()
	s, err := New(Options{DB: d, Backups: b, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("New accepted no database")
	}
}

func TestNoVenueOrSeveralIsNoSummary(t *testing.T) {
	d := newDB(t)
	s := newService(t, d, nil)
	if _, err := s.Summary(context.Background()); !errors.Is(err, ErrNoVenue) {
		t.Errorf("with no venue: %v, want ErrNoVenue", err)
	}
	addVenue(t, d, "ven_ONE00001")
	addVenue(t, d, "ven_TWO00002")
	if _, err := s.Summary(context.Background()); !errors.Is(err, ErrNoVenue) {
		t.Errorf("with two venues: %v, want ErrNoVenue", err)
	}
}

func TestAVenueWithNothingToSay(t *testing.T) {
	d := newDB(t)
	addVenue(t, d, "ven_TEST0001")
	s := newService(t, d, nil)
	h, err := s.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !h.At.Equal(now) || h.Timezone != "Asia/Kolkata" {
		t.Errorf("At %v, Timezone %q", h.At, h.Timezone)
	}
	if h.Printers == nil || len(h.Printers) != 0 {
		t.Errorf("Printers = %#v, want an empty list", h.Printers)
	}
	if h.Backup.State != "off" || h.Backup.LastSyncedAt != nil {
		t.Errorf("Backup = %+v, want off", h.Backup)
	}
	if h.Shift != nil || h.Hours != nil {
		t.Errorf("Shift = %+v, Hours = %v, want neither", h.Shift, h.Hours)
	}
}

func TestPrintersSayTheirLastSuccessAndFailure(t *testing.T) {
	d := newDB(t)
	venue := addVenue(t, d, "ven_TEST0001")
	counter := addPrinter(t, d, venue, "Counter")
	kitchen := addPrinter(t, d, venue, "Kitchen")
	bar := addPrinter(t, d, venue, "Bar")
	addPrinter(t, d, venue, "Quiet")

	t1, t2, t3 := now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour)
	// Counter: printed, then failed: it is down.
	finishJob(t, d, venue, counter, "done", "", t1)
	finishJob(t, d, venue, counter, "failed", "connection refused", t2)
	finishJob(t, d, venue, counter, "dead", "no answer from 192.168.1.50", t3)
	// Kitchen: failed, then printed: the failure is over.
	finishJob(t, d, venue, kitchen, "failed", "paper out", t1)
	finishJob(t, d, venue, kitchen, "done", "", t2)
	// Bar: only ever printed.
	finishJob(t, d, venue, bar, "done", "", t3)

	s := newService(t, d, nil)
	h, err := s.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Printers) != 4 {
		t.Fatalf("got %d printers, want 4: %+v", len(h.Printers), h.Printers)
	}
	tests := []struct {
		name        string
		success     time.Time
		failure     time.Time
		errorWords  string
		active      bool
		description string
	}{
		{"Counter", t1, t3, "no answer from 192.168.1.50", true, "its newest failure, the dead job, with its words"},
		{"Kitchen", t2, t1, "", true, "a failure older than its success has no words"},
		{"Bar", t3, time.Time{}, "", true, "no failure at all"},
		{"Quiet", time.Time{}, time.Time{}, "", true, "no job ever"},
	}
	for i, tt := range tests {
		p := h.Printers[i]
		if p.Name != tt.name || p.Active != tt.active {
			t.Errorf("printer %d is %q active=%v, want %q", i, p.Name, p.Active, tt.name)
		}
		if got := deref(p.LastSuccess); !got.Equal(tt.success) {
			t.Errorf("%s: LastSuccess = %v, want %v (%s)", tt.name, got, tt.success, tt.description)
		}
		if got := deref(p.LastFailure); !got.Equal(tt.failure) {
			t.Errorf("%s: LastFailure = %v, want %v (%s)", tt.name, got, tt.failure, tt.description)
		}
		if p.Error != tt.errorWords {
			t.Errorf("%s: Error = %q, want %q (%s)", tt.name, p.Error, tt.errorWords, tt.description)
		}
	}
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func TestTheOpenShiftIsNamed(t *testing.T) {
	d := newDB(t)
	venue := addVenue(t, d, "ven_TEST0001")
	opened := now.Add(-5 * time.Hour)
	write(t, d, func(q *gen.Queries) error {
		_, err := q.CreateShift(context.Background(), gen.CreateShiftParams{
			PublicID: "sh_TEST0001", VenueID: venue, TradingDay: "2026-10-22", OpenedBy: 1, OpenedAt: opened,
		})
		return err
	})
	s := newService(t, d, nil)
	h, err := s.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Shift == nil || h.Shift.ID != "sh_TEST0001" || h.Shift.TradingDay != "2026-10-22" || !h.Shift.OpenedAt.Equal(opened) {
		t.Errorf("Shift = %+v", h.Shift)
	}
}

func TestTheOpeningHoursAreCarriedAsSet(t *testing.T) {
	d := newDB(t)
	venue := addVenue(t, d, "ven_TEST0001")
	write(t, d, func(q *gen.Queries) error {
		return q.SetSetting(context.Background(), gen.SetSettingParams{
			VenueID: venue, Key: svcshift.SettingOpeningHours,
			Value: `{"mon":{"open":"09:00","close":"23:00"},"sat":{"open":"17:00","close":"02:00"}}`, UpdatedAt: now,
		})
	})
	s := newService(t, d, nil)
	h, err := s.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Hours) != 2 || h.Hours["mon"].Open != "09:00" || h.Hours["sat"].Close != "02:00" {
		t.Errorf("Hours = %v", h.Hours)
	}

	// A stored value nothing can read is an error, not a venue with no hours.
	write(t, d, func(q *gen.Queries) error {
		return q.SetSetting(context.Background(), gen.SetSettingParams{VenueID: venue, Key: svcshift.SettingOpeningHours, Value: "9 to 5", UpdatedAt: now})
	})
	if _, err := s.Summary(context.Background()); !errors.Is(err, svcshift.ErrVenueConfig) {
		t.Errorf("with unreadable hours: %v, want ErrVenueConfig", err)
	}
}

func TestTheBackupIsAsTheVenueSeesIt(t *testing.T) {
	d := newDB(t)
	addVenue(t, d, "ven_TEST0001")
	synced := now.Add(-30 * time.Second)
	tests := []struct {
		name   string
		status backup.Status
		state  string
		synced *time.Time
		words  string
	}{
		{"ok", backup.Status{State: backup.StateOK, LastSyncedAt: synced}, "ok", &synced, ""},
		{"failing", backup.Status{State: backup.StateFailing, LastSyncedAt: synced, Error: "bucket refused the key"}, "failing", &synced, "bucket refused the key"},
		{"not yet", backup.Status{State: backup.StateStarting}, "starting", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newService(t, d, fakeBackups{tt.status})
			h, err := s.Summary(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if h.Backup.State != tt.state || h.Backup.Error != tt.words || !deref(h.Backup.LastSyncedAt).Equal(deref(tt.synced)) {
				t.Errorf("Backup = %+v", h.Backup)
			}
		})
	}
}
