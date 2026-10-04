package shift

import (
	"errors"
	"testing"
	"time"
)

func TestNextDayStart(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	at := func(loc *time.Location, y int, mo time.Month, d, h, mi int) time.Time {
		return time.Date(y, mo, d, h, mi, 0, 0, loc)
	}
	tests := []struct {
		name     string
		now      time.Time
		loc      *time.Location
		dayStart time.Duration
		want     time.Time
	}{
		{"an evening: tomorrow at four", at(kolkata, 2026, 9, 30, 23, 0), kolkata, DefaultDayStart, at(kolkata, 2026, 10, 1, 4, 0)},
		{"just after midnight: still last night, so four this morning", at(kolkata, 2026, 10, 1, 1, 30), kolkata, DefaultDayStart, at(kolkata, 2026, 10, 1, 4, 0)},
		{"exactly at the start: a new day, so tomorrow", at(kolkata, 2026, 10, 1, 4, 0), kolkata, DefaultDayStart, at(kolkata, 2026, 10, 2, 4, 0)},
		{"the end of a month", at(kolkata, 2026, 12, 31, 12, 0), kolkata, DefaultDayStart, at(kolkata, 2027, 1, 1, 4, 0)},
		{"a day that starts at midnight", at(kolkata, 2026, 9, 30, 23, 59), kolkata, 0, at(kolkata, 2026, 10, 1, 0, 0)},
		{"a day that starts at 05:30", at(kolkata, 2026, 9, 30, 5, 0), kolkata, 5*time.Hour + 30*time.Minute, at(kolkata, 2026, 9, 30, 5, 30)},
		// 29 March 2026: London skips 01:00 to 02:00. A 01:30 start does not
		// exist that night, and time.Date moves it to 02:30 BST.
		{"a start the clocks skip", at(london, 2026, 3, 28, 20, 0), london, 90 * time.Minute, time.Date(2026, 3, 29, 1, 30, 0, 0, london)},
		{"across the autumn change", at(london, 2026, 10, 24, 20, 0), london, DefaultDayStart, at(london, 2026, 10, 25, 4, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NextDayStart(tt.now, tt.loc, tt.dayStart)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("NextDayStart = %s, want %s", got, tt.want)
			}
			if !got.After(tt.now) {
				t.Errorf("%s is not after %s", got, tt.now)
			}
		})
	}

	if _, err := NextDayStart(time.Now(), nil, DefaultDayStart); err == nil {
		t.Error("no timezone must fail")
	}
	if _, err := NextDayStart(time.Now(), kolkata, 25*time.Hour); !errors.Is(err, ErrInvalidDayStart) {
		t.Errorf("a day start past the day: %v", err)
	}
}

func TestDayStartAt(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	at := func(loc *time.Location, y int, mo time.Month, d, h, mi int) time.Time {
		return time.Date(y, mo, d, h, mi, 0, 0, loc)
	}
	tests := []struct {
		name     string
		now      time.Time
		loc      *time.Location
		dayStart time.Duration
		want     time.Time
	}{
		{"an evening: four this morning", at(kolkata, 2026, 9, 30, 23, 0), kolkata, DefaultDayStart, at(kolkata, 2026, 9, 30, 4, 0)},
		{"just after midnight: still last night", at(kolkata, 2026, 10, 1, 1, 30), kolkata, DefaultDayStart, at(kolkata, 2026, 9, 30, 4, 0)},
		{"exactly at the start: the day that has just begun", at(kolkata, 2026, 10, 1, 4, 0), kolkata, DefaultDayStart, at(kolkata, 2026, 10, 1, 4, 0)},
		{"the first of a month, before the start", at(kolkata, 2027, 1, 1, 2, 0), kolkata, DefaultDayStart, at(kolkata, 2026, 12, 31, 4, 0)},
		{"a day that starts at midnight", at(kolkata, 2026, 9, 30, 23, 59), kolkata, 0, at(kolkata, 2026, 9, 30, 0, 0)},
		// 25 October 2026: London's clocks go back, and the day is 25 hours long.
		{"across the autumn change", at(london, 2026, 10, 25, 20, 0), london, DefaultDayStart, at(london, 2026, 10, 25, 4, 0)},
		{"the night of the change itself", at(london, 2026, 10, 25, 3, 0), london, DefaultDayStart, at(london, 2026, 10, 24, 4, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DayStartAt(tt.now, tt.loc, tt.dayStart)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("DayStartAt = %s, want %s", got, tt.want)
			}
			if got.After(tt.now) {
				t.Errorf("%s is after %s", got, tt.now)
			}
		})
	}
	if _, err := DayStartAt(time.Now(), nil, DefaultDayStart); err == nil {
		t.Error("no timezone was accepted")
	}
}
