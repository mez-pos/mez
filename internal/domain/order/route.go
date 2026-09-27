package order

import "slices"

// Station routing (phase 08, Task 3, ADR-089). Every line carries its
// station and its course from the item it was rung up from. A course's
// lines go to the kitchen only when the course is fired, and then only to
// their own stations: the bar gets its drinks and nothing else, the
// kitchen its food. Each station gets one ticket per course fired.

// Tickets is the tickets that send a course's lines not yet sent: one per
// station, stations in the order their first line was rung up, lines in
// order. A voided line is never sent; a comped one is, since it is still
// made. A line with no station has nowhere to go and is left out.
func Tickets(s State, course int64) []TicketSent {
	var out []TicketSent
	at := map[string]int{}
	for _, l := range s.Lines {
		if l.Course != course || l.Sent || l.Status == LineStatusVoided || l.Station == "" {
			continue
		}
		i, ok := at[l.Station]
		if !ok {
			i = len(out)
			at[l.Station] = i
			out = append(out, TicketSent{Station: l.Station})
		}
		out[i].LineIDs = append(out[i].LineIDs, l.ID)
	}
	return out
}

// Fire is what firing a course appends: course.fired, then a ticket for
// each station with lines in it. A course fired with nothing in it yet is
// still fired, so lines added to it later go on Send.
func Fire(s State, course int64) []Payload {
	out := []Payload{CourseFired{Course: course}}
	for _, t := range Tickets(s, course) {
		out = append(out, t)
	}
	return out
}

// ToSend is what Send appends for the order as it stands:
//
//   - every line not yet sent in a course already fired: a round of drinks
//     added after the starters went;
//   - lines in course 0, which is no course at all, whenever they are rung
//     up;
//   - and, while no course has been fired yet, the first course with lines
//     waiting, fired now, so a cafe that never courses sends everything
//     with one tap. Later courses wait for their own fire.
//
// Nothing to send is an empty list; the caller refuses it.
func ToSend(s State) []Payload {
	var out []Payload
	add := func(course int64) {
		for _, t := range Tickets(s, course) {
			out = append(out, t)
		}
	}
	add(0)
	fired := slices.Clone(s.FiredCourses)
	slices.Sort(fired)
	for _, c := range fired {
		add(c)
	}
	if len(s.FiredCourses) == 0 {
		if first, ok := firstWaiting(s); ok {
			out = append(out, Fire(s, first)...)
		}
	}
	return out
}

// firstWaiting is the lowest course with a line still to send.
func firstWaiting(s State) (int64, bool) {
	var first int64
	found := false
	for _, l := range s.Lines {
		if l.Course < 1 || l.Sent || l.Status == LineStatusVoided || l.Station == "" {
			continue
		}
		if !found || l.Course < first {
			first, found = l.Course, true
		}
	}
	return first, found
}
