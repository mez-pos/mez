package kitchen

import (
	"reflect"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	noon := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	sent := func(order, seq int64, after time.Duration) DayTicket {
		return DayTicket{OrderID: order, Seq: seq, SentAt: noon.Add(after)}
	}
	ready := func(order, seq int64, after, took time.Duration) DayTicket {
		tk := sent(order, seq, after)
		tk.ReadyAt = tk.SentAt.Add(took)
		return tk
	}
	tests := []struct {
		name    string
		tickets []DayTicket
		want    DayStats
	}{
		{name: "a day with nothing sent", tickets: nil, want: DayStats{}},
		{
			name:    "all still in the kitchen: no time to give yet",
			tickets: []DayTicket{sent(1, 3, 0), sent(2, 3, time.Minute)},
			want:    DayStats{Sent: 2, InKitchen: 2},
		},
		{
			name:    "the mean of the ready ones, and the slowest of them",
			tickets: []DayTicket{ready(1, 3, 0, 8*time.Minute), ready(2, 3, time.Minute, 19*time.Minute), sent(3, 3, 2*time.Minute), ready(4, 5, 3*time.Minute, 6*time.Minute)},
			want:    DayStats{Sent: 4, InKitchen: 1, Ready: 3, Average: 11 * time.Minute, Slowest: &SlowTicket{OrderID: 2, Seq: 3, Took: 19 * time.Minute}},
		},
		{
			name:    "the mean is to the whole second, never rounded up",
			tickets: []DayTicket{ready(1, 3, 0, 10*time.Second), ready(2, 3, 0, 11*time.Second), ready(3, 3, 0, 11*time.Second)},
			want:    DayStats{Sent: 3, Ready: 3, Average: 10 * time.Second, Slowest: &SlowTicket{OrderID: 2, Seq: 3, Took: 11 * time.Second}},
		},
		{
			name:    "of two as slow as each other, the first sent",
			tickets: []DayTicket{ready(1, 3, 0, 5*time.Minute), ready(2, 3, time.Minute, 5*time.Minute)},
			want:    DayStats{Sent: 2, Ready: 2, Average: 5 * time.Minute, Slowest: &SlowTicket{OrderID: 1, Seq: 3, Took: 5 * time.Minute}},
		},
		{
			name:    "ready before it was sent, by a clock set back, took no time",
			tickets: []DayTicket{ready(1, 3, 0, -2*time.Minute)},
			want:    DayStats{Sent: 1, Ready: 1, Slowest: &SlowTicket{OrderID: 1, Seq: 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Stats(tt.tickets); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stats\n got %+v (slowest %+v)\nwant %+v (slowest %+v)", got, got.Slowest, tt.want, tt.want.Slowest)
			}
		})
	}
}
