// Package printers is the back office's printer settings (phase 07, Task
// 5c, ADR-082): the venue's printers, which prints receipts, which prints
// each station's tickets, and a test page through the print queue.
package printers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/device/escpos"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/domain/printjob"
)

// The refusals. The API maps them to codes; Invalid carries the field.
var (
	ErrNotFound  = errors.New("no such printer at this venue")
	ErrRetired   = errors.New("the printer is retired")
	ErrNoStation = errors.New("no such station at this venue")
	ErrDuplicate = errors.New("a printer with that name already exists")
)

// Invalid is a setting that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// Queue is what the service needs of the print queue: a test page is a job
// like any other, so it retries and shows on the terminals' queue.
type Queue interface {
	Enqueue(ctx context.Context, j printq.Job) (int64, error)
}

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service is the printer settings.
type Service struct {
	db      *db.DB
	queue   Queue
	clock   Clock
	version string
}

// New returns a Service. version is printed on the test page.
func New(d *db.DB, q Queue, clock Clock, version string) (*Service, error) {
	if d == nil || q == nil {
		return nil, errors.New("printers: a database and a queue are required")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: d, queue: q, clock: clock, version: version}, nil
}

// Transport is how the server reaches a printer: its network address, or a
// Windows printer this computer drives. printers.transport stores it.
type Transport string

// The two transports (ADR-080).
const (
	Network Transport = printq.TCP
	USB     Transport = printq.Windows
)

// Transports is every transport, for the generated vocabulary.
func Transports() []Transport { return []Transport{Network, USB} }

// Settings is a printer as the back office edits it.
type Settings struct {
	Name      string    `json:"name"`
	Transport Transport `json:"transport"`
	Address   string    `json:"address"`
	PaperMM   int64     `json:"paper_mm"`
}

// maxName bounds a printer's name: it is a label, not a description.
const maxName = 40

// clean trims a printer's settings and refuses what cannot work.
func (s Settings) clean() (Settings, error) {
	s.Name, s.Address = strings.TrimSpace(s.Name), strings.TrimSpace(s.Address)
	switch {
	case s.Name == "":
		return s, &Invalid{"name", "Give the printer a name, like Counter or Kitchen."}
	case len(s.Name) > maxName:
		return s, &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", maxName)}
	case s.PaperMM != 80 && s.PaperMM != 58:
		return s, &Invalid{"paper_mm", "Choose 80mm or 58mm."}
	}
	switch s.Transport {
	case Network:
		if !validNetworkAddress(s.Address) {
			return s, &Invalid{"address", lanAddress}
		}
	case USB:
		if problem := windowsNameProblem(s.Address); problem != "" {
			return s, &Invalid{"address", problem}
		}
	default:
		return s, &Invalid{"transport", "Choose network or USB."}
	}
	return s, nil
}

// lanAddress is what a network printer's address must be, said the way the
// printers screen says it.
const lanAddress = "A printer's address is its IP on this network, such as 192.168.1.50, with :port after it if the port is not 9100."

// validNetworkAddress accepts an IP on this network, alone or with a port 1
// to 65535; alone, the send adds 9100 (escpos.DefaultPort). A hostname is
// refused, because what it resolves to can change after it is saved, and so
// is any address outside the private, link-local and loopback ranges: a
// printer is on the venue's LAN, and an address anywhere else would let
// the printers screen aim the server's sends at the internet (ADR-259).
func validNetworkAddress(addr string) bool {
	if addr == "" || strings.ContainsAny(addr, " /\\") {
		return false
	}
	host, port, err := net.SplitHostPort(addr)
	switch {
	case err == nil:
		if !validPort(port) {
			return false
		}
	case strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]"):
		host = addr[1 : len(addr)-1] // an IPv6 address in brackets, no port
	default:
		host = addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false // a hostname, or not an address at all
	}
	if strings.HasPrefix(addr, "[") && !ip.Is6() {
		return false // brackets are for IPv6
	}
	if ip.Zone() != "" && !ip.IsLinkLocalUnicast() {
		return false // a zone names the interface a link-local address is on
	}
	ip = ip.Unmap()
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLoopback()
}

// validPort accepts 1 to 65535, written plainly.
func validPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535 && strconv.Itoa(n) == port
}

// maxWindowsName bounds a Windows printer's name. Windows allows longer;
// no receipt printer's name comes close.
const maxWindowsName = 128

// windowsNameProblem says what is wrong with a Windows printer's name, or
// returns "". The name goes to OpenPrinterW, which takes more than a local
// printer's name: "\PC\Printer" makes the server connect to another
// computer, and a comma opens a port or monitor handle instead of a
// printer. Windows printer names contain neither a backslash nor a comma,
// so refusing them costs no real printer anything (ADR-259).
func windowsNameProblem(name string) string {
	switch {
	case name == "":
		return "Give the printer's name as Windows shows it, like EPSON TM-T82."
	case utf8.RuneCountInString(name) > maxWindowsName:
		return fmt.Sprintf("Keep the printer's name to %d characters.", maxWindowsName)
	case strings.IndexFunc(name, unicode.IsControl) >= 0 || !utf8.ValidString(name):
		return "Give the printer's name as Windows shows it, on one line."
	case strings.HasPrefix(name, `\`) || strings.HasPrefix(name, "//"):
		return "Give the name of a printer plugged into this computer. A printer shared from another computer cannot be used."
	case strings.Contains(name, ":/") || strings.ContainsAny(name, `\,`):
		return "Give the printer's name as Windows shows it, like EPSON TM-T82, not a path or a link."
	}
	return ""
}

// Overview is everything the printers screen shows.
type Overview struct {
	Printers []gen.PrintersForVenueRow
	Stations []gen.StationsWithPrintersRow
}

// List reads the venue's printers, each with its newest job, and its
// active stations with the printer each one prints on.
func (s *Service) List(ctx context.Context, venueID int64) (Overview, error) {
	q := s.db.Queries()
	ps, err := q.PrintersForVenue(ctx, venueID)
	if err != nil {
		return Overview{}, fmt.Errorf("printers: listing: %w", err)
	}
	st, err := q.StationsWithPrinters(ctx, venueID)
	if err != nil {
		return Overview{}, fmt.Errorf("printers: listing stations: %w", err)
	}
	return Overview{Printers: ps, Stations: st}, nil
}

// Create adds a printer.
func (s *Service) Create(ctx context.Context, venueID int64, in Settings) (int64, error) {
	in, err := in.clean()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now().UTC()
	var id int64
	err = s.db.WriteTx(ctx, func(tx *gen.Queries) error {
		var err error
		id, err = tx.CreatePrinter(ctx, gen.CreatePrinterParams{
			VenueID: venueID, Name: in.Name, Transport: string(in.Transport), Address: in.Address, PaperMm: in.PaperMM,
			CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, duplicate(err)
}

// Update changes a printer's name, connection or paper. Jobs already queued
// keep the bytes they were laid out with (ADR-080).
func (s *Service) Update(ctx context.Context, venueID, id int64, in Settings) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	return duplicate(s.write(ctx, func(tx *gen.Queries, now time.Time) (int64, error) {
		return tx.UpdatePrinter(ctx, gen.UpdatePrinterParams{
			Name: in.Name, Transport: string(in.Transport), Address: in.Address, PaperMm: in.PaperMM, Now: now, ID: id, VenueID: venueID,
		})
	}))
}

// SetActive retires a printer or brings it back. Retired, it prints
// nothing: its waiting jobs go dead, it stops printing receipts, and its
// stations print nowhere until given another. Brought back, it does not
// take the receipts back.
func (s *Service) SetActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.write(ctx, func(tx *gen.Queries, now time.Time) (int64, error) {
		return tx.SetPrinterActive(ctx, gen.SetPrinterActiveParams{Active: active, Now: now, ID: id, VenueID: venueID})
	})
}

// SetReceipt makes a printer the receipt printer, or, with id 0, leaves the
// venue with none.
func (s *Service) SetReceipt(ctx context.Context, venueID, id int64) error {
	now := s.clock.Now().UTC()
	return s.db.WriteTx(ctx, func(tx *gen.Queries) error {
		if id != 0 {
			if err := s.usable(ctx, tx, venueID, id); err != nil {
				return err
			}
		}
		if err := tx.ClearReceiptPrinter(ctx, gen.ClearReceiptPrinterParams{Now: now, VenueID: venueID}); err != nil {
			return err
		}
		if id == 0 {
			return nil
		}
		_, err := tx.MarkReceiptPrinter(ctx, gen.MarkReceiptPrinterParams{Now: now, ID: id, VenueID: venueID})
		return err
	})
}

// SetStation sets the printer a station's tickets print on, or, with id 0,
// none.
func (s *Service) SetStation(ctx context.Context, venueID int64, code string, id int64) error {
	now := s.clock.Now().UTC()
	return s.db.WriteTx(ctx, func(tx *gen.Queries) error {
		printer := sql.NullInt64{}
		if id != 0 {
			if err := s.usable(ctx, tx, venueID, id); err != nil {
				return err
			}
			printer = sql.NullInt64{Int64: id, Valid: true}
		}
		n, err := tx.SetStationPrinter(ctx, gen.SetStationPrinterParams{PrinterID: printer, Now: now, VenueID: venueID, Code: code})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: %q", ErrNoStation, code)
		}
		return nil
	})
}

// TestPage queues escpos.TestPage for a printer, at its paper width, and
// returns the job's id.
func (s *Service) TestPage(ctx context.Context, venueID, id int64) (int64, error) {
	p, err := s.printer(ctx, s.db.Queries(), venueID, id)
	if err != nil {
		return 0, err
	}
	if !p.Active {
		return 0, fmt.Errorf("%w: %s", ErrRetired, p.Name)
	}
	paper := escpos.Paper80
	if p.PaperMm == 58 {
		paper = escpos.Paper58
	}
	page, err := escpos.TestPage(paper, s.clock.Now(), s.version, false)
	if err != nil {
		return 0, err
	}
	return s.queue.Enqueue(ctx, printq.Job{
		VenueID: venueID, PrinterID: id, Kind: printjob.Test, Label: "Test page, " + p.Name, Payload: page,
	})
}

// write runs one update of a printer and refuses it when it matched none.
func (s *Service) write(ctx context.Context, fn func(tx *gen.Queries, now time.Time) (int64, error)) error {
	return s.db.WriteTx(ctx, func(tx *gen.Queries) error {
		n, err := fn(tx, s.clock.Now().UTC())
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Service) printer(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.PrinterByIDRow, error) {
	p, err := q.PrinterByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && p.VenueID != venueID) {
		return gen.PrinterByIDRow{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if err != nil {
		return gen.PrinterByIDRow{}, fmt.Errorf("printers: reading %d: %w", id, err)
	}
	return p, nil
}

// usable refuses a printer from another venue, or a retired one.
func (s *Service) usable(ctx context.Context, q *gen.Queries, venueID, id int64) error {
	p, err := s.printer(ctx, q, venueID, id)
	if err != nil {
		return err
	}
	if !p.Active {
		return fmt.Errorf("%w: %s", ErrRetired, p.Name)
	}
	return nil
}

func duplicate(err error) error {
	if db.IsUniqueViolation(err) {
		return fmt.Errorf("%w: %w", ErrDuplicate, &Invalid{"name", "A printer with that name already exists."})
	}
	return err
}
