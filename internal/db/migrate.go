package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

// migrationDir is the directory inside migrationFS. goose is given the
// embedded filesystem, not the disk, so a binary carries its own schema and a
// venue machine needs no files beside it.
const migrationDir = "migrations"

// migrationFS holds the schema. The pattern is deliberately strict: an embed
// that matches nothing still compiles, and a binary with no migrations creates
// an empty database and fails on the first query instead of at startup.
//
//go:embed migrations/*.sql
var migrationFS embed.FS

// gooseDialect is what goose calls SQLite. It is not the driver name.
const gooseDialect = "sqlite3"

// Migrate applies every migration that has not run yet, in order.
//
// It runs on startup, every startup, which is why it has to be idempotent: a
// venue machine restarts several times a day and nobody is going to run a
// migration tool by hand at six in the morning. A failure here is a failure to
// start, on purpose. A server that runs against a half-migrated database
// writes rows nothing can read back.
//
// It uses the writer, which is a single connection, so migrations serialise
// against everything else by construction.
func (d *DB) Migrate(ctx context.Context) error {
	provider, err := d.migrationProvider()
	if err != nil {
		return err
	}

	before, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("db: reading the schema version: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("db: applying migrations: %w", err)
	}

	after, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("db: reading the schema version: %w", err)
	}

	// One line either way. "Already at version 3" is the line an operator
	// needs when they are looking at a machine that will not start, and its
	// absence is itself information.
	if len(results) == 0 {
		d.log.Info("database schema is current", "version", after)
		return nil
	}

	for _, result := range results {
		d.log.Info("migration applied",
			"version", result.Source.Version,
			"name", result.Source.Path,
			"took", result.Duration,
		)
	}
	d.log.Info("database schema migrated", "from", before, "to", after, "applied", len(results))

	return nil
}

// Version is the schema version the database is at, for the startup log and
// for a test that wants to prove a second run changed nothing.
func (d *DB) Version(ctx context.Context) (int64, error) {
	provider, err := d.migrationProvider()
	if err != nil {
		return 0, err
	}

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("db: reading the schema version: %w", err)
	}
	return version, nil
}

// migrationProvider builds a goose provider over the embedded migrations and
// the writer connection.
//
// A provider is built per call rather than held on DB because migrations run
// once at startup and it keeps the cost of a failed one at zero: there is
// nothing half-initialised left behind.
func (d *DB) migrationProvider() (*goose.Provider, error) {
	// goose reads from the root of the filesystem it is given, so the embedded
	// tree is rooted at the migrations directory before it is handed over.
	migrations, err := fs.Sub(migrationFS, migrationDir)
	if err != nil {
		return nil, fmt.Errorf("db: reading embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.Dialect(gooseDialect),
		d.write,
		migrations,
		goose.WithLogger(gooseLogger{log: d.log}),
	)
	if err != nil {
		return nil, fmt.Errorf("db: preparing migrations: %w", err)
	}
	return provider, nil
}

// gooseLogger sends goose's own output into slog, so a migration line looks
// like every other line in the log rather than arriving on stderr unformatted.
//
// docs/03-conventions.md bans fmt.Println and expects structured logs; a
// library printing straight to stderr is the same problem arriving from
// outside.
type gooseLogger struct {
	log *slog.Logger
}

func (g gooseLogger) Printf(format string, v ...any) {
	g.log.Info("goose: " + fmt.Sprintf(format, v...))
}

// Fatalf must not exit. goose calls it on errors it also returns, and a
// library calling os.Exit inside a server is how a venue machine stops mid
// service with nothing in the log explaining it.
func (g gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error("goose: " + fmt.Sprintf(format, v...))
}
