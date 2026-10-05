package order

import (
	"reflect"
	"testing"
)

// routed is a line at a station in a course, priced at 100.00.
func routed(name, station string, course int64) LineAdded {
	return LineAdded{ItemID: 1, Name: name, Qty: 1, UnitPrice: inr(10000), Course: course, Station: station}
}

// dinner is a table's order: drinks in course 1 at the bar, starters in
// course 1 in the kitchen, mains in course 2, one starter voided.
func dinner(t *testing.T, more ...Payload) State {
	t.Helper()
	events := []Payload{
		atTable(4),
		routed("Lime soda", "bar", 1),          // 2
		routed("Samosa", "kitchen", 1),         // 3
		routed("Butter chicken", "kitchen", 2), // 4
		routed("Beer", "bar", 1),               // 5
		routed("Chaat", "kitchen", 1),          // 6
		LineVoided{LineID: 6, ReasonCode: "wrong_item", AuthorisedBy: 1},
		routed("Dal", "kitchen", 2), // 8
	}
	return mustProject(t, append(events, more...)...)
}

func TestTicketsGoOnlyToTheirStations(t *testing.T) {
	s := dinner(t)
	got := Tickets(s, 1)
	want := []TicketSent{
		{Station: "bar", LineIDs: []int64{2, 5}},
		{Station: "kitchen", LineIDs: []int64{3}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("course 1 = %+v, want %+v", got, want)
	}
	if got := Tickets(s, 2); !reflect.DeepEqual(got, []TicketSent{{Station: "kitchen", LineIDs: []int64{4, 8}}}) {
		t.Errorf("course 2 = %+v", got)
	}
	if got := Tickets(s, 3); got != nil {
		t.Errorf("course 3 = %+v", got)
	}
}

func TestFireSendsTheCourseAndNothingElse(t *testing.T) {
	s := dinner(t)
	got := Fire(s, 2)
	want := []Payload{CourseFired{Course: 2}, TicketSent{Station: "kitchen", LineIDs: []int64{4, 8}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fire 2 = %+v", got)
	}
	// What Fire gives, the order takes.
	after := dinner(t, got...)
	for _, l := range after.Lines {
		if (l.Course == 2) != l.Sent {
			t.Errorf("line %d (course %d) sent = %v", l.ID, l.Course, l.Sent)
		}
	}
	// An empty course is still fired, with no tickets.
	if got := Fire(s, 3); !reflect.DeepEqual(got, []Payload{CourseFired{Course: 3}}) {
		t.Errorf("fire 3 = %+v", got)
	}
}

func TestToSend(t *testing.T) {
	t.Run("nothing fired: the first course goes", func(t *testing.T) {
		got := ToSend(dinner(t))
		want := []Payload{
			CourseFired{Course: 1},
			TicketSent{Station: "bar", LineIDs: []int64{2, 5}},
			TicketSent{Station: "kitchen", LineIDs: []int64{3}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("= %+v", got)
		}
	})
	t.Run("course 1 out: only what was added to it since, course 2 waits", func(t *testing.T) {
		s := dinner(t)
		s = dinner(t, append(Fire(s, 1), routed("Another beer", "bar", 1), routed("Kulfi", "kitchen", 3))...)
		got := ToSend(s)
		want := []Payload{TicketSent{Station: "bar", LineIDs: []int64{12}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("= %+v", got)
		}
	})
	t.Run("all sent: nothing", func(t *testing.T) {
		s := dinner(t)
		s = dinner(t, Fire(s, 1)...)
		if got := ToSend(s); got != nil {
			t.Errorf("= %+v", got)
		}
	})
	t.Run("a line in no course goes at once", func(t *testing.T) {
		s := mustProject(t, atTable(4), routed("Water", "bar", 0), routed("Soup", "kitchen", 2))
		got := ToSend(s)
		want := []Payload{
			TicketSent{Station: "bar", LineIDs: []int64{2}},
			CourseFired{Course: 2},
			TicketSent{Station: "kitchen", LineIDs: []int64{3}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("= %+v", got)
		}
	})
	t.Run("a line with no station goes nowhere", func(t *testing.T) {
		if got := ToSend(mustProject(t, atTable(4), routed("Mystery", "", 1))); got != nil {
			t.Errorf("= %+v", got)
		}
	})
}

func TestHeld(t *testing.T) {
	mains := []HeldCourse{{Course: 2, LineIDs: []int64{4, 8}}}
	tests := []struct {
		name    string
		then    []Payload
		station string
		want    []HeldCourse
	}{
		{name: "nothing sent yet: a basket, not a course held back", station: "kitchen"},
		{name: "the starters have gone: the mains are coming", then: Fire(dinner(t), 1), station: "kitchen", want: mains},
		{name: "the bar has nothing behind its first round", then: Fire(dinner(t), 1), station: "bar"},
		{name: "no station is nobody's", then: Fire(dinner(t), 1), station: ""},
		{
			name:    "the mains fired too: nothing is held",
			then:    append(Fire(dinner(t), 1), CourseFired{Course: 2}, TicketSent{Station: "kitchen", LineIDs: []int64{4, 8}}),
			station: "kitchen",
		},
		{
			name:    "a held dish voided is not coming",
			then:    append(Fire(dinner(t), 1), LineVoided{LineID: 4, ReasonCode: "wrong_item", AuthorisedBy: 1}),
			station: "kitchen",
			want:    []HeldCourse{{Course: 2, LineIDs: []int64{8}}},
		},
		{
			name:    "a dessert rung up later is held behind the mains, lowest course first",
			then:    append(Fire(dinner(t), 1), routed("Kulfi", "kitchen", 3), routed("Naan", "kitchen", 2)),
			station: "kitchen",
			want:    []HeldCourse{{Course: 2, LineIDs: []int64{4, 8, 13}}, {Course: 3, LineIDs: []int64{12}}},
		},
		{name: "a parked order is not about to be fired", then: append(Fire(dinner(t), 1), OrderHeld{}), station: "kitchen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Held(dinner(t, tt.then...), tt.station); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Held = %+v, want %+v", got, tt.want)
			}
		})
	}
}
