package tablecodes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

var seededAt = time.Date(2026, 10, 5, 4, 30, 0, 0, time.UTC)

// openDB is a venue with three tables on its floor, and a second venue with
// one, so that a code cannot be read across them.
func openDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for i, name := range []string{"Filter Coffee House", "The Other Place"} {
			venueID := int64(i + 1)
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: fmt.Sprintf("ven_CODES%03d", venueID), Name: name, Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: seededAt, UpdatedAt: seededAt,
			}); err != nil {
				return err
			}
			area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: venueID, Name: "Floor", CreatedAt: seededAt, UpdatedAt: seededAt})
			if err != nil {
				return err
			}
			tables := 3
			if venueID == 2 {
				tables = 1
			}
			for n := range tables {
				if _, err := q.CreateTable(ctx, gen.CreateTableParams{
					VenueID: venueID, AreaID: area, Name: fmt.Sprintf("T%d", n+1), Seats: 4, Shape: "square", X: int64(n), Y: 0,
					CreatedAt: seededAt, UpdatedAt: seededAt,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newService(t *testing.T, d *db.DB) *Service {
	t.Helper()
	s, err := New(d, func() time.Time { return seededAt.Add(time.Hour) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(nil, nil, nil); err == nil {
		t.Fatal("New accepted no database")
	}
}

func TestEnsureGivesEveryTableACodeAndKeepsTheOnesItHas(t *testing.T) {
	ctx := context.Background()
	s := newService(t, openDB(t))

	before, err := s.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 3 || before[0].Code != "" || before[0].TableName != "T1" {
		t.Fatalf("a new floor: %+v, want three tables with no codes", before)
	}

	first, err := s.Ensure(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range first {
		if !WellFormed(c.Code) {
			t.Errorf("table %s got %q, which is not a code", c.TableName, c.Code)
		}
		if seen[c.Code] {
			t.Errorf("two tables share the code %q", c.Code)
		}
		seen[c.Code] = true
	}

	// A sticker already on a table goes on working.
	again, err := s.Ensure(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if again[i] != first[i] {
			t.Errorf("Ensure changed table %s's code from %q to %q", first[i].TableName, first[i].Code, again[i].Code)
		}
	}

	// And the other venue's floor was not touched.
	other, err := s.List(ctx, 2)
	if err != nil || len(other) != 1 || other[0].Code != "" {
		t.Errorf("the other venue: %+v, %v", other, err)
	}
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	s := newService(t, d)
	codes, err := s.Ensure(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := s.Ensure(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Resolve(ctx, codes[1].Code)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Table{VenueID: 1, VenueName: "Filter Coffee House", Currency: "INR", TableID: codes[1].TableID, TableName: "T2"}
	if got != want {
		t.Errorf("Resolve = %+v, want %+v", got, want)
	}
	// A code says which venue it is for; nobody has to be told.
	if got, err := s.Resolve(ctx, theirs[0].Code); err != nil || got.VenueID != 2 || got.VenueName != "The Other Place" {
		t.Errorf("the other venue's code: %+v, %v", got, err)
	}

	code := codes[0].Code
	tests := []struct {
		name string
		code string
	}{
		{"empty", ""},
		{"a code nobody made", strings.Repeat("2", Length)},
		{"the right code in capitals", strings.ToUpper(code)},
		{"one character short", code[:Length-1]},
		{"one character over", code + "2"},
		{"with a space", code[:Length-1] + " "},
		{"a letter the alphabet leaves out", code[:Length-1] + "o"},
		{"an attempt at SQL", "' OR 1=1 --x"},
		{"a wildcard", strings.Repeat("%", Length)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := s.Resolve(ctx, tt.code); !errors.Is(err, ErrUnknown) {
				t.Errorf("Resolve = %+v, %v; want ErrUnknown", got, err)
			}
		})
	}

	t.Run("a table taken off the floor", func(t *testing.T) {
		err := d.WriteTx(ctx, func(q *gen.Queries) error {
			_, err := q.SetTableActive(ctx, gen.SetTableActiveParams{Active: false, Now: seededAt, ID: codes[2].TableID, VenueID: 1})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Resolve(ctx, codes[2].Code); !errors.Is(err, ErrUnknown) {
			t.Errorf("Resolve = %v, want ErrUnknown", err)
		}
		if list, _ := s.List(ctx, 1); len(list) != 2 {
			t.Errorf("the list still has %d tables", len(list))
		}
	})
}

func TestRenewTurnsTheOldCodeOffForGood(t *testing.T) {
	ctx := context.Background()
	s := newService(t, openDB(t))
	codes, err := s.Ensure(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	old := codes[0].Code

	fresh, err := s.Renew(ctx, 1, codes[0].TableID)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if fresh == old || !WellFormed(fresh) {
		t.Fatalf("Renew gave %q after %q", fresh, old)
	}
	if _, err := s.Resolve(ctx, old); !errors.Is(err, ErrUnknown) {
		t.Errorf("the sticker that was replaced still works: %v", err)
	}
	if got, err := s.Resolve(ctx, fresh); err != nil || got.TableID != codes[0].TableID {
		t.Errorf("the new code: %+v, %v", got, err)
	}
	// The table has one live code, and its neighbours kept theirs.
	list, err := s.List(ctx, 1)
	if err != nil || len(list) != 3 || list[0].Code != fresh || list[1].Code != codes[1].Code {
		t.Errorf("after a renewal: %+v, %v", list, err)
	}

	if _, err := s.Renew(ctx, 1, 9999); err == nil {
		t.Error("Renew made a code for a table that does not exist")
	}
}

func TestDrawIsUnbiasedAndFailsWithItsSource(t *testing.T) {
	// 0xFF and 0xF8 are past the last whole multiple of the alphabet and
	// must be thrown away; then twelve bytes that index it in order.
	src := append([]byte{0xFF, 0xF8}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 30}...)
	s := &Service{random: bytes.NewReader(src)}
	got, err := s.draw()
	if err != nil {
		t.Fatal(err)
	}
	if want := alphabet[:11] + alphabet[30:]; got != want {
		t.Errorf("draw = %q, want %q", got, want)
	}

	short := &Service{random: bytes.NewReader([]byte{1, 2, 3})}
	if code, err := short.draw(); err == nil {
		t.Errorf("draw made %q from a source that ran dry", code)
	}
}

func TestWellFormed(t *testing.T) {
	if len(alphabet) != 31 || strings.ContainsAny(alphabet, "01ilo") {
		t.Fatalf("the alphabet %q has a character that reads as another", alphabet)
	}
	tests := []struct {
		in   string
		want bool
	}{
		{"23456789abcd", true},
		{strings.Repeat("z", Length), true},
		{"", false},
		{"23456789abc", false},
		{"23456789abcde", false},
		{"23456789ABCD", false},
		{"23456789abc0", false},
		{"23456789abc/", false},
		{"23456789abé", false},
	}
	for _, tt := range tests {
		if got := WellFormed(tt.in); got != tt.want {
			t.Errorf("WellFormed(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
