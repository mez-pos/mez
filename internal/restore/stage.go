package restore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
)

const (
	// StagedSuffix is added to the database's name for a restored database
	// that waits beside it to be adopted.
	StagedSuffix = ".restored"

	// PendingFile, in the data directory, says a staged database is whole,
	// checked, and to be adopted at the next start. It is written last:
	// a staged database with no such file is one that was never finished.
	PendingFile = "restore.json"

	photosDir = "photos"
)

var (
	// ErrWrongVenue is a backup that holds a venue other than the one asked
	// for. Nothing of it is kept.
	ErrWrongVenue = errors.New("the backup holds a different venue from the one asked for")

	// ErrNoVenueInBackup is a backup made before its venue was: there is
	// nothing in it to bring back.
	ErrNoVenueInBackup = errors.New("the backup has no venue in it")

	// ErrPending is a restore.json that cannot be used: unreadable, or with
	// no staged database beside it.
	ErrPending = errors.New("the restore that was waiting cannot be used")
)

// Pending is a restore that waits for the next start. PairingCode is a
// secret for as long as it is good: a pending restore is never logged or
// printed whole.
type Pending struct {
	// VenueID is the public id of the venue in the staged database, and
	// VenueName its name.
	VenueID   string    `json:"venue_id"`
	VenueName string    `json:"venue_name"`
	StagedAt  time.Time `json:"staged_at"`
	// PairingCode pairs the venue to its mez Cloud account once it is up
	// (ADR-332). Empty for a restore that came from nowhere of the cloud's.
	PairingCode string `json:"pairing_code,omitempty"`
	// ClaimHash is the hash of what the page that asked for the restore
	// must show, after the start, to be told how it went (ADR-334). The
	// claim itself is never written. Empty is nobody to tell.
	ClaimHash string `json:"claim_hash,omitempty"`
}

// String keeps the code out of an error or a %v that prints the whole.
func (p Pending) String() string {
	return fmt.Sprintf("{restore of %s (%s), staged %s}", p.VenueName, p.VenueID, p.StagedAt.UTC().Format(time.RFC3339))
}

// GoString covers %#v, which goes around String.
func (p Pending) GoString() string { return "restore.Pending" + p.String() }

// LogValue is what a log line says of one: which venue and when.
func (p Pending) LogValue() slog.Value {
	return slog.GroupValue(slog.String("venue", p.VenueID), slog.String("name", p.VenueName), slog.Time("staged_at", p.StagedAt))
}

// StageOptions say what to stage. DataDir, From and VenueID are required.
type StageOptions struct {
	// DataDir is the data directory the server is running on.
	DataDir string
	// From is where the backup is, with a key that need only read it.
	From backup.Config
	// VenueID is the public id of the venue the backup must hold.
	VenueID string
	// PairingCode is kept for after the next start. It may be empty.
	PairingCode string
	// ClaimHash is kept with it, as given. It may be empty.
	ClaimHash string
	// Now is the clock. Nil is time.Now.
	Now func() time.Time
}

// Staged is a restore that is ready for the next start. The pairing code is
// not in it: it is what a screen is told.
type Staged struct {
	VenueID   string
	VenueName string
	StagedAt  time.Time
	// Photos is how many menu photos were brought back, and PhotosErr why
	// not all of them, if not. The venue's sales are staged all the same.
	Photos    int
	PhotosErr error
}

// Stage restores the newest backup at From into a file beside the database,
// checks it holds the venue asked for, and leaves word for the next start to
// adopt it. The server may be running on the data directory, and must be on
// an empty install or on nothing: a restore never writes over a venue.
//
// It writes nothing the running server reads but the venue's photos, which
// an install with no venue has none of. One Stage at a time is the caller's
// to see to. An earlier one that was left unfinished is cleared first.
func Stage(ctx context.Context, o StageOptions) (Staged, error) {
	if o.DataDir == "" || o.VenueID == "" || !o.From.Enabled() {
		return Staged{}, errors.New("restore: a data directory, a backup and the venue it must hold are required")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	dbPath := filepath.Join(o.DataDir, db.FileName)
	// A database with a venue in it is ErrVenueThere. An empty one and none
	// at all are both somewhere a venue can be brought back to.
	if _, err := EmptyInstall(ctx, dbPath); err != nil {
		return Staged{}, err
	}
	if err := clearStaged(o.DataDir); err != nil {
		return Staged{}, err
	}

	staged := dbPath + StagedSuffix
	if err := backup.Restore(ctx, o.From, staged); err != nil {
		_ = clearStaged(o.DataDir)
		return Staged{}, fmt.Errorf("restore: %w", err)
	}
	name, err := venueIn(ctx, staged, o.VenueID)
	if err != nil {
		_ = clearStaged(o.DataDir)
		return Staged{}, err
	}

	// From here the venue's sales are staged. The photos can fail without
	// taking that away, and say so.
	out := Staged{VenueID: o.VenueID, VenueName: name, StagedAt: o.Now().UTC().Truncate(time.Second)}
	out.Photos, out.PhotosErr = backup.RestorePhotos(ctx, o.From, filepath.Join(o.DataDir, photosDir))

	raw, err := json.MarshalIndent(Pending{VenueID: out.VenueID, VenueName: out.VenueName, StagedAt: out.StagedAt, PairingCode: o.PairingCode, ClaimHash: o.ClaimHash}, "", "  ")
	if err != nil {
		_ = clearStaged(o.DataDir)
		return Staged{}, fmt.Errorf("restore: %w", err)
	}
	if err := writeWhole(filepath.Join(o.DataDir, PendingFile), append(raw, '\n')); err != nil {
		_ = clearStaged(o.DataDir)
		return Staged{}, fmt.Errorf("restore: leaving word of the staged database: %w", err)
	}
	return out, nil
}

// venueIn reads the name of the one venue a staged database holds, which
// must be the venue wanted. The file is read as it lies and nothing is made
// beside it: it is not this binary's to migrate until it is adopted.
func venueIn(ctx context.Context, staged, wantID string) (string, error) {
	probe, err := sql.Open("sqlite", "file:"+staged+"?mode=ro&immutable=1")
	if err != nil {
		return "", fmt.Errorf("restore: the restored database cannot be opened: %w", err)
	}
	defer probe.Close()

	rows, err := probe.QueryContext(ctx, "SELECT public_id, name FROM venues ORDER BY id")
	if err != nil {
		return "", fmt.Errorf("restore: the restored database cannot be read: %w", err)
	}
	defer rows.Close()
	found, name := 0, ""
	for rows.Next() {
		var id, n string
		if err := rows.Scan(&id, &n); err != nil {
			return "", fmt.Errorf("restore: the restored database cannot be read: %w", err)
		}
		found++
		if id == wantID {
			name = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("restore: the restored database cannot be read: %w", err)
	}
	switch {
	case found == 0:
		return "", fmt.Errorf("restore: %w", ErrNoVenueInBackup)
	case name == "" || found != 1:
		// What it does hold is not said: it is somebody's venue.
		return "", fmt.Errorf("restore: %w", ErrWrongVenue)
	}
	return name, nil
}

// Adopt puts a staged database in the database's place, if one is waiting.
// It is called as mez starts, before anything opens the database, and only
// then: nothing else has the file.
//
// Nothing waiting is nil and no error, which is every start but one. A
// database with a venue in it is never moved: the restore is dropped, with
// ErrVenueThere, and the staged file left for a person to look at. Whatever
// the error, the start can go on with the database that is there.
//
// The word left for it is removed as it is adopted, so the pairing code it
// held is from then on only in what Adopt returns.
func Adopt(ctx context.Context, dataDir string, now time.Time) (*Pending, error) {
	pendingPath := filepath.Join(dataDir, PendingFile)
	// What a staging cut short as it left word may have left: it holds the
	// code and is word of nothing.
	_ = os.Remove(pendingPath + tmpSuffix)
	raw, err := os.ReadFile(pendingPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("restore: %w: reading %s: %w", ErrPending, pendingPath, err)
	}
	dbPath := filepath.Join(dataDir, db.FileName)
	staged := dbPath + StagedSuffix

	var p Pending
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil || p.VenueID == "" {
		// Not tried again at every start: the word is removed. The staged
		// file, if there is one, is not a database anyone vouched for.
		_ = os.Remove(pendingPath)
		return nil, fmt.Errorf("restore: %w: %s is not what was left there", ErrPending, pendingPath)
	}
	if _, err := os.Stat(staged); errors.Is(err, fs.ErrNotExist) {
		// Either the staged file was removed by hand, or a start was cut
		// short between putting it in place and removing this word.
		_ = os.Remove(pendingPath)
		return nil, fmt.Errorf("restore: %w: %s is not there. If the venue is here, an earlier start put it in place and was cut short: pair it to mez Cloud again from Devices", ErrPending, staged)
	} else if err != nil {
		// Not known to be gone, so the word is left for the next start.
		return nil, fmt.Errorf("restore: %w: looking for the staged database: %w", ErrPending, err)
	}

	empty, err := EmptyInstall(ctx, dbPath)
	if err != nil {
		if errors.Is(err, ErrVenueThere) {
			// Someone finished the wizard, or ran `mez restore`, in between.
			// Theirs stands. The code is not left lying in a file.
			_ = os.Remove(pendingPath)
		}
		return nil, err
	}
	if empty {
		// Holding the port is not holding the database: a mez on another
		// port, or one still closing down, may have it. The word is left,
		// and the next start asks again.
		if err := NotRunning(ctx, dbPath); err != nil {
			return nil, err
		}
		if _, err := MoveAside(dbPath, now); err != nil {
			return nil, err
		}
	} else if err := clearBeside(dbPath); err != nil {
		// No database, and still there may be the log of one that was
		// here: deleted by hand, or moved aside by a start that was cut
		// short before it cleared up. Beside the restored database it
		// would be read as that database's own.
		return nil, err
	}
	if err := os.Rename(staged, dbPath); err != nil {
		// The word is left: the next start tries again.
		return nil, fmt.Errorf("restore: putting the restored database in place: %w", err)
	}
	if err := os.Remove(pendingPath); err != nil {
		// Harmless to the venue: at the next start there is no staged file,
		// and the word is removed then.
		return &p, fmt.Errorf("restore: the restored database is in place, and %s could not be removed: %w", pendingPath, err)
	}
	return &p, nil
}

// clearStaged removes a staged database and the word left for it, and what
// a restore that stopped half way leaves beside them.
func clearStaged(dataDir string) error {
	staged := filepath.Join(dataDir, db.FileName+StagedSuffix)
	// The word first: without it, a staged file is never adopted.
	word := filepath.Join(dataDir, PendingFile)
	for _, path := range []string{word, word + tmpSuffix, staged, staged + "-wal", staged + "-shm", staged + ".tmp"} {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("restore: clearing %s: %w", path, err)
		}
	}
	return nil
}

// tmpSuffix names the file word is written in before it is renamed into
// place. One name and not a random one: it holds the code, and what is cut
// short must be findable to be removed.
const tmpSuffix = ".tmp"

// writeWhole writes body to path so that a reader sees the old file or the
// new and never half of one. It is made 0600 and the rename keeps that; on
// Windows the data directory's own permissions keep it (ADR-259).
func writeWhole(path string, body []byte) (err error) {
	if err := os.Remove(path + tmpSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp, err := os.OpenFile(path+tmpSuffix, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
