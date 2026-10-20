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

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/config"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/logging"
)

// resetPINOptions is everything `mez reset-pin` takes.
type resetPINOptions struct {
	DataDir string
	Staff   string
}

// runResetPIN is `mez reset-pin`: a new PIN for one person, drawn here and
// shown once, at the machine itself (ADR-304).
//
// In the back office a PIN is reset only by someone who holds more than the
// person does, and an owner's only by another owner. A venue with one owner
// who has forgotten theirs has nobody to ask and no screen to ask from, and
// `mez unlock` clears a lock without giving them anything to type. Being
// sat at the machine is the trust `mez pair` and `mez unlock` rely on, so
// the same door is open here.
//
// The PIN is drawn, not taken as a flag: a PIN typed on a command line is in
// the shell's history for whoever looks. It clears the person's lock, since
// someone who forgot a PIN has usually been locked trying, and signs them
// out everywhere, as a reset in the back office does. Without -staff it
// lists who there is, because the person who needs this cannot open the
// Staff screen to find their id.
func runResetPIN(args []string, stdout io.Writer) error {
	opts, err := parseResetPINFlags(args, stdout)
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
			return fmt.Errorf("reset-pin: there is no database at %s: run `mez setup` first, or pass -data-dir", path)
		}
		return fmt.Errorf("reset-pin: %w", err)
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

	// Listing is the first step and not a mistake: it changes nothing.
	if opts.Staff == "" {
		return writeStaffList(ctx, stdout, database)
	}

	pin, err := drawPIN()
	if err != nil {
		return err
	}
	name, err := resetStaffPIN(ctx, database, opts.Staff, pin, time.Now())
	if err != nil {
		return err
	}
	// The record of it: the staff tables keep no history of their own.
	log.Warn("a PIN was reset at the machine", "staff_id", opts.Staff)
	fmt.Fprintf(stdout, "\n  %s (%s) has a new PIN, shown here once:\n\n      %s\n\n", name, opts.Staff, pin)
	fmt.Fprintf(stdout, "  They are signed out everywhere and any lock on them is cleared.\n")
	fmt.Fprintf(stdout, "  Sign in with it, then change it under Staff in the back office.\n")
	return nil
}

// parseResetPINFlags reads the command line for reset-pin.
func parseResetPINFlags(args []string, out io.Writer) (resetPINOptions, error) {
	var o resetPINOptions
	set := flag.NewFlagSet("mez reset-pin", flag.ContinueOnError)
	set.SetOutput(out)
	set.StringVar(&o.DataDir, "data-dir", "", "directory holding the database (default: as the server resolves it)")
	set.StringVar(&o.Staff, "staff", "", "the person's id, stf_...; leave it out to list everyone's")
	if err := set.Parse(args); err != nil {
		return resetPINOptions{}, err
	}
	if set.NArg() > 0 {
		return resetPINOptions{}, fmt.Errorf("reset-pin: unexpected argument %q", set.Arg(0))
	}
	return o, nil
}

// writeStaffList prints who works at the venue with the id each is reset
// by. It is what the lock screen shows of them and their id: no secret.
func writeStaffList(ctx context.Context, w io.Writer, database *db.DB) error {
	q := database.Queries()
	venues, err := q.ListVenues(ctx)
	if err != nil {
		return fmt.Errorf("reset-pin: reading the venue: %w", err)
	}
	if len(venues) == 0 {
		return ErrNoVenue
	}
	staff, err := q.StaffForVenue(ctx, venues[0].ID)
	if err != nil {
		return fmt.Errorf("reset-pin: reading who works here: %w", err)
	}
	fmt.Fprintf(w, "\n  Who works at %s:\n\n", venues[0].Name)
	for _, s := range staff {
		note := ""
		switch {
		case !s.Active:
			note = "  (no longer works here)"
		case s.LockedAt.Valid:
			note = "  (locked)"
		}
		fmt.Fprintf(w, "    %-14s %-20s %s%s\n", s.PublicID, s.Name, s.RoleName, note)
	}
	fmt.Fprintf(w, "\n  For a new PIN:  mez reset-pin -staff stf_...\n\n")
	return nil
}

// resetStaffPIN gives one person a PIN, clears their hard lock and the
// lockouts that led to it, and ends their sessions, in one transaction. It
// answers their name. The PIN must be long enough for the most a role may
// do (ADR-260), so it is right for whatever role they have.
func resetStaffPIN(ctx context.Context, database *db.DB, publicID, pin string, now time.Time) (string, error) {
	hash, err := auth.HashPIN(pin)
	if err != nil {
		return "", fmt.Errorf("reset-pin: %w", err)
	}
	var name string
	err = database.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := q.StaffByPublicID(ctx, publicID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("reset-pin: no person has the id %q: run `mez reset-pin` with no -staff to list them", publicID)
		}
		if err != nil {
			return fmt.Errorf("reset-pin: reading %s: %w", publicID, err)
		}
		name = row.Name
		// Not even from the machine: with a PIN the cloud's owner would be
		// a way in that the account knows nothing of (ADR-323).
		if row.Cloud {
			return fmt.Errorf("reset-pin: %s signs in from mez Cloud and has no PIN: add them under Staff as a person of their own to give them one", row.Name)
		}
		if !row.Active {
			return fmt.Errorf("reset-pin: %s no longer works here: bring them back under Staff first", row.Name)
		}
		// Long enough for any role, whatever theirs is (ADR-260): the role is
		// not read, so a PIN drawn here can never be too short for it.
		if len(pin) < auth.MinPowerfulPINLength {
			return fmt.Errorf("reset-pin: a PIN set here is %d digits or more", auth.MinPowerfulPINLength)
		}

		at := now.UTC()
		stamp := sql.NullTime{Time: at, Valid: true}
		if _, err := q.SetStaffPIN(ctx, gen.SetStaffPINParams{PinHash: hash, Now: at, VenueID: row.VenueID, ID: row.ID}); err != nil {
			return fmt.Errorf("reset-pin: setting %s's PIN: %w", publicID, err)
		}
		if _, err := q.ClearStaffHardLock(ctx, gen.ClearStaffHardLockParams{Now: at, VenueID: row.VenueID, ID: row.ID}); err != nil {
			return fmt.Errorf("reset-pin: clearing %s's lock: %w", publicID, err)
		}
		if err := q.ClearStaffLockouts(ctx, gen.ClearStaffLockoutsParams{Now: stamp, StaffID: row.ID}); err != nil {
			return fmt.Errorf("reset-pin: clearing %s's lockouts: %w", publicID, err)
		}
		if err := q.EndSessionsOfStaff(ctx, gen.EndSessionsOfStaffParams{Now: stamp, StaffID: row.ID}); err != nil {
			return fmt.Errorf("reset-pin: signing %s out: %w", publicID, err)
		}
		return nil
	})
	return name, err
}
