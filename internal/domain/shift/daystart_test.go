package shift

import (
	"errors"
	"testing"
	"time"
)

func TestParseDayStart(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		invalid bool
	}{
		{"04:00", 4 * time.Hour, false},
		{"00:00", 0, false},
		{"05:30", 5*time.Hour + 30*time.Minute, false},
		{"23:59", 23*time.Hour + 59*time.Minute, false},
		{"24:00", 0, true},
		{"12:60", 0, true},
		{"4:00", 0, true},
		{"04:0", 0, true},
		{"0400", 0, true},
		{"04-00", 0, true},
		{"04:00:00", 0, true},
		{" 04:00", 0, true},
		{"aa:bb", 0, true},
		{"+4:00", 0, true},
		{"-1:00", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseDayStart(tt.in)
			if tt.invalid {
				if !errors.Is(err, ErrInvalidDayStart) {
					t.Fatalf("ParseDayStart(%q) = %s, %v; want ErrInvalidDayStart", tt.in, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseDayStart(%q) = %s, %v; want %s", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestFormatDayStartReadsBackAsParsed(t *testing.T) {
	for _, d := range []time.Duration{0, time.Minute, 4 * time.Hour, DefaultDayStart, 5*time.Hour + 30*time.Minute, 23*time.Hour + 59*time.Minute} {
		got, err := ParseDayStart(FormatDayStart(d))
		if err != nil || got != d {
			t.Errorf("%s: wrote %q, read back %s, %v", d, FormatDayStart(d), got, err)
		}
	}
	if got := FormatDayStart(DefaultDayStart); got != "04:00" {
		t.Errorf("the default day start is written %q, want 04:00", got)
	}
}
