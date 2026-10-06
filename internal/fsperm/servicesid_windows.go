//go:build windows

package fsperm

import (
	"crypto/sha1" // #nosec G505 -- the algorithm is Windows's, not a choice made here
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// serviceSID is the SID of the account Windows makes for a service by name,
// NT SERVICE\<name>: S-1-5-80 and the SHA-1 of the upper-cased name in
// UTF-16, as five numbers.
//
// It is worked out and not looked up, so that the venue's data can name the
// service's account before the service is installed and after an
// administrator's command has rewritten the ACL. No process holds this SID
// unless a service of this name exists, and making one takes an
// administrator, who can read the data already.
func serviceSID(name string) (*windows.SID, error) {
	units := utf16.Encode([]rune(strings.ToUpper(name)))
	raw := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(raw[2*i:], u)
	}
	sum := sha1.Sum(raw) // #nosec G401 -- see the import
	text := "S-1-5-80"
	for i := 0; i < len(sum); i += 4 {
		text += fmt.Sprintf("-%d", binary.LittleEndian.Uint32(sum[i:]))
	}
	sid, err := windows.StringToSid(text)
	if err != nil {
		return nil, fmt.Errorf("the service account's SID %s: %w", text, err)
	}
	return sid, nil
}
