//go:build !frontend

package web

import "embed"

// files holds the placeholder bundles. They are committed, tiny and always
// present, which is what lets `go build ./...` work on a machine that has never
// run npm. web/*/dist is build output and is gitignored, so embedding it
// unconditionally would break a fresh clone and every CI run before Phase 05.
//
//go:embed all:placeholder
var files embed.FS

func bundleRoots() (map[string]string, embed.FS) {
	return map[string]string{
		MountTerminal:   "placeholder/terminal",
		MountBackoffice: "placeholder/backoffice",
		MountGuest:      "placeholder/guest",
	}, files
}

// Placeholder reports whether these are the stand-in bundles rather than the
// real frontends, so the server can say so at startup instead of leaving
// someone to wonder why the register is a paragraph of text.
func Placeholder() bool { return true }
