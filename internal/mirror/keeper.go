// Package mirror is the door of a mirror: who it has let in (ADR-326,
// ADR-327). A mirror serves a copy of a venue's backup and never writes to
// it, so what it must remember of its own is kept here, beside the copy:
// the browsers paired to it, who is signed in, and the wrong PINs typed.
//
// A Keeper is that memory, and outlives every copy the mirror serves. An
// Auth is the door over one copy: it reads people and their PINs from the
// copy and remembers through the Keeper.
package mirror

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/domain/id"
)

const (
	// MaxWrongPINs is how many wrong PINs in a row lock a person on a
	// mirror, and LockFor for how long. Nobody is watching a mirror, and
	// nobody at one can clear a lock, since clearing is a change: so it is
	// few, and it lifts by itself. Three a day is a thousand years of a
	// six-digit PIN.
	MaxWrongPINs = 3
	LockFor      = 24 * time.Hour

	// SessionTTL is how long a sign-in on a mirror lasts: what one from
	// away does (ADR-314).
	SessionTTL = 12 * time.Hour

	// CodeTTL is how long the pairing code a mirror prints is good for.
	CodeTTL = 10 * time.Minute

	// tokenPrefix marks a browser's token and a session's as a mirror's,
	// for whoever finds one in a log.
	deviceTokenPrefix  = "nfm_"
	sessionTokenPrefix = "nfms_"
	tokenBytes         = 32

	// spentMargin is how long past its end a spent assertion is still
	// remembered, as at the venue (ADR-323): against this computer's clock
	// having run fast and been put right.
	spentMargin = time.Hour

	// maxBrowsers bounds the browsers a mirror remembers. A code pairs one
	// and a mirror prints one code a start, so this is never near.
	maxBrowsers = 200
)

// Browser is a browser paired to a mirror.
type Browser struct {
	// ID is negative: it is an id among a venue's devices that no device of
	// the venue's can have.
	ID        int64     `json:"id"`
	PublicID  string    `json:"public_id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	PairedAt  time.Time `json:"paired_at"`
	// LastSeenAt is to the minute, and only as of the last time the file
	// was written for another reason: it is not worth a write of its own.
	LastSeenAt time.Time `json:"last_seen_at,omitzero"`
}

// strikes is one person's wrong PINs on a mirror.
type strikes struct {
	Wrong       int       `json:"wrong"`
	LockedUntil time.Time `json:"locked_until,omitzero"`
}

// kept is the file.
type kept struct {
	Browsers []Browser          `json:"browsers"`
	People   map[string]strikes `json:"people"`
}

// session is a sign-in on a mirror. It is not in the file: a mirror that
// is started again asks for PINs again.
type session struct {
	id        int64
	staffRef  string
	browserID int64
	expiresAt time.Time
	// cloud is whether mez Cloud opened it. Its browserID is then the id of
	// the cloud's device in the copy, which no browser paired here has.
	cloud bool
}

// Keeper is what a mirror remembers. It is safe for use by every request
// at once.
type Keeper struct {
	path    string
	now     func() time.Time
	entropy io.Reader

	// signing is held for the whole of a sign-in, one at a time across
	// every copy this mirror serves: a PIN is compared only once the one
	// before it has been counted. Without it, twenty guesses sent at once
	// would all be compared before the third was written down. A mirror has
	// a handful of people at its door, so one queue for all of them costs
	// nothing, and unlike a queue for each person it cannot be grown by
	// made-up names. It is a channel and not a mutex so that a request
	// whose caller has gone leaves the queue without its turn.
	signing chan struct{}

	mu   sync.Mutex
	kept kept
	// seen is the file as this process last read or wrote it, and unsaved
	// the people whose count here is newer than the file's because a write
	// failed. Another mirror of the same venue may be writing the file too
	// (ADR-327): it is read again whenever it has changed, and what is in
	// it stands, except for those people.
	seen    fileMark
	unsaved map[string]struct{}

	sessions map[string]session // by token hash
	// used is when a browser paired to this mirror, or a session it let in,
	// was last found good: the last request someone it let in made, which is
	// what keeps a mirror in use (ADR-356). A request anyone can send is not.
	used     time.Time
	lastID   int64
	codeHash string
	codeEnds time.Time
}

// KeeperOptions configure a Keeper. Path is required.
type KeeperOptions struct {
	// Path is the file the browsers and the locks are kept in. Its
	// directory is made if it is not there.
	Path    string
	Now     func() time.Time
	Entropy io.Reader
}

// OpenKeeper reads what a mirror remembered, or begins with nothing. A
// file that cannot be read as one is an error and is not written over: it
// holds locks, and a mirror that started without them would have lifted
// every one.
func OpenKeeper(o KeeperOptions) (*Keeper, error) {
	if o.Path == "" {
		return nil, errors.New("mirror: no file to remember in")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Entropy == nil {
		o.Entropy = rand.Reader
	}
	k := &Keeper{
		path: o.Path, now: o.Now, entropy: o.Entropy, sessions: map[string]session{},
		signing: make(chan struct{}, 1), unsaved: map[string]struct{}{},
	}
	k.kept.People = map[string]strikes{}

	read, mark, err := readKept(o.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(o.Path), 0o700); err != nil {
			return nil, fmt.Errorf("mirror: making %s: %w", filepath.Dir(o.Path), err)
		}
		return k, nil
	case err != nil:
		return nil, err
	}
	k.kept, k.seen = read, mark
	return k, nil
}

// fileMark tells one writing of the file from the next.
type fileMark struct {
	at   time.Time
	size int64
}

// readKept reads the file, and says which writing of it this was.
func readKept(path string) (kept, fileMark, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return kept{}, fileMark{}, err
		}
		return kept{}, fileMark{}, fmt.Errorf("mirror: reading %s: %w", path, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return kept{}, fileMark{}, fmt.Errorf("mirror: reading %s: %w", path, err)
	}
	var read kept
	if err := json.Unmarshal(raw, &read); err != nil {
		return kept{}, fileMark{}, fmt.Errorf("mirror: %s is not what a mirror wrote; remove it to start afresh, which unpairs every browser and lifts every lock: %w", path, err)
	}
	if read.People == nil {
		read.People = map[string]strikes{}
	}
	return read, fileMark{at: info.ModTime(), size: info.Size()}, nil
}

// refresh takes up what another mirror of this venue has written since
// this one last looked: a lock it set, a browser it paired. The caller
// holds mu. A file that has gone, or cannot be read just now, changes
// nothing: what this process remembers stands.
func (k *Keeper) refresh() {
	info, err := os.Stat(k.path)
	if err != nil || (info.ModTime().Equal(k.seen.at) && info.Size() == k.seen.size) {
		return
	}
	read, mark, err := readKept(k.path)
	if err != nil {
		return
	}
	// Counts this process could not write down are newer than the file.
	for ref := range k.unsaved {
		if s, ok := k.kept.People[ref]; ok {
			read.People[ref] = s
		} else {
			delete(read.People, ref)
		}
	}
	k.kept, k.seen = read, mark
}

// turn waits for this request's turn to sign in, and returns what gives
// the turn back. It gives up when the caller has.
func (k *Keeper) turn(done <-chan struct{}) (release func(), ok bool) {
	select {
	case k.signing <- struct{}{}:
		return func() { <-k.signing }, true
	case <-done:
		return nil, false
	}
}

// save writes the file whole and renames it into place, so that a reader
// sees the old or the new. The caller holds mu.
func (k *Keeper) save() error {
	raw, err := json.MarshalIndent(k.kept, "", "  ")
	if err != nil {
		return fmt.Errorf("mirror: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(k.path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("mirror: writing %s: %w", k.path, err)
	}
	_, werr := tmp.Write(append(raw, '\n'))
	if werr == nil {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	// Windows refuses a rename while a virus scanner is still reading the
	// file it has just seen appear, and lets it through a moment later.
	for try := 0; werr == nil; try++ {
		if werr = os.Rename(tmp.Name(), k.path); werr == nil || try == 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
		werr = nil
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("mirror: writing %s: %w", k.path, werr)
	}
	if info, err := os.Stat(k.path); err == nil {
		k.seen = fileMark{at: info.ModTime(), size: info.Size()}
	}
	k.unsaved = map[string]struct{}{}
	return nil
}

// newToken draws a token and returns it with its hash.
func (k *Keeper) newToken(prefix string) (token, hash string, err error) {
	var b [tokenBytes]byte
	if _, err := io.ReadFull(k.entropy, b[:]); err != nil {
		return "", "", fmt.Errorf("mirror: drawing a token: %w", err)
	}
	token = prefix + base64.RawURLEncoding.EncodeToString(b[:])
	return token, hashOf(token), nil
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// cleanCode is a pairing code as it is compared, read as every pairing
// code is: case, spaces, hyphens and the letters mistaken for digits. One
// that could not be a code compares equal to nothing.
func cleanCode(typed string) string {
	norm, err := id.Normalize(typed)
	if err != nil || len(norm) != id.PairingCodeLength {
		return ""
	}
	return norm
}

// NewCode makes the one pairing code this mirror takes, good for CodeTTL
// and once, and returns it as it is read out. A code made before it is
// good no longer.
func (k *Keeper) NewCode() (code string, expires time.Time, err error) {
	code, err = id.NewGenerator(k.entropy).PairingCode()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("mirror: drawing a pairing code: %w", err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.codeHash, k.codeEnds = hashOf(cleanCode(code)), k.now().Add(CodeTTL)
	// In two groups, as it is read out.
	return code[:len(code)/2] + "-" + code[len(code)/2:], k.codeEnds, nil
}

// errNoCode is a code that pairs nothing: not the one made, used, or old.
var errNoCode = errors.New("mirror: that code pairs nothing")

// pair spends the code on a new browser and returns it with its token.
func (k *Keeper) pair(typed, name string) (Browser, string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	clean := cleanCode(typed)
	if clean == "" || k.codeHash == "" || !k.now().Before(k.codeEnds) || hashOf(clean) != k.codeHash {
		return Browser{}, "", errNoCode
	}
	token, hash, err := k.newToken(deviceTokenPrefix)
	if err != nil {
		return Browser{}, "", err
	}
	publicID, err := id.NewPublic("dev")
	if err != nil {
		return Browser{}, "", fmt.Errorf("mirror: drawing an id: %w", err)
	}
	lowest := int64(0)
	for _, b := range k.kept.Browsers {
		lowest = min(lowest, b.ID)
	}
	b := Browser{ID: lowest - 1, PublicID: publicID, Name: name, TokenHash: hash, PairedAt: k.now().UTC()}
	was := k.kept.Browsers
	k.kept.Browsers = append(append([]Browser{}, was...), b)
	// The oldest go first, should there ever be this many.
	if over := len(k.kept.Browsers) - maxBrowsers; over > 0 {
		k.kept.Browsers = k.kept.Browsers[over:]
	}
	if err := k.save(); err != nil {
		k.kept.Browsers = was
		return Browser{}, "", err
	}
	// Spent only once the browser is remembered: a code that paired
	// nothing is still good.
	k.codeHash = ""
	return b, token, nil
}

// browser is the browser a token is for.
func (k *Keeper) browser(token string) (Browser, bool) {
	if !strings.HasPrefix(token, deviceTokenPrefix) {
		return Browser{}, false
	}
	hash := hashOf(token)
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	for i, b := range k.kept.Browsers {
		if b.TokenHash == hash {
			k.kept.Browsers[i].LastSeenAt = k.now().UTC().Truncate(time.Minute)
			k.used = k.now()
			return k.kept.Browsers[i], true
		}
	}
	return Browser{}, false
}

// Browsers is every browser paired to this mirror, oldest first.
func (k *Keeper) Browsers() []Browser {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	return append([]Browser{}, k.kept.Browsers...)
}

// locked reports whether a person is locked on this mirror now.
func (k *Keeper) locked(staffRef string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	return k.now().Before(k.kept.People[staffRef].LockedUntil)
}

// wrong counts a wrong PIN for someone who could have signed in, and
// reports whether it locked them. It is written down before it is
// answered: a strike that a restart forgot would be a free guess.
//
// A count that cannot be written down is kept all the same, and the error
// says so: the lock still falls on the third, in this process. A file
// that cannot be written must not be a way to guess for ever.
func (k *Keeper) wrong(staffRef string) (lockedNow bool, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	s := k.kept.People[staffRef]
	// A lock that has run out leaves a clean slate behind it.
	if !s.LockedUntil.IsZero() && !k.now().Before(s.LockedUntil) {
		s = strikes{}
	}
	s.Wrong++
	if s.Wrong >= MaxWrongPINs {
		s = strikes{LockedUntil: k.now().UTC().Add(LockFor)}
		lockedNow = true
	}
	k.kept.People[staffRef] = s
	k.unsaved[staffRef] = struct{}{}
	err = k.save()
	if lockedNow {
		// Whoever was guessing may have been at a browser the person had
		// left signed in.
		for hash, sess := range k.sessions {
			if sess.staffRef == staffRef {
				delete(k.sessions, hash)
			}
		}
	}
	return lockedNow, err
}

// right forgets a person's wrong PINs: they typed the right one. Forgotten
// here whether or not that could be written down.
func (k *Keeper) right(staffRef string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refresh()
	if _, had := k.kept.People[staffRef]; !had {
		return nil
	}
	delete(k.kept.People, staffRef)
	k.unsaved[staffRef] = struct{}{}
	return k.save()
}

// errSpent is an assertion presented a second time.
var errSpent = errors.New("mirror: that assertion has been used")

// spentDir is where the assertions mez Cloud has signed in with are
// remembered: a directory beside the file, one small file to an assertion.
func (k *Keeper) spentDir() string { return strings.TrimSuffix(k.path, ".json") + ".spent" }

// spend remembers an assertion's nonce until keepUntil, and is errSpent
// for one remembered already (ADR-331).
//
// Each is a file of its own, made only if it is not there: the one step
// that two mirrors of the same venue, or one started again, cannot both
// take. The browsers' file would not do: it is read, changed and written
// whole, and two processes doing that at once would both find the nonce
// new. It is on disk before it is answered, and one that cannot be put
// there is not accepted.
func (k *Keeper) spend(nonce string, keepUntil time.Time) error {
	dir := k.spentDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mirror: making %s: %w", dir, err)
	}
	k.forgetSpent(dir)
	// By its hash: a name of one case and no surprises, whatever was signed.
	name := filepath.Join(dir, hashOf(nonce))
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return errSpent
	}
	if err != nil {
		return fmt.Errorf("mirror: remembering an assertion: %w", err)
	}
	// Until when, for whoever tidies up. The file is the memory; this is
	// only how long to keep it.
	_, werr := f.WriteString(keepUntil.UTC().Format(time.RFC3339))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		// It stays, saying nothing of its age, and is tidied by its date.
		return fmt.Errorf("mirror: remembering an assertion: %w", werr)
	}
	return nil
}

// forgetSpent removes the assertions long dead: those past the time
// written in them and spentMargin more, and any that say no time and are a
// day old. One that cannot be read or removed is left for the next time:
// keeping too long harms nothing.
func (k *Keeper) forgetSpent(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := k.now()
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		until, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
		if err != nil {
			info, ierr := e.Info()
			if ierr != nil || now.Sub(info.ModTime()) < 24*time.Hour {
				continue
			}
		} else if now.Before(until.Add(spentMargin)) {
			continue
		}
		_ = os.Remove(path)
	}
}

// cloudSession is the live session a token opens, if mez Cloud opened it.
// A PIN's session is none, whatever its token.
func (k *Keeper) cloudSession(token string) (session, bool) {
	if !strings.HasPrefix(token, sessionTokenPrefix) {
		return session{}, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	s, ok := k.sessions[hashOf(token)]
	if !ok || !s.cloud || !k.now().Before(s.expiresAt) {
		return session{}, false
	}
	k.used = k.now()
	return s, true
}

// open begins a session for a person on a browser and returns its token.
// cloud is whether mez Cloud is opening it, on the cloud's device.
func (k *Keeper) open(staffRef string, browserID int64, cloud bool) (session, string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	token, hash, err := k.newToken(sessionTokenPrefix)
	if err != nil {
		return session{}, "", err
	}
	now := k.now()
	// Swept here, where there is a write to the map anyway.
	for h, s := range k.sessions {
		if !now.Before(s.expiresAt) {
			delete(k.sessions, h)
		}
	}
	k.lastID++
	s := session{id: k.lastID, staffRef: staffRef, browserID: browserID, expiresAt: now.UTC().Add(SessionTTL), cloud: cloud}
	k.sessions[hash] = s
	return s, token, nil
}

// session is the live session a token opens on a browser.
func (k *Keeper) session(token string, browserID int64) (session, bool) {
	if !strings.HasPrefix(token, sessionTokenPrefix) {
		return session{}, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	s, ok := k.sessions[hashOf(token)]
	if !ok || s.browserID != browserID || !k.now().Before(s.expiresAt) {
		return session{}, false
	}
	k.used = k.now()
	return s, true
}

// Used is when a request was last made by a browser paired to this mirror
// or on a session it let in, and zero if none has been (ADR-356).
func (k *Keeper) Used() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.used
}

// end closes a session by its id.
func (k *Keeper) end(id int64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for hash, s := range k.sessions {
		if s.id == id {
			delete(k.sessions, hash)
		}
	}
}
