package mirror

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/service/cloudsignin"
)

// BrowserName is what a browser paired to a mirror is called.
const BrowserName = "Mirror"

// ErrReadOnly is a change asked of a mirror's door: making a code,
// renaming or revoking a device. The API refuses these before they reach
// here (ADR-326); this is what is said if one does.
var ErrReadOnly = errors.New("a mirror changes nothing")

// Staff is what a mirror reads of people from its copy.
// *auth.StaffAuth is the implementation.
type Staff interface {
	ActiveStaff(ctx context.Context, venueID int64) ([]auth.Staff, error)
	WithPermission(ctx context.Context, venueID int64, p auth.Permission) ([]auth.Staff, error)
	Permissions(ctx context.Context, roleID int64) (auth.PermissionSet, error)
	CheckPIN(ctx context.Context, venueID int64, staffRef, pin string) (auth.Staff, auth.PINVerdict, error)
	CloudSeat(ctx context.Context, venueID int64) (auth.Staff, auth.Device, error)
}

// Devices is what a mirror reads of the venue's devices from its copy.
// *auth.Devices is the implementation.
type Devices interface {
	Roster(ctx context.Context, venueID int64) (auth.Roster, error)
}

// Options configure an Auth. Keeper, Staff, Devices and VenueID are
// required.
type Options struct {
	Keeper *Keeper
	// Staff and Devices read the copy this door is over.
	Staff   Staff
	Devices Devices
	// VenueID is the one venue the copy holds.
	VenueID int64
	// CloudKey is mez Cloud's public key, for a mirror that lets the
	// account's owner in on the cloud's word (ADR-331), and VenuePublicID
	// the venue an assertion must be for. With no key the cloud signs
	// nobody in here. The key is not in the backup: it is beside the
	// venue's database, not in it.
	CloudKey      ed25519.PublicKey
	VenuePublicID string
	Now           func() time.Time
	Log           *slog.Logger
}

// Auth is a mirror's door over one copy. It is the API's devices and its
// staff sign-in both: who works at the venue, their PINs and what their
// roles grant are read from the copy; the browsers paired here, the
// sessions and the wrong PINs are the Keeper's. It writes nothing to the
// copy.
type Auth struct {
	keeper  *Keeper
	staff   Staff
	devices Devices
	venueID int64
	key     ed25519.PublicKey
	venue   string
	now     func() time.Time
	log     *slog.Logger
}

// NewAuth returns the door over a copy.
func NewAuth(o Options) (*Auth, error) {
	if o.Keeper == nil || o.Staff == nil || o.Devices == nil || o.VenueID == 0 {
		return nil, errors.New("mirror: a keeper, a copy's staff and devices, and its venue are required")
	}
	if len(o.CloudKey) != 0 && (len(o.CloudKey) != ed25519.PublicKeySize || o.VenuePublicID == "") {
		return nil, errors.New("mirror: mez Cloud's key must be an Ed25519 public key, with the venue's public id beside it")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Auth{
		keeper: o.Keeper, staff: o.Staff, devices: o.Devices, venueID: o.VenueID,
		key: o.CloudKey, venue: o.VenuePublicID, now: o.Now, log: o.Log,
	}, nil
}

func (a *Auth) deviceOf(b Browser) auth.Device {
	return auth.Device{ID: b.ID, PublicID: b.PublicID, VenueID: a.venueID, Name: b.Name, Kind: auth.KindBackOffice}
}

// Authenticate finds the browser a token was given to when it was paired to
// this mirror. A device of the venue's own opens nothing here, whatever the
// copy says of it: it may have been revoked since the backup was written.
func (a *Auth) Authenticate(_ context.Context, token string) (auth.Device, error) {
	b, ok := a.keeper.browser(token)
	if !ok {
		return auth.Device{}, auth.ErrUnauthenticated
	}
	return a.deviceOf(b), nil
}

// Redeem spends the mirror's pairing code on a new browser.
func (a *Auth) Redeem(_ context.Context, code string) (auth.Paired, error) {
	b, token, err := a.keeper.pair(code, BrowserName)
	if errors.Is(err, errNoCode) {
		return auth.Paired{}, auth.ErrInvalidPairingCode
	}
	if err != nil {
		return auth.Paired{}, err
	}
	a.log.Info("a browser was paired to the mirror", "device_id", b.ID)
	return auth.Paired{Device: a.deviceOf(b), Token: token}, nil
}

// Roster is the venue's devices as the copy has them, and after them the
// browsers paired to this mirror, so the one asking can be told which it is.
func (a *Auth) Roster(ctx context.Context, venueID int64) (auth.Roster, error) {
	roster, err := a.devices.Roster(ctx, venueID)
	if err != nil {
		return auth.Roster{}, err
	}
	// Codes waiting at the venue when the backup was written pair nothing
	// here.
	out := auth.Roster{Devices: append([]auth.Record{}, roster.Devices...)}
	for _, b := range a.keeper.Browsers() {
		rec := auth.Record{Device: a.deviceOf(b), PairedAt: b.PairedAt}
		if !b.LastSeenAt.IsZero() {
			seen := b.LastSeenAt
			rec.LastSeenAt = &seen
		}
		out.Devices = append(out.Devices, rec)
	}
	return out, nil
}

// CreatePairingCode, Rename and RevokeByPublicID are changes.

func (a *Auth) CreatePairingCode(context.Context, auth.NewCode) (auth.PairingCode, error) {
	return auth.PairingCode{}, ErrReadOnly
}

func (a *Auth) Rename(context.Context, int64, string, string, int64) error { return ErrReadOnly }

func (a *Auth) RevokeByPublicID(context.Context, int64, string, int64, int64) error {
	return ErrReadOnly
}

// ActiveStaff, WithPermission and Permissions are read from the copy.

func (a *Auth) ActiveStaff(ctx context.Context, venueID int64) ([]auth.Staff, error) {
	return a.staff.ActiveStaff(ctx, venueID)
}

func (a *Auth) WithPermission(ctx context.Context, venueID int64, p auth.Permission) ([]auth.Staff, error) {
	return a.staff.WithPermission(ctx, venueID, p)
}

func (a *Auth) Permissions(ctx context.Context, roleID int64) (auth.PermissionSet, error) {
	return a.staff.Permissions(ctx, roleID)
}

// SignIn checks a PIN against the copy and opens a session in the mirror's
// memory. A wrong PIN for someone who could have signed in is counted, and
// the MaxWrongPINs-th in a row locks them on this mirror for LockFor.
//
// Every refusal reads alike, as at the venue: nobody there, a wrong PIN,
// and a right PIN for someone locked here are all ErrInvalidPIN, and each
// takes as long as the others.
func (a *Auth) SignIn(ctx context.Context, dev auth.Device, staffRef, pin string) (auth.SignedIn, error) {
	// The cloud's device is for the one person with no PIN (ADR-324). The
	// API refuses this before it reaches here; it is refused here too.
	if dev.Cloud {
		return auth.SignedIn{}, auth.ErrInvalidPIN
	}
	release, ok := a.keeper.turn(ctx.Done())
	if !ok {
		return auth.SignedIn{}, ctx.Err()
	}
	defer release()

	who, verdict, err := a.staff.CheckPIN(ctx, dev.VenueID, staffRef, pin)
	if err != nil {
		return auth.SignedIn{}, err
	}
	// Counted and locked by the id the copy knows the person by, not by
	// what was typed: one person has one count however they are named.
	// Someone who is nobody has no count to keep.
	if verdict != auth.PINNobody {
		staffRef = who.PublicID
	}
	// Asked after the PIN was compared, so the answer is no quicker for it,
	// and before anything is counted: a lock is not made longer by trying.
	if a.keeper.locked(staffRef) {
		a.log.Warn("PIN refused on the mirror: that person is locked here", "device_id", dev.ID)
		return auth.SignedIn{}, auth.ErrInvalidPIN
	}
	switch verdict {
	case auth.PINNobody:
		return auth.SignedIn{}, auth.ErrInvalidPIN
	case auth.PINWrong:
		// Counted whether or not it could be written down, and answered
		// as a wrong PIN either way: an error here would tell a wrong PIN
		// from a right one, with no lock behind it.
		locked, err := a.keeper.wrong(staffRef)
		if err != nil {
			a.log.Error("a wrong PIN on the mirror was counted and could not be written down; a restart will forget it", "error", err)
		}
		if locked {
			a.log.Warn("someone is locked on the mirror after wrong PINs", "device_id", dev.ID, "staff", staffRef, "for", LockFor.String())
		}
		return auth.SignedIn{}, auth.ErrInvalidPIN
	}
	if err := a.keeper.right(staffRef); err != nil {
		a.log.Error("wrong PINs forgiven on the mirror could not be written down as forgiven", "error", err)
	}
	s, token, err := a.keeper.open(who.PublicID, dev.ID, false)
	if err != nil {
		return auth.SignedIn{}, err
	}
	a.log.Info("PIN accepted on the mirror", "device_id", dev.ID, "actor_id", who.ID)
	return auth.SignedIn{Session: a.sessionOf(s, who), Token: token}, nil
}

// SignInRemote is SignIn: everyone at a mirror is away from the venue.
func (a *Auth) SignInRemote(ctx context.Context, dev auth.Device, staffRef, pin string) (auth.SignedIn, error) {
	return a.SignIn(ctx, dev, staffRef, pin)
}

func (a *Auth) sessionOf(s session, who auth.Staff) auth.Session {
	return auth.Session{ID: s.id, Staff: who, DeviceID: s.browserID, ExpiresAt: s.expiresAt, Remote: true, Cloud: s.cloud}
}

// Session finds the session a token opens on this browser. Who it belongs
// to is read from the copy each time, so someone the venue deactivated or
// locked, or moved to another role, is that from the next copy on.
func (a *Auth) Session(ctx context.Context, dev auth.Device, token string) (auth.Session, error) {
	s, ok := a.keeper.session(token, dev.ID)
	// The cloud's session is carried by the cloud's device and no other,
	// and that device carries no other session.
	if !ok || s.cloud != dev.Cloud {
		return auth.Session{}, auth.ErrNoSession
	}
	if s.cloud {
		who, _, err := a.cloudSeat(ctx, s)
		if errors.Is(err, auth.ErrUnauthenticated) {
			return auth.Session{}, auth.ErrNoSession
		}
		if err != nil {
			return auth.Session{}, err
		}
		return a.sessionOf(s, who), nil
	}
	// Those who can sign in: a short list, and already without the
	// deactivated, the locked and whoever mez Cloud signs in.
	active, err := a.staff.ActiveStaff(ctx, dev.VenueID)
	if err != nil {
		return auth.Session{}, err
	}
	for _, who := range active {
		if who.PublicID == s.staffRef {
			return a.sessionOf(s, who), nil
		}
	}
	a.keeper.end(s.id)
	return auth.Session{}, auth.ErrNoSession
}

// SignOut ends a session.
func (a *Auth) SignOut(_ context.Context, s auth.Session) error {
	a.keeper.end(s.ID)
	return nil
}

// Authorise is a manager's PIN for one action. Nothing on a mirror takes
// one, and it is refused as a wrong PIN is, uncounted.
func (a *Auth) Authorise(context.Context, auth.Device, string, string, auth.Permission) (auth.Staff, error) {
	return auth.Staff{}, auth.ErrInvalidPIN
}

// CloudDevice is the cloud's device, found by the token of a session mez
// Cloud opened here (ADR-324, ADR-331): the device as the copy has it. A
// token that is not such a session's, or whose person or device the venue
// has since turned off, is ErrUnauthenticated.
func (a *Auth) CloudDevice(ctx context.Context, token string) (auth.Device, error) {
	s, ok := a.keeper.cloudSession(token)
	if !ok {
		return auth.Device{}, auth.ErrUnauthenticated
	}
	_, dev, err := a.cloudSeat(ctx, s)
	return dev, err
}

// cloudSeat is who a cloud session belongs to and the device it is on, read
// from the copy each time: the session stands only while the copy still has
// that person active and that device not revoked. A later copy in which the
// venue deactivated the one or revoked the other ends it, and so does one
// in which pairing again made a new device.
func (a *Auth) cloudSeat(ctx context.Context, s session) (auth.Staff, auth.Device, error) {
	who, dev, err := a.staff.CloudSeat(ctx, a.venueID)
	if err != nil && !errors.Is(err, auth.ErrNoCloudOwner) {
		return auth.Staff{}, auth.Device{}, err
	}
	if err != nil || who.PublicID != s.staffRef || dev.ID != s.browserID {
		a.keeper.end(s.id)
		return auth.Staff{}, auth.Device{}, auth.ErrUnauthenticated
	}
	return who, dev, nil
}

// CloudDoor is how mez Cloud signs the account's owner in on a mirror. It
// is the API's CloudSignIn, as cloudsignin.Service is at the venue.
type CloudDoor struct{ a *Auth }

// Cloud returns the door mez Cloud signs in at.
func (a *Auth) Cloud() *CloudDoor { return &CloudDoor{a: a} }

// SignIn exchanges an assertion, as presented, for a session in the
// mirror's memory, on the cloud's device as the copy has it. The errors are
// cloudsignin.ErrRefused and cloudsignin.ErrClock, as at the venue; which
// refusal it was is in the log and is not said.
//
// It is the venue's sign-in (ADR-323) with nothing written to the copy:
// the assertion is checked with the key this mirror was started with, its
// nonce is remembered by the Keeper, and who the cloud may be is read. A
// newer key the assertion's links lead to is used and not kept: the
// mirror's key is the one it was started with, and the links are presented
// again with every assertion (ADR-345).
func (c *CloudDoor) SignIn(ctx context.Context, presented string) (cloudsignin.SignedIn, error) {
	a := c.a
	if len(a.key) == 0 {
		a.log.Warn("a cloud sign-in was refused: this mirror was started with no key of mez Cloud's")
		return cloudsignin.SignedIn{}, cloudsignin.ErrRefused
	}
	now := a.now()
	vouched, _, err := cloud.VerifyAssertion(a.key, presented, a.venue, now)
	if errors.Is(err, cloud.ErrAssertionTime) {
		a.log.Warn("a cloud sign-in was refused: rightly signed and outside its time; check this computer's clock", "now", now.UTC())
		return cloudsignin.SignedIn{}, cloudsignin.ErrClock
	}
	if err != nil {
		a.log.Warn("a cloud sign-in was refused: the assertion is not mez Cloud's for this venue")
		return cloudsignin.SignedIn{}, cloudsignin.ErrRefused
	}
	// Who the cloud may be is asked before the assertion is spent: one
	// presented to a copy with nobody for it to be is not used up.
	who, dev, err := a.staff.CloudSeat(ctx, a.venueID)
	if errors.Is(err, auth.ErrNoCloudOwner) {
		a.log.Warn("a cloud sign-in was refused: the copy has nobody the cloud may be")
		return cloudsignin.SignedIn{}, cloudsignin.ErrRefused
	}
	if err != nil {
		return cloudsignin.SignedIn{}, err
	}
	if err := a.keeper.spend(vouched.Nonce, vouched.KeepUntil()); err != nil {
		if errors.Is(err, errSpent) {
			a.log.Warn("a cloud sign-in was refused: the assertion has been used")
			return cloudsignin.SignedIn{}, cloudsignin.ErrRefused
		}
		return cloudsignin.SignedIn{}, err
	}
	s, token, err := a.keeper.open(who.PublicID, dev.ID, true)
	if err != nil {
		return cloudsignin.SignedIn{}, err
	}
	a.log.Info("the owner signed in to the mirror from mez Cloud", "device_id", dev.ID, "actor_id", who.ID, "by", "cloud")
	return cloudsignin.SignedIn{SignedIn: auth.SignedIn{Session: a.sessionOf(s, who), Token: token}, Device: dev}, nil
}
