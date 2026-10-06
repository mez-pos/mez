package fsperm

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRestrictChmods: off Windows a directory is the owner's alone, 0700,
// and a file 0600.
func TestRestrictChmods(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores the mode; TestRestrictSetsTheDACL covers it")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(file, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		want os.FileMode
	}{
		{"a directory", dir, 0o700},
		{"a file", file, 0o600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Restrict(tt.path); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != tt.want {
				t.Errorf("mode = %o, want %o", got, tt.want)
			}
		})
	}
}

func TestRestrictMissing(t *testing.T) {
	if err := Restrict(filepath.Join(t.TempDir(), "not-there")); err == nil {
		t.Error("restricting a path that is not there succeeded")
	}
}
