// Package restore is what stands between a backup and a data directory: the
// checks that a restore never writes over a venue, and the two steps by
// which a server that is running takes a restored database (ADR-333). It is
// staged beside the database the server has open, and adopted when mez next
// starts, before anything opens it.
//
// `mez restore`, the command, uses the checks. The wizard's "I had mez
// before" uses all of it.
//
// Imports: internal/backup, which does the restoring, and internal/db for
// the database's name and its driver.
package restore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrVenueThere is a restore asked to go into a data directory that already
// holds a venue.
var ErrVenueThere = errors.New("there is already a venue in that data directory")

// ErrStillRunning is `mez restore` into a data directory mez is running on.
var ErrStillRunning = errors.New("mez is running on that data directory")

// EmptyInstall reports whether the database in the way is a new install's:
// one mez made as it first started, with no venue in it. No database at
// all is not in the way and is false; one with a venue is ErrVenueThere.
//
// A new computer always has one. The installer starts the service, the
// service makes its database, and an owner who has come to restore has by
// then been shown a setup wizard they must not finish. Refusing them for a
// database with nothing in it would be refusing everyone.
func EmptyInstall(ctx context.Context, dbPath string) (bool, error) {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("restore: %w", err)
	}

	// Read only, and nothing else. This may be a real venue's database, at
	// the wrong -data-dir or under an older mez still running: to bring
	// its schema up to this binary's in order to refuse to touch it would
	// be to touch it.
	probe, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return false, fmt.Errorf("restore: there is a database at %s that cannot be opened to see whether a venue is in it: %w", dbPath, err)
	}
	defer probe.Close()

	var name string
	err = probe.QueryRowContext(ctx, "SELECT name FROM venues ORDER BY id LIMIT 1").Scan(&name)
	switch {
	case err == nil:
		return false, fmt.Errorf("restore: %w: %s holds %s. Move it aside first, or restore into another with -data-dir. A restore never writes over a venue", ErrVenueThere, dbPath, name)
	case errors.Is(err, sql.ErrNoRows):
		return true, nil
	case strings.Contains(err.Error(), "no such table"):
		// Made, and never migrated: mez was stopped as it first started.
		return true, nil
	default:
		// Not known to be empty, so not treated as empty.
		return false, fmt.Errorf("restore: there is a database at %s that cannot be read to see whether a venue is in it: %w. Move it aside first, or restore into another with -data-dir", dbPath, err)
	}
}

// NotRunning refuses if anything else has the database open.
//
// On Windows a running server holds the file and the move fails by
// itself. Everywhere else a file can be moved out from under a program
// that has it open: the server would go on with the old database, unseen,
// and open the restored one through log files that are not its own.
// SQLite will give the whole database to one connection only when no
// other has it, so asking for that is asking whether anyone does.
func NotRunning(ctx context.Context, dbPath string) error {
	held, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(0)&_pragma=locking_mode(EXCLUSIVE)")
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	defer held.Close()
	held.SetMaxOpenConns(1)
	// A write is what takes the lock. This one changes nothing: the
	// database is about to be moved aside, and is put back as it was.
	if _, err := held.ExecContext(ctx, "BEGIN IMMEDIATE; ROLLBACK"); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "locked") || strings.Contains(strings.ToLower(err.Error()), "busy") {
			return fmt.Errorf("restore: %w: %s. Stop it first (`mez service stop`, or `systemctl stop mez`), then run this again", ErrStillRunning, filepath.Dir(dbPath))
		}
		return fmt.Errorf("restore: checking that mez is not running on %s: %w", filepath.Dir(dbPath), err)
	}
	return nil
}

// MoveAside renames a new install's empty database out of the way, and
// clears what was that database's and would be read as the restored one's:
// its log files, and what Litestream remembered of backing it up.
func MoveAside(dbPath string, now time.Time) (string, error) {
	aside := dbPath + ".empty-" + now.UTC().Format("20060102T150405Z")
	if err := os.Rename(dbPath, aside); err != nil {
		return "", fmt.Errorf("restore: moving the empty database aside: %w. Is mez still running on this directory? Stop it first", err)
	}
	if err := clearBeside(dbPath); err != nil {
		return "", err
	}
	return aside, nil
}

// clearBeside removes what lies beside a database and is about the one that
// was last there: its log files, and what Litestream remembered of backing
// it up. It is safe to do twice, and is done again wherever a database is
// about to be put where none is: a move aside that was cut short after the
// rename leaves all of it behind.
func clearBeside(dbPath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("restore: clearing %s: %w", dbPath+suffix, err)
		}
	}
	// Litestream's working folder and the file that says which run of the
	// backup to carry on in: both are about the database just moved. Left,
	// a backup turned on again here would carry that run on with another
	// database's history, and restore short. The list of runs this
	// directory began is kept, so that they are still tidied away.
	dir, file := filepath.Split(dbPath)
	working := filepath.Join(dir, "."+file+"-litestream")
	if err := os.RemoveAll(working); err != nil {
		return fmt.Errorf("restore: clearing %s: %w", working, err)
	}
	if err := os.Remove(working + "-for"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("restore: clearing %s: %w", working+"-for", err)
	}
	return nil
}
