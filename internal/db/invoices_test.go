package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/invoice"
	"github.com/byjoellobo/mez/internal/domain/invoice/invoicetest"
)

// sqlAllocTx adapts a write transaction to invoicetest.Tx.
type sqlAllocTx struct {
	InvoiceAllocator
	tx *sql.Tx
}

func (a sqlAllocTx) Commit() error   { return a.tx.Commit() }
func (a sqlAllocTx) Rollback() error { return a.tx.Rollback() }

// TestInvoiceAllocatorConformance holds the database allocator to the same
// contract the domain's reference allocator meets, ADR-048. The concurrent
// case runs eight goroutines against the real single-writer pool.
func TestInvoiceAllocatorConformance(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	// The suite uses venues 101 to 108, and the counter row has a foreign
	// key to its venue.
	for id := int64(101); id <= 108; id++ {
		insertVenue(t, d, id)
	}

	invoicetest.Conformance(t, func() invoicetest.Tx {
		tx, err := d.Writer().BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		return sqlAllocTx{InvoiceAllocator: NewInvoiceAllocator(gen.New(tx)), tx: tx}
	})
}

// TestConcurrentIssuingIsGaplessOnDisk is the phase's concurrency test run
// to the end: many goroutines issue invoices the way a sale will, allocating
// a number and writing it to the register in one transaction, with every
// fourth sale failing after it has its number. What is audited is what the
// database holds afterwards, not what the goroutines believe they wrote.
func TestConcurrentIssuingIsGaplessOnDisk(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedTradingVenue(t, d)
	orderID := openTestOrder(t, d, "ord_INVOICES", untaxed)

	series := map[string]invoice.Series{}
	for _, code := range []string{"NF", "DLV"} {
		s, err := invoice.NewSeries(code, code, invoice.IndiaFY, "{prefix}/{fy:short}/{seq:06d}", 16)
		if err != nil {
			t.Fatalf("NewSeries: %v", err)
		}
		series[code] = s
	}
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skipf("no tz database: %v", err)
	}
	errFailedSale := errors.New("the sale failed after it had a number")

	const workers, each = 16, 20
	var (
		mu        sync.Mutex
		committed []invoice.Issued
		wg        sync.WaitGroup
		failures  = make(chan error, workers*each)
	)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := series[[]string{"NF", "DLV"}[w%2]]
			for i := range each {
				var issued invoice.Issued
				err := d.WriteTx(ctx, func(q *gen.Queries) error {
					var err error
					issued, err = invoice.Issue(ctx, NewInvoiceAllocator(q), s, 1, at, ist)
					if err != nil {
						return err
					}
					if err := recordInvoice(ctx, q, 1, orderID, issued, at); err != nil {
						return err
					}
					if (w+i)%4 == 0 {
						return errFailedSale
					}
					return nil
				})
				if errors.Is(err, errFailedSale) {
					continue
				}
				if err != nil {
					failures <- err
					continue
				}
				mu.Lock()
				committed = append(committed, issued)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("a sale failed that should not have: %v", err)
	}

	rows, err := d.Queries().InvoicesForVenue(ctx, 1)
	if err != nil {
		t.Fatalf("InvoicesForVenue: %v", err)
	}
	onDisk := make([]invoice.Issued, len(rows))
	for i, r := range rows {
		onDisk[i] = invoice.Issued{
			Key:       invoice.Key{VenueID: r.VenueID, Series: r.Series, FYStart: int(r.FyStart)},
			Seq:       r.Seq,
			InvoiceNo: r.InvoiceNo,
		}
	}
	if err := invoice.Audit(onDisk); err != nil {
		t.Fatalf("%d invoices on disk: %v", len(onDisk), err)
	}
	if len(onDisk) != len(committed) {
		t.Fatalf("%d invoices on disk, %d sales committed", len(onDisk), len(committed))
	}
	numbers := func(in []invoice.Issued) []string {
		out := make([]string, len(in))
		for i, x := range in {
			out[i] = x.InvoiceNo
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(numbers(onDisk), numbers(committed)) {
		t.Error("the numbers on disk are not the numbers the committed sales were given")
	}

	perSeries := map[string]int64{}
	for _, in := range onDisk {
		perSeries[in.Key.Series]++
	}
	for code, n := range perSeries {
		last, err := d.Queries().InvoiceSeriesLastSeq(ctx, gen.InvoiceSeriesLastSeqParams{VenueID: 1, Series: code, FyStart: 2026})
		if err != nil {
			t.Fatalf("InvoiceSeriesLastSeq(%s): %v", code, err)
		}
		if last != n {
			t.Errorf("series %s: counter at %d, %d invoices written", code, last, n)
		}
	}
}

// TestInvoiceSchemaGuards is the series guarantee held by the database
// itself, for the day some code path forgets to go through the allocator.
func TestInvoiceSchemaGuards(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedTradingVenue(t, d)
	orderID := openTestOrder(t, d, "ord_GUARDS", untaxed)

	key := invoice.Key{VenueID: 1, Series: "NF", FYStart: 2026}
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		for range 2 {
			seq, err := NewInvoiceAllocator(q).Next(ctx, key)
			if err != nil {
				return err
			}
			in := invoice.Issued{Key: key, Seq: seq, InvoiceNo: "NF/26-27/00000" + string(rune('0'+seq))}
			if err := recordInvoice(ctx, q, 1, orderID, in, at); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("issuing two invoices: %v", err)
	}

	const insert = `INSERT INTO invoices (venue_id, order_id, series, fy_start, seq, invoice_no, issued_at)
	                VALUES (1, ?, ?, 2026, ?, ?, '2026-09-18 00:00:00+00:00')`
	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{"a number past the counter", insert, []any{orderID, "NF", 3, "NF/26-27/000003"}},
		{"a number in a series with no counter", insert, []any{orderID, "DLV", 1, "DLV/26-27/000001"}},
		{"a number twice", insert, []any{orderID, "NF", 2, "NF/26-27/000002-again"}},
		{"a counter skipping ahead", `UPDATE invoice_series SET last_seq = last_seq + 2`, nil},
		{"a counter going back", `UPDATE invoice_series SET last_seq = 1`, nil},
		{"a counter changing series", `UPDATE invoice_series SET series = 'DLV', last_seq = last_seq + 1`, nil},
		{"a counter deleted", `DELETE FROM invoice_series`, nil},
		{"an invoice edited", `UPDATE invoices SET invoice_no = 'X'`, nil},
		{"an invoice deleted", `DELETE FROM invoices`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := d.Writer().ExecContext(ctx, tt.sql, tt.args...); err == nil {
				t.Errorf("%s was allowed", tt.name)
			}
		})
	}

	// The one change a counter may make is the one the allocator makes.
	if _, err := d.Writer().ExecContext(ctx, `UPDATE invoice_series SET last_seq = last_seq + 1`); err != nil {
		t.Errorf("advancing a counter by one was refused: %v", err)
	}
}

func TestRecordInvoiceRefusesAnotherVenuesNumber(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedTradingVenue(t, d)
	orderID := openTestOrder(t, d, "ord_ELSEWHERE", untaxed)

	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		in := invoice.Issued{Key: invoice.Key{VenueID: 2, Series: "NF", FYStart: 2026}, Seq: 1, InvoiceNo: "NF/1"}
		return recordInvoice(ctx, q, 1, orderID, in, at)
	})
	if err == nil {
		t.Error("an invoice allocated for venue 2 was recorded against an order at venue 1")
	}
}

// insertVenue adds a bare venue with a chosen ID.
func insertVenue(t *testing.T, d *DB, id int64) {
	t.Helper()
	_, err := d.Writer().ExecContext(context.Background(),
		`INSERT INTO venues (id, public_id, name, currency, timezone, created_at, updated_at)
		 VALUES (?, ?, 'Venue', 'INR', 'Asia/Kolkata', '2026-09-18 00:00:00+00:00', '2026-09-18 00:00:00+00:00')`,
		id, "ven_V"+string(rune('A'+id%26))+string(rune('A'+id/26%26)))
	if err != nil {
		t.Fatalf("inserting venue %d: %v", id, err)
	}
	openTestShift(t, d, id)
}

// openTestShift opens a shift at the venue: an order is refused without
// one (ADR-094).
func openTestShift(t *testing.T, d *DB, venueID int64) {
	t.Helper()
	_, err := d.Writer().ExecContext(context.Background(),
		`INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at)
		 VALUES (?, ?, 'open', '2026-09-18', 1, '2026-09-18 00:00:00+00:00')`,
		fmt.Sprintf("shf_T%07d", venueID), venueID)
	if err != nil {
		t.Fatalf("opening a shift at venue %d: %v", venueID, err)
	}
}

// seedTradingVenue is seedVenue with a shift open, for tests that write
// orders.
func seedTradingVenue(t *testing.T, d *DB) {
	t.Helper()
	seedVenue(t, d)
	openTestShift(t, d, 1)
}
