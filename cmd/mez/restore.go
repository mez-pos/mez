package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/fsperm"
)

// restoreSecretEnv is where `mez restore` reads a bucket's secret from.
// Not a flag: a flag is in the shell's history and in the list of running
// programs for as long as the restore takes.
const restoreSecretEnv = "MEZ_RESTORE_SECRET_ACCESS_KEY"

// ErrVenueThere is `mez restore` asked to restore into a data directory
// that already holds a venue.
var ErrVenueThere = errors.New("there is already a venue in that data directory")

// restoreOptions is everything `mez restore` takes.
type restoreOptions struct {
	DataDir    string
	From       string // a folder
	ConfigFile string // a backup.json
	Bucket     backup.S3
	Generation string
	List       bool
}

// runRestore is `mez restore`: a venue brought back from its backup into
// an empty data directory (phase 12 Task 4f, ADR-278).
//
// It is a command and not a screen because the thing it restores is the
// thing a screen would be served by. It never writes over a venue: the
// database in the way may be the only other copy, and whether it is worse
// than the backup is for a person to decide, by moving it aside.
func runRestore(args []string, stdout io.Writer) error {
	opts, err := parseRestoreFlags(args, stdout)
	if err != nil {
		return err
	}
	from, err := opts.destination(os.Getenv)
	if err != nil {
		return err
	}
	ctx := context.Background()
	// Litestream says what it is doing through Go's default logger when
	// it restores, in its own words and its own format, in the middle of
	// what this prints for an owner. This process does nothing else.
	slog.SetDefault(quietLog())

	if opts.List {
		return listGenerations(ctx, stdout, from)
	}

	var shared []string
	if opts.DataDir != "" {
		shared = append(shared, "-data-dir", opts.DataDir)
	}
	cfg, err := config.LoadArgs(shared)
	if err != nil {
		return err
	}
	// A folder to restore from that is the data directory, or in it, is
	// refused as it would be as a destination: it is not a backup of this
	// directory if it is this directory.
	if err := from.Validate(cfg.DataDir); err != nil {
		return fmt.Errorf("restore: -data-dir %s: %w", cfg.DataDir, err)
	}
	dbPath := filepath.Join(cfg.DataDir, db.FileName)
	empty, err := emptyInstall(ctx, dbPath)
	if err != nil {
		return err
	}

	generation, err := chooseGeneration(ctx, from, opts.Generation)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("restore: creating data directory %s: %w", cfg.DataDir, err)
	}
	// As the server does on its way up (ADR-259): what is about to be
	// written here is a venue's sales.
	if err := fsperm.Restrict(cfg.DataDir); err != nil {
		fmt.Fprintf(stdout, "  Could not restrict %s to its owner: %v\n", cfg.DataDir, err)
	}

	aside := ""
	if empty {
		if err := notRunning(ctx, dbPath); err != nil {
			return err
		}
		if aside, err = moveAside(dbPath, time.Now()); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "A new install with no venue was here. Its empty database is moved to %s\n", aside)
	}

	fmt.Fprintf(stdout, "Restoring from %s\n", from.Destination())
	fmt.Fprintf(stdout, "  The backup begun %s, last written %s\n", localTime(generation.Began), localTime(generation.Last))
	if err := backup.RestoreGeneration(ctx, from, generation.Name, dbPath); err != nil {
		if aside != "" {
			// Said, because the next thing this computer does is start mez,
			// which would make a new empty database and show the wizard.
			return fmt.Errorf("restore: %w. Nothing was restored, and there is now no database in %s: run this again before starting mez", err, cfg.DataDir)
		}
		return fmt.Errorf("restore: %w", err)
	}
	fmt.Fprintf(stdout, "  The database is restored and checked: %s\n", dbPath)

	// From here the venue's sales are back. What follows can fail without
	// taking that away, and says so.
	photos, photosErr := backup.RestorePhotos(ctx, from, filepath.Join(cfg.DataDir, "photos"))
	switch {
	case photosErr != nil:
		fmt.Fprintf(stdout, "  %d menu photos restored, and not all of them: %v\n", photos, photosErr)
	case photos == 0:
		fmt.Fprintln(stdout, "  No menu photos in the backup.")
	default:
		fmt.Fprintf(stdout, "  %d menu photos restored.\n", photos)
	}

	if err := summarise(ctx, stdout, dbPath); err != nil {
		return fmt.Errorf("restore: the database was restored to %s, and could not be opened to say what is in it: %w", dbPath, err)
	}

	fmt.Fprint(stdout, "\nStart mez and look at the last invoices before selling on it.\n\n")
	if _, err := os.Stat(filepath.Join(cfg.DataDir, backup.ConfigFile)); err == nil {
		// A new install on which a backup had already been set.
		fmt.Fprintf(stdout, `This directory already has a %s, so this copy backs up to
where that says as soon as mez starts, in a run of its own beside any
other there. Look at Settings, Backup once it is up.
`, backup.ConfigFile)
	} else {
		fmt.Fprint(stdout, `Backups are off on this copy. Turn them on again under Settings, Backup:
this venue then begins a backup of its own beside the one it came from,
which is left as it was. If the venue you restored from is still running
somewhere, give this one a different folder or bucket.
`)
	}
	if photosErr != nil {
		return fmt.Errorf("restore: the database is restored, and the photos are not all: %w", photosErr)
	}
	return nil
}

// emptyInstall reports whether the database in the way is a new install's:
// one mez made as it first started, with no venue in it. No database at
// all is not in the way and is false; one with a venue is ErrVenueThere.
//
// A new computer always has one. The installer starts the service, the
// service makes its database, and an owner who has come to restore has by
// then been shown a setup wizard they must not finish. Refusing them for a
// database with nothing in it would be refusing everyone.
func emptyInstall(ctx context.Context, dbPath string) (bool, error) {
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

// ErrStillRunning is `mez restore` into a data directory mez is running on.
var ErrStillRunning = errors.New("mez is running on that data directory")

// notRunning refuses if anything else has the database open.
//
// On Windows a running server holds the file and the move fails by
// itself. Everywhere else a file can be moved out from under a program
// that has it open: the server would go on with the old database, unseen,
// and open the restored one through log files that are not its own.
// SQLite will give the whole database to one connection only when no
// other has it, so asking for that is asking whether anyone does.
func notRunning(ctx context.Context, dbPath string) error {
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

// quietLog is a logger for the databases this command opens. What it
// prints is for an owner, and "migration applied" forty times is not; what
// goes wrong is returned and said once.
func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// moveAside renames a new install's empty database out of the way, and
// clears what was that database's and would be read as the restored one's:
// its log files, and what Litestream remembered of backing it up.
func moveAside(dbPath string, now time.Time) (string, error) {
	aside := dbPath + ".empty-" + now.UTC().Format("20060102T150405Z")
	if err := os.Rename(dbPath, aside); err != nil {
		return "", fmt.Errorf("restore: moving the empty database aside: %w. Is mez still running on this directory? Stop it first", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("restore: clearing %s: %w", dbPath+suffix, err)
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
		return "", fmt.Errorf("restore: clearing %s: %w", working, err)
	}
	if err := os.Remove(working + "-for"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("restore: clearing %s: %w", working+"-for", err)
	}
	return aside, nil
}

// destination is where to restore from, as the flags, a backup.json or
// both say it.
func (o restoreOptions) destination(getenv func(string) string) (backup.Config, error) {
	given := 0
	for _, set := range []bool{o.From != "", o.ConfigFile != "", o.Bucket.Bucket != ""} {
		if set {
			given++
		}
	}
	if given != 1 {
		return backup.Config{}, errors.New("restore: say where the backup is, one way: -from FOLDER, or -bucket NAME with its flags, or -config with a saved backup.json")
	}

	switch {
	case o.From != "":
		folder, err := filepath.Abs(o.From)
		if err != nil {
			return backup.Config{}, fmt.Errorf("restore: -from %s: %w", o.From, err)
		}
		return backup.Config{Kind: backup.KindFolder, Folder: folder}, nil
	case o.ConfigFile != "":
		// LoadConfig reads the file of that name in a directory, and checks
		// the destination against that directory as the data directory.
		if filepath.Base(o.ConfigFile) != backup.ConfigFile {
			return backup.Config{}, fmt.Errorf("restore: -config must be a file called %s", backup.ConfigFile)
		}
		cfg, err := backup.LoadConfig(filepath.Dir(o.ConfigFile))
		if err != nil {
			return backup.Config{}, fmt.Errorf("restore: %w", err)
		}
		if !cfg.Enabled() {
			return backup.Config{}, fmt.Errorf("restore: %s names no destination", o.ConfigFile)
		}
		return cfg, nil
	default:
		b := o.Bucket
		b.SecretAccessKey = getenv(restoreSecretEnv)
		if b.SecretAccessKey == "" {
			return backup.Config{}, fmt.Errorf("restore: set %s to the bucket's secret access key. It is read from the environment and not from a flag, so that it is not left in the shell's history", restoreSecretEnv)
		}
		return backup.Config{Kind: backup.KindS3, S3: b}, nil
	}
}

// chooseGeneration is the generation named, or the newest that holds
// anything. Which one is taken is always said: generations sort by the
// clock of the machine that began them, and a clock can be wrong.
func chooseGeneration(ctx context.Context, from backup.Config, name string) (backup.Generation, error) {
	found, err := backup.Generations(ctx, from)
	if err != nil {
		return backup.Generation{}, fmt.Errorf("restore: %w", err)
	}
	for _, g := range found {
		switch {
		case name != "" && g.Name == name && g.Empty():
			return backup.Generation{}, fmt.Errorf("restore: the backup %s has nothing in it: it was begun and its first copy never arrived. `mez restore -list` shows the others", name)
		case name != "" && g.Name == name, name == "" && !g.Empty():
			return g, nil
		}
	}
	if name != "" {
		return backup.Generation{}, fmt.Errorf("restore: %w: %s. `mez restore -list` shows what is there", backup.ErrNoSuchGeneration, name)
	}
	return backup.Generation{}, fmt.Errorf("restore: %w: %s", backup.ErrNothingToRestore, from.Destination())
}

// listGenerations prints what a destination holds, newest first.
func listGenerations(ctx context.Context, stdout io.Writer, from backup.Config) error {
	found, err := backup.Generations(ctx, from)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	fmt.Fprintf(stdout, "At %s\n", from.Destination())
	if len(found) == 0 {
		fmt.Fprintln(stdout, "  There is no backup here.")
		return nil
	}
	fmt.Fprintln(stdout, "  A backup is one unbroken run of backing up. A new one begins when the one")
	fmt.Fprintln(stdout, "  before cannot be carried on: a new drive, a restored venue, a changed folder.")
	fmt.Fprintln(stdout)
	chosen := false
	for _, g := range found {
		switch {
		case g.Empty():
			fmt.Fprintf(stdout, "  %s   begun %s, and nothing in it\n", g.Name, localTime(g.Began))
		case !chosen:
			chosen = true
			fmt.Fprintf(stdout, "  %s   begun %s, last written %s   (the one a restore takes)\n", g.Name, localTime(g.Began), localTime(g.Last))
		default:
			fmt.Fprintf(stdout, "  %s   begun %s, last written %s\n", g.Name, localTime(g.Began), localTime(g.Last))
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "  To restore one that is not the newest: mez restore -generation NAME ...")
	return nil
}

// localTime is a time for a person at this machine. A zero time is not
// known.
func localTime(at time.Time) string {
	if at.IsZero() {
		return "at a time not known"
	}
	return at.Local().Format("2 Jan 2006 15:04")
}

// summarise opens the restored database the way the server will, brings
// its schema up to this binary's, and says what is in it: enough for an
// owner to see it is their venue and how recent.
func summarise(ctx context.Context, stdout io.Writer, dbPath string) error {
	log := quietLog()
	database, err := db.Open(ctx, db.Options{Path: dbPath, Log: log})
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(context.Background()); err != nil {
			log.Warn("closing the restored database", "error", err)
		}
	}()
	if err := database.Migrate(ctx); err != nil {
		return err
	}

	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		return err
	}
	if len(venues) == 0 {
		fmt.Fprintln(stdout, "  There is no venue in it yet: it was backed up before the setup wizard was finished.")
		return nil
	}
	for _, v := range venues {
		invoices, err := database.Queries().InvoicesForVenue(ctx, v.ID)
		if err != nil {
			return err
		}
		if len(invoices) == 0 {
			fmt.Fprintf(stdout, "  %s: no invoices yet.\n", v.Name)
			continue
		}
		last := invoices[0]
		for _, inv := range invoices {
			if inv.IssuedAt.After(last.IssuedAt) {
				last = inv
			}
		}
		fmt.Fprintf(stdout, "  %s: %d invoices, the last %s, issued %s.\n", v.Name, len(invoices), last.InvoiceNo, localTime(last.IssuedAt))
	}
	return nil
}

// parseRestoreFlags reads the command line for restore.
func parseRestoreFlags(args []string, out io.Writer) (restoreOptions, error) {
	var o restoreOptions
	set := flag.NewFlagSet("mez restore", flag.ContinueOnError)
	set.SetOutput(out)
	set.Usage = func() {
		fmt.Fprint(out, strings.TrimLeft(`
mez restore brings a venue back from its backup, into a data directory
with no venue in it. Stop mez first if it is running on that directory.

  mez restore -from D:\mez-backup
  mez restore -from D:\mez-backup -list
  mez restore -bucket corner-cafe -endpoint https://ACCOUNT.r2.cloudflarestorage.com -access-key-id KEY
      (with `+restoreSecretEnv+` set to the secret)

`, "\n"))
		set.PrintDefaults()
	}
	set.StringVar(&o.DataDir, "data-dir", "", "the directory to restore into (default: as the server resolves it)")
	set.StringVar(&o.From, "from", "", "the folder the backup is in")
	set.StringVar(&o.ConfigFile, "config", "", "a saved backup.json naming the destination, in place of -from or -bucket")
	set.StringVar(&o.Bucket.Bucket, "bucket", "", "the bucket the backup is in")
	set.StringVar(&o.Bucket.Path, "path", "", "the folder in the bucket (default mez)")
	set.StringVar(&o.Bucket.Endpoint, "endpoint", "", "the service's address, for anything but Amazon S3")
	set.StringVar(&o.Bucket.Region, "region", "", "the bucket's region, for Amazon S3")
	set.StringVar(&o.Bucket.AccessKeyID, "access-key-id", "", "the bucket's access key id; the secret is read from "+restoreSecretEnv)
	set.StringVar(&o.Generation, "generation", "", "restore this backup and not the newest, by its name from -list")
	set.BoolVar(&o.List, "list", false, "list the backups at the destination and restore nothing")
	if err := set.Parse(args); err != nil {
		return restoreOptions{}, err
	}
	if set.NArg() > 0 {
		return restoreOptions{}, fmt.Errorf("restore: unexpected argument %q", set.Arg(0))
	}
	return o, nil
}
