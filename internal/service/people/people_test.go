package people

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

var now = time.Date(2026, 10, 1, 6, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// venue is a fixture venue: its roles by name, and its people by name as
// {id, public id}.
type venue struct {
	id     int64
	roles  map[string]int64
	people map[string]member
}

type member struct {
	id       int64
	publicID string
}

type fixture struct {
	s    *Service
	d    *db.DB
	cafe venue // Priya owns it, Meera manages it, Asha is a cashier
	solo venue // Ravi owns it alone
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	f := fixture{d: d}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		seed := func(publicID string, people map[string]string) (venue, error) {
			v := venue{roles: map[string]int64{}, people: map[string]member{}}
			row, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: publicID, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
			})
			if err != nil {
				return v, err
			}
			v.id = row.ID
			for _, tpl := range auth.DefaultRoles() {
				keys := make([]string, len(tpl.Permissions))
				for i, p := range tpl.Permissions {
					keys[i] = string(p)
				}
				set, err := auth.NewPermissionSet(keys)
				if err != nil {
					return v, err
				}
				raw, err := set.JSON()
				if err != nil {
					return v, err
				}
				r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.id, Name: tpl.Name, Permissions: raw, CreatedAt: now})
				if err != nil {
					return v, err
				}
				v.roles[tpl.Name] = r.ID
			}
			for name, role := range people {
				pub := "stf_" + name[:4] + publicID[len(publicID)-4:]
				s, err := q.CreateStaff(ctx, gen.CreateStaffParams{
					PublicID: pub, VenueID: v.id, RoleID: v.roles[role], Name: name, PinHash: "x", Active: true, CreatedAt: now, UpdatedAt: now,
				})
				if err != nil {
					return v, err
				}
				v.people[name] = member{id: s.ID, publicID: pub}
			}
			return v, nil
		}
		var err error
		if f.cafe, err = seed("ven_PEOPLE01", map[string]string{"Priya": "owner", "Meera": "manager", "Asha": "cashier"}); err != nil {
			return err
		}
		f.solo, err = seed("ven_PEOPLE02", map[string]string{"Ravi": "owner"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.s, err = New(d, clock{}); err != nil {
		t.Fatal(err)
	}
	return f
}

func fieldOf(err error) string {
	var invalid *Invalid
	if errors.As(err, &invalid) {
		return invalid.Field
	}
	return ""
}

func (f fixture) person(t *testing.T, v venue, publicID string) Person {
	t.Helper()
	all, err := f.s.People(context.Background(), v.id)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.ID == publicID {
			return p
		}
	}
	t.Fatalf("no %s", publicID)
	return Person{}
}

func TestPeopleAndRoles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	people, err := f.s.People(ctx, f.cafe.id)
	if err != nil || len(people) != 3 || people[0].Name != "Asha" || people[0].RoleName != "cashier" || !people[0].Active {
		t.Fatalf("people = %+v, %v", people, err)
	}
	roles, err := f.s.Roles(ctx, f.cafe.id)
	if err != nil || len(roles) != len(auth.DefaultRoles()) {
		t.Fatalf("roles = %+v, %v", roles, err)
	}
	held := map[string]int64{"owner": 1, "manager": 1, "cashier": 1, "waiter": 0, "kitchen": 0}
	for _, r := range roles {
		if r.People != held[r.Name] {
			t.Errorf("%s held by %d, want %d", r.Name, r.People, held[r.Name])
		}
		if r.Name == "owner" && !slices.Equal(r.Permissions, auth.Permissions()) {
			t.Errorf("owner grants %v", r.Permissions)
		}
	}
}

func TestAddingAndChangingStaff(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.cafe.people["Priya"].id

	id, err := f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: " Kiran ", RoleID: f.cafe.roles["cashier"], PIN: "1357"})
	if err != nil {
		t.Fatal(err)
	}
	if p := f.person(t, f.cafe, id); p.Name != "Kiran" || p.RoleName != "cashier" || !p.Active || len(id) < 5 || id[:4] != "stf_" {
		t.Errorf("added = %+v (%s)", p, id)
	}
	if err := f.s.UpdateStaff(ctx, f.cafe.id, owner, id, StaffInput{Name: "Kiran S", RoleID: f.cafe.roles["manager"]}); err != nil {
		t.Fatal(err)
	}
	if p := f.person(t, f.cafe, id); p.Name != "Kiran S" || p.RoleName != "manager" {
		t.Errorf("updated = %+v", p)
	}

	tests := []struct {
		name  string
		err   error
		field string
		is    error
	}{
		{"no name", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: " ", RoleID: f.cafe.roles["cashier"], PIN: "1357"})), "name", nil},
		{"a PIN too short", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "X", RoleID: f.cafe.roles["cashier"], PIN: "12"})), "pin", nil},
		{"a PIN of letters", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "X", RoleID: f.cafe.roles["cashier"], PIN: "abcd"})), "pin", nil},
		{"another venue's role", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "X", RoleID: f.solo.roles["cashier"], PIN: "1357"})), "", ErrRoleNotFound},
		{"another venue's person", f.s.UpdateStaff(ctx, f.cafe.id, owner, f.solo.people["Ravi"].publicID, StaffInput{Name: "X", RoleID: f.cafe.roles["cashier"]}), "", ErrStaffNotFound},
		{"deactivating yourself", f.s.SetStaffActive(ctx, f.cafe.id, owner, f.cafe.people["Priya"].publicID, false), "active", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.is != nil {
				if !errors.Is(tt.err, tt.is) {
					t.Errorf("err = %v, want %v", tt.err, tt.is)
				}
				return
			}
			if fieldOf(tt.err) != tt.field {
				t.Errorf("err = %v, want a refusal on %q", tt.err, tt.field)
			}
		})
	}

	asha := f.cafe.people["Asha"].publicID
	if err := f.s.SetStaffActive(ctx, f.cafe.id, owner, asha, false); err != nil {
		t.Fatal(err)
	}
	if f.person(t, f.cafe, asha).Active {
		t.Error("Asha is still active")
	}
	if err := f.s.SetStaffActive(ctx, f.cafe.id, owner, asha, true); err != nil || !f.person(t, f.cafe, asha).Active {
		t.Errorf("bringing Asha back: %v", err)
	}
}

func ignore(_ string, err error) error { return err }

// A manager can manage staff, but only within what a manager holds.
func TestNobodyHandsOutMoreThanTheyHold(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	meera := f.cafe.people["Meera"].id
	priya := f.cafe.people["Priya"].publicID
	asha := f.cafe.people["Asha"].publicID

	beyond := []struct {
		name string
		err  error
	}{
		{"adding an owner", ignore(f.s.AddStaff(ctx, f.cafe.id, meera, StaffInput{Name: "X", RoleID: f.cafe.roles["owner"], PIN: "1357"}))},
		{"making a cashier an owner", f.s.UpdateStaff(ctx, f.cafe.id, meera, asha, StaffInput{Name: "Asha", RoleID: f.cafe.roles["owner"]})},
		{"renaming the owner", f.s.UpdateStaff(ctx, f.cafe.id, meera, priya, StaffInput{Name: "P", RoleID: f.cafe.roles["owner"]})},
		{"deactivating the owner", f.s.SetStaffActive(ctx, f.cafe.id, meera, priya, false)},
		{"resetting the owner's PIN", f.s.ResetPIN(ctx, f.cafe.id, meera, priya, "0000")},
		{"a role with settings", ignore2(f.s.AddRole(ctx, f.cafe.id, meera, RoleInput{Name: "Shift lead", Permissions: []auth.Permission{auth.PermSettingsEdit}}))},
		{"editing the owner role", f.s.UpdateRole(ctx, f.cafe.id, meera, f.cafe.roles["owner"], RoleInput{Name: "owner", Permissions: nil})},
		{"giving the cashier role more", f.s.UpdateRole(ctx, f.cafe.id, meera, f.cafe.roles["cashier"], RoleInput{Name: "cashier", Permissions: []auth.Permission{auth.PermDeviceManage}})},
		{"deleting the owner role", f.s.DeleteRole(ctx, f.cafe.id, meera, f.cafe.roles["owner"])},
	}
	for _, tt := range beyond {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, ErrBeyondYou) {
				t.Errorf("err = %v, want beyond you", tt.err)
			}
		})
	}

	// Within a manager's own: fine.
	if _, err := f.s.AddStaff(ctx, f.cafe.id, meera, StaffInput{Name: "Kiran", RoleID: f.cafe.roles["cashier"], PIN: "1357"}); err != nil {
		t.Errorf("a manager adding a cashier: %v", err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, meera, asha, "2468"); err != nil {
		t.Errorf("a manager resetting a cashier's PIN: %v", err)
	}
	if _, err := f.s.AddRole(ctx, f.cafe.id, meera, RoleInput{Name: "Barista", Permissions: []auth.Permission{auth.PermOrderOpen, auth.PermKitchenView}}); err != nil {
		t.Errorf("a manager adding a role within theirs: %v", err)
	}
}

func ignore2(_ int64, err error) error { return err }

func TestAVenueKeepsSomeoneWhoCanManageStaff(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ravi := f.solo.people["Ravi"]

	refused := []struct {
		name string
		err  error
	}{
		{"the only owner becomes a cashier", f.s.UpdateStaff(ctx, f.solo.id, ravi.id, ravi.publicID, StaffInput{Name: "Ravi", RoleID: f.solo.roles["cashier"]})},
		{"the owner role loses staff.manage", f.s.UpdateRole(ctx, f.solo.id, ravi.id, f.solo.roles["owner"], RoleInput{Name: "owner", Permissions: []auth.Permission{auth.PermOrderOpen}})},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, ErrLastManager) {
				t.Errorf("err = %v, want last manager", tt.err)
			}
		})
	}
	if p := f.person(t, f.solo, ravi.publicID); p.RoleName != "owner" {
		t.Errorf("a refused change must change nothing: Ravi is %s", p.RoleName)
	}

	// With a manager beside her, the cafe's owner may step down to manager.
	priya := f.cafe.people["Priya"]
	if err := f.s.UpdateStaff(ctx, f.cafe.id, priya.id, priya.publicID, StaffInput{Name: "Priya", RoleID: f.cafe.roles["manager"]}); err != nil {
		t.Errorf("stepping down with a manager left: %v", err)
	}
}

// The cloud's owner is signed in by mez Cloud and nothing else (ADR-323):
// nobody gives them a PIN, the owner included, and they are not who keeps a
// venue managed.
func TestTheCloudsOwnerCannotBeGivenAPIN(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: f.d, Now: clock{}.Now})
	if err != nil {
		t.Fatal(err)
	}
	cloudOwner, err := staff.PairCloudOwner(ctx, f.cafe.id, "Priya Menon")
	if err != nil {
		t.Fatal(err)
	}
	priya := f.cafe.people["Priya"]

	err = f.s.ResetPIN(ctx, f.cafe.id, priya.id, cloudOwner.PublicID, "246813")
	if fieldOf(err) != "pin" || !strings.Contains(err.Error(), "mez Cloud") {
		t.Errorf("the owner giving the cloud's owner a PIN: %v, want it refused on the PIN", err)
	}
	// Not by the cloud's owner either, signed in from away as themselves.
	err = f.s.ResetPIN(ctx, f.cafe.id, cloudOwner.ID, cloudOwner.PublicID, "246813")
	if fieldOf(err) != "pin" {
		t.Errorf("the cloud's owner giving themselves a PIN: %v, want it refused on the PIN", err)
	}
	var hash string
	if err := f.d.Reader().QueryRowContext(ctx, `SELECT pin_hash FROM staff WHERE id = ?`, cloudOwner.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != auth.NoPIN {
		t.Errorf("the cloud's owner has the PIN hash %q, want none", hash)
	}

	// They are on the People list, and can be turned off there.
	if p := f.person(t, f.cafe, priya.publicID); p.Cloud {
		t.Error("the venue's own owner is listed as the cloud's")
	}
	if p := f.person(t, f.cafe, cloudOwner.PublicID); !p.Active || p.Name != "Priya Menon" || !p.Cloud {
		t.Errorf("the cloud's owner on the People list: %+v", p)
	}
	if err := f.s.SetStaffActive(ctx, f.cafe.id, priya.id, cloudOwner.PublicID, false); err != nil {
		t.Errorf("deactivating the cloud's owner: %v", err)
	}

	// A venue's one owner at the venue cannot be lost to the cloud's: the
	// cloud's owner cannot stand at a till, so is not who keeps it managed.
	solo, err := staff.PairCloudOwner(ctx, f.solo.id, "Ravi Kumar")
	if err != nil {
		t.Fatal(err)
	}
	ravi := f.solo.people["Ravi"]
	if err := f.s.SetStaffActive(ctx, f.solo.id, solo.ID, ravi.publicID, false); err == nil {
		t.Error("the cloud's owner deactivated the only person who manages staff at the venue")
	}
}

func TestAResetPINSignsThePersonOut(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	asha := f.cafe.people["Asha"]
	w := f.d.Writer()
	if _, err := w.ExecContext(ctx, `INSERT INTO devices (id, public_id, venue_id, name, kind, created_at)
		VALUES (9, 'dev_PEOPLE01', ?, 'Till', 'terminal', '2026-10-01 06:00:00+00:00')`, f.cafe.id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO staff_sessions (token_hash, venue_id, staff_id, device_id, created_at, expires_at)
		VALUES (?, ?, ?, 9, '2026-10-01 06:00:00+00:00', '2026-10-01 18:00:00+00:00')`, strings.Repeat("a", 64), f.cafe.id, asha.id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, f.cafe.people["Priya"].id, asha.publicID, "97531"); err != nil {
		t.Fatal(err)
	}
	var open int
	if err := w.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_sessions WHERE staff_id = ? AND ended_at IS NULL`, asha.id).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 0 {
		t.Errorf("%d sessions still open after a new PIN", open)
	}
	var hash string
	if err := w.QueryRowContext(ctx, `SELECT pin_hash FROM staff WHERE id = ?`, asha.id).Scan(&hash); err != nil || hash == "x" || hash[:9] != "$argon2id" {
		t.Errorf("stored hash = %q, %v", hash, err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, asha.id, asha.publicID, "1234"); err != nil {
		t.Errorf("changing your own PIN: %v", err)
	}
}

func TestRoles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.cafe.people["Priya"].id

	id, err := f.s.AddRole(ctx, f.cafe.id, owner, RoleInput{Name: "Barista", Permissions: []auth.Permission{auth.PermKitchenView, auth.PermOrderOpen}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.UpdateRole(ctx, f.cafe.id, owner, id, RoleInput{Name: "Head barista", Permissions: []auth.Permission{auth.PermOrderOpen, auth.PermLineVoid}}); err != nil {
		t.Fatal(err)
	}
	roles, _ := f.s.Roles(ctx, f.cafe.id)
	var got Role
	for _, r := range roles {
		if r.ID == id {
			got = r
		}
	}
	if got.Name != "Head barista" || !slices.Equal(got.Permissions, []auth.Permission{auth.PermOrderOpen, auth.PermLineVoid}) || got.People != 0 {
		t.Errorf("role = %+v", got)
	}

	if _, err := f.s.AddRole(ctx, f.cafe.id, owner, RoleInput{Name: "cashier"}); !errors.Is(err, ErrRoleNameTaken) || fieldOf(err) != "name" {
		t.Errorf("a name taken: %v", err)
	}
	if _, err := f.s.AddRole(ctx, f.cafe.id, owner, RoleInput{Name: "X", Permissions: []auth.Permission{"tea.make"}}); fieldOf(err) != "permissions" {
		t.Errorf("a permission that does not exist: %v", err)
	}
	if err := f.s.DeleteRole(ctx, f.cafe.id, owner, f.cafe.roles["cashier"]); !errors.Is(err, ErrRoleInUse) {
		t.Errorf("deleting a role someone holds: %v", err)
	}
	if err := f.s.DeleteRole(ctx, f.cafe.id, owner, id); err != nil {
		t.Errorf("deleting a role nobody holds: %v", err)
	}
	if err := f.s.UpdateRole(ctx, f.cafe.id, owner, f.solo.roles["cashier"], RoleInput{Name: "X"}); !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("another venue's role: %v", err)
	}
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Error("New with no database must fail")
	}
}

// TestAPINIsResetOnlyFromAbove: knowing a PIN is being that person, so a
// peer may not reset a peer's. A manager resets a cashier's, the owner
// resets anyone's, and nobody but the owner resets the owner's.
func TestAPINIsResetOnlyFromAbove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.cafe.people["Priya"].id
	meera := f.cafe.people["Meera"]
	other, err := f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Rohan", RoleID: f.cafe.roles["manager"], PIN: "567890"})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.s.ResetPIN(ctx, f.cafe.id, meera.id, other, "111111"); !errors.Is(err, ErrPeerPIN) {
		t.Errorf("a manager resetting a manager's PIN: err = %v, want ErrPeerPIN", err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, owner, other, "222222"); err != nil {
		t.Errorf("the owner resetting a manager's PIN: %v", err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, meera.id, meera.publicID, "333333"); err != nil {
		t.Errorf("a manager changing their own PIN: %v", err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, meera.id, f.cafe.people["Asha"].publicID, "4444"); err != nil {
		t.Errorf("a manager resetting a cashier's PIN: %v", err)
	}
}

// TestDeactivationEndsTheSessions: a session merely hidden while the person
// is inactive would work again, with no PIN, the moment they were brought
// back. It is ended instead.
func TestDeactivationEndsTheSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	asha := f.cafe.people["Asha"]
	w := f.d.Writer()
	if _, err := w.ExecContext(ctx, `INSERT INTO devices (id, public_id, venue_id, name, kind, created_at)
		VALUES (9, 'dev_PEOPLE02', ?, 'Till', 'terminal', '2026-10-01 06:00:00+00:00')`, f.cafe.id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ExecContext(ctx, `INSERT INTO staff_sessions (token_hash, venue_id, staff_id, device_id, created_at, expires_at)
		VALUES (?, ?, ?, 9, '2026-10-01 06:00:00+00:00', '2026-10-01 18:00:00+00:00')`, strings.Repeat("b", 64), f.cafe.id, asha.id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetStaffActive(ctx, f.cafe.id, f.cafe.people["Priya"].id, asha.publicID, false); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetStaffActive(ctx, f.cafe.id, f.cafe.people["Priya"].id, asha.publicID, true); err != nil {
		t.Fatal(err)
	}
	var open int
	if err := w.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_sessions WHERE staff_id = ? AND ended_at IS NULL`, asha.id).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 0 {
		t.Errorf("%d sessions open again after a deactivation and a return", open)
	}
}

// TestPINLengthFollowsTheRole: a role that manages staff, devices or
// settings needs six digits, at an add and at a reset; any other keeps four.
func TestPINLengthFollowsTheRole(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.cafe.people["Priya"].id
	meera := f.cafe.people["Meera"]
	const tooShort = "A PIN for a role that manages staff, devices or settings is 6 to 8 digits."

	tests := []struct {
		name    string
		err     error
		message string // empty: no refusal
	}{
		{"a manager with four", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Rohan", RoleID: f.cafe.roles["manager"], PIN: "1357"})), tooShort},
		{"an owner with five", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Dev", RoleID: f.cafe.roles["owner"], PIN: "13579"})), tooShort},
		{"a manager with six", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Rohan", RoleID: f.cafe.roles["manager"], PIN: "135791"})), ""},
		{"a cashier with four", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Kiran", RoleID: f.cafe.roles["cashier"], PIN: "1357"})), ""},
		{"a manager with nine", ignore(f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Zoya", RoleID: f.cafe.roles["manager"], PIN: "135791357"})), "A PIN is 4 to 8 digits."},
		{"a reset of a manager's to four", f.s.ResetPIN(ctx, f.cafe.id, owner, meera.publicID, "2468"), tooShort},
		{"a manager's own to five", f.s.ResetPIN(ctx, f.cafe.id, meera.id, meera.publicID, "24680"), tooShort},
		{"a reset of a manager's to six", f.s.ResetPIN(ctx, f.cafe.id, owner, meera.publicID, "246802"), ""},
		{"a reset of a cashier's to four", f.s.ResetPIN(ctx, f.cafe.id, meera.id, f.cafe.people["Asha"].publicID, "2468"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var invalid *Invalid
			switch {
			case tt.message == "" && tt.err != nil:
				t.Errorf("err = %v, want none", tt.err)
			case tt.message != "" && (!errors.As(tt.err, &invalid) || invalid.Field != "pin" || invalid.Message != tt.message):
				t.Errorf("err = %v, want %q on pin", tt.err, tt.message)
			}
		})
	}
}

// hardLock locks a person as auth does at the third lockout, with the
// lockouts that caused it.
func hardLock(t *testing.T, f fixture, staffID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	err := f.d.WriteTx(ctx, func(q *gen.Queries) error {
		for i := range auth.HardLockAfter {
			lockout := gen.RecordStaffLockoutParams{VenueID: f.cafe.id, StaffID: staffID, At: at.Add(time.Duration(i-auth.HardLockAfter) * time.Minute)}
			if err := q.RecordStaffLockout(ctx, lockout); err != nil {
				return err
			}
		}
		_, err := q.SetStaffHardLock(ctx, gen.SetStaffHardLockParams{LockedAt: sql.NullTime{Time: at, Valid: true}, Reason: auth.HardLockReason, ID: staffID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestClearingALock: a hard lock shows on the person, and only someone who
// can manage staff, and reaches them, clears it; nobody clears their own.
func TestClearingALock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.cafe.people["Priya"].id
	meera := f.cafe.people["Meera"]
	asha := f.cafe.people["Asha"]
	kiran, err := f.s.AddStaff(ctx, f.cafe.id, owner, StaffInput{Name: "Kiran", RoleID: f.cafe.roles["cashier"], PIN: "1357"})
	if err != nil {
		t.Fatal(err)
	}
	lockedAt := now.Add(-time.Minute)
	hardLock(t, f, asha.id, lockedAt)
	hardLock(t, f, meera.id, lockedAt)
	if p := f.person(t, f.cafe, asha.publicID); p.LockedAt == nil || !p.LockedAt.Equal(lockedAt) || p.LockedAt.Location() != time.UTC {
		t.Fatalf("a locked person = %+v, want locked at %v in UTC", p, lockedAt)
	}
	if p := f.person(t, f.cafe, kiran); p.LockedAt != nil {
		t.Errorf("an unlocked person shows a lock: %+v", p)
	}

	refused := []struct {
		name  string
		err   error
		is    error
		field string
	}{
		{"a cashier clears a cashier", f.s.ClearLock(ctx, f.cafe.id, asha.id, kiran), ErrBeyondYou, ""},
		{"a manager clears their own", f.s.ClearLock(ctx, f.cafe.id, meera.id, meera.publicID), nil, "locked"},
		{"a manager clears the owner", f.s.ClearLock(ctx, f.cafe.id, meera.id, f.cafe.people["Priya"].publicID), ErrBeyondYou, ""},
		{"another venue's person", f.s.ClearLock(ctx, f.cafe.id, owner, f.solo.people["Ravi"].publicID), ErrStaffNotFound, ""},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if tt.is != nil && !errors.Is(tt.err, tt.is) {
				t.Errorf("err = %v, want %v", tt.err, tt.is)
			}
			if tt.is == nil && fieldOf(tt.err) != tt.field {
				t.Errorf("err = %v, want a refusal on %q", tt.err, tt.field)
			}
		})
	}
	if p := f.person(t, f.cafe, meera.publicID); p.LockedAt == nil {
		t.Error("a refused clear cleared Meera's lock")
	}

	if err := f.s.ClearLock(ctx, f.cafe.id, owner, meera.publicID); err != nil {
		t.Fatalf("the owner clears a manager: %v", err)
	}
	if err := f.s.ClearLock(ctx, f.cafe.id, owner, meera.publicID); err != nil {
		t.Errorf("clearing a lock already cleared: %v", err)
	}
	if p := f.person(t, f.cafe, meera.publicID); p.LockedAt != nil {
		t.Errorf("after the clear: %+v", p)
	}
	var left int
	if err := f.d.Writer().QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_lockouts WHERE staff_id = ? AND cleared_at IS NULL`, meera.id).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d lockouts still count after the clear (%v)", left, err)
	}
}

// TestAClearedPersonSignsInAgain runs the whole of it through auth: three
// lockouts hard-lock Asha, the right PIN is refused, a manager clears the
// lock, and the right PIN signs her in at once, inside the window of the
// wrong PINs that caused it.
func TestAClearedPersonSignsInAgain(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	asha := f.cafe.people["Asha"]
	if _, err := f.d.Writer().ExecContext(ctx, `INSERT INTO devices (id, public_id, venue_id, name, kind, created_at)
		VALUES (9, 'dev_PEOPLE03', ?, 'Till', 'terminal', '2026-10-01 05:00:00+00:00')`, f.cafe.id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ResetPIN(ctx, f.cafe.id, f.cafe.people["Meera"].id, asha.publicID, "2468"); err != nil {
		t.Fatal(err)
	}
	// The wrong PINs end a few seconds before now, when the clear is.
	at := now.Add(-time.Duration(auth.HardLockAfter-1)*auth.DefaultStaffLockout.Window - time.Minute)
	a, err := auth.NewStaffAuth(auth.StaffOptions{DB: f.d, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	till := auth.Device{ID: 9, VenueID: f.cafe.id, Kind: auth.KindTerminal}
	for lockout := range auth.HardLockAfter {
		for range auth.DefaultStaffLockout.MaxFailures {
			at = at.Add(time.Second)
			if _, err := a.SignIn(ctx, till, asha.publicID, "0000"); err != auth.ErrInvalidPIN {
				t.Fatalf("lockout %d: err = %v", lockout+1, err)
			}
		}
		if lockout < auth.HardLockAfter-1 {
			at = at.Add(auth.DefaultStaffLockout.Window)
		}
	}
	at = now.Add(time.Second)
	if _, err := a.SignIn(ctx, till, asha.publicID, "2468"); err == nil {
		t.Fatal("a hard-locked person signed in")
	}
	if err := f.s.ClearLock(ctx, f.cafe.id, f.cafe.people["Meera"].id, asha.publicID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SignIn(ctx, till, asha.publicID, "2468"); err != nil {
		t.Errorf("after the clear, the right PIN: %v", err)
	}
}
