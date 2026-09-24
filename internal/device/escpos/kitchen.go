package escpos

import (
	"fmt"
	"time"
)

// KitchenTicket is what a station is asked to make: no prices, big enough
// to read from the pass at arm's length.
type KitchenTicket struct {
	Station  string    // "Kitchen", "Bar"
	OrderRef string    // the order's public id
	Table    int64     // 0 for a counter sale
	Covers   int64     // 0 when not counted
	Course   int64     // 0 when the lines are not coursed
	At       time.Time // when it was sent, in the venue's zone
	By       string    // who sent it, if known
	Lines    []KitchenLine
	Reprint  bool   // a copy of a ticket already sent: marked, so it is not made twice
	Note     string // the order's note, if any
}

// KitchenLine is one thing to make.
type KitchenLine struct {
	Qty       int64
	Name      string
	Modifiers []string
	Note      string
}

// Layout lays the ticket out for p and ends it with a full cut: a kitchen
// ticket goes on a rail, not into a guest's hand.
func (k KitchenTicket) Layout(p Paper) ([]byte, error) {
	w := p.Columns
	b := New()
	if k.Reprint {
		b.Align(Center).Bold(true).Line("*** REPRINT - ALREADY SENT ***").Bold(false)
	}

	// Where it goes and for whom, double size: 2x halves the columns.
	b.Align(Left).Bold(true).Size(2, 2)
	where := "COUNTER"
	if k.Table > 0 {
		where = fmt.Sprintf("TABLE %d", k.Table)
	}
	b.Line(where)
	b.Size(1, 1)
	b.Line(Printable(k.Station))
	b.Bold(false)

	meta := k.OrderRef
	if k.Covers > 0 {
		meta = fmt.Sprintf("%s  %d covers", meta, k.Covers)
	}
	b.Line(meta)
	if !k.At.IsZero() {
		sent := k.At.Format("15:04")
		if k.By != "" {
			sent += "  " + k.By
		}
		b.Line(sent)
	}
	if k.Course > 0 {
		b.Bold(true).Line(fmt.Sprintf("COURSE %d", k.Course)).Bold(false)
	}
	b.Line(rule('-', w))

	// Each line double height: the quantity is what gets misread across a
	// busy pass. Double width would halve the room for a long dish name.
	for _, l := range k.Lines {
		b.Bold(true).Size(1, 2)
		for _, s := range wrap(fmt.Sprintf("%d x %s", l.Qty, l.Name), w) {
			b.Line(s)
		}
		b.Size(1, 1).Bold(false)
		for _, m := range l.Modifiers {
			indented(b, "+ "+m, w)
		}
		if l.Note != "" {
			b.Bold(true)
			indented(b, "NOTE: "+l.Note, w)
			b.Bold(false)
		}
	}
	if k.Note != "" {
		b.Line(rule('-', w)).Bold(true)
		for _, l := range wrap("ORDER NOTE: "+k.Note, w) {
			b.Line(l)
		}
		b.Bold(false)
	}
	b.Line(rule('-', w)).Feed(2)
	return b.Cut(false).Bytes()
}
