//go:build windows

package fsperm

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// fileAllAccess is FILE_ALL_ACCESS, what Explorer calls Full control.
const fileAllAccess = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF

func restrict(path string, dir bool) error {
	trustees, err := trustees(path)
	if err != nil {
		return err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if dir {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entries := make([]windows.EXPLICIT_ACCESS, len(trustees))
	for i, sid := range trustees {
		entries[i] = windows.EXPLICIT_ACCESS{
			AccessPermissions: fileAllAccess,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		}
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("building the ACL: %w", err)
	}
	// PROTECTED: the parent's ACEs (C:\ProgramData lets Users read) stop
	// flowing in. Windows passes the new ACEs down to what is already in a
	// directory, replacing what those inherited from it.
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("setting the ACL: %w", err)
	}
	return nil
}

// trustees is who keeps full control of path: SYSTEM, the administrators,
// the Windows service's account, this process's account and path's owner,
// each once.
func trustees(path string) ([]*windows.SID, error) {
	var out []*windows.SID
	add := func(sid *windows.SID) {
		for _, s := range out {
			if s.Equals(sid) {
				return
			}
		}
		out = append(out, sid)
	}
	for _, kind := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		sid, err := windows.CreateWellKnownSid(kind)
		if err != nil {
			return nil, fmt.Errorf("looking up a well-known account: %w", err)
		}
		add(sid)
	}
	// The Windows service's own account (ADR-264), named whether or not the
	// service is installed yet: `mez setup` in an administrator's terminal
	// must not shut out the service that starts next.
	service, err := serviceSID(ServiceName)
	if err != nil {
		return nil, err
	}
	add(service)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("reading this process's account: %w", err)
	}
	add(user.User.Sid)
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return nil, fmt.Errorf("reading the owner: %w", err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return nil, fmt.Errorf("reading the owner: %w", err)
	}
	if owner != nil {
		add(owner)
	}
	return out, nil
}
