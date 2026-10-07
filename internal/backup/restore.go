package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/benbjohnson/litestream"
)

// ErrNothingToRestore is a destination with no backup in it.
var ErrNothingToRestore = errors.New("there is no backup at the destination")

// ErrDestinationMissing is a destination that is not there to restore
// from: a drive not plugged in, a share not mounted. Not the same as one
// that is there and empty.
var ErrDestinationMissing = errors.New("the backup destination cannot be found")

// ErrWouldOverwrite is a restore asked to write where a file already is.
var ErrWouldOverwrite = errors.New("a file is already there, and a restore does not write over one")

// Restore writes the most recent state held at the destination to a new
// database file at outputPath, and checks it before returning.
//
// It never writes over a file. The database it would replace may be the
// only other copy of the venue, and whether it is worse than the backup is
// for a person to decide, by moving it aside.
func Restore(ctx context.Context, cfg Config, outputPath string) error {
	if !cfg.Enabled() {
		return ErrOff
	}
	if outputPath == "" {
		return errors.New("backup: no path to restore to")
	}
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("backup: restoring to %s: %w", outputPath, ErrWouldOverwrite)
	}
	// The same checks a running backup gets: a bucket with no key would
	// send the SDK looking through the environment for one.
	if err := cfg.Validate(filepath.Dir(outputPath)); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	// Litestream reads a folder that is not there as one with nothing in
	// it, and "there is no backup" is the wrong thing to tell someone
	// whose drive is not plugged in.
	if cfg.Kind == KindFolder {
		if _, err := os.Stat(cfg.Folder); err != nil {
			return fmt.Errorf("backup: restoring from %s: %w: %w", cfg.Folder, ErrDestinationMissing, err)
		}
	}

	// The newest generation with a whole copy in it. One begun and never
	// finished (the line dropped as the first copy went) has nothing to
	// restore, and the one before it is the venue's backup still.
	listCtx, cancel := context.WithTimeout(ctx, listPhotosTimeout)
	found, err := generations(listCtx, cfg)
	cancel()
	if err != nil {
		return fmt.Errorf("backup: restoring from %s: %w", cfg.Destination(), err)
	}
	for _, generation := range found {
		err := restoreGeneration(ctx, cfg, generation, outputPath)
		if errors.Is(err, ErrNothingToRestore) {
			continue
		}
		if err != nil {
			return fmt.Errorf("backup: restoring from %s: %w", cfg.Destination(), err)
		}
		return nil
	}
	return fmt.Errorf("backup: restoring from %s: %w", cfg.Destination(), ErrNothingToRestore)
}

func restoreGeneration(ctx context.Context, cfg Config, generation, outputPath string) error {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	replica := litestream.NewReplicaWithClient(nil, newDestination(cfg, generation, log).client)
	opt := litestream.NewRestoreOptions()
	opt.OutputPath = outputPath
	// The whole of it. A restore is rare, and a restored database that is
	// quietly damaged is worse than one that is known to be missing.
	opt.IntegrityCheck = litestream.IntegrityCheckFull

	if err := replica.Restore(ctx, opt); err != nil {
		if errors.Is(err, litestream.ErrNoSnapshots) || errors.Is(err, litestream.ErrTxNotAvailable) {
			return ErrNothingToRestore
		}
		return err
	}
	return nil
}
