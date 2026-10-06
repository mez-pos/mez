package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db/gen"
)

// ErrRevokeSelf refuses a device revoking itself: the back office that did it
// would be locked out of the screen that could pair it again.
var ErrRevokeSelf = errors.New("a device cannot revoke itself")

// InvalidDevice is a device name or kind that cannot be used, and the field it
// is about. It is an ErrInvalidDevice.
type InvalidDevice struct {
	Field   string
	Message string
}

// Error is ErrInvalidDevice's text followed by what was wrong.
func (e *InvalidDevice) Error() string { return fmt.Sprintf("%s: %s", ErrInvalidDevice, e.Message) }

// Is makes an InvalidDevice match ErrInvalidDevice.
func (e *InvalidDevice) Is(target error) bool { return target == ErrInvalidDevice }

// cleanDeviceName trims a name and checks its length.
func cleanDeviceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxDeviceName {
		return "", &InvalidDevice{Field: "name", Message: fmt.Sprintf("a name of 1 to %d characters is needed", maxDeviceName)}
	}
	return name, nil
}

// Record is a device as the back office lists it: who it is, when it was
// paired and last heard from, and whether it has been revoked.
type Record struct {
	Device
	PairedAt   time.Time
	LastSeenAt *time.Time // nil: not heard from since it was paired
	RevokedAt  *time.Time
}

// Waiting is a pairing code made and not yet used: a device on its way. The
// code itself is not kept, only what it will pair.
type Waiting struct {
	Name      string
	Kind      DeviceKind
	ExpiresAt time.Time
}

// Roster is a venue's devices, in service first, and the codes still waiting.
type Roster struct {
	Devices []Record
	Waiting []Waiting
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

// Roster lists the venue's paired devices and the pairing codes that have not
// been used or run out. A device that has been revoked stays listed, last.
func (d *Devices) Roster(ctx context.Context, venueID int64) (Roster, error) {
	q := d.db.Queries()
	rows, err := q.DevicesForVenue(ctx, venueID)
	if err != nil {
		return Roster{}, fmt.Errorf("auth: listing the devices of venue %d: %w", venueID, err)
	}
	open, err := q.OpenPairingCodes(ctx, gen.OpenPairingCodesParams{VenueID: venueID, ExpiresAt: d.now().UTC()})
	if err != nil {
		return Roster{}, fmt.Errorf("auth: listing the pairing codes of venue %d: %w", venueID, err)
	}
	out := Roster{Devices: make([]Record, len(rows)), Waiting: make([]Waiting, len(open))}
	for i, r := range rows {
		out.Devices[i] = Record{
			Device:   Device{ID: r.ID, PublicID: r.PublicID, VenueID: venueID, Name: r.Name, Kind: DeviceKind(r.Kind)},
			PairedAt: r.PairedAt.Time.UTC(), LastSeenAt: timePtr(r.LastSeenAt), RevokedAt: timePtr(r.RevokedAt),
		}
	}
	for i, c := range open {
		out.Waiting[i] = Waiting{Name: c.DeviceName, Kind: DeviceKind(c.DeviceKind), ExpiresAt: c.ExpiresAt.UTC()}
	}
	return out, nil
}

// byPublicID finds a device of the venue by the id the API shows.
func byPublicID(ctx context.Context, q *gen.Queries, venueID int64, publicID string) (gen.DeviceByPublicIDRow, error) {
	row, err := q.DeviceByPublicID(ctx, gen.DeviceByPublicIDParams{VenueID: venueID, PublicID: publicID})
	if errors.Is(err, sql.ErrNoRows) {
		return row, fmt.Errorf("%w: %s", ErrDeviceNotFound, publicID)
	}
	if err != nil {
		return row, fmt.Errorf("auth: finding device %s: %w", publicID, err)
	}
	return row, nil
}

// Rename gives a device a new name. A revoked device keeps the name its
// history knows it by (ErrDeviceRevoked).
func (d *Devices) Rename(ctx context.Context, venueID int64, publicID, name string, by int64) error {
	name, err := cleanDeviceName(name)
	if err != nil {
		return err
	}
	var renamed int64
	err = d.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := byPublicID(ctx, q, venueID, publicID)
		if err != nil {
			return err
		}
		if row.RevokedAt.Valid {
			return fmt.Errorf("%w: %s", ErrDeviceRevoked, publicID)
		}
		renamed, err = q.RenameDevice(ctx, gen.RenameDeviceParams{Name: name, VenueID: venueID, ID: row.ID})
		return err
	})
	if err != nil {
		return err
	}
	if renamed == 1 {
		d.log.Info("device renamed", "venue_id", venueID, "device", publicID, "actor_id", by)
	}
	return nil
}

// RevokeByPublicID unpairs a device of the venue, by the id the API shows. The
// device acting, actingID, may not be the one revoked (ErrRevokeSelf): revoking
// the back office in use would leave nobody able to pair another. Revoking a
// device already revoked is not an error.
func (d *Devices) RevokeByPublicID(ctx context.Context, venueID int64, publicID string, actingID, by int64) error {
	var (
		deviceID int64
		revoked  int64
	)
	// One transaction: the device looked up is the device revoked, and a
	// device revoked between the two cannot be revoked again as news.
	err := d.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := byPublicID(ctx, q, venueID, publicID)
		if err != nil {
			return err
		}
		if row.ID == actingID {
			return fmt.Errorf("%w: %s", ErrRevokeSelf, publicID)
		}
		deviceID = row.ID
		revoked, err = q.RevokeDevice(ctx, gen.RevokeDeviceParams{
			RevokedAt: sql.NullTime{Time: d.now().UTC(), Valid: true}, ID: row.ID, VenueID: venueID,
		})
		return err
	})
	if err != nil {
		return err
	}
	if revoked == 0 {
		// Already revoked: not an error, and nothing new to say or do.
		return nil
	}
	d.log.Info("device revoked by staff", "venue_id", venueID, "device", publicID, "device_id", deviceID, "actor_id", by)
	if d.onRevoke != nil {
		d.onRevoke(venueID, deviceID)
	}
	return nil
}
