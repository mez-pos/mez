package mirror

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/service/closeofday"
)

// ReplicaOptions configure a Replica. From is required.
type ReplicaOptions struct {
	// From is where the backup is: a folder or a bucket. It is only read.
	From backup.Config
	// Venue is the public id of the venue the caller expects the backup
	// to hold, and a copy that holds another is refused. Empty takes
	// whichever venue the first copy holds.
	Venue string
	// CopyDir is where each copy is restored, in a directory of its own
	// that begins with DirPrefix. Empty is the system's temporary directory.
	CopyDir string
	// Log is told what goes wrong. Nil is nowhere.
	Log *slog.Logger
	// Patience is how long a copy's database is given to close. Zero is
	// DefaultPatience.
	Patience time.Duration
}

// Replica is a venue's backup restored to be read for its figures, and
// brought up to the backup's newest write when asked (ADR-362): what mez
// Cloud's reader follows for the owner's messages (Phase 15). It is a
// mirror less its door, its server and its photos: the same copy, the
// same check, the same swap, with nobody to answer from the old copy.
//
// A Replica is used by one goroutine: Refresh puts the copy away under
// whoever is still reading it. The caller reads, then refreshes, then
// reads again.
type Replica struct {
	src     *source
	venue   string
	current *mirrorCopy
	// refused is the last write that could not be made into a copy, and
	// when: it is not fetched again for retryAfter.
	refused   backup.Generation
	refusedAt time.Time
	closed    bool
}

// ErrReplicaClosed is a Replica asked something after Close.
var ErrReplicaClosed = errors.New("mirror: the replica is closed")

// OpenReplica restores the newest copy of the backup, checks it and brings
// it up to this build's schema. Nothing is left behind when it fails. It
// holds exactly one venue or it is refused.
func OpenReplica(ctx context.Context, o ReplicaOptions) (_ *Replica, err error) {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Patience <= 0 {
		o.Patience = DefaultPatience
	}
	src := &source{from: o.From, copyDir: o.CopyDir, log: o.Log, patience: o.Patience}
	generation, err := src.newest(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}
	first, err := src.open(ctx, generation)
	if err != nil {
		return nil, err
	}
	if o.Venue != "" && first.venue.PublicID != o.Venue {
		return nil, errors.Join(
			fmt.Errorf("mirror: the backup holds the venue %s, and the replica was opened for %s", first.venue.PublicID, o.Venue),
			first.close())
	}
	return &Replica{src: src, venue: first.venue.PublicID, current: first}, nil
}

// Venue is the venue in the copy.
func (r *Replica) Venue() Venue {
	return Venue{PublicID: r.current.venue.PublicID, Name: r.current.venue.Name, Timezone: r.current.venue.Timezone}
}

// AsOf is when the venue last wrote something the copy holds.
func (r *Replica) AsOf() time.Time { return r.current.asOf }

// Refresh looks at the backup and, when it has been written since the copy
// was made, restores afresh and puts the old copy away. It reports whether
// the copy changed. Whatever goes wrong leaves the copy where it is,
// growing older, and is returned: the next Refresh tries again, except
// for a write that could not be made into a copy, which is left alone for
// a while so a backup this build cannot read is not fetched whole at every
// look.
func (r *Replica) Refresh(ctx context.Context) (bool, error) {
	if r.closed {
		return false, ErrReplicaClosed
	}
	newest, err := r.src.newest(ctx, "")
	if err != nil {
		return false, fmt.Errorf("mirror: looking at the backup: %w", err)
	}
	if newest.Name == r.current.generation && !newest.Last.After(r.current.asOf) {
		return false, nil
	}
	if newest.Name == r.refused.Name && newest.Last.Equal(r.refused.Last) && time.Since(r.refusedAt) < retryAfter {
		return false, nil
	}
	next, err := r.src.open(ctx, newest)
	if err == nil && next.venue.PublicID != r.venue {
		err = errors.Join(
			fmt.Errorf("mirror: the backup now holds the venue %s, and this replica was opened on %s", next.venue.PublicID, r.venue),
			next.close())
	}
	if err != nil {
		if ctx.Err() == nil {
			r.refused, r.refusedAt = newest, time.Now()
		}
		return false, err
	}
	old := r.current
	r.current = next
	if err := old.close(); err != nil {
		r.src.log.Error("a copy the replica had finished with could not be put away", "error", err)
	}
	return true, nil
}

// Closes is the venue's close-of-day figures for each Z-report issued
// after the one with id afterID, oldest first, at most limit of them
// (ADR-360). Zero is from the beginning.
func (r *Replica) Closes(ctx context.Context, afterID, limit int64) ([]closeofday.Close, error) {
	if r.closed {
		return nil, ErrReplicaClosed
	}
	return closeofday.Since(ctx, r.current.database.Queries(), r.current.venue.ID, afterID, limit)
}

// Close puts the copy away. A directory that could not be removed is said
// in the error: it is a copy of a venue's sales.
func (r *Replica) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.current.close()
}
