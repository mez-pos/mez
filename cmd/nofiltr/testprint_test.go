package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/device/escpos"
)

func TestIsTestPrint(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"--test-print", "10.0.0.5"}, true},
		{[]string{"-test-print=10.0.0.5"}, true},
		{[]string{"--test-print=10.0.0.5", "-paper", "58"}, true},
		{[]string{"setup"}, false},
		{[]string{"-listen-addr", ":8080"}, false},
		{[]string{"test-print"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := isTestPrint(tt.args); got != tt.want {
			t.Errorf("isTestPrint(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

// recorder is a printer transport that keeps what it was sent.
type recorder struct {
	addr string
	data []byte
	err  error
}

func (r *recorder) send(_ context.Context, addr string, data []byte) error {
	r.addr, r.data = addr, data
	return r.err
}

func TestRunTestPrint(t *testing.T) {
	at := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)

	t.Run("sends the page for the paper asked for", func(t *testing.T) {
		var out bytes.Buffer
		rec := &recorder{}
		if err := runTestPrint([]string{"--test-print", "10.0.0.5", "-paper", "58", "-drawer"}, &out, at, rec.send); err != nil {
			t.Fatal(err)
		}
		want, _ := escpos.TestPage(escpos.Paper58, at, buildInfo().Version, true)
		if rec.addr != "10.0.0.5" || !bytes.Equal(rec.data, want) {
			t.Errorf("sent %d bytes to %q, want the 58mm page with the drawer", len(rec.data), rec.addr)
		}
		if !strings.Contains(out.String(), "32 columns") || !strings.Contains(out.String(), "drawer should have opened") {
			t.Errorf("told the owner %q", out.String())
		}
	})

	t.Run("saves instead of sending", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.bin")
		rec := &recorder{}
		if err := runTestPrint([]string{"-test-print=", "-save", path}, &bytes.Buffer{}, at, rec.send); err != nil {
			t.Fatal(err)
		}
		saved, err := os.ReadFile(path)
		want, _ := escpos.TestPage(escpos.Paper80, at, buildInfo().Version, false)
		if err != nil || !bytes.Equal(saved, want) || rec.data != nil {
			t.Errorf("saved %d bytes (%v), sent %d", len(saved), err, len(rec.data))
		}
	})

	t.Run("a printer that does not answer says what to check", func(t *testing.T) {
		rec := &recorder{err: errors.New("connection refused")}
		err := runTestPrint([]string{"--test-print", "10.0.0.5"}, &bytes.Buffer{}, at, rec.send)
		if err == nil || !strings.Contains(err.Error(), "Check the printer is on") {
			t.Errorf("err = %v", err)
		}
	})

	refusals := []struct {
		name string
		args []string
		want string
	}{
		{"no address", []string{"--test-print="}, "give the printer's address"},
		{"a paper width no printer has", []string{"--test-print", "10.0.0.5", "-paper", "76"}, "80 or 58"},
		{"a stray argument", []string{"--test-print", "10.0.0.5", "extra"}, "unexpected argument"},
		{"an unknown flag", []string{"--test-print", "10.0.0.5", "-colour"}, "not defined"},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			err := runTestPrint(tt.args, &bytes.Buffer{}, at, (&recorder{}).send)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}

	t.Run("help is a question, not a failure", func(t *testing.T) {
		var out bytes.Buffer
		if err := runTestPrint([]string{"--test-print", "x", "-h"}, &out, at, (&recorder{}).send); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("err = %v", err)
		}
		if !strings.Contains(out.String(), "-drawer") {
			t.Errorf("usage = %q", out.String())
		}
	})
}
