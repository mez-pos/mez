package kitchen

import "time"

// DayTicket is one ticket a station was sent in a day, as its index row has
// it: in the kitchen, or on the pass and since when.
type DayTicket struct {
	OrderID int64
	Seq     int64
	SentAt  time.Time
	// ReadyAt is when it went on the pass; zero while it is in the kitchen.
	ReadyAt time.Time
}

// DayStats is a station's day so far (D10, "Today's stats").
type DayStats struct {
	Sent      int64 // every ticket the station was sent
	InKitchen int64 // new or being made
	Ready     int64 // put on the pass
	// Average is the mean time from sent to ready, over the ready tickets,
	// to the whole second. Zero until one is ready.
	Average time.Duration
	// Slowest is the ready ticket that took longest, nil until one is ready.
	Slowest *SlowTicket
}

// SlowTicket names a ticket and how long it took from sent to ready.
type SlowTicket struct {
	OrderID int64
	Seq     int64
	Took    time.Duration
}

// Stats adds up a station's tickets. A ticket's time runs from when it was
// sent, not from when a cook started it: that is the guest's wait, and a
// display that never taps Start still has it. A ticket recalled and put back
// counts once, with its last time on the pass. One marked ready before it
// was sent, by a clock set back, took no time.
func Stats(tickets []DayTicket) DayStats {
	var out DayStats
	var total time.Duration
	for _, t := range tickets {
		out.Sent++
		if t.ReadyAt.IsZero() {
			out.InKitchen++
			continue
		}
		out.Ready++
		took := max(t.ReadyAt.Sub(t.SentAt), 0)
		total += took
		// The first of two as slow as each other stays: the answer does not
		// change with the order the rows came in, which is by time sent.
		if out.Slowest == nil || took > out.Slowest.Took {
			out.Slowest = &SlowTicket{OrderID: t.OrderID, Seq: t.Seq, Took: took}
		}
	}
	if out.Ready > 0 {
		out.Average = (total / time.Duration(out.Ready)).Truncate(time.Second)
	}
	return out
}
