// Package cloudpair pairs a running venue to a mez Cloud account (ADR-318,
// ADR-320): a code typed at the venue is exchanged for a pairing, the
// pairing is kept, and the venue starts using it.
//
// A code is good once, so the order matters. Everything that can refuse is
// asked before the code is sent. Once it has been sent and accepted, nothing
// after it is allowed to lose the pairing: it is kept first, and what then
// goes wrong is reported beside a pairing that stands.
package cloudpair

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/db"
)

// ErrRelayInSettings is a venue whose settings file names a relay of its
// own. Its pairing would be set aside at the next start (ADR-319), so it is
// refused before the code is spent, with what to do.
var ErrRelayInSettings = errors.New("this venue's settings file names a relay of its own (relay_url); remove relay_url and relay_key from it and restart mez, then pair")

// ErrNotKept is a pairing mez Cloud gave and this machine could not write
// down. The code is spent: the owner makes another.
var ErrNotKept = errors.New("mez Cloud accepted the code, but the pairing could not be saved on this computer; make another code and try again")

// ErrNoVenue is a venue id with no venue behind it.
var ErrNoVenue = errors.New("no such venue")

// Exchanger exchanges a pairing code. *cloud.Client is the implementation.
type Exchanger interface {
	Pair(ctx context.Context, code string, about cloud.About) (cloud.Pairing, error)
}

// Backups is the venue's backup. *backup.Manager is the implementation.
type Backups interface {
	Config() backup.Config
	Set(ctx context.Context, cfg backup.Config) (restartNeeded bool, err error)
}

// BackupOutcome is what pairing did about the venue's backup.
type BackupOutcome string

const (
	// BackupSet: the backup now goes to the bucket the pairing gave.
	BackupSet BackupOutcome = "set"
	// BackupKept: the venue already backs up somewhere of its own, and
	// that was left alone.
	BackupKept BackupOutcome = "kept"
	// BackupFailed: the bucket the pairing gave could not be written to.
	// The venue is paired all the same.
	BackupFailed BackupOutcome = "failed"
	// BackupNone: this server has no backup to set.
	BackupNone BackupOutcome = "none"
)

// BackupOutcomes is every BackupOutcome, for the API's vocabulary.
func BackupOutcomes() []BackupOutcome {
	return []BackupOutcome{BackupSet, BackupKept, BackupFailed, BackupNone}
}

// Result is a venue that has been paired.
type Result struct {
	Owner    string
	Hostname string
	PairedAt time.Time
	Backup   BackupOutcome
	// RestartNeeded says some part of the pairing takes hold only when mez
	// next starts.
	RestartNeeded bool
}

// Options configure a Service. DB, DataDir and Cloud are required.
type Options struct {
	DB *db.DB
	// DataDir is where the pairing is kept.
	DataDir string
	// Version is this build's, said to the cloud.
	Version string
	Cloud   Exchanger
	// Backups is the venue's backup. Nil is a server with none.
	Backups Backups
	// RelayInSettings says the settings file names a relay.
	RelayInSettings bool
	// Apply puts a pairing that has been kept into use: the tunnel, the
	// venue's address. It reports whether a restart is needed for it. Nil
	// is a server that only reads its pairing at start.
	Apply func(ctx context.Context, p cloud.Pairing) (restartNeeded bool, err error)
	// Owner makes the one person mez Cloud may sign in as, named for the
	// account, or brings them back (ADR-323). Nil is a server where the
	// cloud signs nobody in.
	Owner func(ctx context.Context, venueID int64, name string) error
	Log   *slog.Logger
}

// Service pairs the venue.
type Service struct {
	opts Options
	// One pairing at a time: two would each spend a code and the second
	// would revoke the first's key while it was being put to use.
	mu sync.Mutex
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil || o.DataDir == "" || o.Cloud == nil {
		return nil, errors.New("cloudpair: a database, a data directory and a cloud are required")
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{opts: o}, nil
}

// Pair exchanges a code for a pairing and puts it to use.
//
// The errors before the code is sent are ErrRelayInSettings, ErrNoVenue and
// cloud.ErrCodeShape; from the exchange, cloud.ErrCodeRefused, cloud.ErrBusy,
// cloud.ErrUnreachable and cloud.ErrInvalid; and after it, only ErrNotKept.
// Anything else that goes wrong once the pairing is kept is in the Result.
func (s *Service) Pair(ctx context.Context, venueID int64, typed string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opts.RelayInSettings {
		return Result{}, ErrRelayInSettings
	}
	code, err := cloud.CleanCode(typed)
	if err != nil {
		return Result{}, err
	}
	venue, err := s.opts.DB.Queries().VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNoVenue
	}
	if err != nil {
		return Result{}, fmt.Errorf("cloudpair: reading the venue: %w", err)
	}

	p, err := s.opts.Cloud.Pair(ctx, code, cloud.About{VenueID: venue.PublicID, Name: venue.Name, Version: s.opts.Version})
	if err != nil {
		return Result{}, err
	}

	// The code is spent. From here on the caller going away changes
	// nothing: a browser closed mid-pairing must not leave a key that was
	// issued and never written down.
	ctx = context.WithoutCancel(ctx)
	if err := cloud.Save(s.opts.DataDir, p); err != nil {
		s.opts.Log.Error("a pairing was accepted and could not be kept", "pairing", p, "error", err)
		return Result{}, ErrNotKept
	}
	s.opts.Log.Info("this venue was paired to mez Cloud", "pairing", p, "venue_id", venueID)
	out := Result{Owner: p.Owner.Name, Hostname: p.Venue.Hostname, PairedAt: p.PairedAt}

	// Who the cloud may sign in as. Not being able to make them does not
	// unpair the venue: it is reached and backed up all the same, and a
	// paired venue with nobody is given them when mez next starts.
	if s.opts.Owner != nil {
		if err := s.opts.Owner(ctx, venueID, p.Owner.Name); err != nil {
			s.opts.Log.Warn("the venue was paired, and the person mez Cloud signs in as could not be made; it is tried again when mez next starts",
				"venue_id", venueID, "error", err)
		}
	}

	if s.opts.Apply == nil {
		out.RestartNeeded = true
	} else if restart, err := s.opts.Apply(ctx, p); err != nil {
		// Kept, and read at the next start whatever went wrong here.
		s.opts.Log.Warn("the pairing was kept and could not be put to use; it is used when mez next starts", "error", err)
		out.RestartNeeded = true
	} else if restart {
		out.RestartNeeded = true
	}

	restart, outcome := s.backUp(ctx, p)
	out.Backup = outcome
	out.RestartNeeded = out.RestartNeeded || restart
	return out, nil
}

// backUp points the venue's backup at the bucket the pairing gave, unless
// the venue already backs up somewhere of its own.
//
// A venue with a folder or a bucket it chose keeps it: that was the owner's
// decision and pairing is not asked to undo it. A venue with no backup gets
// one. And a venue already on the pairing's bucket, which is a venue paired
// again, gets the new key: the old one has been revoked.
func (s *Service) backUp(ctx context.Context, p cloud.Pairing) (restartNeeded bool, outcome BackupOutcome) {
	if s.opts.Backups == nil {
		return false, BackupNone
	}
	now := s.opts.Backups.Config()
	theirs := now.Kind == backup.KindS3 && now.S3.Bucket == p.Backup.Bucket &&
		now.S3.Endpoint == p.Backup.Endpoint && now.S3.Path == p.Backup.Path
	if now.Enabled() && !theirs {
		return false, BackupKept
	}
	restart, err := s.opts.Backups.Set(ctx, backup.Config{Kind: backup.KindS3, S3: p.Backup})
	if err != nil {
		// The venue is paired and reached all the same. The Backup screen
		// says what a backup that is off or failing says.
		s.opts.Log.Warn("the venue was paired, and its backup could not be set to the bucket the pairing gave", "error", err)
		return restart, BackupFailed
	}
	return restart, BackupSet
}
