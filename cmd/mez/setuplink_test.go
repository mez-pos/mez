package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/service/firstrun"
)

var linkShape = regexp.MustCompile(`^http://localhost:7777/setup#token=[0-9a-f]{32}\n$`)

func TestSetupLink(t *testing.T) {
	t.Run("a machine waiting for its venue gets the address with the token", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := firstrun.OpenToken(dir, true); err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := runSetupLink([]string{"-data-dir", dir}, &out); err != nil {
			t.Fatalf("setup-link: %v", err)
		}
		if !linkShape.MatchString(out.String()) {
			t.Fatalf("printed %q, want one line: the wizard's address with the token", out.String())
		}
		token, _ := firstrun.ReadToken(dir)
		if !strings.HasSuffix(strings.TrimSpace(out.String()), "#token="+token) {
			t.Errorf("printed %q, which is not the token on disk", out.String())
		}
	})

	t.Run("with no token there is the plain address, and no failure", func(t *testing.T) {
		for name, prepare := range map[string]func(dir string){
			"a server that has not started": func(string) {},
			"a venue that exists": func(dir string) {
				if _, err := firstrun.OpenToken(dir, false); err != nil {
					t.Fatal(err)
				}
			},
			"a file that is not a token": func(dir string) {
				if err := os.WriteFile(filepath.Join(dir, firstrun.TokenFile), []byte("hello\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		} {
			dir := t.TempDir()
			prepare(dir)
			var out strings.Builder
			if err := runSetupLink([]string{"-data-dir", dir}, &out); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if out.String() != "http://localhost:7777\n" {
				t.Errorf("%s: printed %q", name, out.String())
			}
		}
	})

	t.Run("refusals", func(t *testing.T) {
		if err := runSetupLink([]string{"now"}, &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
			t.Errorf("an argument: %v", err)
		}
		if err := runSetupLink([]string{"-h"}, &strings.Builder{}); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("-h: %v", err)
		}
	})
}

func TestLocalURL(t *testing.T) {
	tests := []struct {
		listen, want string
	}{
		{":7777", "http://localhost:7777"},
		{"0.0.0.0:7777", "http://localhost:7777"},
		{"192.168.1.10:8080", "http://localhost:8080"},
		{"[::]:7777", "http://localhost:7777"},
		{":80", "http://localhost"},
	}
	for _, tt := range tests {
		if got, err := localURL(tt.listen); err != nil || got != tt.want {
			t.Errorf("localURL(%q) = %q, %v, want %q", tt.listen, got, err, tt.want)
		}
	}
	if _, err := localURL("7777"); err == nil {
		t.Error("localURL took an address with no port")
	}
}

// TestSetupLinkIsNotAnnouncedToAFile: the token is said to a person at a
// terminal and to nothing that keeps what it is told.
func TestSetupLinkIsNotAnnouncedToAFile(t *testing.T) {
	dir := t.TempDir()
	token, err := firstrun.OpenToken(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ListenAddr: ":7777", DataDir: dir}

	var buf strings.Builder
	announceSetupLink(&buf, cfg, token)
	file, err := os.Create(filepath.Join(t.TempDir(), "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	announceSetupLink(file, cfg, token)
	_ = file.Close()
	written, _ := os.ReadFile(file.Name())
	if buf.Len() != 0 || len(written) != 0 {
		t.Errorf("the link was written to a buffer (%q) or a file (%q)", buf.String(), written)
	}
}
