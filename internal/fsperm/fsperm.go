// Package fsperm keeps the venue's data to the people who run the server
// (ADR-259). The data directory holds the database, with every sale, staff
// PIN hash and device token in it; the logs; and config.toml can hold the
// relay key and a payment gateway's secret. None of it is for another
// account on the same computer.
//
// Off Windows a mode does this. On Windows the mode is ignored, and a
// directory made under C:\ProgramData inherits an ACL that lets every user
// on the machine read it, so Restrict replaces the ACL.
package fsperm

import (
	"fmt"
	"os"
)

// ServiceName is the Windows service whose own account, NT SERVICE\mez, is
// always among those Restrict leaves path to (ADR-264). It is winsvc.Name,
// restated so that this package imports nothing of ours; a test in cmd/mez
// holds the two together.
const ServiceName = "mez"

// Restrict leaves path to its owner. A directory keeps what is made in it
// later to the same people.
//
// On Windows that is SYSTEM, the Administrators group, the Windows service's
// own account, the account running this process, and the account that owns
// path (the one that ran setup), each with full control, inherited by
// everything beneath a directory, and nothing inherited from the parent.
// Elsewhere it is mode 0700 for a directory and 0600 for a file.
func Restrict(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("restricting %s: %w", path, err)
	}
	if err := restrict(path, info.IsDir()); err != nil {
		return fmt.Errorf("restricting %s: %w", path, err)
	}
	return nil
}
