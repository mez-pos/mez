package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Permission is one thing a staff member may do. The string is what a role
// stores, so a constant here is never renamed once it has shipped.
type Permission string

// The permissions this build knows. The list is exhaustive: a role granting
// anything not here grants nothing, because nothing asks for it.
const (
	// The register.
	PermOrderOpen      Permission = "order.open"      // open an order and add to it
	PermOrderEdit      Permission = "order.edit"      // change quantities, notes, courses, tickets
	PermLineVoid       Permission = "line.void"       // take a line off the bill
	PermLineComp       Permission = "line.comp"       // give a line away
	PermDiscountGive   Permission = "discount.give"   // discount a line or the order
	PermPaymentTake    Permission = "payment.take"    // tender a payment, issue the invoice, close
	PermPaymentVoid    Permission = "payment.void"    // void a tender
	PermRefundGive     Permission = "refund.give"     // refund against a tender
	PermOrderReopen    Permission = "order.reopen"    // reopen a closed order
	PermInvoiceReprint Permission = "invoice.reprint" // print a copy of an issued invoice
	PermKitchenView    Permission = "kitchen.view"    // see and bump the kitchen display
	PermShiftManage    Permission = "shift.manage"    // open and close the shift
	PermCashMove       Permission = "cash.move"       // authorise cash into or out of the drawer

	// The back office.
	PermDeviceManage Permission = "device.manage" // pair and revoke devices
	PermStaffManage  Permission = "staff.manage"  // add staff, set PINs and roles
	PermMenuEdit     Permission = "menu.edit"     // prices and the menu
	PermSettingsEdit Permission = "settings.edit" // tax, invoice series, venue settings
	PermReportsView  Permission = "reports.view"  // sales, voids, the Z-report
)

// all is every permission, in the order a back office screen should list
// them: the register first, then the office.
var all = []Permission{
	PermOrderOpen, PermOrderEdit, PermLineVoid, PermLineComp, PermDiscountGive,
	PermPaymentTake, PermPaymentVoid, PermRefundGive, PermOrderReopen, PermInvoiceReprint, PermKitchenView,
	PermShiftManage, PermCashMove,
	PermDeviceManage, PermStaffManage, PermMenuEdit, PermSettingsEdit, PermReportsView,
}

// ErrUnknownPermission means a permission key this build does not have.
var ErrUnknownPermission = errors.New("unknown permission")

// ErrForbidden means the staff member has no such permission.
var ErrForbidden = errors.New("not allowed")

// Permissions is every permission this build knows.
func Permissions() []Permission { return slices.Clone(all) }

// ParsePermission reads one permission key.
func ParsePermission(s string) (Permission, error) {
	p := Permission(s)
	if !slices.Contains(all, p) {
		return "", fmt.Errorf("%w: %q", ErrUnknownPermission, s)
	}
	return p, nil
}

// PermissionSet is what a role grants.
type PermissionSet map[Permission]struct{}

// NewPermissionSet builds a set, refusing any key this build does not know.
// Roles are written through this, so a typo fails in the back office rather
// than silently granting nothing.
func NewPermissionSet(keys []string) (PermissionSet, error) {
	set := make(PermissionSet, len(keys))
	for _, k := range keys {
		p, err := ParsePermission(k)
		if err != nil {
			return nil, err
		}
		set[p] = struct{}{}
	}
	return set, nil
}

// Has reports whether the set grants p.
func (s PermissionSet) Has(p Permission) bool {
	_, ok := s[p]
	return ok
}

// Keys is the set's permissions, in the canonical order.
func (s PermissionSet) Keys() []Permission {
	out := make([]Permission, 0, len(s))
	for _, p := range all {
		if s.Has(p) {
			out = append(out, p)
		}
	}
	return out
}

// JSON encodes the set for roles.permissions.
func (s PermissionSet) JSON() (string, error) {
	keys := make([]string, 0, len(s))
	for _, p := range s.Keys() {
		keys = append(keys, string(p))
	}
	raw, err := json.Marshal(keys)
	if err != nil {
		return "", fmt.Errorf("auth: encoding permissions: %w", err)
	}
	return string(raw), nil
}

// parseStoredPermissions reads a role's stored JSON array.
//
// A key this build does not know is dropped rather than refused, and
// returned so the caller can say so: a database written by a newer build
// must still let today's binary serve what it does understand, and a
// permission it cannot name is one it will never grant anyway.
func parseStoredPermissions(raw string) (PermissionSet, []string, error) {
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil, nil, fmt.Errorf("auth: reading a role's permissions: %w", err)
	}
	set := make(PermissionSet, len(keys))
	var unknown []string
	for _, k := range keys {
		p, err := ParsePermission(k)
		if err != nil {
			unknown = append(unknown, k)
			continue
		}
		set[p] = struct{}{}
	}
	return set, unknown, nil
}

// RoleTemplate is a role a venue starts with.
type RoleTemplate struct {
	Name        string
	Permissions []Permission
}

// DefaultRoles are the roles a new venue is created with. They are a
// starting point the owner edits, not a fixed hierarchy: the only rule the
// code enforces is that the owner has everything.
func DefaultRoles() []RoleTemplate {
	manager := []Permission{
		PermOrderOpen, PermOrderEdit, PermLineVoid, PermLineComp, PermDiscountGive,
		PermPaymentTake, PermPaymentVoid, PermRefundGive, PermOrderReopen, PermInvoiceReprint, PermKitchenView,
		PermShiftManage, PermCashMove,
		PermStaffManage, PermReportsView,
	}
	return []RoleTemplate{
		{Name: "owner", Permissions: Permissions()},
		{Name: "manager", Permissions: manager},
		{Name: "cashier", Permissions: []Permission{
			PermOrderOpen, PermOrderEdit, PermPaymentTake, PermKitchenView,
		}},
		{Name: "kitchen", Permissions: []Permission{PermKitchenView}},
	}
}
