// Package people is the back office's staff and roles (phase 10, Task 4a,
// ADR-115): who works here, what each role may do, and their PINs.
//
// Two rules hold on every change, inside its transaction:
//
//   - Someone may only hand out what they hold. The person acting must hold
//     every permission of a role they create, edit, delete or assign, and of
//     the role of anyone they change, deactivate or give a new PIN. A
//     manager cannot make anyone an owner, nor reset the owner's PIN.
//   - The venue keeps someone active who can manage staff. A change that
//     would leave nobody is refused: no screen could undo it.
package people

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
)

// The refusals. The API maps them to codes; Invalid carries the field.
var (
	ErrStaffNotFound = errors.New("no such staff member at this venue")
	ErrRoleNotFound  = errors.New("no such role at this venue")
	ErrRoleInUse     = errors.New("people still hold that role")
	ErrRoleNameTaken = errors.New("a role with that name already exists")
	// ErrBeyondYou is a change that would hand out, or reach, permissions
	// the person acting does not hold.
	ErrBeyondYou = errors.New("that is beyond what you may give")
	// ErrPeerPIN is a reset of the PIN of someone who holds as much as the
	// person acting: knowing a PIN is being that person, and between equals
	// that is a way to act in a peer's name.
	ErrPeerPIN = errors.New("only someone who holds more than they do may reset their PIN")
	// ErrLastManager is a change that would leave nobody active who can
	// manage staff.
	ErrLastManager = errors.New("nobody would be left who can manage staff")
)

// Invalid is an input that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

// Error names the field and says what is wrong with it.
func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// MaxNameLength is the longest name of a person or a role, in characters.
const MaxNameLength = 40

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service is the venue's people.
type Service struct {
	db    *db.DB
	clock Clock
}

// New returns a Service.
func New(d *db.DB, clock Clock) (*Service, error) {
	if d == nil {
		return nil, errors.New("people: a database is required")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: d, clock: clock}, nil
}

// Person is a staff member as the People screen shows them.
type Person struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	RoleID   int64     `json:"role_id"`
	RoleName string    `json:"role_name"`
	Active   bool      `json:"active"`
	Since    time.Time `json:"since"`
}

// Role is a role with what it grants and how many active people hold it.
type Role struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Permissions []auth.Permission `json:"permissions"`
	People      int64             `json:"people"`
}

// StaffInput is a person as the back office sends them. PIN is needed to
// add someone, and ignored on an edit: a PIN has its own reset.
type StaffInput struct {
	Name   string `json:"name"`
	RoleID int64  `json:"role_id"`
	PIN    string `json:"pin,omitempty"`
}

// RoleInput is a role as the back office sends it.
type RoleInput struct {
	Name        string            `json:"name"`
	Permissions []auth.Permission `json:"permissions"`
}

// People is everyone who works or worked here: the active first, by name.
func (s *Service) People(ctx context.Context, venueID int64) ([]Person, error) {
	rows, err := s.db.Queries().StaffForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("people: reading staff: %w", err)
	}
	out := make([]Person, len(rows))
	for i, r := range rows {
		out[i] = Person{ID: r.PublicID, Name: r.Name, RoleID: r.RoleID, RoleName: r.RoleName, Active: r.Active, Since: r.CreatedAt.UTC()}
	}
	return out, nil
}

// Roles is every role, by name, with what it grants in the canonical order.
func (s *Service) Roles(ctx context.Context, venueID int64) ([]Role, error) {
	q := s.db.Queries()
	rows, err := q.RolesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("people: reading roles: %w", err)
	}
	counts, err := q.StaffCountByRole(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("people: counting who holds each role: %w", err)
	}
	held := make(map[int64]int64, len(counts))
	for _, c := range counts {
		held[c.RoleID] = c.N
	}
	out := make([]Role, len(rows))
	for i, r := range rows {
		out[i] = Role{ID: r.ID, Name: r.Name, Permissions: permsOf(r.Permissions).Keys(), People: held[r.ID]}
	}
	return out, nil
}

// AddStaff adds a person with a role and a PIN, returning their id.
func (s *Service) AddStaff(ctx context.Context, venueID, actorID int64, in StaffInput) (string, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", err
	}
	hash, err := pinHash(in.PIN)
	if err != nil {
		return "", err
	}
	var publicID string
	err = s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		if err := mayGiveRole(ctx, q, venueID, in.RoleID, mine); err != nil {
			return err
		}
		// Public ids are random; the unique index decides, and a clash
		// draws again (ADR-034).
		for range 3 {
			publicID, err = id.NewPublic("stf")
			if err != nil {
				return fmt.Errorf("people: drawing an id: %w", err)
			}
			_, err = q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: publicID, VenueID: venueID, RoleID: in.RoleID, Name: name, PinHash: hash, Active: true,
				CreatedAt: now, UpdatedAt: now,
			})
			if !db.IsUniqueViolation(err) {
				break
			}
		}
		return err
	})
	return publicID, err
}

// UpdateStaff renames a person or moves them to another role.
func (s *Service) UpdateStaff(ctx context.Context, venueID, actorID int64, staffID string, in StaffInput) error {
	name, err := cleanName(in.Name)
	if err != nil {
		return err
	}
	return s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		target, err := mayReach(ctx, q, venueID, staffID, mine)
		if err != nil {
			return err
		}
		if err := mayGiveRole(ctx, q, venueID, in.RoleID, mine); err != nil {
			return err
		}
		n, err := q.UpdateStaff(ctx, gen.UpdateStaffParams{Name: name, RoleID: in.RoleID, Now: now, VenueID: venueID, ID: target.ID})
		return oneRow(n, err, ErrStaffNotFound)
	})
}

// SetStaffActive deactivates a person or brings them back. Deactivated,
// their sessions stop at once (auth checks the person is active) and they
// leave the sign-in tiles; their name stays on everything they did.
// Nobody deactivates themselves: they would be signed out mid-change.
func (s *Service) SetStaffActive(ctx context.Context, venueID, actorID int64, staffID string, active bool) error {
	return s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		target, err := mayReach(ctx, q, venueID, staffID, mine)
		if err != nil {
			return err
		}
		if target.ID == actorID && !active {
			return &Invalid{Field: "active", Message: "You can't deactivate yourself. Ask someone else who can manage staff."}
		}
		n, err := q.SetStaffActive(ctx, gen.SetStaffActiveParams{Active: active, Now: now, VenueID: venueID, ID: target.ID})
		if err := oneRow(n, err, ErrStaffNotFound); err != nil {
			return err
		}
		if active {
			return nil
		}
		// Ended, not merely hidden: a session only hidden by the person
		// being inactive would work again the moment they were brought
		// back, with no PIN asked. Someone brought back signs in afresh.
		if err := q.EndSessionsOfStaff(ctx, gen.EndSessionsOfStaffParams{Now: sql.NullTime{Time: now, Valid: true}, StaffID: target.ID}); err != nil {
			return fmt.Errorf("people: signing %s out: %w", staffID, err)
		}
		return nil
	})
}

// ResetPIN gives a person a new PIN and signs them out everywhere, so
// whoever knew the old one is not left signed in with it. A person may
// change their own.
//
// Someone else's PIN may be reset only by a person who holds strictly more
// than they do (ErrPeerPIN). Knowing a PIN is being that person: with it,
// every override and every sale is in their name. Between equals, a reset
// would be a way to act as a peer and have the record name the peer, so a
// manager may reset a waiter's PIN and an owner a manager's, and nobody
// but the owner may reset the owner's. Someone who holds every permission
// is excepted: among owners there is no one above to turn to.
func (s *Service) ResetPIN(ctx context.Context, venueID, actorID int64, staffID, pin string) error {
	hash, err := pinHash(pin)
	if err != nil {
		return err
	}
	return s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		target, err := mayReach(ctx, q, venueID, staffID, mine)
		if err != nil {
			return err
		}
		if target.ID != actorID && !mine.Everything() {
			theirs, err := rolePerms(ctx, q, venueID, target.RoleID)
			if err != nil {
				return err
			}
			if theirs.Covers(mine) {
				return ErrPeerPIN
			}
		}
		n, err := q.SetStaffPIN(ctx, gen.SetStaffPINParams{PinHash: hash, Now: now, VenueID: venueID, ID: target.ID})
		if err := oneRow(n, err, ErrStaffNotFound); err != nil {
			return err
		}
		if err := q.EndSessionsOfStaff(ctx, gen.EndSessionsOfStaffParams{Now: sql.NullTime{Time: now, Valid: true}, StaffID: target.ID}); err != nil {
			return fmt.Errorf("people: signing %s out: %w", staffID, err)
		}
		return nil
	})
}

// AddRole adds a role, returning its id.
func (s *Service) AddRole(ctx context.Context, venueID, actorID int64, in RoleInput) (int64, error) {
	name, set, err := cleanRole(in)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		if !mine.Covers(set) {
			return ErrBeyondYou
		}
		raw, err := set.JSON()
		if err != nil {
			return err
		}
		r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: venueID, Name: name, Permissions: raw, CreatedAt: now})
		id = r.ID
		return roleNameTaken(err)
	})
	return id, err
}

// UpdateRole renames a role or changes what it grants, for everyone who
// holds it, from their next request.
func (s *Service) UpdateRole(ctx context.Context, venueID, actorID, roleID int64, in RoleInput) error {
	name, set, err := cleanRole(in)
	if err != nil {
		return err
	}
	return s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error {
		if err := mayGiveRole(ctx, q, venueID, roleID, mine); err != nil {
			return err
		}
		if !mine.Covers(set) {
			return ErrBeyondYou
		}
		raw, err := set.JSON()
		if err != nil {
			return err
		}
		n, err := q.UpdateRole(ctx, gen.UpdateRoleParams{Name: name, Permissions: raw, VenueID: venueID, ID: roleID})
		return oneRow(n, roleNameTaken(err), ErrRoleNotFound)
	})
}

// DeleteRole removes a role nobody holds, active or not: a deactivated
// person still names their role.
func (s *Service) DeleteRole(ctx context.Context, venueID, actorID, roleID int64) error {
	return s.change(ctx, venueID, actorID, func(q *gen.Queries, mine auth.PermissionSet, _ time.Time) error {
		if err := mayGiveRole(ctx, q, venueID, roleID, mine); err != nil {
			return err
		}
		n, err := q.DeleteRole(ctx, gen.DeleteRoleParams{VenueID: venueID, ID: roleID})
		if err != nil {
			return fmt.Errorf("people: deleting role %d: %w", roleID, err)
		}
		if n == 0 {
			return ErrRoleInUse
		}
		return nil
	})
}

// change runs one change to people or roles in a transaction: it reads
// what the person acting holds, runs fn, and refuses the lot if nobody
// would be left who can manage staff.
func (s *Service) change(ctx context.Context, venueID, actorID int64, fn func(q *gen.Queries, mine auth.PermissionSet, now time.Time) error) error {
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		roleID, err := q.StaffRoleOf(ctx, gen.StaffRoleOfParams{VenueID: venueID, ID: actorID})
		if err != nil {
			return fmt.Errorf("people: reading the role of the person acting: %w", err)
		}
		mine, err := rolePerms(ctx, q, venueID, roleID)
		if err != nil {
			return err
		}
		if err := fn(q, mine, s.clock.Now().UTC()); err != nil {
			return err
		}
		left, err := q.StaffManagersLeft(ctx, venueID)
		if err != nil {
			return fmt.Errorf("people: counting who can manage staff: %w", err)
		}
		if left == 0 {
			return ErrLastManager
		}
		return nil
	})
}

// mayGiveRole checks a role is this venue's and within what mine holds.
func mayGiveRole(ctx context.Context, q *gen.Queries, venueID, roleID int64, mine auth.PermissionSet) error {
	theirs, err := rolePerms(ctx, q, venueID, roleID)
	if err != nil {
		return err
	}
	if !mine.Covers(theirs) {
		return ErrBeyondYou
	}
	return nil
}

// mayReach finds a person and checks their role is within what mine holds.
func mayReach(ctx context.Context, q *gen.Queries, venueID int64, staffID string, mine auth.PermissionSet) (gen.StaffForEditRow, error) {
	target, err := q.StaffForEdit(ctx, gen.StaffForEditParams{VenueID: venueID, PublicID: staffID})
	if errors.Is(err, sql.ErrNoRows) {
		return target, fmt.Errorf("%w: %s", ErrStaffNotFound, staffID)
	}
	if err != nil {
		return target, fmt.Errorf("people: reading %s: %w", staffID, err)
	}
	return target, mayGiveRole(ctx, q, venueID, target.RoleID, mine)
}

func rolePerms(ctx context.Context, q *gen.Queries, venueID, roleID int64) (auth.PermissionSet, error) {
	r, err := q.RoleForEdit(ctx, gen.RoleForEditParams{VenueID: venueID, ID: roleID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %d", ErrRoleNotFound, roleID)
	}
	if err != nil {
		return nil, fmt.Errorf("people: reading role %d: %w", roleID, err)
	}
	return permsOf(r.Permissions), nil
}

// permsOf reads a role's stored permissions, leniently: a key this build
// does not know grants nothing, as auth reads it at sign-in.
func permsOf(raw string) auth.PermissionSet {
	var keys []string
	set := auth.PermissionSet{}
	if json.Unmarshal([]byte(raw), &keys) != nil {
		return set
	}
	for _, k := range keys {
		if p, err := auth.ParsePermission(k); err == nil {
			set[p] = struct{}{}
		}
	}
	return set
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", &Invalid{Field: "name", Message: "Give a name."}
	case utf8.RuneCountInString(name) > MaxNameLength:
		return "", &Invalid{Field: "name", Message: fmt.Sprintf("Keep the name to %d characters.", MaxNameLength)}
	}
	return name, nil
}

func cleanRole(in RoleInput) (string, auth.PermissionSet, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", nil, err
	}
	keys := make([]string, len(in.Permissions))
	for i, p := range in.Permissions {
		keys[i] = string(p)
	}
	set, err := auth.NewPermissionSet(keys)
	if err != nil {
		return "", nil, &Invalid{Field: "permissions", Message: "One of those permissions does not exist."}
	}
	return name, set, nil
}

func pinHash(pin string) (string, error) {
	hash, err := auth.HashPIN(pin)
	if errors.Is(err, auth.ErrPINFormat) {
		return "", &Invalid{Field: "pin", Message: fmt.Sprintf("A PIN is %d to %d digits.", auth.MinPINLength, auth.MaxPINLength)}
	}
	return hash, err
}

func roleNameTaken(err error) error {
	if db.IsUniqueViolation(err) {
		return fmt.Errorf("%w: %w", ErrRoleNameTaken, &Invalid{Field: "name", Message: "A role with that name already exists."})
	}
	return err
}

func oneRow(n int64, err error, notFound error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return notFound
	}
	return nil
}
