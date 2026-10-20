package invoicetest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mez-pos/mez/internal/domain/invoice"
)

// Conformance runs the invoice.Allocator contract against an implementation.
// begin must start a fresh write transaction on a shared store, and must be
// safe to call from several goroutines. Each case uses its own venue, so the
// cases do not disturb one another.
func Conformance(t *testing.T, begin func() Tx) {
	t.Helper()
	ctx := context.Background()
	key := func(venue int64) invoice.Key { return invoice.Key{VenueID: venue, Series: "NF", FYStart: 2026} }

	t.Run("the first number is 1", func(t *testing.T) {
		tx := begin()
		expect(t, next(t, tx, key(101)), 1)
		mustEnd(t, tx.Commit())
	})

	t.Run("numbers follow on, within and across transactions", func(t *testing.T) {
		tx := begin()
		for want := int64(1); want <= 3; want++ {
			expect(t, next(t, tx, key(102)), want)
		}
		mustEnd(t, tx.Commit())
		tx = begin()
		expect(t, next(t, tx, key(102)), 4)
		mustEnd(t, tx.Commit())
	})

	t.Run("a rollback gives its numbers back", func(t *testing.T) {
		tx := begin()
		expect(t, next(t, tx, key(103)), 1)
		expect(t, next(t, tx, key(103)), 2)
		mustEnd(t, tx.Rollback())
		tx = begin()
		expect(t, next(t, tx, key(103)), 1)
		mustEnd(t, tx.Commit())
	})

	t.Run("venues, series and years never share a counter", func(t *testing.T) {
		keys := []invoice.Key{
			key(104),
			{VenueID: 105, Series: "NF", FYStart: 2026},
			{VenueID: 104, Series: "DLV", FYStart: 2026},
			{VenueID: 104, Series: "NF", FYStart: 2027},
		}
		tx := begin()
		expect(t, next(t, tx, keys[0]), 1)
		expect(t, next(t, tx, keys[0]), 2)
		for _, k := range keys[1:] {
			expect(t, next(t, tx, k), 1)
		}
		mustEnd(t, tx.Commit())
	})

	t.Run("an invalid key is refused", func(t *testing.T) {
		tx := begin()
		defer func() { mustEnd(t, tx.Rollback()) }()
		for _, k := range []invoice.Key{{}, {VenueID: 106, FYStart: 2026}, {VenueID: 106, Series: "NF"}} {
			if _, err := tx.Next(ctx, k); !errors.Is(err, invoice.ErrInvalidKey) {
				t.Errorf("Next(%+v) err = %v, want ErrInvalidKey", k, err)
			}
		}
	})

	t.Run("a cancelled context is refused", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		tx := begin()
		defer func() { mustEnd(t, tx.Rollback()) }()
		if _, err := tx.Next(cancelled, key(107)); err == nil {
			t.Error("Next with a cancelled context allocated a number")
		}
	})

	t.Run("concurrent issuers leave no gap and no duplicate", func(t *testing.T) {
		concurrent(t, begin, key(108))
	})
}

// concurrent runs several goroutines issuing into one key, rolling back every
// third transaction, and audits what was committed.
func concurrent(t *testing.T, begin func() Tx, k invoice.Key) {
	const workers, each = 8, 30
	var (
		mu        sync.Mutex
		committed []invoice.Issued
		wg        sync.WaitGroup
		errs      = make(chan error, workers*each)
	)
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range each {
				tx := begin()
				seq, err := tx.Next(context.Background(), k)
				if err != nil {
					errs <- err
					_ = tx.Rollback()
					continue
				}
				if (w+i)%3 == 0 {
					errs <- tx.Rollback()
					continue
				}
				errs <- tx.Commit()
				mu.Lock()
				committed = append(committed, invoice.Issued{Key: k, Seq: seq})
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a transaction failed: %v", err)
		}
	}
	if err := invoice.Audit(committed); err != nil {
		t.Fatalf("%d committed invoices: %v", len(committed), err)
	}
	if len(committed) == 0 {
		t.Fatal("nothing was committed")
	}
}

func next(t *testing.T, tx Tx, k invoice.Key) int64 {
	t.Helper()
	n, err := tx.Next(context.Background(), k)
	if err != nil {
		t.Fatalf("Next(%+v): %v", k, err)
	}
	return n
}

func expect(t *testing.T, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("allocated %d, want %d", got, want)
	}
}

func mustEnd(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("ending the transaction: %v", err)
	}
}
