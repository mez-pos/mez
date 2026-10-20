package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/config"
)

// TestRestrictWarnsAndGoesOn: a path that cannot be restricted is a Warn
// naming it, never a stop (ADR-259).
func TestRestrictWarnsAndGoesOn(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, nil))
	missing := filepath.Join(t.TempDir(), "not-there")
	restrict(log, "data directory", missing)
	if got := out.String(); !strings.Contains(got, "level=WARN") || !strings.Contains(got, "data directory") || !strings.Contains(got, "not-there") {
		t.Errorf("log = %q, want a Warn naming the path", got)
	}
}

// TestRestrictConfig: config.toml beside the binary is restricted on
// Windows only, and a missing one is no warning at all.
func TestRestrictConfig(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		file     bool
		wantWarn bool
	}{
		{"no file on Windows", "windows", false, false},
		{"no file on Linux", "linux", false, false},
		{"a file on Linux is the administrator's", "linux", true, false},
		{"a file on Windows", "windows", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.goos == "windows" && tt.file && runtime.GOOS != "windows" {
				t.Skip("restricting a file the Windows way needs Windows")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, config.FileName)
			if tt.file {
				if err := os.WriteFile(path, []byte("listen_addr = \":7777\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			restrictConfig(slog.New(slog.NewTextHandler(&out, nil)), tt.goos, dir)
			if warned := strings.Contains(out.String(), "level=WARN"); warned != tt.wantWarn {
				t.Errorf("warned = %v, want %v: %s", warned, tt.wantWarn, out.String())
			}
			if !tt.file {
				return
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.goos == "linux" && runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
				t.Errorf("the administrator's file was changed to %o", info.Mode().Perm())
			}
			if _, err := os.ReadFile(path); err != nil {
				t.Errorf("the server can no longer read its config: %v", err)
			}
		})
	}
}
