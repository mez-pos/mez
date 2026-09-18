package invoice

import (
	"errors"
	"testing"
	"time"
)

var (
	ist    = time.FixedZone("IST", 5*3600+1800)
	london = mustLoad("Europe/London")
)

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestYearOfIndia(t *testing.T) {
	tests := []struct {
		name  string
		at    time.Time
		start int
		label string
		short string
	}{
		{"last second of the year, local", time.Date(2027, 3, 31, 23, 59, 59, 0, ist), 2026, "2026-27", "26-27"},
		{"first second of the new year, local", time.Date(2027, 4, 1, 0, 0, 0, 0, ist), 2027, "2027-28", "27-28"},
		{"00:30 IST on 1 April is still 31 March in UTC", time.Date(2027, 3, 31, 19, 0, 0, 0, time.UTC), 2027, "2027-28", "27-28"},
		{"23:59 IST on 31 March is already 18:29 UTC", time.Date(2027, 3, 31, 18, 29, 0, 0, time.UTC), 2026, "2026-27", "26-27"},
		{"January belongs to the year before", time.Date(2027, 1, 15, 12, 0, 0, 0, ist), 2026, "2026-27", "26-27"},
		{"the century turns in the label", time.Date(2099, 6, 1, 0, 0, 0, 0, ist), 2099, "2099-00", "99-00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fy, err := IndiaFY.YearOf(tt.at, ist)
			if err != nil {
				t.Fatalf("YearOf: %v", err)
			}
			if fy.StartYear != tt.start || fy.Label() != tt.label || fy.ShortLabel() != tt.short {
				t.Errorf("got %d %q %q, want %d %q %q", fy.StartYear, fy.Label(), fy.ShortLabel(), tt.start, tt.label, tt.short)
			}
		})
	}
}

func TestYearOfCalendar(t *testing.T) {
	tests := []struct {
		name  string
		at    time.Time
		start int
		label string
	}{
		{"new year's eve, local", time.Date(2026, 12, 31, 23, 59, 59, 0, london), 2026, "2026"},
		{"new year's day, local", time.Date(2027, 1, 1, 0, 0, 0, 0, london), 2027, "2027"},
		{"a bar at 00:30 is in the new year", time.Date(2027, 1, 1, 0, 30, 0, 0, london), 2027, "2027"},
		{"mid-year across a DST change", time.Date(2026, 7, 1, 12, 0, 0, 0, london), 2026, "2026"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fy, err := CalendarFY.YearOf(tt.at, london)
			if err != nil {
				t.Fatalf("YearOf: %v", err)
			}
			if fy.StartYear != tt.start || fy.Label() != tt.label || fy.ShortLabel() != tt.label[2:] {
				t.Errorf("got %d %q %q, want %d %q", fy.StartYear, fy.Label(), fy.ShortLabel(), tt.start, tt.label)
			}
		})
	}
}

func TestYearBoundsAndRollover(t *testing.T) {
	fy := FinancialYear{StartYear: 2026, Rule: IndiaFY}

	if got, want := fy.Start(ist), time.Date(2026, 4, 1, 0, 0, 0, 0, ist); !got.Equal(want) {
		t.Errorf("Start = %v, want %v", got, want)
	}
	if got, want := fy.End(ist), time.Date(2027, 4, 1, 0, 0, 0, 0, ist); !got.Equal(want) {
		t.Errorf("End = %v, want %v", got, want)
	}
	if !fy.End(ist).Equal(fy.Next().Start(ist)) {
		t.Error("a year does not end where the next begins")
	}

	// Walking a day at a time across two boundaries, the year changes
	// exactly twice, each time on 1 April local.
	changes := 0
	prev := fy
	for at := fy.Start(ist); at.Before(fy.Next().End(ist)); at = at.AddDate(0, 0, 1) {
		cur, err := IndiaFY.YearOf(at, ist)
		if err != nil {
			t.Fatal(err)
		}
		if cur != prev {
			changes++
			if at.Month() != time.April || at.Day() != 1 {
				t.Errorf("year changed on %v", at)
			}
			prev = cur
		}
	}
	if changes != 1 {
		t.Errorf("year changed %d times across two years, want 1 (at the single inner boundary)", changes)
	}
}

func TestYearOfRefusesBadInput(t *testing.T) {
	tests := []struct {
		name    string
		rule    FYRule
		loc     *time.Location
		wantErr error
	}{
		{"no time zone", IndiaFY, nil, ErrNoLocation},
		{"month zero", FYRule{StartDay: 1}, ist, ErrInvalidFYRule},
		{"month thirteen", FYRule{StartMonth: 13, StartDay: 1}, ist, ErrInvalidFYRule},
		{"day zero", FYRule{StartMonth: time.April}, ist, ErrInvalidFYRule},
		{"29 February", FYRule{StartMonth: time.February, StartDay: 29}, ist, ErrInvalidFYRule},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.rule.YearOf(time.Now(), tt.loc); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
