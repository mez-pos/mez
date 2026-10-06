//go:build windows

package fsperm

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestRestrictSetsTheDACL: a directory any user on the machine could read,
// as one under C:\ProgramData is, is left to SYSTEM, the administrators and
// this account. Its old ACEs go, inherited ones too, and a file made in it
// afterwards inherits the same three and nothing else.
func TestRestrictSetsTheDACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	everyone := sid(t, windows.WinWorldSid)
	users := sid(t, windows.WinBuiltinUsersSid)
	open := []windows.EXPLICIT_ACCESS{grant(everyone), grant(users)}
	acl, err := windows.ACLFromEntries(open, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if aces := sids(t, dir); !contains(aces, everyone) || !contains(aces, users) {
		t.Fatal("the test could not open the directory up first")
	}
	inside := filepath.Join(dir, "mez.db")
	if err := os.WriteFile(inside, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := Restrict(dir); err != nil {
		t.Fatal(err)
	}

	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Error("the DACL still inherits from the parent")
	}
	me := currentUser(t)
	allowed := []*windows.SID{sid(t, windows.WinLocalSystemSid), sid(t, windows.WinBuiltinAdministratorsSid), me}
	later := filepath.Join(dir, "logs")
	if err := os.Mkdir(later, 0o750); err != nil {
		t.Fatal(err)
	}
	laterFile := filepath.Join(later, "mez.log")
	if err := os.WriteFile(laterFile, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
	}{
		{"the directory", dir},
		{"a file already in it", inside},
		{"a directory made after", later},
		{"a file made after", laterFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aces := sids(t, tt.path)
			for _, bad := range []*windows.SID{everyone, users, sid(t, windows.WinAuthenticatedUserSid)} {
				if contains(aces, bad) {
					t.Errorf("%s still has an ACE for %s", tt.path, bad)
				}
			}
			for _, s := range aces {
				if !contains(allowed, s) && !s.Equals(owner(t, tt.path)) {
					t.Errorf("%s has an ACE for %s, who is not SYSTEM, an administrator or this account", tt.path, s)
				}
			}
			if !contains(aces, me) {
				t.Errorf("%s has no ACE for this account", tt.path)
			}
		})
	}
	// The account still reads and writes what it restricted.
	if err := os.WriteFile(filepath.Join(dir, "after.txt"), []byte("x"), 0o640); err != nil {
		t.Errorf("writing in the restricted directory: %v", err)
	}
}

// TestRestrictAFile: config.toml gets the same three, inheriting nothing.
func TestRestrictAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(file, []byte("x = 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Restrict(file); err != nil {
		t.Fatal(err)
	}
	aces := sids(t, file)
	if contains(aces, sid(t, windows.WinBuiltinUsersSid)) || contains(aces, sid(t, windows.WinWorldSid)) || !contains(aces, currentUser(t)) {
		t.Errorf("the file's ACEs: %v", aces)
	}
	if _, err := os.ReadFile(file); err != nil {
		t.Errorf("reading the restricted file: %v", err)
	}
}

func grant(s *windows.SID) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_READ,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeValue: windows.TrusteeValueFromSID(s)},
	}
}

func sid(t *testing.T, kind windows.WELL_KNOWN_SID_TYPE) *windows.SID {
	t.Helper()
	s, err := windows.CreateWellKnownSid(kind)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func currentUser(t *testing.T) *windows.SID {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Sid
}

func owner(t *testing.T, path string) *windows.SID {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// sids is whom path's DACL names, inherited ACEs included.
func sids(t *testing.T, path string) []*windows.SID {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var out []*windows.SID
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		out = append(out, (*windows.SID)(unsafe.Pointer(&ace.SidStart)))
	}
	return out
}

func contains(list []*windows.SID, s *windows.SID) bool {
	for _, x := range list {
		if x.Equals(s) {
			return true
		}
	}
	return false
}
