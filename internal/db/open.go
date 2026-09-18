package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	// The pure Go SQLite driver. No cgo, so the binary cross-compiles with
	// CGO_ENABLED=0 and a venue machine needs no toolchain. See ADR-036.
	_ "modernc.org/sqlite"
)

// FileName is the database file inside the data directory.
const FileName = "nofiltr.db"

// driverName is what modernc.org/sqlite registers itself as.
const driverName = "sqlite"

// Connection settings, from docs/03-conventions.md.
//
// WAL so a reader never blocks the writer during service. busy_timeout so a
// locked database waits rather than failing a sale. synchronous=NORMAL, which
// under WAL survives a power cut without an fsync on every commit, and a venue
// PC loses power more often than anyone would like. foreign_keys because an
// order line with no order is a wrong bill.
const (
	journalMode = "WAL"
	busyTimeout = 5000
	synchronous = "NORMAL"
)

// defaultProbeInterval is how often the background health check runs. Fifteen
// seconds is often enough that an operator watching /api/v1/health sees a
// failure while it is still the thing they are looking at, and rare enough to
// be invisible on a spinning disk.
const defaultProbeInterval = 15 * time.Second

// ErrClosed is returned by the accessors once Close has run.
var ErrClosed = errors.New("database is closed")

// Options configure Open. Only Path is required.
type Options struct {
	// Path is the database file. Its directory is created if missing.
	Path string

	// Log receives the connection and health lines. Nil discards them, which
	// is what a test wants.
	Log *slog.Logger

	// ReadPoolSize is how many concurrent readers are allowed. Zero picks a
	// size from the machine.
	ReadPoolSize int

	// ProbeInterval is how often health is checked in the background. Zero is
	// defaultProbeInterval.
	ProbeInterval time.Duration
}

// DB owns the connections to one SQLite database, and the goroutine that
// watches whether it is still answering.
//
// There are two pools because SQLite has one writer. The writer is capped at a
// single connection so writes queue in Go, where the wait is orderly, instead
// of in SQLite, where it is a busy error in the middle of a sale. The readers
// are a normal pool, opened read only so that a query which should not write
// cannot.
type DB struct {
	write *sql.DB
	read  *sql.DB
	path  string
	log   *slog.Logger

	// health is read by the HTTP handler and written by the prober.
	healthMu sync.RWMutex
	health   Health

	stopProber context.CancelFunc
	proberDone chan struct{}

	closeOnce sync.Once
	closeErr  error
}

// Open connects to the database at opts.Path, applies the connection settings
// and starts the health prober.
//
// It creates the file and its directory if they are missing, which is what a
// venue machine on its first boot needs, and it verifies the connection before
// returning, so a database that cannot be opened is a startup failure with a
// human watching rather than a failure during service.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.ReadPoolSize <= 0 {
		opts.ReadPoolSize = defaultReadPoolSize()
	}
	if opts.ProbeInterval <= 0 {
		opts.ProbeInterval = defaultProbeInterval
	}

	if err := checkPath(opts.Path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
		return nil, fmt.Errorf("db: creating the directory for %s: %w", opts.Path, err)
	}

	// The writer opens first: it is what creates the file, sets WAL on it and
	// leaves the -shm file the read only connections need.
	write, err := sql.Open(driverName, writeDSN(opts.Path))
	if err != nil {
		return nil, fmt.Errorf("db: opening %s for writing: %w", opts.Path, err)
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)
	write.SetConnMaxLifetime(0)

	if err := write.PingContext(ctx); err != nil {
		write.Close()
		return nil, fmt.Errorf("db: opening %s for writing: %w", opts.Path, err)
	}

	read, err := sql.Open(driverName, readDSN(opts.Path))
	if err != nil {
		write.Close()
		return nil, fmt.Errorf("db: opening %s for reading: %w", opts.Path, err)
	}
	read.SetMaxOpenConns(opts.ReadPoolSize)
	read.SetMaxIdleConns(opts.ReadPoolSize)
	read.SetConnMaxLifetime(0)

	if err := read.PingContext(ctx); err != nil {
		read.Close()
		write.Close()
		return nil, fmt.Errorf("db: opening %s for reading: %w", opts.Path, err)
	}

	d := &DB{
		write:      write,
		read:       read,
		path:       opts.Path,
		log:        opts.Log.With("db_path", opts.Path),
		proberDone: make(chan struct{}),
	}

	// Answer once before returning, so the first request to /api/v1/health
	// after startup reports something real rather than "unknown".
	d.probe(ctx)

	proberCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	d.stopProber = stop
	go d.proberLoop(proberCtx, opts.ProbeInterval)

	d.log.Info("database open",
		"journal_mode", journalMode,
		"busy_timeout_ms", busyTimeout,
		"synchronous", synchronous,
		"read_pool", opts.ReadPoolSize,
	)

	return d, nil
}

// Writer is the single-connection pool that every write goes through. Prefer
// Tx: a bare write outside a transaction is only correct when it is a single
// statement that stands alone.
func (d *DB) Writer() *sql.DB { return d.write }

// Reader is the pool for queries. It is open read only at the driver, so a
// statement that writes fails here instead of racing the writer.
func (d *DB) Reader() *sql.DB { return d.read }

// Path is the database file, for the log line that tells an operator which
// file the process actually opened.
func (d *DB) Path() string { return d.path }

// Close stops the prober and closes both pools. It is safe to call twice,
// because shutdown must not depend on being reached exactly once.
//
// The signature matches the shutdown stack in cmd/nofiltr: ctx is the whole
// shutdown budget, shared with every other subsystem.
func (d *DB) Close(ctx context.Context) error {
	d.closeOnce.Do(func() {
		if d.stopProber != nil {
			d.stopProber()
			<-d.proberDone
		}

		// Fold the WAL back into the database file. A venue machine is
		// switched off at the wall most nights, and a checkpointed file is one
		// a backup or a support engineer can copy on its own.
		if err := d.checkpoint(ctx); err != nil {
			d.log.Warn("could not checkpoint the write-ahead log", "error", err)
		}

		d.closeErr = errors.Join(
			wrapClose("read pool", d.read.Close()),
			wrapClose("writer", d.write.Close()),
		)
	})

	return d.closeErr
}

// checkpoint folds the write-ahead log back into the main database file.
func (d *DB) checkpoint(ctx context.Context) error {
	// A truncating checkpoint blocks until readers are done with the WAL. The
	// caller's shutdown budget bounds the wait; failing it is not fatal, since
	// SQLite recovers the WAL on the next open.
	if _, err := d.write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("db: checkpointing: %w", err)
	}
	return nil
}

func wrapClose(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("db: closing the %s: %w", what, err)
}

// checkPath rejects a path SQLite cannot be given.
func checkPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("db: no database path given: %w", os.ErrInvalid)
	}
	// The path goes into a DSN, where a question mark starts the driver
	// options. A file called a?b.db would silently become a file called a with
	// a nonsense option, so it is refused rather than mangled.
	if strings.ContainsAny(path, "?#") {
		return fmt.Errorf("db: path %q contains ? or #, which the driver reads as options: %w", path, os.ErrInvalid)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("db: path %q is a directory: %w", path, os.ErrInvalid)
	}
	return nil
}

// writeDSN is the connection string for the writer.
//
// The settings live in the DSN rather than in a PRAGMA statement after Open
// because all of them except journal_mode are per connection, and database/sql
// opens connections whenever the pool decides to. A pragma run once would
// protect the first connection and nothing else.
//
// _txlock=immediate makes every transaction take the write lock as it begins,
// instead of discovering on its first write that somebody else has it. That
// turns a possible mid-transaction failure into a wait the busy timeout
// already covers.
func writeDSN(path string) string {
	return "file:" + path +
		"?_pragma=journal_mode(" + journalMode + ")" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(" + fmt.Sprint(busyTimeout) + ")" +
		"&_pragma=synchronous(" + synchronous + ")" +
		"&_txlock=immediate"
}

// readDSN is the connection string for the read pool.
//
// mode=ro is the guard that makes two pools worth having: a query that writes
// fails in SQLite rather than depending on every caller remembering which
// handle they were given. journal_mode is left out because it is stored in the
// file and a read only connection cannot set it anyway.
func readDSN(path string) string {
	return "file:" + path +
		"?mode=ro" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(" + fmt.Sprint(busyTimeout) + ")" +
		"&_pragma=synchronous(" + synchronous + ")"
}

// defaultReadPoolSize picks a pool from the machine. A venue PC has two or
// four cores and a handful of terminals; more connections than cores buys
// nothing against a local file.
func defaultReadPoolSize() int {
	return min(max(runtime.NumCPU(), 2), 8)
}
