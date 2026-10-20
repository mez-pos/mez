package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	csvsheet "github.com/mez-pos/mez/internal/domain/sheet"
	"github.com/mez-pos/mez/internal/domain/stock"
)

// Ingredients from a spreadsheet (phase 10, Task 10a, ADR-298).
// stock.ReadItems reads the file; this reads what the store has for it to
// be read against, and writes the plan it answers. It is the menu import's
// shape (ADR-289): a preview that writes nothing, and an import that reads
// the file again inside the write.

// MaxImportBytes is the largest file an import reads.
const MaxImportBytes = 1 << 20

// ErrImportRefused is an import of a file with a problem in it. Nothing was
// written; what was returned with it says what the problems are.
var ErrImportRefused = errors.New("the file has problems, so nothing was imported")

// errNothingToImport ends an import that has nothing to add without a write.
var errNothingToImport = errors.New("nothing to import")

// ItemsImport is what a file would add to the store, or has added.
type ItemsImport struct {
	ToAdd []ItemImportRow `json:"to_add"`
	// Skipped are the rows left out because the store already has an item
	// of that name.
	Skipped  []ImportSkipped `json:"skipped"`
	Problems []ImportProblem `json:"problems"`
	// Examples is how many rows were the template's examples and left out.
	Examples int `json:"examples"`
	// Imported: ToAdd is in the store now, with nothing on hand. False for
	// a preview, a refusal, and an import that found nothing to add.
	Imported bool `json:"imported"`
}

// ItemImportRow is one stock item a file adds.
type ItemImportRow struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row      int        `json:"row"`
	Name     string     `json:"name"`
	Unit     stock.Unit `json:"unit"`
	LowLevel stock.Qty  `json:"low_level"`
	// Cost is what a kilogram, a litre or one of it costs; zero is not known.
	Cost         money.Amount `json:"cost"`
	CountAtClose bool         `json:"count_at_close"`
}

// ImportSkipped is a row whose item is already there.
type ImportSkipped struct {
	Row  int    `json:"row"`
	Name string `json:"name"`
}

// ImportProblem is why a row cannot be added. Row 0 is the file as a whole,
// and has no column.
type ImportProblem struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// PreviewItems reads a file of ingredients and answers what importing it
// would do. It writes nothing.
func (s *Service) PreviewItems(ctx context.Context, venueID int64, data []byte) (ItemsImport, error) {
	plan, currency, err := planItems(ctx, s.db.Queries(), venueID, data)
	if err != nil {
		return ItemsImport{}, err
	}
	return itemsImportOf(plan, currency), nil
}

// ImportItems adds a file's stock items in one transaction: all of them or
// none. Each has nothing on hand: stock arrives by Receive, so the ledger
// says where it came from. A file with any problem is refused with
// ErrImportRefused and the problems. Sending the same file twice adds
// nothing the second time.
func (s *Service) ImportItems(ctx context.Context, venueID int64, data []byte) (ItemsImport, error) {
	var out ItemsImport
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		plan, currency, err := planItems(ctx, q, venueID, data)
		if err != nil {
			return err
		}
		out = itemsImportOf(plan, currency)
		switch {
		case len(plan.Problems) > 0:
			return ErrImportRefused
		case len(plan.Rows) == 0:
			return errNothingToImport
		}
		now := s.now()
		for _, r := range plan.Rows {
			if _, err := q.CreateStockItem(ctx, gen.CreateStockItemParams{
				VenueID: venueID, Name: r.Name, Unit: string(r.Unit), LowLevel: int64(r.LowLevel), CountAtClose: r.CountAtClose,
				Cost: r.Cost, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("stock: importing row %d (%s): %w", r.Row, r.Name, err)
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errNothingToImport):
		return out, nil
	case errors.Is(err, ErrImportRefused):
		return out, err
	case err != nil:
		return ItemsImport{}, err
	}
	out.Imported = true
	return out, nil
}

// ItemsTemplate is the example rows of the file a venue fills in, under
// stock.ItemColumns, with costs written as its currency writes them.
func (s *Service) ItemsTemplate(ctx context.Context, venueID int64) ([][]string, error) {
	currency, err := currencyOf(ctx, s.db.Queries(), venueID)
	if err != nil {
		return nil, err
	}
	return stock.ItemsTemplate(currency), nil
}

func checkFileSize(data []byte) error {
	if len(data) > MaxImportBytes {
		return &Invalid{"file", fmt.Sprintf("That file is over %d KB.", MaxImportBytes>>10)}
	}
	return nil
}

// planItems reads the store through q and the file against it.
func planItems(ctx context.Context, q *gen.Queries, venueID int64, data []byte) (stock.ItemsPlan, string, error) {
	if err := checkFileSize(data); err != nil {
		return stock.ItemsPlan{}, "", err
	}
	currency, err := currencyOf(ctx, q, venueID)
	if err != nil {
		return stock.ItemsPlan{}, "", err
	}
	rows, err := q.StockItemsWithOnHand(ctx, venueID)
	if err != nil {
		return stock.ItemsPlan{}, "", fmt.Errorf("stock: reading the stock of venue %d: %w", venueID, err)
	}
	v := stock.ItemsVenue{Currency: currency, Names: make([]string, len(rows))}
	for i, r := range rows {
		v.Names[i] = r.Name
	}
	return stock.ReadItems(data, v), currency, nil
}

// itemsImportOf is a plan as the back office reads it. Every list is there,
// if empty.
func itemsImportOf(plan stock.ItemsPlan, currency string) ItemsImport {
	out := ItemsImport{
		ToAdd:    make([]ItemImportRow, len(plan.Rows)),
		Skipped:  make([]ImportSkipped, len(plan.Skipped)),
		Problems: problemsOf(plan.Problems),
		Examples: plan.Examples,
	}
	for i, r := range plan.Rows {
		out.ToAdd[i] = ItemImportRow{
			Row: r.Row, Name: r.Name, Unit: r.Unit, LowLevel: r.LowLevel,
			Cost: money.Amount{Minor: r.Cost, Currency: currency}, CountAtClose: r.CountAtClose,
		}
	}
	for i, sk := range plan.Skipped {
		out.Skipped[i] = ImportSkipped(sk)
	}
	return out
}

func problemsOf(problems []csvsheet.Problem) []ImportProblem {
	out := make([]ImportProblem, len(problems))
	for i, p := range problems {
		out[i] = ImportProblem(p)
	}
	return out
}
