package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"
)

// photosDir is the folder, inside the destination, that the menu's photos
// go in, beside the database's.
const photosDir = "photos"

// photoName is a stored photo's name (ADR-110): the SHA-256 of its bytes,
// in hex. Anything else in the folder is not a photo and is not copied.
var photoName = regexp.MustCompile(`^[0-9a-f]{64}\.jpg$`)

// maxPhotoBytes is more than the menu will store (512 KB), and the most
// this will read into memory for one file from either end.
const maxPhotoBytes = 2 << 20

// photoTimeout bounds one photo going up or coming down, and
// listPhotosTimeout the asking of what is there: a bucket whose line
// stalls after connecting would otherwise hold the probe for good.
const (
	photoTimeout      = 30 * time.Second
	listPhotosTimeout = time.Minute
)

// relistPhotosEvery is how long what the destination holds is remembered
// before it is asked again.
const relistPhotosEvery = 10 * time.Minute

// photoStore is the photos' half of a destination. Litestream's clients
// carry Litestream's own files and nothing else, so a destination that
// keeps photos says how here.
type photoStore interface {
	// list names what the destination holds.
	list(ctx context.Context) ([]string, error)
	// put stores a photo whole, or not at all.
	put(ctx context.Context, name string, body []byte) error
	// open reads one back.
	open(ctx context.Context, name string) (io.ReadCloser, error)
}

// ErrPhotoDamaged is a photo whose bytes are not the ones its name says.
var ErrPhotoDamaged = errors.New("a photo does not match its name")

// photoSync copies the photos folder to a destination: the files that are
// not there yet, and nothing else.
//
// A photo is named by its hash and never changes, so "missing" is the
// whole of the question: there is nothing to compare and nothing to
// update. Nothing is ever deleted at the destination, as nothing sweeps
// the folder here either (ADR-110).
type photoSync struct {
	dir   string
	store photoStore
	log   *slog.Logger

	// busy is held by whoever is copying: a channel and not a mutex, so
	// that someone waiting for a long copy to finish can stop waiting.
	busy chan struct{}

	// The rest is the holder's.

	// there is what the destination is known to hold; nil is not known.
	// It saves asking a bucket for its whole list every minute.
	there map[string]struct{}
	// listedAt is when the destination was last asked.
	listedAt time.Time
	// skipped are local files already found damaged and said so, so that
	// the log is told once and not every fifteen seconds.
	skipped map[string]struct{}
	// stale is set to make the next sync ask the destination afresh.
	stale atomic.Bool
}

func newPhotoSync(dir string, store photoStore, log *slog.Logger) *photoSync {
	return &photoSync{dir: dir, store: store, log: log, busy: make(chan struct{}, 1)}
}

// forget drops what is known of the destination, so the next sync asks.
// A drive that went away may come back a different drive.
func (p *photoSync) forget() { p.stale.Store(true) }

// sync copies up whatever the destination lacks and reports how many
// went. With fresh it asks the destination what it holds and does not
// trust its memory.
//
// One photo that will not go does not hold up the ones after it: they
// are all tried, and the first failure is what is returned.
func (p *photoSync) sync(ctx context.Context, fresh bool) (copied int, err error) {
	select {
	case p.busy <- struct{}{}:
		defer func() { <-p.busy }()
	case <-ctx.Done():
		return 0, fmt.Errorf("the photos were still being copied: %w", ctx.Err())
	}

	local, err := localPhotos(p.dir)
	if err != nil {
		return 0, fmt.Errorf("reading the photos folder: %w", err)
	}
	// On any failure what is known is dropped: a put that half happened
	// is not one to remember either way.
	defer func() {
		if err != nil {
			p.there = nil
		}
	}()

	if p.stale.Swap(false) {
		p.there = nil
	}
	// Memory is trusted for a while and no longer: someone can empty the
	// folder at the destination, or put a new drive in the old one's place.
	if p.there == nil || fresh || time.Since(p.listedAt) > relistPhotosEvery {
		if len(local) == 0 {
			return 0, nil // nothing to send, so nothing to ask
		}
		listCtx, cancel := context.WithTimeout(ctx, listPhotosTimeout)
		names, err := p.store.list(listCtx)
		cancel()
		if err != nil {
			return 0, fmt.Errorf("listing the photos at the destination: %w", err)
		}
		p.there = make(map[string]struct{}, len(names))
		for _, n := range names {
			p.there[n] = struct{}{}
		}
		p.listedAt = time.Now()
		p.skipped = nil
	}

	var first error
	for _, name := range local {
		if _, ok := p.there[name]; ok {
			continue
		}
		if _, ok := p.skipped[name]; ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		body, err := readPhoto(filepath.Join(p.dir, name), name)
		if errors.Is(err, ErrPhotoDamaged) {
			// Not one to send: the copy would be damaged under a name
			// that says it is not.
			p.log.Warn("a photo was not backed up", "photo", name, "error", err)
			if p.skipped == nil {
				p.skipped = map[string]struct{}{}
			}
			p.skipped[name] = struct{}{}
			continue
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue // gone between the listing and the reading
		}
		if err == nil {
			putCtx, cancel := context.WithTimeout(ctx, photoTimeout)
			err = p.store.put(putCtx, name, body)
			cancel()
		}
		if err != nil {
			if ctx.Err() != nil {
				return copied, ctx.Err()
			}
			if first == nil {
				first = fmt.Errorf("copying photo %s: %w", name, err)
			}
			continue
		}
		p.there[name] = struct{}{}
		copied++
	}
	return copied, first
}

// localPhotos names the photos in dir. No folder is no photos: a venue
// that has never added one has none.
func localPhotos(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && photoName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// readPhoto reads a photo and holds it to its name.
func readPhoto(path, name string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return checkedPhoto(f, name)
}

// checkedPhoto reads a photo from anywhere and holds it to its name.
func checkedPhoto(r io.Reader, name string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPhotoBytes {
		return nil, fmt.Errorf("%w: %s is larger than any photo", ErrPhotoDamaged, name)
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:])+".jpg" != name {
		return nil, fmt.Errorf("%w: %s", ErrPhotoDamaged, name)
	}
	return body, nil
}

// RestorePhotos brings the photos at the destination back into dir, and
// reports how many it wrote. A photo already in dir is left as it is; one
// whose bytes are not what its name says is refused, and the rest are
// still brought back.
func RestorePhotos(ctx context.Context, cfg Config, dir string) (restored int, err error) {
	if !cfg.Enabled() {
		return 0, ErrOff
	}
	if dir == "" {
		return 0, errors.New("backup: no folder to restore the photos to")
	}
	if err := cfg.Validate(filepath.Dir(dir)); err != nil {
		return 0, fmt.Errorf("backup: %w", err)
	}
	if cfg.Kind == KindFolder {
		if _, err := os.Stat(cfg.Folder); err != nil {
			return 0, fmt.Errorf("backup: restoring photos from %s: %w: %w", cfg.Folder, ErrDestinationMissing, err)
		}
	}
	store := newDestination(cfg, "", slog.New(slog.NewTextHandler(io.Discard, nil))).photos

	listCtx, cancel := context.WithTimeout(ctx, listPhotosTimeout)
	names, err := store.list(listCtx)
	cancel()
	if err != nil {
		return 0, fmt.Errorf("backup: listing the photos at %s: %w", cfg.Destination(), err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, fmt.Errorf("backup: making %s: %w", dir, err)
	}

	var damaged []error
	for _, name := range names {
		if !photoName.MatchString(name) {
			continue
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		body, err := fetchPhoto(ctx, store, name)
		if errors.Is(err, ErrPhotoDamaged) {
			damaged = append(damaged, err)
			continue
		}
		if err != nil {
			return restored, fmt.Errorf("backup: restoring photo %s: %w", name, err)
		}
		if err := writeWhole(path, body); err != nil {
			return restored, fmt.Errorf("backup: restoring photo %s: %w", name, err)
		}
		restored++
	}
	if len(damaged) > 0 {
		return restored, fmt.Errorf("backup: %d of the photos at %s were not restored: %w", len(damaged), cfg.Destination(), errors.Join(damaged...))
	}
	return restored, nil
}

func fetchPhoto(ctx context.Context, store photoStore, name string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, photoTimeout)
	defer cancel()
	r, err := store.open(ctx, name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return checkedPhoto(r, name)
}

// writeWhole writes a file under another name and renames it into place,
// so that a photo is there whole or not at all, as the menu writes them.
func writeWhole(path string, body []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err := io.Copy(tmp, bytes.NewReader(body)); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Windows refuses a rename while a virus scanner is still reading the
	// file it has just seen appear, and lets it through a moment later.
	for try := 0; ; try++ {
		if err = os.Rename(tmp.Name(), path); err == nil || try == 3 {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// folderPhotos keeps photos in a folder.
type folderPhotos struct{ dir string }

func (f folderPhotos) list(context.Context) ([]string, error) {
	return localPhotos(f.dir)
}

func (f folderPhotos) put(_ context.Context, name string, body []byte) error {
	if err := os.MkdirAll(f.dir, 0o750); err != nil {
		return err
	}
	return writeWhole(filepath.Join(f.dir, name), body)
}

func (f folderPhotos) open(_ context.Context, name string) (io.ReadCloser, error) {
	if !photoName.MatchString(name) {
		return nil, fmt.Errorf("%q is not a photo's name", name)
	}
	return os.Open(filepath.Join(f.dir, name))
}
