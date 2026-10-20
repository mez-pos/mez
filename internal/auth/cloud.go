package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/id"
)

// The cloud's owner (ADR-312, ADR-323). A venue paired to a mez Cloud
// account has one staff member the pairing made: named from the account,
// holding the role that grants everything, and with no PIN. They are signed
// in only by an assertion mez Cloud signs, which internal/cloud checks; what
// is here is the person, the device their sessions belong to, and the
// sign-in that spends an assertion once.
const (
	// CloudDeviceName is what the cloud's device is called on the Devices
	// screen. It has no token: nothing can present itself as it.
	CloudDeviceName = "mez Cloud"

	// NoPIN is the pin_hash of someone who has no PIN. It is not a hash, so
	// no PIN matches it, and verify never compares one with it.
	NoPIN = "none"

	// nonceMargin is how long past its end a spent nonce is still kept. An
	// assertion that old is refused for its age, so the margin is only
	// against this computer's clock: one that ran fast and was then put
	// right would otherwise have forgotten a nonce that is live again.
	nonceMargin = time.Hour

	maxCloudOwnerName = 100
	minCloudNonce     = 16
	maxCloudNonce     = 64
)

var (
	// ErrNoCloudOwner means the cloud cannot sign anyone in here: the venue
	// was never paired, the person pairing made has been deactivated, or the
	// cloud's device has been revoked. Pairing again puts all three right.
	ErrNoCloudOwner = errors.New("this venue has nobody the cloud may sign in as")

	// ErrAssertionSpent means an assertion presented a second time, or one
	// so old that whether it was presented before is no longer known.
	ErrAssertionSpent = errors.New("the cloud's assertion has been used already")

	// ErrNoOwnerRole means the venue has no role that grants everything, so
	// there is no role to give the cloud's owner.
	ErrNoOwnerRole = errors.New("this venue has no role that grants every permission")
)

// Vouched is an assertion that has been checked and not yet spent: the nonce
// it carried, and until when that nonce has to be remembered.
type Vouched struct {
	Nonce     string
	KeepUntil time.Time
}

// PairCloudOwner makes the cloud's owner and the cloud's device for a venue
// that has just been paired, or brings them back as pairing makes them:
// active, named for the account, in the role that grants everything, with a
// new device if the last was revoked, and with every session the pairing
// before this one opened ended. Pairing again is how a venue undoes
// having turned the cloud's sign-in off.
func (a *StaffAuth) PairCloudOwner(ctx context.Context, venueID int64, name string) (Staff, error) {
	return a.cloudOwner(ctx, venueID, name, true)
}

// EnsureCloudOwner makes the cloud's owner for a paired venue that has
// none: one paired by a build from before there was such a person. A venue
// that has one is left exactly as it is, deactivated or revoked included: a
// start does not undo what someone at the venue chose.
func (a *StaffAuth) EnsureCloudOwner(ctx context.Context, venueID int64, name string) (Staff, error) {
	return a.cloudOwner(ctx, venueID, name, false)
}

func (a *StaffAuth) cloudOwner(ctx context.Context, venueID int64, name string, restore bool) (Staff, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxCloudOwnerName {
		return Staff{}, fmt.Errorf("auth: the cloud owner's name is empty or over %d characters", maxCloudOwnerName)
	}
	now := a.now().UTC()
	var out Staff
	made := false
	err := a.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := q.CloudStaffForVenue(ctx, venueID)
		found := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("auth: finding the cloud's owner: %w", err)
		}
		// Left as found, whatever has become of the roles since.
		if found && !restore {
			out = Staff{ID: row.ID, PublicID: row.PublicID, VenueID: venueID, Name: row.Name, RoleID: row.RoleID}
			return nil
		}
		role, err := ownerRole(ctx, q, venueID)
		if err != nil {
			return err
		}
		switch {
		case !found:
			if row, err = createCloudStaff(ctx, q, venueID, role.ID, name, now); err != nil {
				return err
			}
			made = true
		default:
			if err := q.RestoreCloudStaff(ctx, gen.RestoreCloudStaffParams{Name: name, RoleID: role.ID, Now: now, ID: row.ID}); err != nil {
				return fmt.Errorf("auth: restoring the cloud's owner: %w", err)
			}
			// A pairing made again may be to another account, with another
			// key. Whoever the last one let in is let out: a session is the
			// account's that opened it, and that is not written on it.
			if err := q.EndSessionsOfStaff(ctx, gen.EndSessionsOfStaffParams{Now: sql.NullTime{Time: now, Valid: true}, StaffID: row.ID}); err != nil {
				return fmt.Errorf("auth: ending the cloud owner's sessions: %w", err)
			}
			row.Name, row.RoleID = name, role.ID
		}
		out = Staff{ID: row.ID, PublicID: row.PublicID, VenueID: venueID, Name: row.Name, RoleID: row.RoleID, RoleName: role.Name}
		return ensureCloudDevice(ctx, q, venueID, now)
	})
	if err != nil {
		return Staff{}, err
	}
	if made {
		a.log.Info("the cloud's owner was added to the staff", "venue_id", venueID, "actor_id", out.ID, "name", out.Name)
	}
	return out, nil
}

// ownerRole is the venue's role that grants everything: the first of them,
// which in a venue nobody has rearranged is "owner".
func ownerRole(ctx context.Context, q *gen.Queries, venueID int64) (gen.Role, error) {
	roles, err := q.RolesForVenue(ctx, venueID)
	if err != nil {
		return gen.Role{}, fmt.Errorf("auth: reading the roles of venue %d: %w", venueID, err)
	}
	var found *gen.Role
	for i, r := range roles {
		set, _, err := parseStoredPermissions(r.Permissions)
		if err != nil || !set.Everything() {
			continue
		}
		if found == nil || r.ID < found.ID {
			found = &roles[i]
		}
	}
	if found == nil {
		return gen.Role{}, ErrNoOwnerRole
	}
	return *found, nil
}

func createCloudStaff(ctx context.Context, q *gen.Queries, venueID, roleID int64, name string, now time.Time) (gen.Staff, error) {
	// Public ids are random; the unique index decides, and a clash draws
	// again (ADR-034).
	for range maxDraws {
		publicID, err := id.NewPublic("stf")
		if err != nil {
			return gen.Staff{}, fmt.Errorf("auth: drawing an id: %w", err)
		}
		rowID, err := q.CreateCloudStaff(ctx, gen.CreateCloudStaffParams{
			PublicID: publicID, VenueID: venueID, RoleID: roleID, Name: name, PinHash: NoPIN, Now: now,
		})
		if db.IsUniqueViolation(err) && strings.Contains(err.Error(), "public_id") {
			continue
		}
		if err != nil {
			return gen.Staff{}, fmt.Errorf("auth: adding the cloud's owner: %w", err)
		}
		return gen.Staff{ID: rowID, PublicID: publicID, VenueID: venueID, RoleID: roleID, Name: name, Active: true, Cloud: true}, nil
	}
	return gen.Staff{}, errors.New("auth: no free id for the cloud's owner")
}

// ensureCloudDevice gives the venue its cloud device if it has none in
// service.
func ensureCloudDevice(ctx context.Context, q *gen.Queries, venueID int64, now time.Time) error {
	_, err := q.CloudDeviceForVenue(ctx, venueID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("auth: finding the cloud's device: %w", err)
	}
	for range maxDraws {
		publicID, err := id.NewPublic("dev")
		if err != nil {
			return fmt.Errorf("auth: drawing an id: %w", err)
		}
		_, err = q.CreateCloudDevice(ctx, gen.CreateCloudDeviceParams{PublicID: publicID, VenueID: venueID, Name: CloudDeviceName, Now: sql.NullTime{Time: now, Valid: true}})
		if db.IsUniqueViolation(err) && strings.Contains(err.Error(), "public_id") {
			continue
		}
		if err != nil {
			return fmt.Errorf("auth: adding the cloud's device: %w", err)
		}
		return nil
	}
	return errors.New("auth: no free id for the cloud's device")
}

// SignInCloud opens a session for the cloud's owner on the cloud's device,
// spending an assertion the caller has already checked: its signature, who
// it is for and its time are internal/cloud's to judge, and are not judged
// again here. What is judged here is that it has not been spent, and that
// the venue still has someone for the cloud to be.
//
// The session is the one a PIN from away opens (ADR-314): through the relay,
// RemoteSessionTTL from now. The nonce and the session are written together,
// so an assertion is never spent on a sign-in that did not happen.
func (a *StaffAuth) SignInCloud(ctx context.Context, venueID int64, v Vouched) (SignedIn, error) {
	now := a.now().UTC()
	// Past KeepUntil its nonce may already have been forgotten, so it could
	// not be told from one never seen.
	if len(v.Nonce) < minCloudNonce || len(v.Nonce) > maxCloudNonce || !now.Before(v.KeepUntil) {
		return SignedIn{}, ErrAssertionSpent
	}
	token, hashed, err := newToken(a.entropy, sessionTokenPrefix)
	if err != nil {
		return SignedIn{}, err
	}
	var session Session
	err = a.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := q.CloudStaffForVenue(ctx, venueID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !row.Active) {
			return ErrNoCloudOwner
		}
		if err != nil {
			return fmt.Errorf("auth: finding the cloud's owner: %w", err)
		}
		dev, err := q.CloudDeviceForVenue(ctx, venueID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoCloudOwner
		}
		if err != nil {
			return fmt.Errorf("auth: finding the cloud's device: %w", err)
		}
		role, err := q.RoleByID(ctx, row.RoleID)
		if err != nil {
			return fmt.Errorf("auth: reading the role of staff %d: %w", row.ID, err)
		}

		if err := q.PruneCloudAssertions(ctx, now.Add(-nonceMargin)); err != nil {
			return fmt.Errorf("auth: forgetting dead assertions: %w", err)
		}
		fresh, err := q.SpendCloudAssertion(ctx, gen.SpendCloudAssertionParams{Nonce: v.Nonce, VenueID: venueID, ExpiresAt: v.KeepUntil.UTC()})
		if err != nil {
			return fmt.Errorf("auth: spending an assertion: %w", err)
		}
		if fresh == 0 {
			return ErrAssertionSpent
		}

		session = Session{
			Staff:     Staff{ID: row.ID, PublicID: row.PublicID, VenueID: venueID, Name: row.Name, RoleID: row.RoleID, RoleName: role.Name},
			DeviceID:  dev.ID,
			ExpiresAt: now.Add(RemoteSessionTTL),
			Remote:    true,
			Cloud:     true,
		}
		session.ID, err = q.CreateStaffSession(ctx, gen.CreateStaffSessionParams{
			TokenHash: hashed, VenueID: venueID, StaffID: row.ID, DeviceID: dev.ID,
			CreatedAt: now, ExpiresAt: session.ExpiresAt, Via: viaRelay,
		})
		if err != nil {
			return fmt.Errorf("auth: opening a session: %w", err)
		}
		return nil
	})
	switch {
	case errors.Is(err, ErrAssertionSpent):
		a.log.Warn("a cloud sign-in was refused: the assertion has been used", "venue_id", venueID)
		return SignedIn{}, err
	case errors.Is(err, ErrNoCloudOwner):
		a.log.Warn("a cloud sign-in was refused: the venue has nobody the cloud may be", "venue_id", venueID)
		return SignedIn{}, err
	case err != nil:
		return SignedIn{}, err
	}
	a.log.Info("the owner signed in from mez Cloud", "venue_id", venueID, "device_id", session.DeviceID,
		"actor_id", session.Staff.ID, "via", viaRelay, "by", "cloud")
	return SignedIn{Session: session, Token: token}, nil
}

// CloudDevice is the cloud's device, found by the token of a session open on
// it (ADR-324). The device has no token of its own, so this is the one way a
// request is known to come from it. A token that is not a session's, or
// whose session is a PIN's, has ended or expired, or belongs to a device
// since revoked or a person since deactivated, is ErrUnauthenticated.
//
// It says which device, not who: the session is still checked, by Session,
// as any other is.
func (a *StaffAuth) CloudDevice(ctx context.Context, token string) (Device, error) {
	hash, ok := tokenHash(token, sessionTokenPrefix)
	if !ok {
		return Device{}, ErrUnauthenticated
	}
	row, err := a.db.Queries().CloudSessionDevice(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrUnauthenticated
	}
	if err != nil {
		return Device{}, fmt.Errorf("auth: finding the cloud's device: %w", err)
	}
	now := a.now().UTC()
	if !now.Before(row.ExpiresAt) {
		return Device{}, ErrUnauthenticated
	}
	dev := Device{ID: row.ID, PublicID: row.PublicID, VenueID: row.VenueID, Name: row.Name, Kind: DeviceKind(row.Kind), Cloud: true}
	// When it was last heard from, to the minute, as a device with a token
	// has it. A failure is logged and no more: the request is known either way.
	if !row.LastSeenAt.Valid || now.Sub(row.LastSeenAt.Time) >= lastSeenEvery {
		err := a.db.WriteTx(ctx, func(q *gen.Queries) error {
			return q.TouchDevice(ctx, gen.TouchDeviceParams{LastSeenAt: sql.NullTime{Time: now, Valid: true}, ID: dev.ID})
		})
		if err != nil {
			a.log.Warn("could not record when the cloud's device was last seen", "venue_id", dev.VenueID, "device_id", dev.ID, "error", err)
		}
	}
	return dev, nil
}

// CloudSeat is who mez Cloud may sign in as at a venue, and the device its
// sessions are on, as the database has them now: ErrNoCloudOwner for a venue
// never paired, whose cloud's owner was deactivated, or whose cloud's device
// was revoked. It reads and writes nothing, for a mirror, which opens the
// cloud's sessions in its own memory over a copy it may not write (ADR-331).
func (a *StaffAuth) CloudSeat(ctx context.Context, venueID int64) (Staff, Device, error) {
	q := a.db.Queries()
	row, err := q.CloudStaffForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !row.Active) {
		return Staff{}, Device{}, ErrNoCloudOwner
	}
	if err != nil {
		return Staff{}, Device{}, fmt.Errorf("auth: finding the cloud's owner: %w", err)
	}
	dev, err := q.CloudDeviceForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return Staff{}, Device{}, ErrNoCloudOwner
	}
	if err != nil {
		return Staff{}, Device{}, fmt.Errorf("auth: finding the cloud's device: %w", err)
	}
	role, err := q.RoleByID(ctx, row.RoleID)
	if err != nil {
		return Staff{}, Device{}, fmt.Errorf("auth: reading the role of staff %d: %w", row.ID, err)
	}
	return Staff{ID: row.ID, PublicID: row.PublicID, VenueID: venueID, Name: row.Name, RoleID: row.RoleID, RoleName: role.Name},
		Device{ID: dev.ID, PublicID: dev.PublicID, VenueID: dev.VenueID, Name: dev.Name, Kind: DeviceKind(dev.Kind), Cloud: true}, nil
}

// EndCloudSessions ends every session mez Cloud opened on this machine. It
// is for a start that finds the venue on no pairing: unpaired by hand, or
// set aside for a relay of its own (ADR-319). The key those sessions were
// vouched with is no longer one this venue answers to.
func (a *StaffAuth) EndCloudSessions(ctx context.Context) error {
	now := sql.NullTime{Time: a.now().UTC(), Valid: true}
	err := a.db.WriteTx(ctx, func(q *gen.Queries) error { return q.EndCloudSessions(ctx, now) })
	if err != nil {
		return fmt.Errorf("auth: ending the cloud's sessions: %w", err)
	}
	return nil
}
