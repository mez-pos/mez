package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// TestVenueAtStart: health names the one venue a server serves (ADR-416),
// the one `mez setup` printed, and none before there is one.
func TestVenueAtStart(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	empty := opened(t, t.TempDir())
	if err := empty.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if got := venueAtStart(ctx, empty, log); got != "" {
		t.Errorf("before setup: %q, want none", got)
	}

	dir, args := setupArgs(t)
	out := runSetupIn(t, args)
	if got, want := venueAtStart(ctx, opened(t, dir), log), field(t, out, "Venue"); got != want || want == "" {
		t.Errorf("after setup: %q, want %q", got, want)
	}
}
