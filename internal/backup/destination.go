package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	// reach reports whether the destination can be used right now.
	reach func(context.Context) error
	// every is how often to ask.
	every time.Duration
	// photos is where the menu's photos go, beside the database.
	photos photoStore
}

// databasePath is where, in a folder destination, the database's copies go.
func databasePath(cfg Config) string { return filepath.Join(cfg.Folder, databaseDir) }

// newDestination builds the destination a validated Config names. It
// calls nobody and touches no disk.
func newDestination(cfg Config, log *slog.Logger) destination {
	if cfg.Kind == KindS3 {
		return newBucket(cfg.S3, log)
	}
	return newFolder(cfg, log)
}

func newFolder(cfg Config, log *slog.Logger) destination {
	client := file.NewReplicaClient(databasePath(cfg))
	client.SetLogger(log)
	return destination{
		client: client,
		// So the copies are written with the database's own permissions.
		attach: func(r *litestream.Replica) { client.Replica = r },
		// The way Litestream will use it: by making the folder if it is
		// not there.
		reach:  func(context.Context) error { return os.MkdirAll(databasePath(cfg), 0o750) },
		every:  probeFolderEvery,
		photos: folderPhotos{dir: filepath.Join(cfg.Folder, photosDir)},
	}
}

func newBucket(cfg S3, log *slog.Logger) destination {
	client := s3.NewReplicaClient()
	client.SetLogger(log)
	client.Bucket = cfg.Bucket
	client.Path = cfg.prefix() + "/" + databaseDir
	client.Region = cfg.Region
	client.Endpoint = cfg.Endpoint
	client.AccessKeyID = cfg.AccessKeyID
	client.SecretAccessKey = cfg.SecretAccessKey
	// A service that is not Amazon's is reached at its own address with
	// the bucket in the path. R2 and MinIO both take it; the other way
	// needs a DNS name for each bucket, which a MinIO on the LAN has not.
	client.ForcePathStyle = cfg.Endpoint != ""

	return destination{
		client: client,
		attach: func(*litestream.Replica) {},
		reach:  func(ctx context.Context) error { return listOne(ctx, client) },
		every:  probeBucketEvery,
		photos: newBucketPhotos(cfg),
	}
}

// listOne asks the bucket for the nightly copies, of which there are a
// handful: the cheapest question that needs the address, the bucket and
// the key all to be right.
func listOne(ctx context.Context, client litestream.ReplicaClient) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	err := func() error {
		files, err := client.LTXFiles(ctx, litestream.SnapshotLevel, 0, false)
		if err != nil {
			return err
		}
		files.Next()
		if err := files.Err(); err != nil {
			files.Close()
			return err
		}
		return files.Close()
	}()
	// "context deadline exceeded" tells an owner nothing.
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("the bucket did not answer in %s: is there internet? (%w)", probeTimeout, err)
	}
	return err
}
