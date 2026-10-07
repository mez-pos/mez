package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/benbjohnson/litestream"
)

// Generation is one unbroken run of backing up, as found at a destination
// (ADR-276).
type Generation struct {
	// Name is the folder's name under db/, which is when the run began.
	Name string
	// Began is when the run began, by the clock of the machine that began
	// it, UTC.
	Began time.Time
	// First and Last are when its oldest and newest copies were written.
	// Both zero is a generation with nothing in it: begun, and the first
	// copy never arrived.
	First, Last time.Time
}

// Empty reports whether the generation holds nothing to restore.
func (g Generation) Empty() bool { return g.Last.IsZero() }

// ErrNoSuchGeneration is a generation asked for by a name the destination
// does not have.
var ErrNoSuchGeneration = errors.New("the destination has no generation of that name")

// Generations lists what a destination holds, newest first, with when
// each was last written to. It reads the destination and changes nothing.
func Generations(ctx context.Context, cfg Config) ([]Generation, error) {
	if !cfg.Enabled() {
		return nil, ErrOff
	}
	if err := cfg.validateForReading(); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}
	if err := destinationThere(cfg); err != nil {
		return nil, err
	}
	listCtx, cancel := context.WithTimeout(ctx, listPhotosTimeout)
	names, err := generations(listCtx, cfg)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("backup: listing what is at %s: %w", cfg.Destination(), err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	found := make([]Generation, 0, len(names))
	for _, name := range names {
		g := Generation{Name: name}
		g.Began, _ = generationBegan(name)
		replica := litestream.NewReplicaWithClient(nil, newDestination(cfg, name, log).client)
		boundsCtx, cancel := context.WithTimeout(ctx, listPhotosTimeout)
		g.First, g.Last, err = replica.TimeBounds(boundsCtx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("backup: reading generation %s at %s: %w", name, cfg.Destination(), err)
		}
		g.First, g.Last = utcOrZero(g.First), utcOrZero(g.Last)
		found = append(found, g)
	}
	return found, nil
}

// generationBegan reads when a generation began from its name.
func generationBegan(name string) (time.Time, bool) {
	stamp, _, _ := strings.Cut(name, "-")
	at, err := time.Parse(generationTime, stamp)
	return at, err == nil && generationName.MatchString(name)
}

// validateForReading is Validate for a destination that is only read:
// where it stands against a data directory does not come into it.
func (c Config) validateForReading() error {
	// A data directory nothing can be inside.
	return c.Validate(string([]byte{0}))
}

// destinationThere refuses a folder that is not there, with the words for
// it. Litestream reads one as a destination with nothing in it, and "there
// is no backup" is the wrong thing to tell someone whose drive is not
// plugged in.
func destinationThere(cfg Config) error {
	if cfg.Kind != KindFolder {
		return nil
	}
	if _, err := os.Stat(cfg.Folder); err != nil {
		return fmt.Errorf("backup: reading %s: %w: %w", cfg.Folder, ErrDestinationMissing, err)
	}
	return nil
}

// RestoreGeneration is Restore from one generation, by name.
func RestoreGeneration(ctx context.Context, cfg Config, generation, outputPath string) error {
	if !cfg.Enabled() {
		return ErrOff
	}
	if outputPath == "" {
		return errors.New("backup: no path to restore to")
	}
	if !generationName.MatchString(generation) {
		return fmt.Errorf("backup: %q: %w", generation, ErrNoSuchGeneration)
	}
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("backup: restoring to %s: %w", outputPath, ErrWouldOverwrite)
	}
	if err := cfg.validateForReading(); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	if err := destinationThere(cfg); err != nil {
		return err
	}
	if err := restoreGeneration(ctx, cfg, generation, outputPath); err != nil {
		return fmt.Errorf("backup: restoring generation %s from %s: %w", generation, cfg.Destination(), err)
	}
	return nil
}

// madeSuffix names the file, beside Litestream's working folder, that
// lists the generations this data directory began: one to a line, in the
// order they were begun, each with the destination it is at, its name,
// and when it was first seen whole at the destination.
const madeSuffix = "-made"

// madeGeneration is one line of that file.
type madeGeneration struct {
	mark, name string
	// sentAt is when the destination was first seen to hold a copy in
	// this generation, by this machine's clock. Zero is not yet.
	sentAt time.Time
}

// noteMade records that this data directory began a generation at a
// destination.
//
// It is what makes tidying up safe. A generation is only ever removed by
// the data directory that began it: a new install pointed at last year's
// backup, by an owner who means to restore it later, began none of what
// is there and removes none of it.
func noteMade(workingDir, mark, generation string) error {
	made, err := readMade(workingDir)
	if err != nil {
		return err
	}
	made = append(made, madeGeneration{mark: mark, name: generation})
	return writeMade(workingDir, made)
}

// noteSent records when a generation was first seen whole at the
// destination. It is from then, and not from when it was named, that the
// one before it counts as replaced: a generation named on Monday whose
// first copy arrived the Thursday after replaced nothing until Thursday.
func noteSent(workingDir, mark, generation string, at time.Time) error {
	made, err := readMade(workingDir)
	if err != nil {
		return err
	}
	for i := range made {
		if made[i].mark == mark && made[i].name == generation && made[i].sentAt.IsZero() {
			made[i].sentAt = at.UTC()
			return writeMade(workingDir, made)
		}
	}
	return nil
}

func readMade(workingDir string) ([]madeGeneration, error) {
	raw, err := os.ReadFile(workingDir + madeSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", workingDir+madeSuffix, err)
	}
	var made []madeGeneration
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.Fields(line)
		// A line that is not one of ours is not a generation to remove.
		if len(parts) < 2 || !generationName.MatchString(parts[1]) {
			continue
		}
		m := madeGeneration{mark: parts[0], name: parts[1]}
		if len(parts) > 2 {
			// One that cannot be read is not yet: the safe way to be wrong.
			m.sentAt, _ = time.Parse(time.RFC3339, parts[2])
		}
		made = append(made, m)
	}
	return made, nil
}

func writeMade(workingDir string, made []madeGeneration) error {
	var b strings.Builder
	for _, m := range made {
		b.WriteString(m.mark + " " + m.name)
		if !m.sentAt.IsZero() {
			b.WriteString(" " + m.sentAt.UTC().Format(time.RFC3339))
		}
		b.WriteString("\n")
	}
	if err := writeWhole(workingDir+madeSuffix, []byte(b.String())); err != nil {
		return fmt.Errorf("writing %s: %w", workingDir+madeSuffix, err)
	}
	return nil
}

// tidyGenerations removes the generations this data directory began at
// this destination that have been replaced for longer than a restore
// reaches back, and reports how many went.
//
// A generation is kept for as long after it was replaced as the nightly
// copies are kept (snapshotsKept): the promise is that a restore can go
// back a week, and for the first week of a new generation the week before
// is in the old one.
//
// "Replaced" is judged to keep, wherever there is doubt:
//   - The order is the order they were begun in, which is the order of the
//     file. Not the order of their names: a name is a clock's reading, and
//     a venue PC with a flat battery starts in the wrong year.
//   - A generation is replaced when one begun after it is first seen whole
//     at the destination, and of all those begun after it the latest such
//     time is the one counted from. A wrong clock in between can then only
//     put the removal off.
//   - The current generation stays, and so does any with a generation
//     after it that has not yet been seen whole.
func tidyGenerations(ctx context.Context, workingDir string, cfg Config, current string, now time.Time, log *slog.Logger) (removed int, err error) {
	made, err := readMade(workingDir)
	if err != nil {
		return 0, err
	}
	mark := destinationMark(cfg)

	var here []madeGeneration
	for _, m := range made {
		if m.mark == mark {
			here = append(here, m)
		}
	}

	gone := map[string]bool{}
	for i, m := range here {
		if m.name == current || i+1 >= len(here) {
			continue
		}
		// Every one begun after it must have arrived. Until the one begun
		// next has, this one is the venue's backup; and one further on
		// that has not is a time not yet known, which could only be later
		// than those that are.
		var replacedAt time.Time
		arrived := true
		for _, later := range here[i+1:] {
			if later.sentAt.IsZero() {
				arrived = false
				break
			}
			if later.sentAt.After(replacedAt) {
				replacedAt = later.sentAt
			}
		}
		if !arrived || now.Sub(replacedAt) < snapshotsKept {
			continue
		}
		quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
		if err := newDestination(cfg, m.name, quiet).client.DeleteAll(ctx); err != nil {
			return removed, fmt.Errorf("removing generation %s: %w", m.name, err)
		}
		log.Info("removed a generation of the backup that was replaced more than a week ago", "generation", m.name)
		gone[m.name] = true
		removed++
	}
	if removed == 0 {
		return 0, nil
	}
	// Read again: the file may have gained a line while generations were
	// being removed, and that line must not be lost.
	made, err = readMade(workingDir)
	if err != nil {
		return removed, err
	}
	kept := made[:0:0]
	for _, m := range made {
		if !(m.mark == mark && gone[m.name]) {
			kept = append(kept, m)
		}
	}
	return removed, writeMade(workingDir, kept)
}
