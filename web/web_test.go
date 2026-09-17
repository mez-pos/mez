package web

import (
	"io/fs"
	"strings"
	"testing"
)

// TestBundlesAreReallyEmbedded checks the files that ship, not a stand-in. The
// serving logic is tested against fstest.MapFS in internal/api; what can only be
// verified here is that the embed directive actually matched something and that
// each mount has the index.html the fallback depends on.
func TestBundlesAreReallyEmbedded(t *testing.T) {
	bundles, err := Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}

	for _, mount := range []string{MountTerminal, MountBackoffice, MountGuest} {
		t.Run(mount, func(t *testing.T) {
			fsys, ok := bundles[mount]
			if !ok {
				t.Fatalf("no bundle mounted at %s", mount)
			}

			index, err := fs.ReadFile(fsys, "index.html")
			if err != nil {
				t.Fatalf("reading index.html: %v", err)
			}
			if len(index) == 0 {
				t.Fatal("index.html is empty")
			}
			if !strings.Contains(string(index), "<!doctype html>") {
				t.Errorf("index.html does not look like a document: %q", firstLine(index))
			}
		})
	}
}

// TestEveryMountIsDistinct: three apps served from one binary, and a copy-paste
// in bundleRoots would quietly serve the terminal to a guest's phone.
func TestEveryMountIsDistinct(t *testing.T) {
	roots, _ := bundleRoots()

	if len(roots) != 3 {
		t.Fatalf("got %d mounts, want 3", len(roots))
	}

	seen := make(map[string]string, len(roots))
	for mount, root := range roots {
		if other, dup := seen[root]; dup {
			t.Errorf("%s and %s both serve %s", mount, other, root)
		}
		seen[root] = mount
	}
}

// TestPlaceholderSaysSoOnTheDefaultBuild keeps the startup warning honest. A
// build without -tags frontend ships placeholders, and the server has to be
// able to say that rather than leave someone staring at a paragraph of text.
func TestPlaceholderSaysSoOnTheDefaultBuild(t *testing.T) {
	if !Placeholder() {
		t.Skip("built with -tags frontend, so these are the real bundles")
	}

	bundles, err := Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}

	index, err := fs.ReadFile(bundles[MountTerminal], "index.html")
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	if !strings.Contains(string(index), "placeholder") {
		t.Errorf("the placeholder bundle does not say it is one: %q", firstLine(index))
	}
}

func firstLine(b []byte) string {
	if i := strings.IndexByte(string(b), '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}
