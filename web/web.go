// Package web embeds the built SPA bundles into the binary.
//
// It exists here, at the root of web/, because a go:embed directive cannot
// reach outside its own package directory. It holds no serving logic: it knows
// where the bundles are, and internal/api knows how to serve them.
//
// Two builds. By default the placeholder bundles in web/placeholder are
// embedded, so a fresh clone compiles with no Node toolchain anywhere near it.
// Built with -tags frontend, the real web/*/dist output is embedded instead.
// See ADR-025.
package web

import (
	"fmt"
	"io/fs"
)

// Mount points. The terminal is the root because it is what the machine behind
// the counter opens and never navigates away from.
const (
	MountTerminal   = "/"
	MountBackoffice = "/backoffice/"
	MountGuest      = "/order/"
)

// Bundles maps each mount to a filesystem rooted at that bundle's index.html.
//
// It fails rather than returning a bundle that cannot answer its own mount: a
// missing index.html means the frontend build did not run or wrote somewhere
// else, and finding that out at startup is far better than serving 404s to a
// cashier.
func Bundles() (map[string]fs.FS, error) {
	roots, files := bundleRoots()

	bundles := make(map[string]fs.FS, len(roots))
	for mount, root := range roots {
		sub, err := fs.Sub(files, root)
		if err != nil {
			return nil, fmt.Errorf("mounting %s from %s: %w", mount, root, err)
		}
		if _, err := fs.Stat(sub, "index.html"); err != nil {
			return nil, fmt.Errorf("bundle for %s has no index.html in %s: %w", mount, root, err)
		}
		bundles[mount] = sub
	}
	return bundles, nil
}
