package auth

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestPermissionSet(t *testing.T) {
	set, err := NewPermissionSet([]string{"line.void", "payment.take"})
	if err != nil {
		t.Fatalf("NewPermissionSet: %v", err)
	}
	if !set.Has(PermLineVoid) || !set.Has(PermPaymentTake) || set.Has(PermLineComp) {
		t.Errorf("set = %v", set.Keys())
	}
	// Keys come back in the canonical order, not the order they went in.
	if got := set.Keys(); !slices.Equal(got, []Permission{PermLineVoid, PermPaymentTake}) {
		t.Errorf("Keys = %v", got)
	}
	raw, err := set.JSON()
	if err != nil || raw != `["line.void","payment.take"]` {
		t.Fatalf("JSON = %q, %v", raw, err)
	}
	back, unknown, err := parseStoredPermissions(raw)
	if err != nil || len(unknown) != 0 || !slices.Equal(back.Keys(), set.Keys()) {
		t.Errorf("round trip = %v, %v, %v", back.Keys(), unknown, err)
	}

	if _, err := NewPermissionSet([]string{"line.void", "line.burn"}); !errors.Is(err, ErrUnknownPermission) {
		t.Errorf("a typo was accepted: %v", err)
	}
	if _, err := ParsePermission("order.open"); err != nil {
		t.Errorf("ParsePermission: %v", err)
	}
}

// TestAStoredRoleIsReadLeniently is the compatibility rule: a role written
// by a newer build still works here, minus what this build cannot name.
func TestAStoredRoleIsReadLeniently(t *testing.T) {
	set, unknown, err := parseStoredPermissions(`["line.void","loyalty.grant","payment.take"]`)
	if err != nil {
		t.Fatalf("parseStoredPermissions: %v", err)
	}
	if !slices.Equal(unknown, []string{"loyalty.grant"}) {
		t.Errorf("unknown = %v", unknown)
	}
	if !set.Has(PermLineVoid) || !set.Has(PermPaymentTake) || len(set) != 2 {
		t.Errorf("set = %v", set.Keys())
	}
	if _, _, err := parseStoredPermissions(`not json`); err == nil {
		t.Error("a malformed permissions column was read as a role")
	}
	if _, _, err := parseStoredPermissions(`{}`); err == nil {
		t.Error("an object was read as a list of permissions")
	}
}

func TestDefaultRoles(t *testing.T) {
	roles := map[string][]Permission{}
	for _, r := range DefaultRoles() {
		roles[r.Name] = r.Permissions
	}
	if len(roles["owner"]) != len(Permissions()) {
		t.Errorf("the owner has %d of %d permissions", len(roles["owner"]), len(Permissions()))
	}
	// A manager has the menu, as the flow map says, and not the venue's
	// settings or its devices (ADR-342). Migration 00056 gives menu.edit to
	// the managers of venues that already exist.
	if !slices.Contains(roles["manager"], PermMenuEdit) {
		t.Error("a manager cannot edit the menu")
	}
	if slices.Contains(roles["manager"], PermSettingsEdit) || slices.Contains(roles["manager"], PermDeviceManage) {
		t.Error("a manager can change tax settings or pair devices")
	}
	if slices.Contains(roles["cashier"], PermLineVoid) || slices.Contains(roles["cashier"], PermDiscountGive) {
		t.Error("a cashier can void or discount without a manager")
	}
	if !slices.Equal(roles["kitchen"], []Permission{PermKitchenView, PermKitchenWork}) {
		t.Errorf("the kitchen role = %v", roles["kitchen"])
	}
	// The waiter takes orders, sends them and settles the bill at the table
	// (ADR-145). Nothing that gives money back or changes what was rung up
	// without a manager, and no say over the shift. The same list is in
	// migration 00031, which gives the role to venues that already exist.
	wantWaiter := []Permission{PermOrderOpen, PermOrderEdit, PermPaymentTake, PermKitchenView}
	if !slices.Equal(roles["waiter"], wantWaiter) {
		t.Errorf("the waiter role = %v, want %v", roles["waiter"], wantWaiter)
	}
	for _, p := range []Permission{PermLineVoid, PermLineComp, PermDiscountGive, PermPaymentVoid, PermRefundGive,
		PermOrderReopen, PermShiftOpen, PermShiftManage, PermCashMove} {
		if slices.Contains(roles["waiter"], p) {
			t.Errorf("a waiter can %s", p)
		}
	}
	// A cashier's day starts by counting the float in (ADR-152); closing, and
	// what closing brings with it, stays the manager's. The same grants are in
	// migration 00033, for venues that already exist.
	for _, name := range []string{"owner", "manager", "cashier"} {
		if !slices.Contains(roles[name], PermShiftOpen) {
			t.Errorf("the %s cannot open the shift", name)
		}
	}
	if slices.Contains(roles["cashier"], PermShiftManage) {
		t.Error("a cashier can close the shift")
	}
	// The kitchen moves its tickets, and so may the owner and a manager; a
	// cashier and a waiter see the board and no more (ADR-180). The same
	// grants are in migration 00034, for venues that already exist.
	for _, name := range []string{"owner", "manager", "kitchen"} {
		if !slices.Contains(roles[name], PermKitchenWork) {
			t.Errorf("the %s cannot move a ticket", name)
		}
	}
	for _, name := range []string{"cashier", "waiter"} {
		if slices.Contains(roles[name], PermKitchenWork) {
			t.Errorf("a %s can move a ticket", name)
		}
		if !slices.Contains(roles[name], PermKitchenView) {
			t.Errorf("a %s cannot see the kitchen's board", name)
		}
	}
	// A tip over the venue's ceiling is let through by a manager (ADR-261):
	// the owner and the manager, never the people who take the tip. The same
	// grants are in migration 00053, for venues that already exist.
	for _, name := range []string{"owner", "manager"} {
		if !slices.Contains(roles[name], PermTipApprove) {
			t.Errorf("the %s cannot approve a tip", name)
		}
	}
	for _, name := range []string{"cashier", "waiter", "kitchen"} {
		if slices.Contains(roles[name], PermTipApprove) {
			t.Errorf("a %s can approve their own tip", name)
		}
	}
	// Every template is made of permissions this build knows.
	for name, perms := range roles {
		for _, p := range perms {
			if _, err := ParsePermission(string(p)); err != nil {
				t.Errorf("role %s: %v", name, err)
			}
		}
	}
}

// TestAuthoriseChecksTheAuthorisersPermission is the manager override at the
// level it is decided: PIN first, then what that staff member may do.
func TestAuthoriseChecksTheAuthorisersPermission(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	grant(t, w, w.ravi.RoleID, PermLineVoid)

	by, err := w.auth.Authorise(ctx, w.till, "stf_RAVI0001", "1357", PermLineVoid)
	if err != nil || by.ID != w.ravi.ID {
		t.Fatalf("Authorise = %+v, %v", by, err)
	}
	// It opens no session: the manager walks away again.
	if _, err := w.auth.Session(ctx, w.till, "nfs_anything"); !errors.Is(err, ErrNoSession) {
		t.Errorf("session after an override: %v", err)
	}

	tests := []struct {
		name  string
		staff string
		pin   string
		perm  Permission
		want  error
	}{
		{"the right PIN, the wrong permission", "stf_RAVI0001", "1357", PermRefundGive, ErrForbidden},
		{"a staff member with no permissions at all", "stf_ASHA0001", "2468", PermLineVoid, ErrForbidden},
		{"a wrong PIN", "stf_RAVI0001", "9999", PermLineVoid, ErrInvalidPIN},
		{"nobody", "stf_NOBODY00", "1357", PermLineVoid, ErrInvalidPIN},
		{"a staff member of another venue", "stf_MEERA001", "2468", PermLineVoid, ErrInvalidPIN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := w.auth.Authorise(ctx, w.till, tt.staff, tt.pin, tt.perm); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestARefusedPermissionIsNotAFailedPIN keeps the two apart: a manager who
// typed their PIN correctly must not be locked out for lacking a permission.
func TestARefusedPermissionIsNotAFailedPIN(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	for range DefaultStaffLockout.MaxFailures + 2 {
		if _, err := w.auth.Authorise(ctx, w.till, "stf_ASHA0001", "2468", PermLineVoid); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v, want ErrForbidden", err)
		}
	}
	if _, err := w.auth.SignIn(ctx, w.till, "stf_ASHA0001", "2468"); err != nil {
		t.Errorf("signing in after refused overrides: %v", err)
	}
	for _, o := range outcomes(t, w.db) {
		if o != "success" {
			t.Errorf("recorded %q; a correct PIN refused on permission is not a failed attempt", o)
		}
	}
}

func TestPermissionsOfARole(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	grant(t, w, w.ravi.RoleID, PermLineVoid, PermPaymentVoid)

	held, err := w.auth.Permissions(ctx, w.ravi.RoleID)
	if err != nil || !held.Has(PermLineVoid) || !held.Has(PermPaymentVoid) || held.Has(PermMenuEdit) {
		t.Errorf("Permissions = %v, %v", held.Keys(), err)
	}

	// A role that is not there grants nothing, rather than failing: a staff
	// member whose role was deleted can sign in and do nothing.
	held, err = w.auth.Permissions(ctx, 999)
	if err != nil || len(held) != 0 {
		t.Errorf("a missing role = %v, %v", held.Keys(), err)
	}

	// So does one whose column cannot be read at all.
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE roles SET permissions = '"nonsense"' WHERE id = ?`, w.ravi.RoleID); err != nil {
		t.Fatal(err)
	}
	held, err = w.auth.Permissions(ctx, w.ravi.RoleID)
	if err != nil || len(held) != 0 {
		t.Errorf("an unreadable role = %v, %v", held.Keys(), err)
	}
}

// grant replaces a role's permissions.
func grant(t *testing.T, w *staffWorld, roleID int64, perms ...Permission) {
	t.Helper()
	keys := make([]string, len(perms))
	for i, p := range perms {
		keys[i] = string(p)
	}
	set, err := NewPermissionSet(keys)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := set.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Writer().ExecContext(context.Background(),
		`UPDATE roles SET permissions = ? WHERE id = ?`, raw, roleID); err != nil {
		t.Fatal(err)
	}
}

func TestCovers(t *testing.T) {
	set := func(ps ...Permission) PermissionSet {
		s := PermissionSet{}
		for _, p := range ps {
			s[p] = struct{}{}
		}
		return s
	}
	manager := set(PermStaffManage, PermReportsView, PermOrderOpen)
	tests := []struct {
		name  string
		other PermissionSet
		want  bool
	}{
		{"nothing", set(), true},
		{"some of its own", set(PermOrderOpen), true},
		{"all of its own", manager, true},
		{"one it does not hold", set(PermOrderOpen, PermSettingsEdit), false},
		{"the owner's", set(Permissions()...), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manager.Covers(tt.other); got != tt.want {
				t.Errorf("Covers = %v, want %v", got, tt.want)
			}
		})
	}
}
