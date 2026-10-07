package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ConfigFile is where the destination is kept, in the data directory.
//
// A file, not a row (ADR-273). A restore has to read it before there is a
// database to read; a copy of the database opened on another machine must
// not start writing into the venue's backup; and a bucket's secret kept in
// the database would be in every backup of it.
const ConfigFile = "backup.json"

// Kind is what sort of place a backup goes to.
type Kind string

const (
	// KindOff is no backup: the zero value, and what a venue has until it
	// names somewhere.
	KindOff Kind = ""
	// KindFolder is a folder this machine can write to: a second disk, a
	// USB drive, a share.
	KindFolder Kind = "folder"
)

// Config is where the backup goes.
type Config struct {
	Kind Kind `json:"kind"`
	// Folder is the destination of a KindFolder backup, absolute.
	Folder string `json:"folder,omitempty"`
}

// ErrInvalidConfig is a destination that cannot be used as written. What is
// wrong with it is in the message.
var ErrInvalidConfig = errors.New("the backup destination is not usable")

// Enabled says whether there is anywhere to back up to.
func (c Config) Enabled() bool { return c.Kind != KindOff }

// Validate checks the destination against the data directory it would be
// backing up. It touches no disk: a folder that is not there yet, or a
// drive that is unplugged, is a backup that is failing, not a setting that
// is wrong.
func (c Config) Validate(dataDir string) error {
	switch c.Kind {
	case KindOff:
		if c.Folder != "" {
			return fmt.Errorf("%w: a folder is named and the kind is not %q", ErrInvalidConfig, KindFolder)
		}
		return nil
	case KindFolder:
		return validateFolder(c.Folder, dataDir)
	default:
		return fmt.Errorf("%w: kind %q is not one this build knows", ErrInvalidConfig, c.Kind)
	}
}

func validateFolder(folder, dataDir string) error {
	if strings.TrimSpace(folder) == "" {
		return fmt.Errorf("%w: no folder is named", ErrInvalidConfig)
	}
	// A service starts in system32, and a relative folder there would put a
	// venue's backup somewhere nobody would look for it.
	if !filepath.IsAbs(folder) {
		return fmt.Errorf("%w: the folder %q is not a full path", ErrInvalidConfig, folder)
	}
	// A backup inside the data directory is lost with it, and is deleted by
	// whoever clears that directory to start again.
	if within(folder, dataDir) {
		return fmt.Errorf("%w: the folder %q is inside the data directory, and would be lost with it", ErrInvalidConfig, folder)
	}
	// The copies go in a folder of their own inside it, and that must not
	// be the data directory or hold it either: D:\mez as the destination
	// of a venue kept in D:\mez\db would write the backup into the venue.
	if copies := filepath.Join(folder, databaseDir); within(copies, dataDir) || within(dataDir, copies) {
		return fmt.Errorf("%w: the folder %q would put the backup in the data directory", ErrInvalidConfig, folder)
	}
	return nil
}

// within reports whether path is dir or somewhere under it, by their names
// alone. Windows does not tell D:\Mez from d:\mez, so neither does this.
func within(path, dir string) bool {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		path, dir = strings.ToLower(path), strings.ToLower(dir)
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false // different drives
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// LoadConfig reads the destination kept in dataDir. No file is no backup.
// A file that cannot be read as a destination is an error, not no backup:
// the owner who wrote it believes the venue is being backed up.
func LoadConfig(dataDir string) (Config, error) {
	path := filepath.Join(dataDir, ConfigFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("backup: reading %s: %w", path, err)
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	// A misspelt key would otherwise be a setting quietly not applied.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("backup: %s: %w: %w", path, ErrInvalidConfig, err)
	}
	if err := cfg.Validate(dataDir); err != nil {
		return Config{}, fmt.Errorf("backup: %s: %w", path, err)
	}
	return cfg, nil
}
