// Package cloudrecover is "I had mez before" on a computer with no venue
// (ADR-332, ADR-333, ADR-334). In two halves, with a start of the server
// between them.
//
// Service is the half before: it signs the owner in to their mez Cloud
// account, holds that sign-in in memory, and for the venue they choose asks
// the cloud to hand it over, stages its backup beside the empty database,
// and asks the server to start again.
//
// Arrival is the half after: the start that adopted the venue pairs it to
// the account with the code that waited, and tells the page that asked, on
// its showing the claim it was given, how that went and how to pair the
// browser it is in.
package cloudrecover

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/restore"
)

var (
	// ErrAlreadySetUp is a recovery asked of a computer that has its venue.
	// A venue is never brought back over one.
	ErrAlreadySetUp = errors.New("this computer already has a venue")

	// ErrUnderWay is a second restore asked for while one is being staged,
	// or after one was and the server is about to start again.
	ErrUnderWay = errors.New("a venue is already being brought back on this computer")
)

// Cloud is what the service asks of mez Cloud. *cloud.Client is it.
type Cloud interface {
	RecoverStart(ctx context.Context, phone string) (cloud.Attempt, error)
	RecoverVerify(ctx context.Context, attempt cloud.Attempt, otp string) (*cloud.Recovery, error)
}

// Options configure a Service. Cloud, DataDir, Needed and StartAgain are
// required.
type Options struct {
	Cloud Cloud
	// DataDir is the data directory the server runs on.
	DataDir string
	// Needed reports whether this computer still has no venue. The first
	// run service's is it.
	Needed func(ctx context.Context) (bool, error)
	// StartAgain asks the server to start again, and returns at once. The
	// start is what adopts the staged venue (ADR-333).
	StartAgain func()
	// Stage stages a backup. Nil is restore.Stage.
	Stage func(ctx context.Context, o restore.StageOptions) (restore.Staged, error)
	Log   *slog.Logger
}

// Service is the half of a recovery before the start.
type Service struct {
	opts Options

	// mu guards who is signed in. One owner at a time: a second sign-in
	// takes the place of the first.
	mu        sync.Mutex
	signedIn  *cloud.Recovery
	restoring sync.Mutex // held while a venue is staged
	staged    bool       // and this is set once one has been
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.Cloud == nil || o.DataDir == "" || o.Needed == nil || o.StartAgain == nil {
		return nil, errors.New("cloudrecover: a cloud, a data directory, whether a venue is needed and a way to start again are required")
	}
	if o.Stage == nil {
		o.Stage = restore.Stage
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{opts: o}, nil
}

// needed refuses a computer that has its venue, before anything is asked of
// the cloud in its name.
func (s *Service) needed(ctx context.Context) error {
	needed, err := s.opts.Needed(ctx)
	if err != nil {
		return fmt.Errorf("cloudrecover: %w", err)
	}
	if !needed {
		return ErrAlreadySetUp
	}
	return nil
}

// Start asks mez Cloud to send a one-time code to the account's phone. Its
// errors are ErrAlreadySetUp and cloud.RecoverStart's.
func (s *Service) Start(ctx context.Context, phone string) (cloud.Attempt, error) {
	if err := s.needed(ctx); err != nil {
		return "", err
	}
	return s.opts.Cloud.RecoverStart(ctx, phone)
}

// Account is an owner signed in: who, and the venues that can be chosen.
type Account struct {
	Owner     string
	Venues    []cloud.AccountVenue
	ExpiresAt time.Time
}

// Verify gives the one-time code back and holds the sign-in it buys, in
// memory, in the place of any before it. Its errors are ErrAlreadySetUp and
// cloud.RecoverVerify's.
func (s *Service) Verify(ctx context.Context, attempt cloud.Attempt, otp string) (Account, error) {
	if err := s.needed(ctx); err != nil {
		return Account{}, err
	}
	r, err := s.opts.Cloud.RecoverVerify(ctx, attempt, otp)
	if err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	s.signedIn = r
	s.mu.Unlock()
	s.opts.Log.Info("an owner signed in to mez Cloud to bring a venue back", "recovery", r)
	return Account{Owner: r.Owner.Name, Venues: append([]cloud.AccountVenue(nil), r.Venues...), ExpiresAt: r.ExpiresAt}, nil
}

// Restored is a venue staged, with the server about to start again on it.
type Restored struct {
	VenueID   string
	VenueName string
	// Photos is how many menu photos came back, and PhotosIncomplete that
	// not all did. The venue's sales are back all the same.
	Photos           int
	PhotosIncomplete bool
	// Claim is what the page shows after the start to be told how it went
	// (Arrival.Outcome). It is given once, here, and kept nowhere: only its
	// hash waits with the staged venue.
	Claim string
}

// Restore brings one of the signed-in account's venues back: it is handed
// over by the cloud, its backup is staged beside the database, and the
// server is asked to start again.
//
// One at a time, and once: after a venue is staged every call is
// ErrUnderWay until the start. The other errors are ErrAlreadySetUp,
// cloud.ErrSessionOver for nobody signed in, Recovery.Grant's, and
// restore.Stage's. A staging that fails leaves the sign-in as it was, to be
// tried again.
func (s *Service) Restore(ctx context.Context, venueID string) (Restored, error) {
	if !s.restoring.TryLock() {
		return Restored{}, ErrUnderWay
	}
	defer s.restoring.Unlock()
	if s.staged {
		return Restored{}, ErrUnderWay
	}
	if err := s.needed(ctx); err != nil {
		return Restored{}, err
	}
	s.mu.Lock()
	r := s.signedIn
	s.mu.Unlock()
	if r == nil {
		return Restored{}, cloud.ErrSessionOver
	}

	grant, err := r.Grant(ctx, venueID)
	if err != nil {
		return Restored{}, err
	}
	claim, err := newClaim()
	if err != nil {
		return Restored{}, fmt.Errorf("cloudrecover: %w", err)
	}

	// The cloud has handed the venue over and made a pairing code for it.
	// From here the caller going away changes nothing: a browser closed as
	// the backup comes down must not leave the venue half brought back.
	// It is not left to go on for ever either: a line that has stopped
	// answering would hold every later try off until mez was restarted.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stageTimeout)
	defer cancel()
	staged, err := s.opts.Stage(ctx, restore.StageOptions{
		DataDir: s.opts.DataDir, From: backup.Config{Kind: backup.KindS3, S3: grant.Backup}, VenueID: grant.Venue.ID,
		PairingCode: grant.PairingCode, ClaimHash: HashClaim(claim),
	})
	if err != nil {
		s.opts.Log.Warn("a venue was handed over by mez Cloud and could not be staged", "grant", grant, "error", err)
		return Restored{}, err
	}
	if staged.PhotosErr != nil {
		s.opts.Log.Warn("a venue was staged, and not all of its menu photos", "venue", staged.VenueID, "photos", staged.Photos, "error", staged.PhotosErr)
	}

	// The sign-in has done what it was for. One made since is not this one.
	s.mu.Lock()
	if s.signedIn == r {
		s.signedIn = nil
	}
	s.mu.Unlock()
	s.staged = true
	s.opts.Log.Info("a venue was staged from its backup: starting again to put it in place", "venue", staged.VenueID, "name", staged.VenueName)
	s.opts.StartAgain()
	return Restored{
		VenueID: staged.VenueID, VenueName: staged.VenueName,
		Photos: staged.Photos, PhotosIncomplete: staged.PhotosErr != nil, Claim: claim,
	}, nil
}

// claimBytes is how much of a claim is random: as a session token's.
const claimBytes = 32

// stageTimeout is the longest a venue's backup may take to come down and be
// checked. A year of a busy cafe is tens of megabytes; this is for a bad
// line, not a large venue.
const stageTimeout = 30 * time.Minute

func newClaim() (string, error) {
	raw := make([]byte, claimBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// HashClaim is a claim as it is kept: the hash, in hex. A claim is random
// and long, so a plain hash of it is enough, as it is for a device's token.
func HashClaim(claim string) string {
	sum := sha256.Sum256([]byte(claim))
	return hex.EncodeToString(sum[:])
}
