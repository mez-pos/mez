package dbhealth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/db"
)

// TestDBCheckMapsOntoTheAPIVocabulary is the seam between two packages that
// deliberately do not import each other. They use the same three words today,
// so a passthrough would work by coincidence; this fails if either side ever
// changes one.
func TestDBCheckMapsOntoTheAPIVocabulary(t *testing.T) {
	database, err := db.Open(context.Background(), db.Options{
		Path: filepath.Join(t.TempDir(), db.FileName),
	})
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	t.Cleanup(func() { database.Close(context.Background()) })

	check := Check(database)

	// Open probes once, so a database that just opened is already ok.
	if got := check(); got != api.CheckOK {
		t.Errorf("check() = %q on a healthy database, want %q", got, api.CheckOK)
	}

	// Closing both pools is what a deleted file looks like from here.
	if err := database.Close(context.Background()); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if got := check(); got != api.CheckOK {
		t.Errorf("check() = %q after close, want the last cached answer %q", got, api.CheckOK)
	}
}

func TestDBCheckTranslatesEveryStatus(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{name: "ok", from: db.StatusOK, want: api.CheckOK},
		{name: "failing", from: db.StatusFailing, want: api.CheckFailing},
		{name: "unknown", from: db.StatusUnknown, want: api.CheckUnknown},
		{name: "anything else", from: "surprising", want: api.CheckUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := translate(tt.from); got != tt.want {
				t.Errorf("translate(%q) = %q, want %q", tt.from, got, tt.want)
			}
		})
	}
}
