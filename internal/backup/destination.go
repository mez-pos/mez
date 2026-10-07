package backup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/file"
	"github.com/benbjohnson/litestream/s3"
)

// databaseDir is the folder, inside the destination, that the database's
// copies go in. The photos get one beside it.
const databaseDir = "db"

// How often the destination is looked at when nobody is selling.
// Litestream only touches it when there is something to send, so a drive
// pulled out on a quiet afternoon would otherwise read as fine until the
// next sale. A folder is asked often because asking is free; a bucket is
// asked once a minute, because each asking is a request someone bills.
const (
	probeFolderEvery = 15 * time.Second
	probeBucketEvery = time.Minute
)

// probeTimeout bounds one look at the destination. With no internet the
// S3 client tries ten times, backing off between, which is minutes; a
// listing of a handful of names that has not come back in this long is
// a bucket that cannot be reached.
const probeTimeout = 8 * time.Second

// destination is one place a backup goes: Litestream's client for it, and
// how this package asks whether it is there.
type destination struct {
	client litestream.ReplicaClient
	// attach gives the client the replica it serves, where it wants one.
	attach func(*litestream.Replica)
	// reach reports whether the destination can be used right now, and
	// whether it holds a whole copy of the database in this generation.
	reach func(context.Context) (whole bool, err error)
	// every is how often to ask.
	every time.Duration
	// photos is where the menu's photos go, beside the database.
	photos photoStore
	// check proves the destination can be written to, by writing to it:
	// a note that says what the folder is, left there for whoever finds it.
	check func(context.Context) error
}

// noteName is the note Check leaves at a destination, beside db/ and
// photos/, and noteText what it says.
const (
	noteName = "mez-backup.txt"
	noteText = "This folder holds the backup of a mez point of sale.\r\n" +
		"\r\n" +
		"db/      the venue's database, as Litestream keeps it\r\n" +
		"photos/  the menu's photos\r\n" +
		"\r\n" +
		"mez writes these and tidies them itself. Do not edit, rename or\r\n" +
		"delete anything here by hand: a backup with a piece missing does\r\n" +
		"not restore. To get a venue back from it, see \"Backups\" in mez's\r\n" +
		"documentation.\r\n"
)

// checkTimeout bounds proving a destination can be written to. Somebody
// is watching a Save button for the whole of it.
const checkTimeout = 15 * time.Second

// Check proves a destination can be used before it is kept: that it is
// there, and that it can be written to. It writes a small note saying
// what the folder is and leaves it.
//
// A destination that fails is not a setting to save. Whoever is saving it
// is at the screen now and can fix it; found out tonight, it is a venue
// that believes it is backed up.
func Check(ctx context.Context, cfg Config, dataDir string) error {
	if err := cfg.Validate(dataDir); err != nil {
		return err
	}
	if !cfg.Enabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	dest := newDestination(cfg, "", slog.New(slog.NewTextHandler(io.Discard, nil)))

	// To one side: making a folder on a share that has stopped answering
	// does not come back when ctx says, and whoever is saving must.
	done := make(chan error, 1)
	go func() {
		// Read first: a key that cannot list cannot restore.
		if _, err := dest.reach(ctx); err != nil {
			done <- err
			return
		}
		done <- dest.check(ctx)
	}()
	select {
	case err := <-done:
		if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: it did not answer in %s", ErrUnusable, checkTimeout)
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUnusable, err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: it did not answer in %s", ErrUnusable, checkTimeout)
	}
}

// ErrUnusable is a destination that is written correctly and cannot be
// used: not there, not writable, or refusing the key.
var ErrUnusable = errors.New("the backup destination cannot be written to")

// A generation is one unbroken run of backing up to a destination: a
// folder of its own under db/, named for when it began (ADR-276).
//
// Litestream numbers what it sends and expects the destination to hold
// what it last sent. When that stops being true (the destination was
// changed and changed back, a new drive stands in the old one's place,
// the venue was restored on another machine) the numbers start again, and
// two runs of the same numbers in one folder is a backup that restores to
// some of each. So a run that cannot carry on from the one before does
// not write among its files: it starts a folder of its own, and a restore
// takes the newest folder that has a whole copy in it.
//
// Nothing here deletes a generation. An owner who points a new install at
// last year's backup, meaning to restore it later, still has it.
const generationTime = "20060102T150405Z"

// newGeneration names a generation beginning now. The names sort in the
// order they began, by this machine's clock; the tail keeps two begun in
// one second apart.
func newGeneration(now time.Time) (string, error) {
	tail := make([]byte, 3)
	if _, err := rand.Read(tail); err != nil {
		return "", fmt.Errorf("naming a generation: %w", err)
	}
	return now.UTC().Format(generationTime) + "-" + hex.EncodeToString(tail), nil
}

var generationName = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{6}$`)

// databasePath is where, in a folder destination, a generation's copies
// of the database go. With no generation it is the folder they are all in.
func databasePath(cfg Config, generation string) string {
	return filepath.Join(cfg.Folder, databaseDir, generation)
}

// newDestination builds the destination a validated Config names, for one
// generation of the database's copies. It calls nobody and touches no
// disk. With no generation it is the destination for everything but the
// database: the photos, and the checks.
func newDestination(cfg Config, generation string, log *slog.Logger) destination {
	if cfg.Kind == KindS3 {
		return newBucket(cfg.S3, generation, log)
	}
	return newFolder(cfg, generation, log)
}

// generations lists the generations at a destination, newest first.
func generations(ctx context.Context, cfg Config) ([]string, error) {
	var names []string
	var err error
	if cfg.Kind == KindS3 {
		names, err = newBucketPhotos(cfg.S3).folders(ctx, cfg.S3.prefix()+"/"+databaseDir+"/")
	} else {
		names, err = foldersIn(databasePath(cfg, ""))
	}
	if err != nil {
		return nil, err
	}
	var found []string
	for _, n := range names {
		if generationName.MatchString(n) {
			found = append(found, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(found)))
	return found, nil
}

// foldersIn names the folders in dir. No dir is none.
func foldersIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func newFolder(cfg Config, generation string, log *slog.Logger) destination {
	client := file.NewReplicaClient(databasePath(cfg, generation))
	client.SetLogger(log)
	return destination{
		client: client,
		// So the copies are written with the database's own permissions.
		attach: func(r *litestream.Replica) { client.Replica = r },
		// The way Litestream will use it: by making the folder if it is
		// not there.
		reach: func(ctx context.Context) (bool, error) {
			if err := os.MkdirAll(databasePath(cfg, generation), 0o750); err != nil {
				return false, err
			}
			return listOne(ctx, client)
		},
		every:  probeFolderEvery,
		photos: folderPhotos{dir: filepath.Join(cfg.Folder, photosDir)},
		check: func(context.Context) error {
			return writeWhole(filepath.Join(cfg.Folder, noteName), []byte(noteText))
		},
	}
}

func newBucket(cfg S3, generation string, log *slog.Logger) destination {
	client := s3.NewReplicaClient()
	client.SetLogger(log)
	client.Bucket = cfg.Bucket
	client.Path = cfg.prefix() + "/" + databaseDir
	if generation != "" {
		client.Path += "/" + generation
	}
	client.Region = cfg.Region
	client.Endpoint = cfg.Endpoint
	client.AccessKeyID = cfg.AccessKeyID
	client.SecretAccessKey = cfg.SecretAccessKey
	// A service that is not Amazon's is reached at its own address with
	// the bucket in the path. R2 and MinIO both take it; the other way
	// needs a DNS name for each bucket, which a MinIO on the LAN has not.
	client.ForcePathStyle = cfg.Endpoint != ""

	photos := newBucketPhotos(cfg)
	return destination{
		client: client,
		attach: func(*litestream.Replica) {},
		reach:  func(ctx context.Context) (bool, error) { return listOne(ctx, client) },
		every:  probeBucketEvery,
		photos: photos,
		check:  func(ctx context.Context) error { return photos.putNote(ctx, cfg.prefix()+"/"+noteName, noteText) },
	}
}

// listOne asks the destination for the nightly whole copies, of which
// there are a handful, and reports whether there is one. For a bucket it
// is the cheapest question that needs the address, the bucket and the key
// all to be right; for either kind the answer is whether what was sent is
// still there.
func listOne(ctx context.Context, client litestream.ReplicaClient) (whole bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	whole, err = func() (bool, error) {
		files, err := client.LTXFiles(ctx, litestream.SnapshotLevel, 0, false)
		if err != nil {
			return false, err
		}
		found := files.Next()
		if err := files.Err(); err != nil {
			files.Close()
			return false, err
		}
		return found, files.Close()
	}()
	// "context deadline exceeded" tells an owner nothing.
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, fmt.Errorf("the destination did not answer in %s: is there internet? (%w)", probeTimeout, err)
	}
	return whole, err
}
