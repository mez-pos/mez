package wire

import (
	"errors"
	"testing"
	"time"
)

func TestHealthRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)
	synced := at.Add(-30 * time.Second)
	failed := at.Add(-time.Hour)
	tests := []struct {
		name string
		in   Health
	}{
		{"nothing set", Health{At: at, Timezone: "Asia/Kolkata", Backup: BackupHealth{State: "off"}}},
		{"everything", Health{
			At: at, Timezone: "Asia/Kolkata",
			Printers: []PrinterHealth{
				{Name: "Counter", Active: true, LastSuccess: &synced},
				{Name: "Kitchen", Active: true, LastSuccess: &synced, LastFailure: &failed, Error: "connection refused"},
				{Name: "Old", Active: false},
			},
			Backup: BackupHealth{State: "ok", LastSyncedAt: &synced},
			Shift:  &ShiftHealth{ID: "sh_1", TradingDay: "2026-10-22", OpenedAt: at.Add(-9 * time.Hour)},
			Hours: map[string]DayHours{
				"mon": {Open: "09:00", Close: "23:00"},
				"sat": {Open: "17:00", Close: "02:00"},
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := EncodeHealth(tt.in)
			if err != nil {
				t.Fatalf("EncodeHealth: %v", err)
			}
			f, err := Decode(msg)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if f.Type != TypeHealth || f.Stream != 0 {
				t.Fatalf("health went as type %d on stream %d", f.Type, f.Stream)
			}
			got, err := DecodeHealth(f)
			if err != nil {
				t.Fatalf("DecodeHealth: %v", err)
			}
			if !got.At.Equal(tt.in.At) || got.Timezone != tt.in.Timezone || got.Backup.State != tt.in.Backup.State {
				t.Errorf("round trip changed the summary: %+v", got)
			}
			if len(got.Printers) != len(tt.in.Printers) || len(got.Hours) != len(tt.in.Hours) {
				t.Errorf("round trip lost printers or hours: %+v", got)
			}
			if (got.Shift == nil) != (tt.in.Shift == nil) {
				t.Errorf("round trip changed the shift: %+v", got.Shift)
			}
			if tt.in.Shift != nil && got.Shift.ID != tt.in.Shift.ID {
				t.Errorf("shift id = %q, want %q", got.Shift.ID, tt.in.Shift.ID)
			}
			if len(tt.in.Printers) > 1 {
				p := got.Printers[1]
				if p.LastFailure == nil || !p.LastFailure.Equal(failed) || p.Error != "connection refused" {
					t.Errorf("printer failure lost: %+v", p)
				}
			}
			if got.Hours["sat"] != (DayHours{Open: "17:00", Close: "02:00"}) && len(tt.in.Hours) > 0 {
				t.Errorf("saturday's hours = %+v", got.Hours["sat"])
			}
		})
	}
}

func TestDecodeHealthRefuses(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{"not a health frame", Frame{Type: TypeNotice, Payload: []byte(`{}`)}},
		{"not json", Frame{Type: TypeHealth, Payload: []byte(`{"at":`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeHealth(tt.frame); !errors.Is(err, ErrMalformed) {
				t.Errorf("DecodeHealth = %v, want ErrMalformed", err)
			}
		})
	}
}
