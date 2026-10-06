package order

import "testing"

// TestProgress: how far a round's lines have come is read off the tickets
// they were sent on. The order below has a waiter's line (2) and a guest's
// round of a coffee (3) for the bar and a dosa (4) for the kitchen.
func TestProgress(t *testing.T) {
	round := []int64{3, 4}
	base := []Payload{
		item("Lassi", 1, 9000),       // 2: the waiter's
		item("Flat white", 1, 18000), // 3
		LineAdded{Name: "Dosa", Qty: 1, UnitPrice: inr(12000), Station: "kitchen"}, // 4
	}
	sent := append(append([]Payload{}, base...),
		TicketSent{Station: "bar", LineIDs: []int64{3}},     // 5
		TicketSent{Station: "kitchen", LineIDs: []int64{4}}, // 6
	)
	with := func(more ...Payload) []Payload { return append(append([]Payload{}, sent...), more...) }

	tests := []struct {
		name   string
		events []Payload
		lines  []int64
		want   Progress
	}{
		{"on the bill and sent nowhere", base, round, ProgressAccepted},
		{"with the kitchen, and nobody has started", sent, round, ProgressSent},
		{"one station has started", with(TicketStarted{TicketSeq: 5, Station: "bar"}), round, ProgressPreparing},
		{"the coffee is up and the dosa is not", with(TicketBumped{TicketSeq: 5, Station: "bar"}), round, ProgressPreparing},
		{
			"both are up",
			with(TicketBumped{TicketSeq: 5, Station: "bar"}, TicketStarted{TicketSeq: 6, Station: "kitchen"}, TicketBumped{TicketSeq: 6, Station: "kitchen"}),
			round, ProgressReady,
		},
		{
			"bumped by mistake and recalled",
			with(TicketBumped{TicketSeq: 5, Station: "bar"}, TicketBumped{TicketSeq: 6, Station: "kitchen"}, TicketRecalled{TicketSeq: 6, Station: "kitchen"}),
			round, ProgressPreparing,
		},
		{
			"the dosa was voided: the coffee is all there is to wait for",
			with(LineVoided{LineID: 4, ReasonCode: "guest_changed_mind", AuthorisedBy: 3}, TicketBumped{TicketSeq: 5, Station: "bar"}),
			round, ProgressReady,
		},
		{
			"everything in it was voided",
			with(LineVoided{LineID: 3, ReasonCode: "x", AuthorisedBy: 3}, LineVoided{LineID: 4, ReasonCode: "x", AuthorisedBy: 3}),
			round, ProgressCancelled,
		},
		{"the waiter's line moving says nothing of the round", append(append([]Payload{}, base...), TicketSent{Station: "bar", LineIDs: []int64{2}}), round, ProgressAccepted},
		{"lines that are not on the order", sent, []int64{40, 41}, ""},
		{"no lines", sent, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := mustProject(t, append([]Payload{opened()}, tt.events...)...)
			if got := s.Progress(tt.lines); got != tt.want {
				t.Errorf("Progress(%v) = %q, want %q", tt.lines, got, tt.want)
			}
		})
	}
}
