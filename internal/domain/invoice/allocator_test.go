package invoice_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/invoice"
	"github.com/byjoellobo/mez/internal/domain/invoice/invoicetest"
)

var ist = time.FixedZone("IST", 5*3600+1800)

func gstSeries(t *testing.T, code string) invoice.Series {
	t.Helper()
	s, err := invoice.NewSeries(code, code, invoice.IndiaFY, "{prefix}/{fy:short}/{seq:06d}", 16)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The reference allocator must meet the contract, or Conformance could not
// be trusted to judge the real one.
func TestMemoryAllocatorConforms(t *testing.T) {
	m := invoicetest.NewMemory()
	invoicetest.Conformance(t, m.Begin)
}

func TestMemoryTransactionsEndOnce(t *testing.T) {
	m := invoicetest.NewMemory()
	for _, end := range []func(invoicetest.Tx) error{invoicetest.Tx.Commit, invoicetest.Tx.Rollback} {
		tx := m.Begin()
		if err := end(tx); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); !errors.Is(err, invoicetest.ErrTxDone) {
			t.Errorf("Commit after end: err = %v", err)
		}
		if err := tx.Rollback(); !errors.Is(err, invoicetest.ErrTxDone) {
			t.Errorf("Rollback after end: err = %v", err)
		}
		if _, err := tx.Next(context.Background(), invoice.Key{VenueID: 1, Series: "NF", FYStart: 2026}); !errors.Is(err, invoicetest.ErrTxDone) {
			t.Errorf("Next after end: err = %v", err)
		}
	}
}

func TestIssueAcrossTheYearEnd(t *testing.T) {
	m := invoicetest.NewMemory()
	s := gstSeries(t, "NF")
	ctx := context.Background()

	sales := []struct {
		at   time.Time
		want string
	}{
		{time.Date(2027, 3, 31, 21, 0, 0, 0, ist), "NF/26-27/000001"},
		{time.Date(2027, 3, 31, 23, 59, 0, 0, ist), "NF/26-27/000002"},
		{time.Date(2027, 3, 31, 18, 45, 0, 0, time.UTC), "NF/27-28/000001"}, // 00:15 IST, 1 April
		{time.Date(2027, 4, 1, 9, 0, 0, 0, ist), "NF/27-28/000002"},
	}
	var issued []invoice.Issued
	for _, sale := range sales {
		tx := m.Begin()
		got, err := invoice.Issue(ctx, tx, s, 1, sale.at, ist)
		if err != nil {
			t.Fatalf("Issue at %v: %v", sale.at, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if got.InvoiceNo != sale.want {
			t.Errorf("sale at %v got %q, want %q", sale.at, got.InvoiceNo, sale.want)
		}
		issued = append(issued, got)
	}
	if err := invoice.Audit(issued); err != nil {
		t.Errorf("audit: %v", err)
	}
}

// stubAllocator returns what it is told to, for the error paths of Issue.
type stubAllocator struct {
	seq int64
	err error
}

func (s stubAllocator) Next(context.Context, invoice.Key) (int64, error) { return s.seq, s.err }

func TestIssueRefuses(t *testing.T) {
	ctx := context.Background()
	s := gstSeries(t, "NFX") // NFX/26-27/NNNNNN is exactly 16
	at := time.Date(2026, 9, 18, 12, 0, 0, 0, ist)
	boom := errors.New("disk full")

	tests := []struct {
		name    string
		a       invoice.Allocator
		venue   int64
		loc     *time.Location
		wantErr error
	}{
		{"no time zone", stubAllocator{seq: 1}, 1, nil, invoice.ErrNoLocation},
		{"no venue", stubAllocator{seq: 1}, 0, ist, invoice.ErrInvalidKey},
		{"the allocator fails", stubAllocator{err: boom}, 1, ist, boom},
		{"the allocator returns zero", stubAllocator{seq: 0}, 1, ist, invoice.ErrInvalidSeq},
		{"the number outgrows the cap", stubAllocator{seq: 1000000}, 1, ist, invoice.ErrTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := invoice.Issue(ctx, tt.a, s, tt.venue, at, tt.loc); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// naive hands out numbers the way an autoincrement or a sequence does: the
// counter moves on every call and never moves back. It is what "allocate
// outside the sale's transaction" amounts to.
type naive struct{ counters map[invoice.Key]int64 }

func (n *naive) Begin() invoicetest.Tx { return naiveTx{n} }

type naiveTx struct{ n *naive }

func (tx naiveTx) Next(_ context.Context, k invoice.Key) (int64, error) {
	tx.n.counters[k]++
	return tx.n.counters[k], nil
}
func (naiveTx) Commit() error   { return nil }
func (naiveTx) Rollback() error { return nil }

// simulate runs a random script of sales against an allocator: each
// transaction issues one to three invoices across two venues and two series,
// at times straddling the India year end, and a third of them fail to commit.
// It returns what was committed.
func simulate(t *testing.T, seed uint64, begin func() invoicetest.Tx) []invoice.Issued {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	series := []invoice.Series{gstSeries(t, "NF"), gstSeries(t, "DLV")}
	yearEnd := time.Date(2027, 4, 1, 0, 0, 0, 0, ist)

	var committed []invoice.Issued
	for range 1 + rng.IntN(40) {
		tx := begin()
		var pending []invoice.Issued
		for range 1 + rng.IntN(3) {
			at := yearEnd.Add(time.Duration(rng.IntN(48*60)-24*60) * time.Minute)
			s := series[rng.IntN(len(series))]
			in, err := invoice.Issue(context.Background(), tx, s, int64(1+rng.IntN(2)), at, ist)
			if err != nil {
				t.Fatalf("seed %d: Issue: %v", seed, err)
			}
			pending = append(pending, in)
		}
		if rng.IntN(3) == 0 { // the sale failed: the print, the network, the power
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		committed = append(committed, pending...)
	}
	return committed
}

// TestGaplessProperty is the guarantee as a property: for any interleaving
// of committed and rolled-back sales, across venues, series and a year end,
// the committed numbers are exactly 1..n per series and year, and no two
// committed invoices share a number.
func TestGaplessProperty(t *testing.T) {
	for seed := range uint64(300) {
		m := invoicetest.NewMemory()
		committed := simulate(t, seed, m.Begin)
		if err := invoice.Audit(committed); err != nil {
			t.Fatalf("seed %d, %d invoices: %v", seed, len(committed), err)
		}
		type venueNumber struct {
			venue int64
			no    string
		}
		seen := map[venueNumber]bool{}
		for _, in := range committed {
			id := venueNumber{in.Key.VenueID, in.InvoiceNo}
			if seen[id] {
				t.Fatalf("seed %d: %q issued twice at venue %d", seed, in.InvoiceNo, in.Key.VenueID)
			}
			seen[id] = true
		}
	}
}

// TestTheNaiveAllocatorFailsTheProperty proves the property has teeth: an
// allocator that does not give numbers back on rollback leaves gaps, and
// Audit finds them.
func TestTheNaiveAllocatorFailsTheProperty(t *testing.T) {
	failures := 0
	for seed := range uint64(300) {
		n := &naive{counters: map[invoice.Key]int64{}}
		err := invoice.Audit(simulate(t, seed, n.Begin))
		if err != nil {
			if !errors.Is(err, invoice.ErrGap) {
				t.Fatalf("seed %d: err = %v, want ErrGap", seed, err)
			}
			failures++
		}
	}
	if failures == 0 {
		t.Fatal("the naive allocator passed every run: the property is not testing rollback")
	}
	t.Logf("the naive allocator left a gap in %d of 300 runs", failures)
}
