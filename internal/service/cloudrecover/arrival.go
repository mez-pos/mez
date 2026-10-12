package cloudrecover

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/restore"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
)

// ErrNoClaim is an outcome asked for with a claim that is not the one this
// arrival was staged with, with none, or with one that has run out. It is
// all a stranger is told.
var ErrNoClaim = errors.New("there is nothing to tell about a venue brought back")

const (
	// ClaimLife is how long after the start a claim is good for. The page
	// that holds it is waiting on that start; half an hour is a page left
	// open over a cup of tea, and not one found open the next morning.
	ClaimLife = 30 * time.Minute

	// closeWait is the longest a stop waits on a pairing still being asked
	// for. The stop's time is the database's and the backup's to close in
	// (ADR-273), and a pairing past its exchange is not cut short by being
	// asked (ADR-320): it is left, and what it could not write is its
	// warning to give.
	closeWait = 2 * time.Second
)

// CloudState is where the venue's pairing to its account stands after the
// start.
type CloudState string

const (
	// CloudPairing: the code that waited is being exchanged.
	CloudPairing CloudState = "pairing"
	// CloudPaired: the venue is paired to its account again.
	CloudPaired CloudState = "paired"
	// CloudNotPaired: it could not be. The venue is back all the same, and
	// is paired from Devices with a code from the account.
	CloudNotPaired CloudState = "not_paired"
	// CloudNone: there was no code: the restore did not come from a cloud.
	CloudNone CloudState = "none"
)

// CloudStates is every CloudState, for the API's vocabulary.
func CloudStates() []CloudState {
	return []CloudState{CloudPairing, CloudPaired, CloudNotPaired, CloudNone}
}

// Outcome is what the page that asked is told once the venue is back.
type Outcome struct {
	VenueID   string
	VenueName string
	Cloud     CloudState
	// Hostname is where the venue is reached from away, once paired.
	Hostname string
}

// Codes makes a device's pairing code. *auth.Devices is it.
type Codes interface {
	CreatePairingCode(ctx context.Context, n auth.NewCode) (auth.PairingCode, error)
}

// ArrivalOptions configure an Arrival. Adopted, VenueID and Codes are
// required.
type ArrivalOptions struct {
	// Adopted is what restore.Adopt returned as this server started.
	Adopted restore.Pending
	// VenueID is the adopted venue's id in its database.
	VenueID int64
	// Pair exchanges the code that waited. The pairing service's is it.
	// Nil is a server that cannot pair, and the venue is not paired.
	Pair  func(ctx context.Context, venueID int64, code string) (cloudpair.Result, error)
	Codes Codes
	// Now is the clock. Nil is time.Now.
	Now func() time.Time
	Log *slog.Logger
}

// Arrival is the half of a recovery after the start: a venue that this
// start put in place, for as long as this server runs.
type Arrival struct {
	opts ArrivalOptions

	// The pairing is one goroutine, begun by Arrive and ended by Close.
	cancel context.CancelFunc
	done   chan struct{}

	// until is when the claim stops being one.
	until time.Time

	mu       sync.Mutex
	cloud    CloudState
	hostname string
	code     *auth.PairingCode // the one code made for the claim
}

// Arrive begins what follows a venue being adopted. It returns at once: the
// pairing is asked for in the background, so that a computer with no
// internet is serving its venue all the same. Close ends it.
func Arrive(o ArrivalOptions) (*Arrival, error) {
	if o.Adopted.VenueID == "" || o.VenueID <= 0 || o.Codes == nil {
		return nil, errors.New("cloudrecover: an adopted venue, its id and device pairing are required")
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Arrival{opts: o, cancel: cancel, done: make(chan struct{}), cloud: CloudPairing, until: o.Now().Add(ClaimLife)}
	code := o.Adopted.PairingCode
	switch {
	case code == "":
		a.cloud = CloudNone
	case o.Pair == nil:
		a.cloud = CloudNotPaired
	}
	if a.cloud != CloudPairing {
		close(a.done)
		return a, nil
	}
	go a.pair(ctx, code)
	return a, nil
}

// pair exchanges the code that waited, once. There is no second try: a code
// the cloud has seen is spent, and one it never saw is good for the rest of
// its hour from the owner's account, typed in at Devices.
func (a *Arrival) pair(ctx context.Context, code string) {
	defer close(a.done)
	res, err := a.opts.Pair(ctx, a.opts.VenueID, code)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.cloud = CloudNotPaired
		a.opts.Log.Warn("the venue was brought back, and could not be paired to its mez Cloud account again; pair it from Devices with a code from the account",
			"venue_id", a.opts.VenueID, "error", err)
		return
	}
	a.cloud, a.hostname = CloudPaired, res.Hostname
	a.opts.Log.Info("the venue brought back is paired to its mez Cloud account again",
		"venue_id", a.opts.VenueID, "hostname", res.Hostname, "backup", res.Backup, "restart_needed", res.RestartNeeded)
}

// Close stops the pairing if it is still being asked for, and waits for it,
// a little.
func (a *Arrival) Close(ctx context.Context) error {
	a.cancel()
	wait := time.NewTimer(closeWait)
	defer wait.Stop()
	select {
	case <-a.done:
		return nil
	case <-wait.C:
		return errors.New("cloudrecover: the pairing was still being put to use, and was left to it")
	case <-ctx.Done():
		return fmt.Errorf("cloudrecover: the pairing did not stop: %w", ctx.Err())
	}
}

// claimed reports whether claim is the one this arrival was staged with.
func (a *Arrival) claimed(claim string) bool {
	want := a.opts.Adopted.ClaimHash
	return claim != "" && want != "" && a.opts.Now().Before(a.until) &&
		subtle.ConstantTimeCompare([]byte(HashClaim(claim)), []byte(want)) == 1
}

// Outcome tells the page that asked how the venue's return went. Anyone
// else, and a nil Arrival, which is a start that adopted nothing, is
// ErrNoClaim.
func (a *Arrival) Outcome(claim string) (Outcome, error) {
	if a == nil || !a.claimed(claim) {
		return Outcome{}, ErrNoClaim
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return Outcome{VenueID: a.opts.Adopted.VenueID, VenueName: a.opts.Adopted.VenueName, Cloud: a.cloud, Hostname: a.hostname}, nil
}

// DeviceCode makes the code that pairs the browser the owner is sitting at,
// which is no device of the venue's: its devices were the old computer's.
// One code to a claim: asked again, it is the same code, good or not. One
// that has run out is replaced at the machine, by `mez pair`.
//
// Its errors are ErrNoClaim and auth.Devices.CreatePairingCode's.
func (a *Arrival) DeviceCode(ctx context.Context, claim, deviceName string, kind auth.DeviceKind) (auth.PairingCode, error) {
	if a == nil || !a.claimed(claim) {
		return auth.PairingCode{}, ErrNoClaim
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.code != nil {
		return *a.code, nil
	}
	code, err := a.opts.Codes.CreatePairingCode(ctx, auth.NewCode{VenueID: a.opts.VenueID, DeviceName: deviceName, DeviceKind: kind})
	if err != nil {
		return auth.PairingCode{}, err
	}
	a.code = &code
	a.opts.Log.Info("a pairing code was made for the browser that brought the venue back", "venue_id", a.opts.VenueID, "device_kind", kind)
	return code, nil
}
