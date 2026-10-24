package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/export"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// ErrExportNotFound means no such export file at this venue.
var ErrExportNotFound = errors.New("no such export at this venue")

// ExportKind is which file an export is.
type ExportKind string

// The files a close writes.
const (
	ExportTaxRegister ExportKind = "tax_register"
	ExportTally       ExportKind = "tally"
	ExportSales       ExportKind = "sales"
)

// ExportKinds is every kind, for the generated vocabulary.
func ExportKinds() []ExportKind { return []ExportKind{ExportTaxRegister, ExportTally, ExportSales} }

// ExportFile is a stored export, without its contents.
type ExportFile struct {
	ID        int64      `json:"id"`
	Kind      ExportKind `json:"kind"`
	Filename  string     `json:"filename"`
	Size      int64      `json:"size"` // bytes
	CreatedAt time.Time  `json:"created_at"`
}

// ExportBody is a stored export and its contents, CSV text.
type ExportBody struct {
	ExportFile
	ShiftID int64  `json:"shift_id"`
	Text    string `json:"text"`
}

// writeExports builds a closing shift's files and stores them, in the
// closing transaction (ADR-104): a closed shift always has them, and a
// shift whose files cannot be written does not close.
func writeExports(ctx context.Context, q *gen.Queries, venueID int64, sh Shift, v venue, now time.Time) error {
	taxes, err := svcorder.VenueTaxes(ctx, q, venueID)
	if err != nil {
		return err
	}
	orders, err := stampedOrders(ctx, q, sh.ID)
	if err != nil {
		return err
	}
	return writeExportsFrom(ctx, q, venueID, sh, v, now, taxes, orders)
}

// writeExportsFrom is writeExports over the venue's taxes and the shift's
// orders already read (ADR-372).
func writeExportsFrom(ctx context.Context, q *gen.Queries, venueID int64, sh Shift, v venue, now time.Time, taxes order.Taxes, orders [][]shift.StampedEvent) error {
	files, err := export.Build(export.Input{ShiftID: sh.ID, Currency: v.currency, Taxes: taxes, Location: v.location, Orders: orders})
	if err != nil {
		return fmt.Errorf("the exports for %s: %w", sh.PublicID, err)
	}
	for _, f := range []struct {
		kind  ExportKind
		table export.Table
	}{{ExportTaxRegister, files.Register}, {ExportTally, files.Tally}, {ExportSales, files.Sales}} {
		body, err := f.table.CSV()
		if err != nil {
			return fmt.Errorf("writing the %s file: %w", f.kind, err)
		}
		err = q.CreateExport(ctx, gen.CreateExportParams{
			ShiftID: sh.ID, Kind: string(f.kind), Filename: filename(sh, f.kind), Body: string(body), CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("storing the %s file: %w", f.kind, err)
		}
	}
	return nil
}

// filename is what the file is saved as: the trading day first, so a
// folder of them sorts by day. "2026-09-28-shf_7K3M9QX2-tax-register.csv".
func filename(sh Shift, kind ExportKind) string {
	return fmt.Sprintf("%s-%s-%s.csv", sh.TradingDay, sh.PublicID, strings.ReplaceAll(string(kind), "_", "-"))
}

// Exports lists a shift's export files.
func (s *Service) Exports(ctx context.Context, venueID, shiftID int64) ([]ExportFile, error) {
	q := s.db.Queries()
	if _, err := shiftAt(ctx, q, venueID, shiftID); err != nil {
		return nil, err
	}
	rows, err := q.ExportsForShift(ctx, shiftID)
	if err != nil {
		return nil, fmt.Errorf("listing shift %d's exports: %w", shiftID, err)
	}
	out := make([]ExportFile, len(rows))
	for i, r := range rows {
		out[i] = ExportFile{ID: r.ID, Kind: ExportKind(r.Kind), Filename: r.Filename, Size: r.Size.Int64, CreatedAt: r.CreatedAt.UTC()}
	}
	return out, nil
}

// Export is one export file and its contents; another venue's is not found.
func (s *Service) Export(ctx context.Context, venueID, id int64) (ExportBody, error) {
	row, err := s.db.Queries().ExportByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.VenueID != venueID) {
		return ExportBody{}, fmt.Errorf("%w: %d", ErrExportNotFound, id)
	}
	if err != nil {
		return ExportBody{}, fmt.Errorf("reading export %d: %w", id, err)
	}
	return ExportBody{
		ExportFile: ExportFile{ID: row.ID, Kind: ExportKind(row.Kind), Filename: row.Filename, Size: int64(len(row.Body)), CreatedAt: row.CreatedAt.UTC()},
		ShiftID:    row.ShiftID,
		Text:       row.Body,
	}, nil
}
