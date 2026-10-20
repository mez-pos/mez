package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/logging"
)

// ErrNotLocked is `mez unlock` asked to clear a lock that is not there.
var ErrNotLocked = errors.New("that person is not locked")

// unlockOptions is everything `mez unlock` takes.
type unlockOptions struct {
	DataDir string
	Staff   string
}

// runUnlock is `mez unlock`: a hard lock cleared at the machine itself.
//
// A PIN that keeps failing locks the person until someone with staff.manage
// clears it in the back office (ADR-260). Anyone at a till can lock a named
// colleague that way, the owner included, and if every person who could
// clear a lock is locked there is no screen left to do it from. Being sat at
// the machine is the trust `mez pair` relies on, so the same door is open
// here, and no more of it: this clears one lock and changes nothing else.
func runUnlock(args []string, stdout io.Writer) error {
	opts, err := parseUnlockFlags(args, stdout)
	if err != nil {
		return err
	}

	var shared []string
	if opts.DataDir != "" {
		shared = append(shared, "-data-dir", opts.DataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, db.FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unlock: there is no database at %s: run `mez setup` first, or pass -data-dir", path)
		}
		return fmt.Errorf("unlock: %w", err)
	}

	ctx := context.Background()
	log := logging.Bootstrap()
	database, err := db.Open(ctx, db.Options{Path: path, Log: log})
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(context.Background()); err != nil {
			log.Warn("closing the database", "error", err)
		}
	}()
	if err := database.Migrate(ctx); err != nil {
		return err
	}

	name, err := unlockStaff(ctx, database, opts.Staff, time.Now())
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "  %s (%s) is unlocked and can sign in again.\n", name, opts.Staff)
	return nil
}

// parseUnlockFlags reads the command line for unlock.
func parseUnlockFlags(args []string, out io.Writer) (unlockOptions, error) {
	var o unlockOptions
	set := flag.NewFlagSet("mez unlock", flag.ContinueOnError)
	set.SetOutput(out)
	set.StringVar(&o.DataDir, "data-dir", "", "directory holding the database (default: as the server resolves it)")
	set.StringVar(&o.Staff, "staff", "", "the person's id as the back office shows it, stf_... (required)")
	if err := set.Parse(args); err != nil {
		return unlockOptions{}, err
	}
	if set.NArg() > 0 {
		return unlockOptions{}, fmt.Errorf("unlock: unexpected argument %q", set.Arg(0))
	}
	if o.Staff == "" {
		return unlockOptions{}, errors.New("unlock: -staff is required: the person's id, stf_..., from the People screen or the log")
	}
	return o, nil
}

// unlockStaff clears one person's hard lock and the lockouts that led to
// it, in one transaction, and answers their name. The sessions ended when
// the lock was set stay ended: the person signs in afresh.
func unlockStaff(ctx context.Context, database *db.DB, publicID string, now time.Time) (string, error) {
	var name string
	err := database.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := q.StaffByPublicID(ctx, publicID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("unlock: no person has the id %q", publicID)
		}
		if err != nil {
			return fmt.Errorf("unlock: reading %s: %w", publicID, err)
		}
		name = row.Name
		if !row.LockedAt.Valid {
			return fmt.Errorf("%w: %s", ErrNotLocked, row.Name)
		}
		at := now.UTC()
		if _, err := q.ClearStaffHardLock(ctx, gen.ClearStaffHardLockParams{Now: at, VenueID: row.VenueID, ID: row.ID}); err != nil {
			return fmt.Errorf("unlock: clearing %s's lock: %w", publicID, err)
		}
		if err := q.ClearStaffLockouts(ctx, gen.ClearStaffLockoutsParams{Now: sql.NullTime{Time: at, Valid: true}, StaffID: row.ID}); err != nil {
			return fmt.Errorf("unlock: clearing %s's lockouts: %w", publicID, err)
		}
		return nil
	})
	return name, err
}
