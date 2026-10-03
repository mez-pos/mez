package shift

import (
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// ahead of the default: 04:30 in Kolkata on the 28th, a half hour after the
// default trading day starts and half an hour before a 05:00 one does.
var halfPastFour = time.Date(2026, 9, 27, 23, 0, 0, 0, time.UTC)

func TestDayStartReadsTheSettingAndDefaultsWhenUnset(t *testing.T) {
	f := newFixture(t)
	read := func() (time.Duration, error) {
		var d time.Duration
		err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
			var err error
			d, err = DayStart(ctx, q, 1)
			return err
		})
		return d, err
	}
	if d, err := read(); err != nil || d != shift.DefaultDayStart {
		t.Fatalf("unset: %s, %v; want the default %s", d, err, shift.DefaultDayStart)
	}
	setting(t, f, SettingDayStart, "05:30")
	if d, err := read(); err != nil || d != 5*time.Hour+30*time.Minute {
		t.Fatalf("05:30: %s, %v", d, err)
	}
	for _, bad := range []string{"5:30", "25:00", "dawn", "04:00:00"} {
		setting(t, f, SettingDayStart, bad)
		if _, err := read(); !errors.Is(err, ErrVenueConfig) {
			t.Errorf("%q: err = %v, want ErrVenueConfig", bad, err)
		}
	}
}

func TestAShiftOpensUnderTheVenuesDayStart(t *testing.T) {
	tests := []struct {
		name     string
		dayStart string // "" leaves it unset
		want     string
	}{
		{"unset: 04:30 is after the default four", "", "2026-09-28"},
		{"05:00: 04:30 is still the night before", "05:00", "2026-09-27"},
		{"04:30 exactly begins the new day", "04:30", "2026-09-28"},
		{"midnight: the calendar day", "00:00", "2026-09-28"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.dayStart != "" {
				setting(t, f, SettingDayStart, tt.dayStart)
			}
			f.clock.set(halfPastFour)
			got, err := f.svc.Open(ctx, cashier, float(t, 0))
			if err != nil {
				t.Fatal(err)
			}
			if got.TradingDay != tt.want {
				t.Errorf("trading day = %s, want %s", got.TradingDay, tt.want)
			}
		})
	}
}

// TestAChangeReachesTheNextShiftNotTheOpenOne: an open shift keeps the trading
// day it was opened under; the one opened after the change gets the new rule.
func TestAChangeReachesTheNextShiftNotTheOpenOne(t *testing.T) {
	f := newFixture(t)
	f.clock.set(halfPastFour)
	first, err := f.svc.Open(ctx, cashier, float(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if first.TradingDay != "2026-09-28" {
		t.Fatalf("first trading day = %s, want 2026-09-28", first.TradingDay)
	}

	setting(t, f, SettingDayStart, "05:00")
	again, err := f.svc.ByID(ctx, 1, first.ID)
	if err != nil || again.TradingDay != "2026-09-28" {
		t.Fatalf("the open shift after the change: %s, %v; want 2026-09-28", again.TradingDay, err)
	}

	countExact(t, f, first.ID)
	if _, err := f.svc.Close(ctx, cashier, first.ID); err != nil {
		t.Fatal(err)
	}
	next, err := f.svc.Open(ctx, cashier, float(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if next.TradingDay != "2026-09-27" {
		t.Errorf("the next shift's trading day = %s, want 2026-09-27", next.TradingDay)
	}
}

func TestADayStartThatIsNotATimeRefusesToOpenAShift(t *testing.T) {
	f := newFixture(t)
	setting(t, f, SettingDayStart, "dawn")
	_, err := f.svc.Open(ctx, cashier, float(t, 0))
	if !errors.Is(err, ErrVenueConfig) {
		t.Fatalf("err = %v, want ErrVenueConfig", err)
	}
	if _, err := f.svc.Current(ctx, 1); !errors.Is(err, ErrNoOpenShift) {
		t.Errorf("a shift exists after the refusal: %v", err)
	}
}
