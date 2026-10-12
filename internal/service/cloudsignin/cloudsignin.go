// Package cloudsignin signs the owner in on mez Cloud's word (ADR-312,
// ADR-323, ADR-324): an assertion the cloud signed is checked with the key
// the venue's pairing holds, spent, and exchanged for the remote session a
// PIN from away opens.
//
// It holds the order of things and none of the rules. What an assertion is
// and whether one is good is internal/cloud's; who the cloud may be, and
// that an assertion is spent once, is internal/auth's.
package cloudsignin

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/db"
)

var (
	// ErrRefused is a sign-in the venue does not accept: it is not paired,
	// it has nobody for the cloud to be, or the assertion is not the
	// cloud's, not for this venue, or spent. Which is in the log, and is
	// not said to whoever presented it.
	ErrRefused = errors.New("the cloud's sign-in is not accepted")

	// ErrClock is an assertion rightly signed for this venue and outside
	// its time: this computer's clock is wrong, or it was held too long.
	ErrClock = errors.New("the cloud's sign-in is outside its time; check this computer's clock")
)

// Staff is what is needed of staff sign-in. *auth.StaffAuth is the
// implementation.
type Staff interface {
	SignInCloud(ctx context.Context, venueID int64, v auth.Vouched) (auth.SignedIn, error)
	CloudDevice(ctx context.Context, token string) (auth.Device, error)
	SignOut(ctx context.Context, s auth.Session) error
}

// Options configure a Service. DB, Staff and Key are required.
type Options struct {
	DB    *db.DB
	Staff Staff
	// Key is the cloud's public key from the pairing the venue runs on, and
	// false for a venue that is not paired. It is asked each time, so a
	// venue paired again checks with its new key from that moment.
	Key func() (ed25519.PublicKey, bool)
	Now func() time.Time
	Log *slog.Logger
}

// Service signs the owner in from mez Cloud.
type Service struct {
	opts Options
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil || o.Staff == nil || o.Key == nil {
		return nil, errors.New("cloudsignin: a database, staff sign-in and a key are required")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{opts: o}, nil
}

// SignedIn is a session mez Cloud opened, its token, and the device it is on.
type SignedIn struct {
	auth.SignedIn
	Device auth.Device
}

// SignIn exchanges an assertion, as presented, for a session. The errors are
// ErrRefused and ErrClock; anything else is this machine's fault.
func (s *Service) SignIn(ctx context.Context, presented string) (SignedIn, error) {
	key, paired := s.opts.Key()
	if !paired {
		s.opts.Log.Warn("a cloud sign-in was refused: this venue is not paired to mez Cloud")
		return SignedIn{}, ErrRefused
	}
	venues, err := s.opts.DB.Queries().ListVenues(ctx)
	if err != nil {
		return SignedIn{}, fmt.Errorf("cloudsignin: listing venues: %w", err)
	}

	// An assertion names its venue, and nothing in it is read before its
	// signature is checked. So it is checked against each venue here, of
	// which there is nearly always one, and belongs to the one it verifies
	// for.
	now := s.opts.Now()
	var (
		venueID int64
		vouched cloud.Assertion
		found   bool
	)
	for _, v := range venues {
		a, err := cloud.VerifyAssertion(key, presented, v.PublicID, now)
		// For this venue and rightly signed, but out of its time: that is
		// the answer, whatever the venues after it would say.
		if errors.Is(err, cloud.ErrAssertionTime) {
			s.opts.Log.Warn("a cloud sign-in was refused: rightly signed and outside its time; check this computer's clock",
				"venue_id", v.ID, "now", now.UTC())
			return SignedIn{}, ErrClock
		}
		if err == nil {
			venueID, vouched, found = v.ID, a, true
			break
		}
	}
	if !found {
		s.opts.Log.Warn("a cloud sign-in was refused: the assertion is not mez Cloud's for a venue here")
		return SignedIn{}, ErrRefused
	}

	in, err := s.opts.Staff.SignInCloud(ctx, venueID, auth.Vouched{Nonce: vouched.Nonce, KeepUntil: vouched.KeepUntil()})
	switch {
	case errors.Is(err, auth.ErrAssertionSpent), errors.Is(err, auth.ErrNoCloudOwner):
		// auth has said which in the log.
		return SignedIn{}, ErrRefused
	case err != nil:
		return SignedIn{}, fmt.Errorf("cloudsignin: %w", err)
	}
	dev, err := s.opts.Staff.CloudDevice(ctx, in.Token)
	if err != nil {
		// Revoked in the moment between: the session is no use to anyone,
		// and is closed and not left open with its token never sent.
		if out := s.opts.Staff.SignOut(context.WithoutCancel(ctx), in.Session); out != nil {
			s.opts.Log.Warn("a cloud session with no device to be on could not be closed", "error", out)
		}
		if errors.Is(err, auth.ErrUnauthenticated) {
			return SignedIn{}, ErrRefused
		}
		return SignedIn{}, fmt.Errorf("cloudsignin: finding the device of a session just opened: %w", err)
	}
	return SignedIn{SignedIn: in, Device: dev}, nil
}
