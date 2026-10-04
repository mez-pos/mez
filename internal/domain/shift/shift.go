// Package shift is the reporting unit of a trading day, from
// docs/02-domain.md: a shift opens with a counted float, moves cash in and
// out with a reason and a manager's say-so, and closes.
//
//	open -> closed
//
// shift.go is the lifecycle and the trading day; count.go is a drawer count;
// cash.go is a cash movement and the drawer's running balance. Storage is
// internal/service/shift.
package shift

import (
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/domain/fsm"
)

// Status is where a shift is in its life.
type Status string

// The shift statuses, in order.
const (
	StatusOpen   Status = "open"   // selling; the drawer is in use
	StatusClosed Status = "closed" // done; nothing more is attached to it
)

// Statuses is every status, for the generated vocabulary.
func Statuses() []Status { return []Status{StatusOpen, StatusClosed} }

// machine is the shift's lifecycle. Closing is final: a mistake found after
// close is corrected by a later document (the Z-report's correction, Task 3),
// never by reopening the shift the report was drawn from.
var machine = fsm.Must("shift",
	[]Status{StatusOpen, StatusClosed},
	map[Status][]Status{
		StatusOpen: {StatusClosed},
	})

// Machine is the lifecycle, for the exhaustive test.
func Machine() fsm.Machine[Status] { return machine }

// Transition checks a shift status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func Transition(from, to Status) (Status, error) {
	return machine.Transition(from, to)
}

// DefaultDayStart is when a new trading day begins, in the venue's local
// time. A shift opened before it belongs to the day before: the bar's
// late shift that someone opens at 00:30 is still last night's trade.
const DefaultDayStart = 4 * time.Hour

// dayFormat is how a trading day is written: a civil date, no zone.
const dayFormat = "2006-01-02"

// ErrInvalidDayStart refuses a day start outside the day.
var ErrInvalidDayStart = errors.New("the trading day must start within the day")

// ParseDayStart reads a day start as a venue writes it: a 24 hour time of
// day, "04:00", with two digits each side. A refusal unwraps to
// ErrInvalidDayStart.
func ParseDayStart(s string) (time.Duration, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("%w: %q is not a time like 04:00", ErrInvalidDayStart, s)
	}
	digit := func(i int) (int, bool) { return int(s[i] - '0'), s[i] >= '0' && s[i] <= '9' }
	var n [4]int
	for i, at := range []int{0, 1, 3, 4} {
		d, ok := digit(at)
		if !ok {
			return 0, fmt.Errorf("%w: %q is not a time like 04:00", ErrInvalidDayStart, s)
		}
		n[i] = d
	}
	h, m := n[0]*10+n[1], n[2]*10+n[3]
	if h > 23 || m > 59 {
		return 0, fmt.Errorf("%w: %q is not a time of day", ErrInvalidDayStart, s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

// FormatDayStart writes a day start as ParseDayStart reads it. Seconds are
// dropped: a venue's day starts on a minute.
func FormatDayStart(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d/time.Hour), int(d%time.Hour/time.Minute))
}

// TradingDay is the trading day a shift opened at openedAt belongs to: the
// venue's local date then, or the date before when it opened ahead of
// dayStart. It is decided once, at opening, and stored, so a bar that
// closes at 2am reports the whole night under the day it opened.
func TradingDay(openedAt time.Time, loc *time.Location, dayStart time.Duration) (string, error) {
	if loc == nil {
		return "", errors.New("shift: a trading day needs the venue's timezone")
	}
	if dayStart < 0 || dayStart >= 24*time.Hour {
		return "", fmt.Errorf("%w: %s", ErrInvalidDayStart, dayStart)
	}
	local := openedAt.In(loc)
	// The wall clock, not time since midnight: on a daylight-saving night
	// the two differ by an hour, and staff read the clock on the wall.
	wall := time.Duration(local.Hour())*time.Hour + time.Duration(local.Minute())*time.Minute +
		time.Duration(local.Second())*time.Second + time.Duration(local.Nanosecond())
	// The date alone, at noon UTC, so stepping back a day is calendar
	// arithmetic that no zone rule can bend.
	day := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, time.UTC)
	if wall < dayStart {
		day = day.AddDate(0, 0, -1)
	}
	return day.Format(dayFormat), nil
}

// DayStartAt is the moment the trading day at now began: dayStart on the
// venue's wall clock, on now's trading day. "Today's orders" are those since
// then, so a bar still serving at 01:30 lists the night it is in.
func DayStartAt(now time.Time, loc *time.Location, dayStart time.Duration) (time.Time, error) {
	next, err := NextDayStart(now, loc, dayStart)
	if err != nil {
		return time.Time{}, err
	}
	// The same wall clock a calendar day earlier, not 24 hours: a
	// daylight-saving night is 23 or 25 hours long.
	return time.Date(next.Year(), next.Month(), next.Day()-1, next.Hour(), next.Minute(), next.Second(), 0, loc), nil
}

// NextDayStart is the moment the trading day after the one at now begins:
// dayStart on the venue's wall clock, on the day after now's trading day.
// Something good "for today" is good until then: sold out at 23:00, or at
// 01:30 after midnight, is back on sale at the next dayStart, not at
// midnight. On a daylight-saving night whose clocks skip dayStart, it is
// the first moment after the gap, as time.Date normalises it.
func NextDayStart(now time.Time, loc *time.Location, dayStart time.Duration) (time.Time, error) {
	day, err := TradingDay(now, loc, dayStart)
	if err != nil {
		return time.Time{}, err
	}
	d, err := time.Parse(dayFormat, day)
	if err != nil {
		return time.Time{}, fmt.Errorf("shift: reading trading day %q: %w", day, err)
	}
	h := int(dayStart / time.Hour)
	m := int(dayStart % time.Hour / time.Minute)
	s := int(dayStart % time.Minute / time.Second)
	return time.Date(d.Year(), d.Month(), d.Day()+1, h, m, s, 0, loc), nil
}
