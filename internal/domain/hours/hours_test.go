package hours

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAndFormat(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Week
		// stored is what Format gives back: the canonical text.
		stored string
	}{
		{"empty is not set", "", nil, ""},
		{"blank is not set", "  \n", nil, ""},
		{"an empty week is not set", "{}", nil, ""},
		{"one day", `{"mon":{"open":"09:00","close":"23:00"}}`, Week{"mon": {"09:00", "23:00"}}, `{"mon":{"open":"09:00","close":"23:00"}}`},
		{"days come out in order", `{"sun":{"open":"10:00","close":"22:00"},"mon":{"open":"09:00","close":"23:00"}}`,
			Week{"mon": {"09:00", "23:00"}, "sun": {"10:00", "22:00"}},
			`{"mon":{"open":"09:00","close":"23:00"},"sun":{"open":"10:00","close":"22:00"}}`},
		{"a bar closes after midnight", `{"sat":{"open":"17:00","close":"02:00"}}`, Week{"sat": {"17:00", "02:00"}}, `{"sat":{"open":"17:00","close":"02:00"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Parse = %v, want %v", got, tt.want)
			}
			for k, d := range tt.want {
				if got[k] != d {
					t.Errorf("Parse[%s] = %v, want %v", k, got[k], d)
				}
			}
			stored, err := got.Format()
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			if stored != tt.stored {
				t.Errorf("Format = %s, want %s", stored, tt.stored)
			}
		})
	}
}

func TestParseRefuses(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		says string
	}{
		{"not json", `mon 9-11`, "not written as a week"},
		{"a list", `[]`, "not written as a week"},
		{"a field nobody reads", `{"mon":{"opens":"09:00","close":"23:00"}}`, "not written as a week"},
		{"not a weekday", `{"monday":{"open":"09:00","close":"23:00"}}`, `"monday" is not a weekday`},
		{"an opening that is not a time", `{"tue":{"open":"9am","close":"23:00"}}`, `Tuesday opens at "9am"`},
		{"a closing past the clock", `{"wed":{"open":"09:00","close":"24:00"}}`, `Wednesday closes at "24:00"`},
		{"a time with seconds", `{"thu":{"open":"09:00:00","close":"23:00"}}`, `Thursday opens at`},
		{"open round the clock", `{"fri":{"open":"09:00","close":"09:00"}}`, "Friday opens and closes at 09:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.raw)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Parse = %v, want ErrInvalid", err)
			}
			var e *Error
			if !errors.As(err, &e) || !strings.Contains(e.Message, tt.says) {
				t.Errorf("Parse said %q, want it to mention %q", err, tt.says)
			}
		})
	}
}

func TestCheckNamesTheEarliestDayFirst(t *testing.T) {
	w := Week{"sun": {"x", "23:00"}, "mon": {"09:00", "y"}}
	err := w.Check()
	if err == nil || !strings.HasPrefix(err.(*Error).Message, "Monday") {
		t.Errorf("Check = %v, want Monday's problem first", err)
	}
}

func TestOvernight(t *testing.T) {
	tests := []struct {
		day  Day
		want bool
	}{
		{Day{"09:00", "23:00"}, false},
		{Day{"17:00", "02:00"}, true},
		{Day{"00:00", "23:59"}, false},
		{Day{"bad", "02:00"}, false},
	}
	for _, tt := range tests {
		if got := tt.day.Overnight(); got != tt.want {
			t.Errorf("%v.Overnight() = %v, want %v", tt.day, got, tt.want)
		}
	}
}

func TestClock(t *testing.T) {
	if d, err := Clock("23:45"); err != nil || d != 23*time.Hour+45*time.Minute {
		t.Errorf("Clock(23:45) = %v, %v", d, err)
	}
	for _, bad := range []string{"", "9:00", "24:00", "12:60", "12:00:00", "noon"} {
		if _, err := Clock(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("Clock(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestKey(t *testing.T) {
	tests := []struct {
		day  time.Weekday
		want string
	}{
		{time.Monday, "mon"}, {time.Saturday, "sat"}, {time.Sunday, "sun"},
	}
	for _, tt := range tests {
		if got := Key(tt.day); got != tt.want {
			t.Errorf("Key(%v) = %q, want %q", tt.day, got, tt.want)
		}
	}
}
