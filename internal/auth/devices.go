package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
)

// PairingCodeTTL is how long a pairing code works. Long enough to walk from
// the office to the counter; short enough that a code on a sticky note is
// dead by the time anyone finds it.
const PairingCodeTTL = 10 * time.Minute

// lastSeenEvery is how stale devices.last_seen_at may get. Writing it on every
// request would put a write in front of every read; once a minute is enough
// to tell a live terminal from one in a drawer.
const lastSeenEvery = time.Minute

// maxDraws bounds the redraw when a new code or public ID collides.
const maxDraws = 5

// maxDeviceName is the longest device name accepted, in characters.
const maxDeviceName = 64

// DeviceKind is what a device is. The schema knows the same four; a guest
// session is the fourth and is not a paired device.
type DeviceKind string

// The kinds a device can pair as.
const (
	KindTerminal   DeviceKind = "terminal"
	KindKitchen    DeviceKind = "kitchen"
	KindBackOffice DeviceKind = "back_office"
	KindGuest      DeviceKind = "guest"
)

var (
	// ErrInvalidPairingCode means a code cannot be redeemed. Unknown,
	// expired and already used all return it, so a guesser learns nothing
	// about which codes exist.
	ErrInvalidPairingCode = errors.New("pairing code is not valid")

	// ErrUnauthenticated means no device token, or one that is not a token
	// of any device.
	ErrUnauthenticated = errors.New("not authenticated")

	// ErrDeviceRevoked means the token belongs to a device the venue has
	// revoked. It is told apart from ErrUnauthenticated so the device can
	// say "this terminal was unpaired" rather than "try again".
	ErrDeviceRevoked = errors.New("device has been revoked")

	// ErrDeviceNotFound means no such device at the venue.
	ErrDeviceNotFound = errors.New("device not found")

	// ErrInvalidDevice means a pairing code was asked for with a device name
	// or kind that cannot be used.
	ErrInvalidDevice = errors.New("invalid device")
)

// Device is an authenticated device: who is making the request.
type Device struct {
	ID       int64
	PublicID string
	VenueID  int64
	Name     string
	Kind     DeviceKind
}

// NewCode is a request for a pairing code: the venue, who is asking (0 when
// no staff member is, for a venue's first device), and what the device will
// be.
type NewCode struct {
	VenueID    int64
	CreatedBy  int64
	DeviceName string
	DeviceKind DeviceKind
}

// PairingCode is a code as shown to the person pairing: grouped for reading,
// with when it stops working.
type PairingCode struct {
	Code      string
	ExpiresAt time.Time
}

// Paired is a freshly paired device and its token. The token is shown once,
// here, and never again: only its hash is kept.
type Paired struct {
	Device Device
	Token  string
}

// DevicesOptions configure Devices. DB is required.
type DevicesOptions struct {
	DB      *db.DB
	Now     func() time.Time // nil is time.Now
	Entropy io.Reader        // nil is crypto/rand; tests inject a fixed source
	Log     *slog.Logger
}

// Devices pairs and authenticates devices.
type Devices struct {
	db      *db.DB
	now     func() time.Time
	entropy io.Reader
	ids     *id.Generator
	log     *slog.Logger
}

// NewDevices returns a Devices.
func NewDevices(opts DevicesOptions) (*Devices, error) {
	if opts.DB == nil {
		return nil, errors.New("auth: no database")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Entropy == nil {
		opts.Entropy = rand.Reader
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Devices{
		db: opts.DB, now: opts.Now, entropy: opts.Entropy,
		ids: id.NewGenerator(opts.Entropy), log: opts.Log,
	}, nil
}

// CreatePairingCode makes a code that pairs one device of the given name and
// kind at the venue, valid for PairingCodeTTL.
func (d *Devices) CreatePairingCode(ctx context.Context, n NewCode) (PairingCode, error) {
	name, err := cleanDeviceName(n.DeviceName)
	if err != nil {
		return PairingCode{}, err
	}
	switch n.DeviceKind {
	case KindTerminal, KindKitchen, KindBackOffice:
	default:
		return PairingCode{}, &InvalidDevice{Field: "kind", Message: fmt.Sprintf("kind %q is not a device that can be paired", n.DeviceKind)}
	}
	createdBy := sql.NullInt64{Int64: n.CreatedBy, Valid: n.CreatedBy > 0}
	now := d.now().UTC()
	expires := now.Add(PairingCodeTTL)

	for range maxDraws {
		code, err := d.ids.PairingCode()
		if err != nil {
			return PairingCode{}, fmt.Errorf("auth: drawing a pairing code: %w", err)
		}
		err = d.db.WriteTx(ctx, func(q *gen.Queries) error {
			_, err := q.CreatePairingCode(ctx, gen.CreatePairingCodeParams{
				VenueID: n.VenueID, CodeHash: hashSecret(code), DeviceName: name, DeviceKind: string(n.DeviceKind),
				CreatedBy: createdBy, CreatedAt: now, ExpiresAt: expires,
			})
			return err
		})
		if db.IsUniqueViolation(err) {
			continue
		}
		if err != nil {
			return PairingCode{}, fmt.Errorf("auth: saving a pairing code: %w", err)
		}
		d.log.Info("pairing code created", "venue_id", n.VenueID, "actor_id", n.CreatedBy, "device_kind", n.DeviceKind)
		return PairingCode{Code: code[:4] + "-" + code[4:], ExpiresAt: expires}, nil
	}
	return PairingCode{}, fmt.Errorf("auth: %d pairing code collisions in a row", maxDraws)
}

// Redeem exchanges a pairing code for a new device and its token, in one
// transaction: the code is marked used by the same commit that creates the
// device, so a code can pair exactly one.
//
// The code may be typed as read: lowercase, with the hyphen or without, O for
// 0 and I or L for 1.
func (d *Devices) Redeem(ctx context.Context, code string) (Paired, error) {
	norm, err := id.Normalize(code)
	if err != nil || len(norm) != id.PairingCodeLength {
		return Paired{}, ErrInvalidPairingCode
	}
	codeHash := hashSecret(norm)

	for range maxDraws {
		token, hash, err := newToken(d.entropy, deviceTokenPrefix)
		if err != nil {
			return Paired{}, err
		}
		publicID, err := d.ids.Public("dev")
		if err != nil {
			return Paired{}, fmt.Errorf("auth: drawing a device id: %w", err)
		}
		now := d.now().UTC()

		var dev Device
		err = d.db.WriteTx(ctx, func(q *gen.Queries) error {
			pc, err := q.PairingCodeByHash(ctx, codeHash)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidPairingCode
			}
			if err != nil {
				return err
			}
			if pc.RedeemedAt.Valid || !now.Before(pc.ExpiresAt) {
				return ErrInvalidPairingCode
			}
			devID, err := q.CreatePairedDevice(ctx, gen.CreatePairedDeviceParams{
				PublicID: publicID, VenueID: pc.VenueID, Name: pc.DeviceName, Kind: pc.DeviceKind,
				TokenHash: hash, PairedAt: sql.NullTime{Time: now, Valid: true}, CreatedAt: now,
			})
			if err != nil {
				return err
			}
			used, err := q.RedeemPairingCode(ctx, gen.RedeemPairingCodeParams{
				RedeemedAt: sql.NullTime{Time: now, Valid: true}, DeviceID: sql.NullInt64{Int64: devID, Valid: true}, ID: pc.ID,
			})
			if err != nil {
				return err
			}
			if used != 1 {
				return ErrInvalidPairingCode
			}
			dev = Device{ID: devID, PublicID: publicID, VenueID: pc.VenueID, Name: pc.DeviceName, Kind: DeviceKind(pc.DeviceKind)}
			return nil
		})
		switch {
		case errors.Is(err, ErrInvalidPairingCode):
			d.log.Warn("pairing code refused")
			return Paired{}, ErrInvalidPairingCode
		case db.IsUniqueViolation(err):
			// The device's public ID or token hash collided: draw both again.
			continue
		case err != nil:
			return Paired{}, fmt.Errorf("auth: redeeming a pairing code: %w", err)
		}
		d.log.Info("device paired", "venue_id", dev.VenueID, "device_id", dev.ID, "device_kind", dev.Kind)
		return Paired{Device: dev, Token: token}, nil
	}
	return Paired{}, fmt.Errorf("auth: %d device id collisions in a row", maxDraws)
}

// Authenticate finds the device a token belongs to. It reads on the read
// pool, and writes last_seen_at at most once a minute.
func (d *Devices) Authenticate(ctx context.Context, token string) (Device, error) {
	hash, ok := tokenHash(token, deviceTokenPrefix)
	if !ok {
		return Device{}, ErrUnauthenticated
	}
	row, err := d.db.Queries().DeviceByTokenHash(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrUnauthenticated
	}
	if err != nil {
		return Device{}, fmt.Errorf("auth: finding a device: %w", err)
	}
	dev := Device{ID: row.ID, PublicID: row.PublicID, VenueID: row.VenueID, Name: row.Name, Kind: DeviceKind(row.Kind)}
	if row.RevokedAt.Valid {
		return Device{}, ErrDeviceRevoked
	}
	d.touch(ctx, dev, row.LastSeenAt)
	return dev, nil
}

// touch brings last_seen_at up to date when it is more than a minute old. A
// failure is logged, not returned: the request is authenticated either way.
func (d *Devices) touch(ctx context.Context, dev Device, last sql.NullTime) {
	now := d.now().UTC()
	if last.Valid && now.Sub(last.Time) < lastSeenEvery {
		return
	}
	err := d.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.TouchDevice(ctx, gen.TouchDeviceParams{LastSeenAt: sql.NullTime{Time: now, Valid: true}, ID: dev.ID})
	})
	if err != nil {
		d.log.Warn("could not record when a device was last seen", "venue_id", dev.VenueID, "device_id", dev.ID, "error", err)
	}
}

// Revoke unpairs a device at a venue. Its token stops working on the next
// request. Revoking a revoked device is not an error.
func (d *Devices) Revoke(ctx context.Context, venueID, deviceID int64) error {
	var revoked int64
	err := d.db.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		revoked, err = q.RevokeDevice(ctx, gen.RevokeDeviceParams{
			RevokedAt: sql.NullTime{Time: d.now().UTC(), Valid: true}, ID: deviceID, VenueID: venueID,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("auth: revoking device %d: %w", deviceID, err)
	}
	if revoked == 1 {
		d.log.Info("device revoked", "venue_id", venueID, "device_id", deviceID)
		return nil
	}
	row, err := d.db.Queries().DeviceByID(ctx, deviceID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.VenueID != venueID) {
		return fmt.Errorf("%w: %d", ErrDeviceNotFound, deviceID)
	}
	if err != nil {
		return fmt.Errorf("auth: revoking device %d: %w", deviceID, err)
	}
	return nil
}
