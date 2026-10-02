package printers

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/domain/printjob"
)

var now = time.Date(2026, 9, 25, 4, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// queue records what was queued.
type queue struct{ jobs []printq.Job }

func (q *queue) Enqueue(_ context.Context, j printq.Job) (int64, error) {
	q.jobs = append(q.jobs, j)
	return int64(len(q.jobs)), nil
}

// newService is the settings over a fresh database: venue 1 with kitchen
// and bar stations, and venue 2 with a printer of its own (id 1).
func newService(t *testing.T) (*Service, *db.DB, *queue) {
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
		for _, v := range []string{"ven_PRINTER1", "ven_PRINTER2"} {
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: v, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		for i, code := range []string{"kitchen", "bar"} {
			if _, err := q.CreateStation(ctx, gen.CreateStationParams{VenueID: 1, Code: code, Name: code, SortOrder: int64(i), CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
		}
		_, err := q.CreatePrinter(ctx, gen.CreatePrinterParams{VenueID: 2, Name: "Elsewhere", Transport: "tcp", Address: "10.0.0.9", PaperMm: 80, CreatedAt: now, UpdatedAt: now})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	q := &queue{}
	s, err := New(d, q, clock{}, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	return s, d, q
}

func TestSettingsAreChecked(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	good := Settings{Name: " Counter ", Transport: "tcp", Address: "192.168.1.50", PaperMM: 80}
	tests := []struct {
		name  string
		edit  func(*Settings)
		field string
	}{
		{"no name", func(s *Settings) { s.Name = "  " }, "name"},
		{"a long name", func(s *Settings) { s.Name = string(bytes.Repeat([]byte("x"), 41)) }, "name"},
		{"76mm paper", func(s *Settings) { s.PaperMM = 76 }, "paper_mm"},
		{"an unknown transport", func(s *Settings) { s.Transport = "bluetooth" }, "transport"},
		{"no address", func(s *Settings) { s.Address = "" }, "address"},
		{"a URL, not an address", func(s *Settings) { s.Address = "http://10.0.0.5" }, "address"},
		{"port 0", func(s *Settings) { s.Address = "10.0.0.5:0" }, "address"},
		{"port 70000", func(s *Settings) { s.Address = "10.0.0.5:70000" }, "address"},
		{"a port that is not a number", func(s *Settings) { s.Address = "10.0.0.5:print" }, "address"},
		{"no Windows name", func(s *Settings) { s.Transport, s.Address = "windows", " " }, "address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := good
			tt.edit(&in)
			var inv *Invalid
			if _, err := s.Create(ctx, 1, in); !errors.As(err, &inv) || inv.Field != tt.field {
				t.Errorf("err = %v, want Invalid on %s", err, tt.field)
			}
		})
	}
	for _, addr := range []string{"192.168.1.50", "192.168.1.50:9101", "printer.local", "[fe80::1]:9100", "fe80::1"} {
		in := good
		in.Name, in.Address = addr, addr
		if _, err := s.Create(ctx, 1, in); err != nil {
			t.Errorf("address %q refused: %v", addr, err)
		}
	}
	usb := Settings{Name: "USB", Transport: "windows", Address: "EPSON TM-T82 Receipt", PaperMM: 58}
	if _, err := s.Create(ctx, 1, usb); err != nil {
		t.Errorf("a Windows printer: %v", err)
	}
}

// TestPrinterLifecycle: add, route, test, retire, bring back; and nothing
// reaches another venue's printer.
func TestPrinterLifecycle(t *testing.T) {
	s, d, q := newService(t)
	ctx := context.Background()
	counter, err := s.Create(ctx, 1, Settings{Name: "Counter", Transport: "tcp", Address: "10.0.0.5", PaperMM: 58})
	if err != nil {
		t.Fatal(err)
	}
	kitchen, err := s.Create(ctx, 1, Settings{Name: "Kitchen", Transport: "tcp", Address: "10.0.0.6", PaperMM: 80})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, 1, Settings{Name: "Kitchen", Transport: "tcp", Address: "10.0.0.7", PaperMM: 80}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("a second Kitchen: %v", err)
	}
	// The same name at another venue is fine.
	if _, err := s.Create(ctx, 2, Settings{Name: "Kitchen", Transport: "tcp", Address: "10.0.0.7", PaperMM: 80}); err != nil {
		t.Errorf("Kitchen at venue 2: %v", err)
	}

	if err := s.SetReceipt(ctx, 1, counter); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStation(ctx, 1, "kitchen", kitchen); err != nil {
		t.Fatal(err)
	}
	route := func(kind printjob.Kind, station string) int64 {
		t.Helper()
		r, err := printq.RouteFor(ctx, d.Queries(), 1, kind, station)
		if err != nil {
			t.Fatal(err)
		}
		return r.PrinterID
	}
	if route(printjob.Receipt, "") != counter || route(printjob.Kitchen, "kitchen") != kitchen || route(printjob.Kitchen, "bar") != 0 {
		t.Fatal("routing does not follow the settings")
	}

	// Moving the receipt printer leaves one, not two.
	if err := s.SetReceipt(ctx, 1, kitchen); err != nil {
		t.Fatal(err)
	}
	if route(printjob.Receipt, "") != kitchen {
		t.Error("the receipt printer did not move")
	}

	refusals := []struct {
		name string
		err  error
		want error
	}{
		{"another venue's printer as the receipt printer", s.SetReceipt(ctx, 1, 1), ErrNotFound},
		{"another venue's printer at a station", s.SetStation(ctx, 1, "bar", 1), ErrNotFound},
		{"a station that does not exist", s.SetStation(ctx, 1, "pastry", kitchen), ErrNoStation},
		{"editing another venue's printer", s.Update(ctx, 1, 1, Settings{Name: "Mine", Transport: "tcp", Address: "10.0.0.1", PaperMM: 80}), ErrNotFound},
		{"retiring another venue's printer", s.SetActive(ctx, 1, 1, false), ErrNotFound},
	}
	for _, r := range refusals {
		if !errors.Is(r.err, r.want) {
			t.Errorf("%s: %v, want %v", r.name, r.err, r.want)
		}
	}

	if _, err := s.TestPage(ctx, 1, counter); err != nil {
		t.Fatal(err)
	}
	j := q.jobs[0]
	if j.Kind != printjob.Test || j.PrinterID != counter || j.Label != "Test page, Counter" ||
		!bytes.Contains(j.Payload, []byte("32 columns")) || !bytes.Contains(j.Payload, []byte("v1.2.3")) {
		t.Errorf("test job = %+v", j)
	}

	if err := s.Update(ctx, 1, counter, Settings{Name: "Front", Transport: "windows", Address: "EPSON", PaperMM: 80}); err != nil {
		t.Fatal(err)
	}
	// Retiring the receipt printer leaves the venue with none; a station's
	// retired printer prints nowhere.
	if err := s.SetActive(ctx, 1, kitchen, false); err != nil {
		t.Fatal(err)
	}
	if route(printjob.Receipt, "") != 0 || route(printjob.Kitchen, "kitchen") != 0 {
		t.Error("a retired printer still gets jobs")
	}
	if _, err := s.TestPage(ctx, 1, kitchen); !errors.Is(err, ErrRetired) {
		t.Errorf("a test page on a retired printer: %v", err)
	}
	if err := s.SetReceipt(ctx, 1, kitchen); !errors.Is(err, ErrRetired) {
		t.Errorf("a retired printer for receipts: %v", err)
	}
	if err := s.SetActive(ctx, 1, kitchen, true); err != nil {
		t.Fatal(err)
	}
	if route(printjob.Kitchen, "kitchen") != kitchen || route(printjob.Receipt, "") != 0 {
		t.Error("bringing it back should restore its station but not the receipts")
	}
	if err := s.SetReceipt(ctx, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStation(ctx, 1, "kitchen", 0); err != nil || route(printjob.Kitchen, "kitchen") != 0 {
		t.Errorf("clearing a station: %v", err)
	}

	o, err := s.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Printers) != 2 || o.Printers[0].Name != "Front" || o.Printers[0].Transport != "windows" || len(o.Stations) != 2 {
		t.Errorf("overview = %+v", o)
	}
}

func TestNewNeedsItsParts(t *testing.T) {
	if _, err := New(nil, &queue{}, nil, ""); err == nil {
		t.Error("no database")
	}
}
