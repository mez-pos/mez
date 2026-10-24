package winsvc

import "strings"

// startByUsersACE is the right for every signed-in user to start the
// service and ask after it, and no more: not to stop it, not to change it.
// In Windows's own letters: query its configuration (CC), its status (LC)
// and its dependents (SW), start it (RP), interrogate it (LO) and read its
// security (RC), for Authenticated Users (AU).
//
// The desktop shortcut starts a stopped service (ADR-366), and the person at
// the till is not an administrator; without this every start would ask for
// one.
const startByUsersACE = "(A;;CCLCSWRPLORC;;;AU)"

// withStartByUsers adds the right to a service's security descriptor in
// SDDL, as `sc sdshow` prints it and `sc sdset` takes it. A descriptor that
// already grants it is returned as it is. The entry goes at the end of the
// discretionary list (D:), before the audit list (S:) if there is one.
func withStartByUsers(sddl string) string {
	sddl = strings.TrimSpace(sddl)
	if strings.Contains(sddl, startByUsersACE) {
		return sddl
	}
	if i := strings.Index(sddl, "S:"); i >= 0 {
		return sddl[:i] + startByUsersACE + sddl[i:]
	}
	if !strings.HasPrefix(sddl, "D:") {
		return "D:" + startByUsersACE + sddl
	}
	return sddl + startByUsersACE
}
