package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// Sign-in outcomes, as sign_in_attempts records them.
const (
	outcomeSuccess       = "success"
	outcomeInvalidPIN    = "invalid_pin"
	outcomeUnknownStaff  = "unknown_staff"
	outcomeInactiveStaff = "inactive_staff"
	outcomeLockedOut     = "locked_out"
)

// maxStaffRef bounds what is kept of a staff ID that matched nobody.
const maxStaffRef = 64

// maxPresentedPIN bounds a presented PIN. Anything longer cannot be a PIN,
// and is not worth the memory Argon2 would spend on it.
const maxPresentedPIN = 64

// LockoutPolicy is when failures lock sign-in: MaxFailures within Window.
// The lock lifts when the oldest of those failures leaves the window.
type LockoutPolicy struct {
	MaxFailures int
	Window      time.Duration
}

// The defaults. Five wrong PINs in a quarter hour locks the staff member for
// what remains of it: generous to a cashier who fumbles twice, useless to
// anyone working through ten thousand PINs. A device gets twenty across all
// its staff, so one tablet cannot cycle through everyone's four tries each.
var (
	DefaultStaffLockout  = LockoutPolicy{MaxFailures: 5, Window: 15 * time.Minute}
	DefaultDeviceLockout = LockoutPolicy{MaxFailures: 20, Window: 15 * time.Minute}
)

// DefaultSessionTTL is how long a staff session lasts: a long shift. The
// terminal locks itself sooner; this is the backstop.
const DefaultSessionTTL = 12 * time.Hour

var (
	// ErrInvalidPIN means the staff member and PIN did not sign in. An
	// unknown or deactivated staff member gets this too.
	ErrInvalidPIN = errors.New("staff member or PIN is not right")

	// ErrLockedOut means sign-in is locked for now. The error is a
	// *LockedError saying for how long.
	ErrLockedOut = errors.New("sign-in is locked")

	// ErrNoSession means no staff session: none given, or one that has
	// ended, expired, belongs to another device or to someone deactivated.
	ErrNoSession = errors.New("no staff session")
)

// LockedError is a refused sign-in during a lockout.
type LockedError struct {
	// Scope is "staff" or "device": which limit was hit.
	Scope      string
	RetryAfter time.Duration
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("sign-in is locked for this %s for %s", e.Scope, e.RetryAfter.Round(time.Second))
}

// Is lets errors.Is(err, ErrLockedOut) match.
func (e *LockedError) Is(target error) bool { return target == ErrLockedOut }

// Staff is a staff member as a sign-in or a session knows them.
type Staff struct {
	ID       int64
	PublicID string
	VenueID  int64
	Name     string
	RoleID   int64
	// RoleName is what the role is called, for the sign-in screen to show
	// under the name. It is a label: what a person may do is the role's
	// permissions, never its name.
	RoleName string
}

// Session is a signed-in staff member on a device.
type Session struct {
	ID        int64
	Staff     Staff
	DeviceID  int64
	ExpiresAt time.Time
}

// SignedIn is a new session and its token. The token is shown once.
type SignedIn struct {
	Session Session
	Token   string
}

// StaffOptions configure StaffAuth. DB is required; zero values take the
// defaults.
type StaffOptions struct {
	DB            *db.DB
	Now           func() time.Time
	Entropy       io.Reader
	Log           *slog.Logger
	SessionTTL    time.Duration
	StaffLockout  LockoutPolicy
	DeviceLockout LockoutPolicy
}

// StaffAuth signs staff in and checks their sessions.
type StaffAuth struct {
	db            *db.DB
	now           func() time.Time
	entropy       io.Reader
	log           *slog.Logger
	ttl           time.Duration
	staffLockout  LockoutPolicy
	deviceLockout LockoutPolicy
	locks         keyedMutex
}

// NewStaffAuth returns a StaffAuth.
func NewStaffAuth(opts StaffOptions) (*StaffAuth, error) {
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
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = DefaultSessionTTL
	}
	if opts.StaffLockout.MaxFailures < 1 || opts.StaffLockout.Window <= 0 {
		opts.StaffLockout = DefaultStaffLockout
	}
	if opts.DeviceLockout.MaxFailures < 1 || opts.DeviceLockout.Window <= 0 {
		opts.DeviceLockout = DefaultDeviceLockout
	}
	return &StaffAuth{
		db: opts.DB, now: opts.Now, entropy: opts.Entropy, log: opts.Log, ttl: opts.SessionTTL,
		staffLockout: opts.StaffLockout, deviceLockout: opts.DeviceLockout,
	}, nil
}

// ActiveStaff is who can sign in at a venue, for the terminal's name tiles.
func (a *StaffAuth) ActiveStaff(ctx context.Context, venueID int64) ([]Staff, error) {
	rows, err := a.db.Queries().ActiveStaffForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("auth: listing staff: %w", err)
	}
	out := make([]Staff, len(rows))
	for i, r := range rows {
		out[i] = Staff{ID: r.ID, PublicID: r.PublicID, VenueID: r.VenueID, Name: r.Name, RoleID: r.RoleID, RoleName: r.RoleName}
	}
	return out, nil
}

// WithPermission is the active staff of a venue whose role grants p, by name.
// A till with no shift open uses it to say who can open one (ADR-150). Each
// role is read once, however many people hold it.
func (a *StaffAuth) WithPermission(ctx context.Context, venueID int64, p Permission) ([]Staff, error) {
	active, err := a.ActiveStaff(ctx, venueID)
	if err != nil {
		return nil, err
	}
	grants := map[int64]bool{}
	out := make([]Staff, 0, len(active))
	for _, member := range active {
		held, seen := grants[member.RoleID]
		if !seen {
			set, err := a.Permissions(ctx, member.RoleID)
			if err != nil {
				return nil, err
			}
			held = set.Has(p)
			grants[member.RoleID] = held
		}
		if held {
			out = append(out, member)
		}
	}
	return out, nil
}

// SignIn checks a staff member's PIN on a device and opens a session.
//
// Every attempt is recorded, whatever its outcome. The checks run in an order
// that gives nothing away: a lockout is refused before any PIN is compared,
// and an unknown or deactivated staff member is compared against a dummy
// hash, so all three wrong answers take as long as each other and read the
// same.
func (a *StaffAuth) SignIn(ctx context.Context, dev Device, staffRef, pin string) (SignedIn, error) {
	row, err := a.verify(ctx, dev, staffRef, pin, nil)
	if err != nil {
		return SignedIn{}, err
	}
	who, err := a.staffOf(ctx, row)
	if err != nil {
		return SignedIn{}, err
	}
	now := a.now().UTC()
	token, hashed, err := newToken(a.entropy, sessionTokenPrefix)
	if err != nil {
		return SignedIn{}, err
	}
	session := Session{
		Staff:     who,
		DeviceID:  dev.ID,
		ExpiresAt: now.Add(a.ttl),
	}
	err = a.db.WriteTx(ctx, func(q *gen.Queries) error {
		session.ID, err = q.CreateStaffSession(ctx, gen.CreateStaffSessionParams{
			TokenHash: hashed, VenueID: dev.VenueID, StaffID: row.ID, DeviceID: dev.ID,
			CreatedAt: now, ExpiresAt: session.ExpiresAt,
		})
		return err
	})
	if err != nil {
		return SignedIn{}, fmt.Errorf("auth: opening a session: %w", err)
	}
	return SignedIn{Session: session, Token: token}, nil
}

// Authorise checks a manager's PIN for one action, without opening a
// session: the override behind a void, a comp, a discount, a payment void or
// a reopen. The permission checked is the authoriser's, never the caller's.
//
// It runs the same gauntlet as a sign-in, so override attempts count toward
// the same lockouts and are recorded in the same place.
func (a *StaffAuth) Authorise(ctx context.Context, dev Device, staffRef, pin string, p Permission) (Staff, error) {
	row, err := a.verify(ctx, dev, staffRef, pin, &p)
	if err != nil {
		return Staff{}, err
	}
	return a.staffOf(ctx, row)
}

// staffOf is a staff row as the rest of the system sees it, with the name of
// their role, which the row itself does not carry.
func (a *StaffAuth) staffOf(ctx context.Context, row gen.Staff) (Staff, error) {
	role, err := a.db.Queries().RoleByID(ctx, row.RoleID)
	if err != nil {
		return Staff{}, fmt.Errorf("auth: reading the role of staff %d: %w", row.ID, err)
	}
	return Staff{ID: row.ID, PublicID: row.PublicID, VenueID: row.VenueID, Name: row.Name, RoleID: row.RoleID, RoleName: role.Name}, nil
}

// verify is the PIN check both a sign-in and an override run: find the staff
// member, refuse a locked-out attempt before comparing anything, compare
// against a dummy hash when there is nobody to compare against, and record
// what happened. When need is set, the staff member must also hold that
// permission, which is checked after the PIN so a manager who lacks it is
// told apart from one who mistyped.
func (a *StaffAuth) verify(ctx context.Context, dev Device, staffRef, pin string, need *Permission) (gen.Staff, error) {
	row, known, err := a.findStaff(ctx, dev.VenueID, staffRef)
	if err != nil {
		return gen.Staff{}, err
	}

	// One attempt per staff member at a time, so parallel guesses cannot all
	// pass the lockout check before any of them is recorded. An unknown
	// staff member is keyed by device, so made-up IDs cannot grow the map.
	key := fmt.Sprintf("device:%d", dev.ID)
	if known {
		key = fmt.Sprintf("staff:%d", row.ID)
	}
	unlock := a.locks.lock(key)
	defer unlock()

	now := a.now().UTC()

	attempt := gen.RecordSignInAttemptParams{
		VenueID: dev.VenueID, DeviceID: dev.ID, StaffRef: clip(staffRef), At: now,
		StaffID: sql.NullInt64{Int64: row.ID, Valid: known},
	}

	if locked, err := a.locked(ctx, dev, row.ID, known, now); err != nil || locked != nil {
		if err != nil {
			return gen.Staff{}, err
		}
		attempt.Outcome = outcomeLockedOut
		if err := a.record(ctx, attempt, nil); err != nil {
			return gen.Staff{}, err
		}
		a.log.Warn("sign-in refused: locked out", "venue_id", dev.VenueID, "device_id", dev.ID,
			"actor_id", row.ID, "scope", locked.Scope, "retry_after_s", int(locked.RetryAfter.Seconds()))
		return gen.Staff{}, locked
	}

	hash, outcome := dummyHash(), outcomeUnknownStaff
	if known {
		hash, outcome = row.PinHash, outcomeInactiveStaff
		if row.Active {
			outcome = outcomeInvalidPIN
		}
	}
	if len(pin) > maxPresentedPIN {
		pin = ""
	}
	match, err := verifyPIN(hash, pin)
	if err != nil {
		a.log.Error("a stored PIN hash cannot be read", "venue_id", dev.VenueID, "actor_id", row.ID, "error", err)
		match = false
	}
	if !match || outcome != outcomeInvalidPIN {
		attempt.Outcome = outcome
		if err := a.record(ctx, attempt, nil); err != nil {
			return gen.Staff{}, err
		}
		a.log.Warn("PIN refused", "venue_id", dev.VenueID, "device_id", dev.ID, "actor_id", row.ID, "outcome", outcome)
		return gen.Staff{}, ErrInvalidPIN
	}

	// The PIN was right, so the attempt succeeded and is recorded as such,
	// whatever the permission check makes of it. A manager who is refused
	// here mistyped nothing, and must not count toward a lockout.
	attempt.Outcome = outcomeSuccess
	if err := a.record(ctx, attempt, nil); err != nil {
		return gen.Staff{}, err
	}
	if need != nil {
		held, err := a.Permissions(ctx, row.RoleID)
		if err != nil {
			return gen.Staff{}, err
		}
		if !held.Has(*need) {
			a.log.Warn("authorisation refused", "venue_id", dev.VenueID, "device_id", dev.ID,
				"actor_id", row.ID, "permission", string(*need))
			return gen.Staff{}, fmt.Errorf("%w: %s cannot %s", ErrForbidden, row.Name, *need)
		}
	}
	a.log.Info("PIN accepted", "venue_id", dev.VenueID, "device_id", dev.ID, "actor_id", row.ID)
	return row, nil
}

// Permissions is what a role grants. A key this build does not know is
// dropped and logged: it cannot be granted by code that never asks for it.
func (a *StaffAuth) Permissions(ctx context.Context, roleID int64) (PermissionSet, error) {
	role, err := a.db.Queries().RoleByID(ctx, roleID)
	if errors.Is(err, sql.ErrNoRows) {
		return PermissionSet{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: reading role %d: %w", roleID, err)
	}
	set, unknown, err := parseStoredPermissions(role.Permissions)
	if err != nil {
		a.log.Error("a role's permissions cannot be read", "venue_id", role.VenueID, "role", role.Name, "error", err)
		return PermissionSet{}, nil
	}
	if len(unknown) > 0 {
		a.log.Warn("a role grants permissions this build does not know",
			"venue_id", role.VenueID, "role", role.Name, "unknown", unknown)
	}
	return set, nil
}

// findStaff looks up the staff member a sign-in names, at the device's venue.
// Someone at another venue is nobody here.
func (a *StaffAuth) findStaff(ctx context.Context, venueID int64, ref string) (gen.Staff, bool, error) {
	if ref == "" || len(ref) > maxStaffRef {
		return gen.Staff{}, false, nil
	}
	row, err := a.db.Queries().StaffByPublicID(ctx, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Staff{}, false, nil
	}
	if err != nil {
		return gen.Staff{}, false, fmt.Errorf("auth: finding staff: %w", err)
	}
	if row.VenueID != venueID {
		return gen.Staff{}, false, nil
	}
	return row, true, nil
}

// locked checks the device's limit, then the staff member's.
func (a *StaffAuth) locked(ctx context.Context, dev Device, staffID int64, known bool, now time.Time) (*LockedError, error) {
	q := a.db.Queries()
	failures, err := q.RecentDeviceFailures(ctx, gen.RecentDeviceFailuresParams{
		DeviceID: dev.ID, Limit: int64(a.deviceLockout.MaxFailures),
	})
	if err != nil {
		return nil, fmt.Errorf("auth: reading sign-in attempts: %w", err)
	}
	if wait := lockRemaining(failures, a.deviceLockout, now); wait > 0 {
		return &LockedError{Scope: "device", RetryAfter: wait}, nil
	}
	if !known {
		return nil, nil
	}

	verdicts, err := q.RecentStaffVerdicts(ctx, gen.RecentStaffVerdictsParams{
		StaffID: sql.NullInt64{Int64: staffID, Valid: true}, Limit: int64(a.staffLockout.MaxFailures),
	})
	if err != nil {
		return nil, fmt.Errorf("auth: reading sign-in attempts: %w", err)
	}
	// A success resets the count: only failures since the last one matter.
	var since []time.Time
	for _, v := range verdicts {
		if v.Outcome == outcomeSuccess {
			break
		}
		since = append(since, v.At)
	}
	if wait := lockRemaining(since, a.staffLockout, now); wait > 0 {
		return &LockedError{Scope: "staff", RetryAfter: wait}, nil
	}
	return nil, nil
}

// lockRemaining is how long a lock has left, given failures newest first: if
// MaxFailures of them fall inside the window, until the oldest of those
// leaves it.
func lockRemaining(failures []time.Time, p LockoutPolicy, now time.Time) time.Duration {
	if len(failures) < p.MaxFailures {
		return 0
	}
	oldest := failures[p.MaxFailures-1].UTC()
	if wait := oldest.Add(p.Window).Sub(now); wait > 0 {
		return wait
	}
	return 0
}

// record writes an attempt, and on success the session with it, in one
// transaction.
func (a *StaffAuth) record(ctx context.Context, attempt gen.RecordSignInAttemptParams, also func(*gen.Queries) error) error {
	err := a.db.WriteTx(ctx, func(q *gen.Queries) error {
		if err := q.RecordSignInAttempt(ctx, attempt); err != nil {
			return err
		}
		if also != nil {
			return also(q)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("auth: recording a sign-in attempt: %w", err)
	}
	return nil
}

// Session finds the staff session a token opens on this device. A session
// that has ended or expired, that was opened on another device, or whose
// staff member has since been deactivated, is no session.
func (a *StaffAuth) Session(ctx context.Context, dev Device, token string) (Session, error) {
	hash, ok := tokenHash(token, sessionTokenPrefix)
	if !ok {
		return Session{}, ErrNoSession
	}
	row, err := a.db.Queries().StaffSessionByTokenHash(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("auth: finding a session: %w", err)
	}
	if row.DeviceID != dev.ID || row.VenueID != dev.VenueID {
		a.log.Warn("a staff session was presented by another device",
			"venue_id", dev.VenueID, "device_id", dev.ID, "actor_id", row.StaffID, "session_device_id", row.DeviceID)
		return Session{}, ErrNoSession
	}
	if row.EndedAt.Valid || !a.now().Before(row.ExpiresAt) || !row.StaffActive {
		return Session{}, ErrNoSession
	}
	return Session{
		ID:        row.ID,
		Staff:     Staff{ID: row.StaffID, PublicID: row.StaffPublicID, VenueID: row.VenueID, Name: row.StaffName, RoleID: row.RoleID, RoleName: row.RoleName},
		DeviceID:  row.DeviceID,
		ExpiresAt: row.ExpiresAt.UTC(),
	}, nil
}

// SignOut ends a session. Ending one that has already ended is not an error.
func (a *StaffAuth) SignOut(ctx context.Context, s Session) error {
	err := a.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.EndStaffSession(ctx, gen.EndStaffSessionParams{
			EndedAt: sql.NullTime{Time: a.now().UTC(), Valid: true}, ID: s.ID,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("auth: ending a session: %w", err)
	}
	a.log.Info("staff signed out", "venue_id", s.Staff.VenueID, "device_id", s.DeviceID, "actor_id", s.Staff.ID)
	return nil
}

// clip keeps at most maxStaffRef bytes of a presented staff ID, as valid
// UTF-8, for the attempt log.
func clip(ref string) string {
	if len(ref) > maxStaffRef {
		ref = ref[:maxStaffRef]
	}
	return strings.ToValidUTF8(ref, "")
}
