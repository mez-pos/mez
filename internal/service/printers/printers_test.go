package printers

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
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
	for _, addr := range []string{"192.168.1.50", "192.168.1.50:9101", "[fe80::1]:9100", "fe80::1"} {
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

// TestNetworkAddressIsOnTheLAN: a network printer is an IP on this network
// (ADR-259). A hostname, a public address, or anything with a scheme or a
// path is refused, so the printers screen cannot point the server's sends
// at the internet or at a name that resolves there later.
func TestNetworkAddressIsOnTheLAN(t *testing.T) {
	tests := []struct {
		addr string
		ok   bool
	}{
		{"192.168.1.50", true},
		{"192.168.1.50:9100", true},
		{"192.168.1.50:9101", true},
		{"10.0.0.5", true},
		{"10.255.255.254:65535", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.50:6379", true},
		{"169.254.10.20", true},
		{"169.254.10.20:9100", true},
		{"127.0.0.1", true},
		{"127.0.0.1:9100", true},
		{"[fe80::1%eth0]:9100", true},

		// Loopback and link-local addresses reach this computer and the
		// cloud's metadata service, so only the printer's port is allowed.
		{"127.0.0.1:1", false},
		{"127.0.0.1:6379", false},
		{"127.0.0.1:9101", false},
		{"[::1]:9101", false},
		{"169.254.169.254:80", false},
		{"169.254.10.20:9101", false},
		{"[fe80::1]:9101", false},
		{"[fe80::1%eth0]:631", false},
		{"fc00::1", true},
		{"fd12:3456::50", true},
		{"[fd12:3456::50]:9100", true},
		{"[fd12:3456::50]", true},
		{"fe80::1", true},
		{"[fe80::1]:9100", true},
		{"fe80::1%eth0", true},
		{"::1", true},
		{"[::1]:9100", true},
		{"::ffff:192.168.1.50", true},

		{"", false},
		{"printer.local", false},
		{"printer.local:9100", false},
		{"localhost", false},
		{"kitchen", false},
		{"8.8.8.8", false},
		{"8.8.8.8:9100", false},
		{"1.1.1.1", false},
		{"172.15.255.255", false},
		{"172.32.0.1", false},
		{"192.169.0.1", false},
		{"100.64.0.1", false},
		{"0.0.0.0", false},
		{"255.255.255.255", false},
		{"224.0.0.1", false},
		{"2001:4860:4860::8888", false},
		{"[2001:4860:4860::8888]:9100", false},
		{"::", false},
		{"::ffff:8.8.8.8", false},
		{"ff02::1", false},
		{"fd12::1%eth0", false},
		{"http://192.168.1.50", false},
		{"192.168.1.50/24", false},
		{"192.168.1.50:", false},
		{"192.168.1.50:0", false},
		{"192.168.1.50:65536", false},
		{"192.168.1.50:09100", false},
		{"192.168.1.50:+9100", false},
		{"192.168.1.50:print", false},
		{"192.168.1.50 9100", false},
		{"192.168.1", false},
		{"0x0a.0.0.1", false},
		{"010.0.0.1", false},
		{"[192.168.1.50]:9100", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			in := Settings{Name: "Counter", Transport: Network, Address: tt.addr, PaperMM: 80}
			_, err := in.clean()
			var inv *Invalid
			switch {
			case tt.ok && err != nil:
				t.Errorf("refused: %v", err)
			case !tt.ok && (!errors.As(err, &inv) || inv.Field != "address"):
				t.Errorf("err = %v, want Invalid on address", err)
			case !tt.ok && inv.Message != lanAddress:
				t.Errorf("message = %q, want %q", inv.Message, lanAddress)
			}
		})
	}
}

// TestWindowsPrinterName: a USB printer's address is a name the spooler
// opens. A share on another computer, a URL, or a name with control
// characters in it is refused before it reaches OpenPrinterW (ADR-259).
func TestWindowsPrinterName(t *testing.T) {
	tests := []struct {
		name string
		addr string
		ok   bool
	}{
		{"a name as Windows shows it", "EPSON TM-T82 Receipt", true},
		{"a name with punctuation", "POS-80C (copy 1)", true},
		{"a name in another script", "प्रिंटर रसोई", true},
		{"128 characters", strings.Repeat("x", 128), true},
		{"128 characters, not bytes", strings.Repeat("é", 128), true},

		{"nothing", "", false},
		{"spaces only", "   ", false},
		{"129 characters", strings.Repeat("x", 129), false},
		{"a share on another computer", `\FRONTPC\EPSON`, false},
		{"a share by IP", `\203.0.113.9\print`, false},
		{"a share with forward slashes", "//FRONTPC/EPSON", false},
		{"a URL", "http://203.0.113.9/printers/x", false},
		{"a file URL", "file:/C:/x", false},
		{"a drive path", `C:\Windows\x`, false},
		{"a backslash", `EPSON\TM`, false},
		{"a comma, which opens a port monitor", "EPSON,XcvMonitor Local Port", false},
		{"a NUL", "EPSON\x00TM", false},
		{"a newline", "EPSON\nTM", false},
		{"a tab", "EPSON\tTM", false},
		{"a DEL", "EPSON\x7fTM", false},
		{"a C1 control", "EPSON\u0085TM", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := Settings{Name: "USB", Transport: USB, Address: tt.addr, PaperMM: 80}
			_, err := in.clean()
			var inv *Invalid
			switch {
			case tt.ok && err != nil:
				t.Errorf("refused: %v", err)
			case !tt.ok && (!errors.As(err, &inv) || inv.Field != "address"):
				t.Errorf("err = %v, want Invalid on address", err)
			}
		})
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
