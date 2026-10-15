// Package cloud is the venue's two meetings with mez Cloud that it starts
// itself: pairing (ADR-312, ADR-318), and asking for a venue that is gone
// (ADR-332, recover.go). Pairing first. A code from the owner's account, typed
// in at the venue once, is exchanged for what the venue needs to be reached
// from outside and backed up off the premises: a relay key, a hostname, a
// bucket's credentials, the cloud's public key and the owner's name.
//
// What comes back is kept in the data directory, beside the database, and
// nowhere else. Nothing here checks whether the venue has paid, and nothing
// in the binary does: the relay decides that (ADR-316).
//
// Imports: internal/backup, for the shape of a bucket. Never internal/relay:
// the venue's binary carries none of the relay's server.
package cloud

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
)

// File is where a pairing is kept, in the data directory.
//
// A file there and not a section of the settings file, which the phase file
// asked for (ADR-318). Nothing in mez writes its own settings file, and on
// Windows that file is the administrators' (ADR-310); the data directory is
// the server's own to write, is already kept from every other account
// (ADR-259), and is where the backup's destination and its secret already
// live for the same reasons (ADR-273). Not a row either: a restore has to
// know which account a venue belonged to before there is a database.
const File = "cloud.json"

// DefaultURL is mez Cloud's API.
const DefaultURL = "https://api.mezpos.com"

// CodeLen is how long a pairing code is, and codeAlphabet what it is made
// of: capitals and digits with the four that are misread for each other on
// a phone screen left out (0 and O, 1 and I).
const (
	CodeLen      = 8
	codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// ErrCodeShape is a pairing code that could not be one, as typed. It is
// said before anything is sent: a code is good once, and a mistyped one is
// not worth an owner's hour of waiting for the next.
var ErrCodeShape = fmt.Errorf("a pairing code is %d letters and digits", CodeLen)

// CleanCode is a pairing code as it is sent: capitals, with the spaces and
// hyphens a person adds when reading one out taken away.
func CleanCode(typed string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToUpper(typed) {
		switch {
		case r == ' ' || r == '-' || r == '\t':
		case strings.ContainsRune(codeAlphabet, r):
			b.WriteRune(r)
		default:
			return "", ErrCodeShape
		}
	}
	if b.Len() != CodeLen {
		return "", ErrCodeShape
	}
	return b.String(), nil
}

// Pairing is what mez Cloud gave this venue when it was paired. It holds two
// secrets, the relay key and the bucket's: it is never logged whole and
// never sent anywhere.
type Pairing struct {
	// PairedAt is when, UTC, and CloudURL which cloud.
	PairedAt time.Time `json:"paired_at"`
	CloudURL string    `json:"cloud_url"`

	Venue Venue `json:"venue"`
	Relay Relay `json:"relay"`
	// Backup is the bucket this venue's backup goes to, and the key to it,
	// good for this venue's own folder in it and nothing else.
	Backup backup.S3 `json:"backup"`
	// PublicKey is the cloud's Ed25519 public key, base64. An owner signed
	// in to the cloud is vouched for with it (Phase 14, Task 5).
	PublicKey string `json:"public_key"`
	Owner     Owner  `json:"owner"`
}

// Venue is the venue as the cloud knows it.
type Venue struct {
	// Slug is its name on the relay, and Hostname where it is reached.
	Slug     string `json:"slug"`
	Hostname string `json:"hostname"`
}

// Relay is the relay the cloud runs for this venue, and the venue's key to
// it. The key is a secret.
type Relay struct {
	URL string `json:"url"`
	Key string `json:"key"`
}

// Owner is the account the venue was paired to, by the name it gave.
type Owner struct {
	Name string `json:"name"`
}

// PublicURL is where guests, and the owner from away, reach the venue.
func (p Pairing) PublicURL() string { return "https://" + p.Venue.Hostname }

// Key is the cloud's public key, decoded.
func (p Pairing) Key() (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(p.PublicKey)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("the public key is not an Ed25519 key in base64")
	}
	return ed25519.PublicKey(raw), nil
}

// WithKey is the pairing holding key as the cloud's public key in place of
// the one it holds: the cloud's next, which the venue was led to by links
// its own key signed (ADR-345). The pairing is otherwise as it was.
func (p Pairing) WithKey(key ed25519.PublicKey) Pairing {
	p.PublicKey = base64.StdEncoding.EncodeToString(key)
	return p
}

// LogValue is what a log line says of a pairing: who and where, and neither
// secret.
func (p Pairing) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("venue", p.Venue.Slug), slog.String("hostname", p.Venue.Hostname),
		slog.String("owner", p.Owner.Name), slog.Time("paired_at", p.PairedAt))
}

// String keeps the secrets out of an error or a %v that prints the whole.
func (p Pairing) String() string {
	return fmt.Sprintf("{venue %s at %s, paired %s}", p.Venue.Slug, p.Venue.Hostname, p.PairedAt.UTC().Format(time.RFC3339))
}

// GoString covers %#v, which goes around String.
func (p Pairing) GoString() string { return "cloud.Pairing" + p.String() }

// ErrInvalid is a pairing that cannot be used as written: an answer from
// the cloud that is not what a cloud answers, or a file someone edited.
var ErrInvalid = errors.New("the pairing is not usable")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

const (
	maxOwnerName = 100
	// keyPrefix and keyScope are how a relay key that carries the back
	// office reads: "mzr_<venue>.<key id>.remote.<mac>" (ADR-306). The
	// words are the relay's, restated: this package does not import it.
	keyPrefix = "mzr_"
	keyScope  = "remote"
)

// Validate checks a pairing before it is kept or used. It calls nobody: a
// key the relay will refuse, or a bucket that is not there, is found out by
// the first dial and the first copy, and said on the screens that show
// those.
func (p Pairing) Validate() error {
	if !validLabel(p.Venue.Slug) {
		return invalid("%q is not a venue's name on a relay", p.Venue.Slug)
	}
	if !validHostname(p.Venue.Hostname) {
		return invalid("%q is not a hostname", p.Venue.Hostname)
	}
	if err := secureURL(p.Relay.URL); err != nil {
		return invalid("the relay's address: %v", err)
	}
	// The venue is paired to have its back office reached, so the key must
	// be one the relay issued for that, and for this venue.
	parts := strings.Split(strings.TrimPrefix(p.Relay.Key, keyPrefix), ".")
	if !strings.HasPrefix(p.Relay.Key, keyPrefix) || len(parts) != 4 || parts[2] != keyScope {
		return invalid("the relay key is not one issued for a back office")
	}
	if parts[0] != p.Venue.Slug {
		return invalid("the relay key is for another venue")
	}
	if err := (backup.Config{Kind: backup.KindS3, S3: p.Backup}).Validate(""); err != nil {
		return invalid("the backup's bucket: %v", err)
	}
	// The key the cloud hands over is good for this venue's folder alone,
	// so there must be one: the default would be every venue's.
	if p.Backup.Path == "" {
		return invalid("the backup's bucket has no folder for this venue")
	}
	if _, err := p.Key(); err != nil {
		return invalid("%v", err)
	}
	if !validName(p.Owner.Name, maxOwnerName) {
		return invalid("the owner's name is empty, or not a name")
	}
	if p.PairedAt.IsZero() {
		return invalid("it does not say when it was paired")
	}
	if err := secureURL(p.CloudURL); err != nil {
		return invalid("the cloud's address: %v", err)
	}
	return nil
}

// validLabel is one name between the dots of a hostname: lower-case letters,
// digits and hyphens, not starting or ending with a hyphen, 63 at most.
func validLabel(s string) bool {
	if s == "" || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func validHostname(h string) bool {
	if len(h) > 253 || !strings.Contains(h, ".") {
		return false
	}
	for label := range strings.SplitSeq(h, ".") {
		if !validLabel(label) {
			return false
		}
	}
	return true
}

// secureURL accepts an address a secret may be sent to: https, or plain http
// to this machine alone, which is a test's and a developer's. Nothing after
// the host: the paths are this package's and the relay client's to add.
func secureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return fmt.Errorf("%q is not an address like https://host", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if loopback(u.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf("%q must be https: a key travels on this connection", raw)
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CheckURL says whether raw may be set as the cloud's address.
func CheckURL(raw string) error { return secureURL(raw) }

// Load reads the pairing kept in dataDir. No file is no pairing, and not an
// error: it is how nearly every venue is. A file that cannot be read as a
// pairing is an error: the owner believes the venue is connected.
func Load(dataDir string) (*Pairing, error) {
	path := filepath.Join(dataDir, File)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cloud: reading %s: %w", path, err)
	}
	var p Pairing
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("cloud: %s: %w: %w", path, ErrInvalid, err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("cloud: %s: %w", path, err)
	}
	return &p, nil
}

// Save keeps a pairing in dataDir, for Load to find. It replaces the one
// there: a venue paired again has a new key, and the old is revoked. The
// file is written whole and renamed into place, readable by its owner only
// where modes mean something; on Windows the data directory's own
// permissions keep it (ADR-259).
func Save(dataDir string, p Pairing) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("cloud: %w", err)
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("cloud: %w", err)
	}
	path := filepath.Join(dataDir, File)
	if err := writeWhole(path, append(raw, '\n')); err != nil {
		return fmt.Errorf("cloud: writing %s: %w", path, err)
	}
	return nil
}

// Remove forgets the pairing kept in dataDir. None there is not an error.
func Remove(dataDir string) error {
	path := filepath.Join(dataDir, File)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cloud: removing %s: %w", path, err)
	}
	return nil
}

// writeWhole writes body to path so that a reader sees the old file or the
// new and never half of one. CreateTemp makes it 0600 and the rename keeps
// that.
func writeWhole(path string, body []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(body); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
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
