package menus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mez-pos/mez/internal/db/gen"
)

// Photo limits (ADR-110). The editor resizes to 480px before it uploads;
// these leave room for a larger screen later without letting a phone's
// full-size photo onto every till.
const (
	MaxPhotoBytes = 512 << 10
	MinPhotoSide  = 64
	MaxPhotoSide  = 1024
)

// ErrPhotoNotFound is a photo name that is not a stored photo.
var ErrPhotoNotFound = errors.New("no such photo")

// photoName is a stored photo's name: the SHA-256 of its bytes, in hex.
var photoName = regexp.MustCompile(`^[0-9a-f]{64}\.jpg$`)

// PhotoStore keeps item photos as files in one directory, each named by
// the hash of its bytes, so the same photo is stored once and a name never
// changes what it shows.
type PhotoStore struct{ dir string }

// NewPhotoStore opens the photos directory, creating it if needed.
func NewPhotoStore(dir string) (*PhotoStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("menus: creating the photos directory %s: %w", dir, err)
	}
	return &PhotoStore{dir: dir}, nil
}

// Open returns a stored photo for reading. A name that is not one this
// store could have written is not found, so no path can be reached
// through it.
func (p *PhotoStore) Open(name string) (*os.File, error) {
	if !photoName.MatchString(name) {
		return nil, ErrPhotoNotFound
	}
	f, err := os.Open(filepath.Join(p.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrPhotoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("menus: opening photo %s: %w", name, err)
	}
	return f, nil
}

// put checks a photo and stores it, returning its name. Storing a photo
// already there is a no-op. The file is written whole and renamed into
// place, so a reader never sees half of one.
func (p *PhotoStore) put(data []byte) (string, error) {
	if err := checkPhoto(data); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + ".jpg"
	final := filepath.Join(p.dir, name)
	if _, err := os.Stat(final); err == nil {
		return name, nil
	}
	tmp, err := os.CreateTemp(p.dir, "upload-*.tmp")
	if err != nil {
		return "", fmt.Errorf("menus: storing a photo: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("menus: writing a photo: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("menus: writing a photo: %w", err)
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return "", fmt.Errorf("menus: storing photo %s: %w", name, err)
	}
	return name, nil
}

// checkPhoto refuses anything but a whole JPEG of a sensible size. The
// header is read first, so a small file claiming a huge image is refused
// before it is decoded.
func checkPhoto(data []byte) error {
	if len(data) > MaxPhotoBytes {
		return &Invalid{"photo", fmt.Sprintf("That photo is over %d KB. The editor makes them smaller; try choosing it again.", MaxPhotoBytes>>10)}
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return &Invalid{"photo", "That is not a JPEG photo."}
	}
	if cfg.Width < MinPhotoSide || cfg.Height < MinPhotoSide || cfg.Width > MaxPhotoSide || cfg.Height > MaxPhotoSide {
		return &Invalid{"photo", fmt.Sprintf("A photo must be %d to %d pixels on each side.", MinPhotoSide, MaxPhotoSide)}
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		return &Invalid{"photo", "That photo is damaged and cannot be read."}
	}
	return nil
}

// SetItemPhoto stores a photo and puts it on an item, returning its name.
// A till showing the old one reads the menu again, as with any edit.
func (s *Service) SetItemPhoto(ctx context.Context, venueID, id int64, data []byte) (string, error) {
	if s.photos == nil {
		return "", errors.New("menus: no photo store")
	}
	name, err := s.photos.put(data)
	if err != nil {
		return "", err
	}
	err = s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetItemPhoto(ctx, gen.SetItemPhotoParams{Photo: name, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrItemNotFound)
	})
	if err != nil {
		return "", err
	}
	return name, nil
}

// ClearItemPhoto takes an item's photo off. The file stays: another item,
// or a line's history, may still name it.
func (s *Service) ClearItemPhoto(ctx context.Context, venueID, id int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetItemPhoto(ctx, gen.SetItemPhotoParams{Photo: "", Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrItemNotFound)
	})
}

// OpenPhoto opens a stored photo by name.
func (s *Service) OpenPhoto(name string) (*os.File, error) {
	if s.photos == nil {
		return nil, ErrPhotoNotFound
	}
	return s.photos.Open(name)
}
