// Package invoicetest holds the invoice.Allocator contract as a runnable
// suite, and a reference allocator that meets it.
//
// The database layer's allocator must pass Conformance. The reference
// allocator, Memory, is how the domain's own tests exercise Issue and the
// gapless property without a database, and how Conformance itself is known
// to be satisfiable.
package invoicetest

import (
	"context"
	"errors"
	"sync"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/invoice"
)

// ErrTxDone means a transaction was used after Commit or Rollback.
var ErrTxDone = errors.New("invoicetest: transaction already finished")

// Tx is an allocator bound to one transaction, which the test can end either
// way. The database layer adapts its write transaction to this.
type Tx interface {
	invoice.Allocator
	Commit() error
	Rollback() error
}

// Memory is a reference allocator that behaves like the real one: one writer
// at a time (SQLite has one), counters that only move on commit, and numbers
// that come back on rollback.
type Memory struct {
	writer    sync.Mutex // held from Begin to Commit or Rollback
	mu        sync.Mutex // guards committed
	committed map[invoice.Key]int64
}

// NewMemory returns an empty reference allocator.
func NewMemory() *Memory {
	return &Memory{committed: map[invoice.Key]int64{}}
}

// Begin starts a write transaction, waiting for any other to finish, as
// SQLite's single writer does.
func (m *Memory) Begin() Tx {
	m.writer.Lock()
	return &memTx{m: m, pending: map[invoice.Key]int64{}}
}

// Committed is the last committed number for a key, 0 if none.
func (m *Memory) Committed(k invoice.Key) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.committed[k]
}

type memTx struct {
	m       *Memory
	pending map[invoice.Key]int64 // numbers allocated in this transaction
	done    bool
}

func (tx *memTx) Next(ctx context.Context, key invoice.Key) (int64, error) {
	if tx.done {
		return 0, ErrTxDone
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := key.Validate(); err != nil {
		return 0, err
	}
	tx.pending[key]++
	return tx.m.Committed(key) + tx.pending[key], nil
}

func (tx *memTx) Commit() error {
	if tx.done {
		return ErrTxDone
	}
	tx.m.mu.Lock()
	for k, n := range tx.pending {
		tx.m.committed[k] += n
	}
	tx.m.mu.Unlock()
	tx.finish()
	return nil
}

func (tx *memTx) Rollback() error {
	if tx.done {
		return ErrTxDone
	}
	tx.finish()
	return nil
}

func (tx *memTx) finish() {
	tx.done = true
	tx.m.writer.Unlock()
}
