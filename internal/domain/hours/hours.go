// Package hours is a venue's opening hours (ADR-360): an opening and a
// closing time for each weekday it is open, in the venue's own timezone.
// The venue stores them as a setting and carries them in its health summary
// (ADR-361); it computes nothing from them. mez Cloud's clock alerts read
// them: a shift still open past closing, the venue not heard from while
// open. A venue that has set none has no such alerts.
package hours

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Weekdays are the keys of a Week, Monday first, as the setting is written.
var Weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

var names = map[string]string{
	"mon": "Monday", "tue": "Tuesday", "wed": "Wednesday", "thu": "Thursday",
	"fri": "Friday", "sat": "Saturday", "sun": "Sunday",
}

// Key is the Week key of a Go weekday.
func Key(d time.Weekday) string { return Weekdays[(int(d)+6)%7] }

// Day is one weekday's hours, "09:00" to "23:00". A close before the open
// is the next morning's: "17:00" to "02:00" is a bar. A close equal to the
// open is refused: a day open round the clock is not one a venue can set.
type Day struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// Overnight says the day closes after midnight. False for a Day that does
// not pass Check.
func (d Day) Overnight() bool {
	open, err := Clock(d.Open)
	if err != nil {
		return false
	}
	closing, err := Clock(d.Close)
	if err != nil {
		return false
	}
	return closing < open
}

// Week is a venue's hours: a Day for each weekday it is open. A weekday not
// in it is one the venue is closed. Nil or empty is "not set".
type Week map[string]Day

// ErrInvalid is hours that cannot be read as a week. The error carries
// the owner's words; errors.Is finds this.
var ErrInvalid = errors.New("hours: not opening hours")

// Error is what is wrong with a week, said for the owner.
type Error struct{ Message string }

func (e *Error) Error() string { return "hours: " + e.Message }

// Is makes every Error an ErrInvalid.
func (e *Error) Is(target error) bool { return target == ErrInvalid }

// Parse reads the setting's text. Empty is not set, and so is a week with
// no days in it. Anything else must be a week that passes Check.
func Parse(raw string) (Week, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var w Week
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return nil, &Error{"The hours are not written as a week of opening and closing times."}
	}
	if err := w.Check(); err != nil {
		return nil, err
	}
	if len(w) == 0 {
		return nil, nil
	}
	return w, nil
}

// Check says what is wrong with the week, or nil.
func (w Week) Check() error {
	for _, key := range sortedKeys(w) {
		name, ok := names[key]
		if !ok {
			return &Error{fmt.Sprintf("%q is not a weekday; the days are mon to sun.", key)}
		}
		d := w[key]
		open, err := Clock(d.Open)
		if err != nil {
			return &Error{fmt.Sprintf("%s opens at %q, which is not a time of day like 09:00.", name, d.Open)}
		}
		closing, err := Clock(d.Close)
		if err != nil {
			return &Error{fmt.Sprintf("%s closes at %q, which is not a time of day like 23:00.", name, d.Close)}
		}
		if open == closing {
			return &Error{fmt.Sprintf("%s opens and closes at %s. Give it a closing time after its opening, or before it for the next morning.", name, d.Open)}
		}
	}
	return nil
}

// Format is the week as the setting stores it: one JSON object, the days in
// their order, so the same hours are always the same text. An empty week
// formats as "", the text of a setting that is not set.
func (w Week) Format() (string, error) {
	if err := w.Check(); err != nil {
		return "", err
	}
	if len(w) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for _, key := range Weekdays {
		d, ok := w[key]
		if !ok {
			continue
		}
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		// Both times passed Clock, so they are digits and a colon.
		fmt.Fprintf(&b, `%q:{"open":%q,"close":%q}`, key, d.Open, d.Close)
	}
	b.WriteByte('}')
	return b.String(), nil
}

// sortedKeys is the week's days in weekday order, then any key that is not
// one, so that Check names the same problem first every time.
func sortedKeys(w Week) []string {
	keys := make([]string, 0, len(w))
	for _, key := range Weekdays {
		if _, ok := w[key]; ok {
			keys = append(keys, key)
		}
	}
	var others []string
	for key := range w {
		if _, ok := names[key]; !ok {
			others = append(others, key)
		}
	}
	if len(others) > 1 {
		// Deterministic without importing sort for seven keys.
		for i := 1; i < len(others); i++ {
			for j := i; j > 0 && others[j] < others[j-1]; j-- {
				others[j], others[j-1] = others[j-1], others[j]
			}
		}
	}
	return append(keys, others...)
}

// Clock reads "HH:MM", 24-hour, as the time since midnight.
func Clock(s string) (time.Duration, error) {
	if len(s) != 5 {
		return 0, ErrInvalid
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, ErrInvalid
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}
