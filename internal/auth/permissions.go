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
	PermKitchenView    Permission = "kitchen.view"    // see the kitchen's board
	PermKitchenWork    Permission = "kitchen.work"    // start a ticket, mark it ready, recall it (ADR-180)
	PermShiftOpen      Permission = "shift.open"      // count the float in and open the shift (ADR-152)
	PermShiftManage    Permission = "shift.manage"    // close the shift, pool its tips, correct its Z-report
	PermCashMove       Permission = "cash.move"       // authorise cash into or out of the drawer

	// The back office.
	PermDeviceManage Permission = "device.manage" // pair and revoke devices
	PermStaffManage  Permission = "staff.manage"  // add staff, set PINs and roles
	PermMenuEdit     Permission = "menu.edit"     // prices and the menu
	PermSettingsEdit Permission = "settings.edit" // tax, invoice series, venue settings
	PermReportsView  Permission = "reports.view"  // sales, voids, the Z-report
	PermStockManage  Permission = "stock.manage"  // stock items, deliveries, waste, counts (ADR-111)
)

// all is every permission, in the order a back office screen should list
// them: the register first, then the office.
var all = []Permission{
	PermOrderOpen, PermOrderEdit, PermLineVoid, PermLineComp, PermDiscountGive,
	PermPaymentTake, PermPaymentVoid, PermRefundGive, PermOrderReopen, PermInvoiceReprint, PermKitchenView, PermKitchenWork,
	PermShiftOpen, PermShiftManage, PermCashMove,
	PermDeviceManage, PermStaffManage, PermMenuEdit, PermSettingsEdit, PermReportsView, PermStockManage,
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

// Covers reports whether s holds every permission in other. Someone may
// only hand out what they hold themselves (ADR-115): a manager who can
// manage staff cannot make anyone, themselves included, an owner.
func (s PermissionSet) Covers(other PermissionSet) bool {
	for p := range other {
		if !s.Has(p) {
			return false
		}
	}
	return true
}

// Everything reports whether the set holds every permission this build
// has: the owner's set. Nobody holds more, so some checks that look for
// someone above are answered by this instead.
func (s PermissionSet) Everything() bool {
	for _, p := range all {
		if !s.Has(p) {
			return false
		}
	}
	return true
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
		PermPaymentTake, PermPaymentVoid, PermRefundGive, PermOrderReopen, PermInvoiceReprint, PermKitchenView, PermKitchenWork,
		PermShiftOpen, PermShiftManage, PermCashMove,
		PermStaffManage, PermReportsView, PermStockManage,
	}
	return []RoleTemplate{
		{Name: "owner", Permissions: Permissions()},
		{Name: "manager", Permissions: manager},
		// A cashier counts the float in and opens the shift (ADR-152); closing
		// it stays with shift.manage. Migration 00033 gives shift.open to the
		// cashiers, and to whoever could already open, of venues that exist.
		{Name: "cashier", Permissions: []Permission{
			PermOrderOpen, PermOrderEdit, PermPaymentTake, PermKitchenView, PermShiftOpen,
		}},
		// A waiter works a tablet: takes orders, sends them to the kitchen and
		// settles the bill at the table (ADR-145). The cashier's permissions
		// less opening the shift; they also land elsewhere after signing in.
		// Migration 00031 gives this role to venues that already exist.
		{Name: "waiter", Permissions: []Permission{
			PermOrderOpen, PermOrderEdit, PermPaymentTake, PermKitchenView,
		}},
		// The kitchen sees its board and moves the tickets on it. A cashier
		// and a waiter see it and no more: the pass is not theirs to clear
		// (ADR-180). Migration 00034 gives kitchen.work to venues that exist.
		{Name: "kitchen", Permissions: []Permission{PermKitchenView, PermKitchenWork}},
	}
}
