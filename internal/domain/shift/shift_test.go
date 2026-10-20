package shift

import (
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/fsm"
	"github.com/mez-pos/mez/internal/domain/fsm/fsmtest"
)

// Restated from docs/02-domain.md, not copied from shift.go.
func TestShiftTransitionsAreExactlyTheDomainTable(t *testing.T) {
	fsmtest.Exhaustive(t, Machine(),
		[]Status{StatusOpen, StatusClosed},
		map[Status][]Status{
			StatusOpen: {StatusClosed},
		})
}

func TestAClosedShiftStaysClosed(t *testing.T) {
	got, err := Transition(StatusClosed, StatusOpen)
	if !errors.Is(err, fsm.ErrInvalidTransition) || got != StatusClosed {
		t.Errorf("Transition(closed, open) = %s, %v; want closed, ErrInvalidTransition", got, err)
	}
}

func TestTradingDay(t *testing.T) {
	kolkata := mustZone(t, "Asia/Kolkata")
	newYork := mustZone(t, "America/New_York")

	tests := []struct {
		name     string
		openedAt time.Time
		loc      *time.Location
		dayStart time.Duration
		want     string
	}{
		{
			name:     "a morning shift is its own day",
			openedAt: time.Date(2026, 9, 28, 3, 30, 0, 0, time.UTC), // 09:00 IST
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-09-28",
		},
		{
			name:     "the local date, not the UTC one",
			openedAt: time.Date(2026, 9, 27, 20, 0, 0, 0, time.UTC), // 01:30 IST on the 28th
			loc:      kolkata, dayStart: 0, want: "2026-09-28",
		},
		{
			name:     "opened after midnight, before the day starts, is last night",
			openedAt: time.Date(2026, 9, 27, 19, 0, 0, 0, time.UTC), // 00:30 IST on the 28th
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-09-27",
		},
		{
			name:     "exactly at the day start is the new day",
			openedAt: time.Date(2026, 9, 27, 22, 30, 0, 0, time.UTC), // 04:00 IST on the 28th
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-09-28",
		},
		{
			name:     "a nanosecond before the day start is the old day",
			openedAt: time.Date(2026, 9, 27, 22, 29, 59, 999_999_999, time.UTC),
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-09-27",
		},
		{
			name:     "stepping back over a month end",
			openedAt: time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC), // 01:30 IST on 1 October
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-09-30",
		},
		{
			name:     "stepping back over a year end",
			openedAt: time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC), // 01:30 IST on 1 January
			loc:      kolkata, dayStart: DefaultDayStart, want: "2026-12-31",
		},
		{
			// Clocks go back at 02:00 on 1 November 2026. 03:30 by the wall
			// is four and a half hours after midnight by elapsed time, but
			// the wall says it is before the 04:00 start.
			name:     "the wall clock decides on a daylight-saving night",
			openedAt: time.Date(2026, 11, 1, 8, 30, 0, 0, time.UTC), // 03:30 EST
			loc:      newYork, dayStart: DefaultDayStart, want: "2026-10-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TradingDay(tt.openedAt, tt.loc, tt.dayStart)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("TradingDay = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTradingDayRefusals(t *testing.T) {
	at := time.Date(2026, 9, 28, 3, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		loc      *time.Location
		dayStart time.Duration
		want     error
	}{
		{"no timezone", nil, DefaultDayStart, nil},
		{"negative start", time.UTC, -time.Minute, ErrInvalidDayStart},
		{"a start that is tomorrow", time.UTC, 24 * time.Hour, ErrInvalidDayStart},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TradingDay(at, tt.loc, tt.dayStart)
			if err == nil {
				t.Fatal("want an error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("no zone data for %s: %v", name, err)
	}
	return loc
}
