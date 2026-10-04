package db

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// staleConns writes a marker row and reports how many of the read pool's
// connections cannot see it.
func staleConns(t *testing.T, d *DB, pool int, marker int) int {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Writer().ExecContext(ctx, "INSERT INTO t (x) VALUES (?)", marker); err != nil {
		t.Fatalf("insert marker: %v", err)
	}
	// Hold every connection at once, so each is looked at.
	conns := make([]*sql.Conn, 0, pool)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	stale := 0
	for i := range pool {
		c, err := d.Reader().Conn(ctx)
		if err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		conns = append(conns, c)
		var seen int
		if err := c.QueryRowContext(ctx, "SELECT count(*) FROM t WHERE x = ?", marker).Scan(&seen); err != nil {
			t.Logf("conn %d: reading the marker: %v", i, err)
			stale++
			continue
		}
		if seen != 1 {
			stale++
		}
	}
	return stale
}

// A read that is cancelled must not leave its connection looking at the
// database as it was. A browser that reloads cancels its requests mid-query;
// with modernc.org/sqlite v1.39.0 that left a pooled read connection on its
// old WAL snapshot, silently, so a till could not find an order it had just
// opened or an invoice it had just issued until the server was restarted. The
// driver fixed it in v1.41.0 (ADR-157). This fails within a round or two on
// the old driver.
func TestCancelledReadsLeaveNoReadConnectionOnAnOldSnapshot(t *testing.T) {
	const pool = 4
	d, err := Open(context.Background(), Options{
		Path:         filepath.Join(t.TempDir(), FileName),
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReadPoolSize: pool,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close(context.Background())
	bg := context.Background()
	if _, err := d.Writer().ExecContext(bg, "CREATE TABLE t (x INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 1; i <= 400; i++ {
		if _, err := d.Writer().ExecContext(bg, "INSERT INTO t (x) VALUES (?)", i); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	readTx := func(ctx context.Context) {
		tx, err := d.Reader().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return
		}
		defer func() { _ = tx.Rollback() }()
		for range 6 {
			rows, err := tx.QueryContext(ctx, "SELECT x FROM t ORDER BY x")
			if err != nil {
				return
			}
			for rows.Next() {
				var x int
				if rows.Scan(&x) != nil {
					break
				}
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return
			}
		}
		_ = tx.Commit()
	}
	plain := func(ctx context.Context) {
		rows, err := d.Reader().QueryContext(ctx, "SELECT x FROM t ORDER BY x")
		if err != nil {
			return
		}
		for rows.Next() {
			var x int
			if rows.Scan(&x) != nil {
				break
			}
		}
		_ = rows.Err()
		_ = rows.Close()
	}

	for round := 1; round <= 40; round++ {
		var wg sync.WaitGroup
		for g := range 16 {
			wg.Add(1)
			go func(seed int64) {
				defer wg.Done()
				r := rand.New(rand.NewSource(seed))
				for range 25 {
					ctx, cancel := context.WithTimeout(bg, time.Duration(r.Intn(1500))*time.Microsecond)
					if r.Intn(2) == 0 {
						readTx(ctx)
					} else {
						plain(ctx)
					}
					cancel()
				}
			}(int64(round*100 + g))
		}
		wg.Wait()
		if n := staleConns(t, d, pool, 100000+round); n > 0 {
			t.Fatalf("round %d: %d of %d read connections cannot see a row written after the cancelled reads", round, n, pool)
		}
	}
}
