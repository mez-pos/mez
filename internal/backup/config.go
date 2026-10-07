package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
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
	// KindS3 is a bucket: Amazon's S3, Cloudflare's R2, or anything else
	// that speaks S3.
	KindS3 Kind = "s3"
)

// Kinds is every Kind, for the API's vocabulary.
func Kinds() []Kind { return []Kind{KindOff, KindFolder, KindS3} }

// Config is where the backup goes.
type Config struct {
	Kind Kind `json:"kind"`
	// Folder is the destination of a KindFolder backup, absolute.
	Folder string `json:"folder,omitempty"`
	// S3 is the destination of a KindS3 backup.
	S3 S3 `json:"s3,omitzero"`
}

// defaultS3Path is where in a bucket the backup goes when no path is
// given. Never the bucket's root: a bucket is often shared, and the
// backup is pruned by the program that wrote it.
const defaultS3Path = "mez"

// S3 is a bucket and what it takes to write to it.
type S3 struct {
	// Bucket is the bucket's name.
	Bucket string `json:"bucket"`
	// Path is the folder inside the bucket. Empty is "mez".
	Path string `json:"path,omitempty"`
	// Region is the bucket's region. Amazon needs it; with an Endpoint it
	// may be left out.
	Region string `json:"region,omitempty"`
	// Endpoint is the address of a service that is not Amazon's, with its
	// scheme: for R2, https://<account>.r2.cloudflarestorage.com.
	Endpoint string `json:"endpoint,omitempty"`
	// AccessKeyID and SecretAccessKey are the key the bucket is written
	// with. The secret is a secret: it is never logged and never printed.
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// prefix is the folder inside the bucket, with the default applied.
func (s S3) prefix() string {
	if s.Path == "" {
		return defaultS3Path
	}
	return s.Path
}

// String and LogValue keep the secret out of a log line or an error that
// prints the whole destination.
func (s S3) String() string {
	return fmt.Sprintf("{Bucket:%s Path:%s Region:%s Endpoint:%s AccessKeyID:%s SecretAccessKey:%s}",
		s.Bucket, s.Path, s.Region, s.Endpoint, s.AccessKeyID, redact(s.SecretAccessKey))
}

// GoString covers %#v, which goes around String.
func (s S3) GoString() string { return "backup.S3" + s.String() }

func (s S3) LogValue() slog.Value { return slog.StringValue(s.String()) }

func redact(secret string) string {
	if secret == "" {
		return ""
	}
	return "[hidden]"
}

// LogValue is what a log line says of the whole destination. Without it
// a JSON log would write the struct out field by field, secret and all.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(slog.String("kind", string(c.Kind)), slog.String("destination", c.Destination()))
}

// Destination names where the backup goes, for a log line or a person:
// the folder, or the bucket and path. Never the key.
func (c Config) Destination() string {
	switch c.Kind {
	case KindFolder:
		return c.Folder
	case KindS3:
		where := "s3://" + c.S3.Bucket + "/" + c.S3.prefix()
		if c.S3.Endpoint != "" {
			where += " at " + c.S3.Endpoint
		}
		return where
	default:
		return ""
	}
}

// ErrInvalidConfig is a destination that cannot be used as written. What is
// wrong with it is in the message.
var ErrInvalidConfig = errors.New("the backup destination is not usable")

// InvalidConfig is an ErrInvalidConfig that knows which part is wrong, so
// that a screen can put the reason beside the box it is about. Field is
// the name the part has in backup.json.
type InvalidConfig struct {
	Field  string
	Reason string
}

func (e *InvalidConfig) Error() string { return ErrInvalidConfig.Error() + ": " + e.Reason }

func (e *InvalidConfig) Is(target error) bool { return target == ErrInvalidConfig }

func invalidf(field, format string, args ...any) error {
	return &InvalidConfig{Field: field, Reason: fmt.Sprintf(format, args...)}
}

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
			return invalidf("folder", "a folder is named and the kind is not %q", KindFolder)
		}
		if c.S3 != (S3{}) {
			return invalidf("bucket", "a bucket is named and the kind is not %q", KindS3)
		}
		return nil
	case KindFolder:
		if c.S3 != (S3{}) {
			return invalidf("bucket", "a bucket is named and the kind is %q", KindFolder)
		}
		return validateFolder(c.Folder, dataDir)
	case KindS3:
		if c.Folder != "" {
			return invalidf("folder", "a folder is named and the kind is %q", KindS3)
		}
		return c.S3.validate()
	default:
		return invalidf("kind", "kind %q is not one this build knows", c.Kind)
	}
}

func validateFolder(folder, dataDir string) error {
	if strings.TrimSpace(folder) == "" {
		return invalidf("folder", "no folder is named")
	}
	// A service starts in system32, and a relative folder there would put a
	// venue's backup somewhere nobody would look for it.
	if !filepath.IsAbs(folder) {
		return invalidf("folder", "the folder %s is not a full path", folder)
	}
	// A backup inside the data directory is lost with it, and is deleted by
	// whoever clears that directory to start again.
	if within(folder, dataDir) {
		return invalidf("folder", "the folder %s is inside the data directory, and would be lost with it", folder)
	}
	// The copies go in a folder of their own inside it, and that must not
	// be the data directory or hold it either: D:\mez as the destination
	// of a venue kept in D:\mez\db would write the backup into the venue.
	if copies := filepath.Join(folder, databaseDir); within(copies, dataDir) || within(dataDir, copies) {
		return invalidf("folder", "the folder %s would put the backup in the data directory", folder)
	}
	return nil
}

// validate checks a bucket as written. It calls nobody: a key that is
// wrong, or a bucket that is not there, is found out by the first copy.
func (s S3) validate() error {
	if !validBucket(s.Bucket) {
		return invalidf("bucket", "%q is not a bucket's name (3 to 63 of a-z, 0-9, dot and hyphen)", s.Bucket)
	}
	if s.AccessKeyID == "" || s.SecretAccessKey == "" {
		// Litestream would go looking in the environment and in the
		// user's own AWS files, and a service's account has neither.
		return invalidf("secret_access_key", "access_key_id and secret_access_key are not both set")
	}
	if p := s.Path; p != "" {
		if strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.Contains(p, "//") || strings.Contains(p, "\\") {
			return invalidf("path", "the path %q must be like venue/backups, with no slash at either end", p)
		}
		for _, part := range strings.Split(p, "/") {
			if part == "." || part == ".." || strings.TrimSpace(part) != part || part == "" {
				return invalidf("path", "the path %q has a part that is not a name", p)
			}
		}
	}
	if s.Endpoint == "" {
		// Without one Litestream asks Amazon where the bucket is, as it
		// starts, and a venue with no internet that morning would not open.
		if s.Region == "" {
			return invalidf("region", "a bucket at Amazon needs its region, and one elsewhere needs its endpoint")
		}
		return nil
	}
	// Asked of the text, before it is parsed: an address that does not
	// parse still has the key in it, and is not repeated back either.
	if strings.Contains(s.Endpoint, "@") {
		return invalidf("endpoint", "the endpoint must not carry a key; the key goes in access_key_id and secret_access_key")
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" || strings.Trim(u.Path, "/") != "" {
		return invalidf("endpoint", "the endpoint %q must be an address like https://host, and nothing after it", s.Endpoint)
	}
	return nil
}

// validBucket is the common rule of S3 and the services that copy it.
func validBucket(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	// The start counts as a dot, so a name opens with neither mark.
	prev := byte('.')
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && prev != '.':
		case c == '.' && prev != '.' && prev != '-':
		default:
			return false
		}
		prev = c
	}
	return prev != '.' && prev != '-'
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

// SaveConfig keeps the destination in dataDir, for LoadConfig to find. Off
// is no file. The file is written whole and renamed into place, readable
// by its owner only where modes mean something: it may hold a bucket's
// secret. On Windows the data directory's own permissions keep it
// (ADR-259).
func SaveConfig(dataDir string, cfg Config) error {
	if err := cfg.Validate(dataDir); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	path := filepath.Join(dataDir, ConfigFile)
	if !cfg.Enabled() {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("backup: removing %s: %w", path, err)
		}
		return nil
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("backup: writing %s: %w", path, err)
	}
	// CreateTemp makes it 0600, and the rename keeps that.
	if err := writeWhole(path, append(raw, '\n')); err != nil {
		return fmt.Errorf("backup: writing %s: %w", path, err)
	}
	return nil
}
