package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db"
)

// runPairIn runs the command and returns what it printed.
func runPairIn(t *testing.T, args ...string) string {
	t.Helper()
	var out strings.Builder
	if err := runPair(args, &out); err != nil {
		t.Fatalf("pair: %v\n%s", err, out.String())
	}
	return out.String()
}

// TestPairMakesACodeThatPairs: the point of the command. A venue set up with
// a till has no screen to reach the back office from, and this is the way.
func TestPairMakesACodeThatPairs(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t) // the first device is a terminal

	runSetupIn(t, args)
	out := runPairIn(t, "-data-dir", dir, "-device-kind", "back_office", "-device-name", "Office")

	d := opened(t, dir)
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(ctx, field(t, out, "Pairing code"))
	if err != nil {
		t.Fatalf("redeeming the printed code: %v", err)
	}
	if paired.Device.Name != "Office" || paired.Device.Kind != auth.KindBackOffice {
		t.Errorf("paired = %+v", paired.Device)
	}
	if !strings.Contains(out, "/backoffice/") {
		t.Errorf("the summary does not say where the back office is:\n%s", out)
	}
}

// TestPairNamesADeviceByItsKind: the name is optional.
func TestPairNamesADeviceByItsKind(t *testing.T) {
	tests := []struct {
		kind auth.DeviceKind
		name string
	}{
		{auth.KindTerminal, "Counter"},
		{auth.KindKitchen, "Kitchen"},
		{auth.KindBackOffice, "Back office"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			dir, args := setupArgs(t)
			runSetupIn(t, args)
			out := runPairIn(t, "-data-dir", dir, "-device-kind", string(tt.kind))

			d := opened(t, dir)
			devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
			if err != nil {
				t.Fatal(err)
			}
			paired, err := devices.Redeem(context.Background(), field(t, out, "Pairing code"))
			if err != nil {
				t.Fatal(err)
			}
			if paired.Device.Name != tt.name || paired.Device.Kind != tt.kind {
				t.Errorf("paired = %+v, want %q, %s", paired.Device, tt.name, tt.kind)
			}
		})
	}
}

// TestPairCanBeRunAgain: unlike setup, it is meant to be run again, and each
// run is a code of its own.
func TestPairCanBeRunAgain(t *testing.T) {
	dir, args := setupArgs(t)
	runSetupIn(t, args)
	first := field(t, runPairIn(t, "-data-dir", dir), "Pairing code")
	second := field(t, runPairIn(t, "-data-dir", dir), "Pairing code")
	if first == second {
		t.Errorf("two runs made the same code %s", first)
	}
}

// TestPairExpiryIsOnTheVenuesClock: a Docker container is on UTC, and a code
// that says 08:40 for a cafe in Bengaluru sends the owner to the wrong minute.
func TestPairExpiryIsOnTheVenuesClock(t *testing.T) {
	dir, args := setupArgs(t) // Asia/Kolkata
	runSetupIn(t, args)
	if out := runPairIn(t, "-data-dir", dir); !strings.Contains(out, "IST") {
		t.Errorf("the expiry is not in the venue's zone:\n%s", out)
	}
}

func TestPairRefusals(t *testing.T) {
	t.Run("no database leaves none behind", func(t *testing.T) {
		dir := t.TempDir()
		err := runPair([]string{"-data-dir", dir}, &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "run `mez setup` first") {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, db.FileName)); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("a refused pair made a database: %v", statErr)
		}
	})

	t.Run("a database with no venue", func(t *testing.T) {
		dir := t.TempDir()
		d, err := db.Open(context.Background(), db.Options{Path: filepath.Join(dir, db.FileName)})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		err = runPair([]string{"-data-dir", dir}, &strings.Builder{})
		if !errors.Is(err, ErrNoVenue) {
			t.Errorf("err = %v, want ErrNoVenue", err)
		}
	})

	t.Run("flags", func(t *testing.T) {
		dir, args := setupArgs(t)
		runSetupIn(t, args)
		tests := []struct {
			name  string
			extra []string
			says  string
		}{
			{"a device that cannot be paired", []string{"-device-kind", "guest"}, "cannot be a"},
			{"a name too long", []string{"-device-name", strings.Repeat("x", 200)}, "1 to 64 characters"},
			{"an argument that is not a flag", []string{"today"}, "unexpected argument"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := runPair(append([]string{"-data-dir", dir}, tt.extra...), &strings.Builder{})
				if err == nil || !strings.Contains(err.Error(), tt.says) {
					t.Errorf("err = %v, want one mentioning %q", err, tt.says)
				}
			})
		}
	})
}
