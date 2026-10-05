package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/logging"
	"github.com/byjoellobo/mez/internal/service/tablecodes"
	"github.com/byjoellobo/mez/web"
)

// runTableCodes is `mez table-codes`: every table on the floor with the code
// a guest's phone presents for it, making one for any table that has none.
//
// It is the way to a code until the back office prints them (phase 11,
// Task 6). Like pair, it is run at the machine itself, which is the trust
// setup already relies on. It changes no code a table already has: a
// sticker on a table goes on working.
func runTableCodes(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("mez table-codes", flag.ContinueOnError)
	set.SetOutput(stdout)
	dataDir := set.String("data-dir", "", "directory holding the database (default: as the server resolves it)")
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() > 0 {
		return fmt.Errorf("table-codes: unexpected argument %q", set.Arg(0))
	}

	var shared []string
	if *dataDir != "" {
		shared = append(shared, "-data-dir", *dataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, db.FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("table-codes: there is no database at %s: run `mez setup` first, or pass -data-dir", path)
		}
		return fmt.Errorf("table-codes: %w", err)
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

	codes, err := ensureTableCodes(ctx, database)
	if err != nil {
		return err
	}
	writeTableCodes(stdout, codes)
	return nil
}

// ensureTableCodes finds the venue and gives every table without a code one.
func ensureTableCodes(ctx context.Context, database *db.DB) ([]tablecodes.Coded, error) {
	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		return nil, fmt.Errorf("table-codes: reading the venue: %w", err)
	}
	if len(venues) == 0 {
		return nil, ErrNoVenue
	}
	svc, err := tablecodes.New(database, nil, nil)
	if err != nil {
		return nil, err
	}
	codes, err := svc.Ensure(ctx, venues[0].ID)
	if err != nil {
		return nil, fmt.Errorf("table-codes: %w", err)
	}
	return codes, nil
}

// writeTableCodes prints each table's code, and the path a guest opens.
func writeTableCodes(w io.Writer, codes []tablecodes.Coded) {
	if len(codes) == 0 {
		fmt.Fprintln(w, "  This venue has no tables yet. Draw the floor in the back office first.")
		return
	}
	for _, c := range codes {
		fmt.Fprintf(w, "  Table %-12s %s   %st/%s\n", c.TableName, c.Code, web.MountGuest, c.Code)
	}
	fmt.Fprintln(w, "  A guest opens the path on this machine's address, or on the venue's address at the relay.")
}
