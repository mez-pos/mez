//go:build frontend

package web

import "embed"

// files holds the real Vite output. This file only compiles once the frontends
// have been built: with no web/*/dist on disk, the directive below fails with
// "pattern ... no matching files found", which is the correct and obvious
// error for asking to ship bundles that were never built.
//
// Phase 05 creates the frontends and adds -tags frontend to the release build.
//
//go:embed all:terminal/dist all:backoffice/dist all:guest/dist
var files embed.FS

func bundleRoots() (map[string]string, embed.FS) {
	return map[string]string{
		MountTerminal:   "terminal/dist",
		MountBackoffice: "backoffice/dist",
		MountGuest:      "guest/dist",
	}, files
}

// Placeholder reports whether these are the stand-in bundles.
func Placeholder() bool { return false }
