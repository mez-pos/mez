package order

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/kitchen"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// The kitchen's side of an order (phase 08, Task 4, ADR-090): a station's
// tickets on the display, bumped to the pass and recalled.

// RecallWindow is how long a bumped ticket stays on the board to be
// recalled. After it, a bump is final as far as the display is concerned.
const RecallWindow = 30 * time.Minute

// maxBoard caps a station's board: a station with more tickets than this in
// the kitchen at once has a problem a longer list will not solve.
const maxBoard = 100

// maxDayTickets caps the tickets a station's day is added up from.
const maxDayTickets = 2000

// maxComing is how many held courses a board lists: the next five (D10).
const maxComing = 5

// BumpTicket names a ticket on an order: the seq of its ticket.sent.
type BumpTicket struct {
	TicketSeq int64 `json:"ticket_seq"`
}

// Bump puts a ticket on the pass: ready.
func (s *Service) Bump(ctx context.Context, meta Meta, orderID int64, cmd BumpTicket) (Result, error) {
	return s.run(ctx, meta, orderID, "bump", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		station, err := stationOfTicket(tx.State, cmd.TicketSeq)
		if err != nil {
			return nil, err
		}
		return []order.Payload{order.TicketBumped{TicketSeq: cmd.TicketSeq, Station: station}}, nil
	})
}

// Start marks a ticket as picked up by a cook: in progress (ADR-175).
func (s *Service) Start(ctx context.Context, meta Meta, orderID int64, cmd BumpTicket) (Result, error) {
	return s.run(ctx, meta, orderID, "start", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		station, err := stationOfTicket(tx.State, cmd.TicketSeq)
		if err != nil {
			return nil, err
		}
		return []order.Payload{order.TicketStarted{TicketSeq: cmd.TicketSeq, Station: station}}, nil
	})
}

// Recall puts a bumped ticket back in the kitchen.
func (s *Service) Recall(ctx context.Context, meta Meta, orderID int64, cmd BumpTicket) (Result, error) {
	return s.run(ctx, meta, orderID, "recall", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		station, err := stationOfTicket(tx.State, cmd.TicketSeq)
		if err != nil {
			return nil, err
		}
		return []order.Payload{order.TicketRecalled{TicketSeq: cmd.TicketSeq, Station: station}}, nil
	})
}

func stationOfTicket(s order.State, seq int64) (string, error) {
	for _, t := range s.Tickets {
		if t.Seq == seq {
			return t.Station, nil
		}
	}
	return "", fmt.Errorf("%w: %d", order.ErrTicketNotFound, seq)
}

// BoardTicket is one ticket on a station's board, with the order it is
// from, projected, for its lines, notes, table and covers.
type BoardTicket struct {
	OrderID  int64
	PublicID string
	Ticket   order.Ticket
	Order    order.State
}

// Board is a station's display: its tickets in the kitchen, oldest first,
// then those bumped within the recall window, and the venue's table names.
type Board struct {
	Tickets []BoardTicket
	Tables  map[int64]string
	// LateAfter is the venue's threshold for a late ticket, which the
	// display's timers escalate to.
	LateAfter time.Duration
	// Day is the station's trading day so far (ADR-178).
	Day BoardDay
	// Coming is what the station will be sent when later courses are fired:
	// the first few, from the orders open longest (ADR-181). ComingMore is
	// how many more there are behind them.
	Coming     []ComingCourse
	ComingMore int64
}

// ComingCourse is one course of an open order that a station has not been
// sent yet: the mains behind the starters that have gone.
type ComingCourse struct {
	PublicID string
	Order    order.State
	Course   int64
	LineIDs  []int64
}

// BoardDay is a station's figures for the trading day: how many tickets it
// was sent, how many are still in the kitchen and how many went on the pass,
// and how long from sent to ready, on average and at worst.
type BoardDay struct {
	kitchen.DayStats
	// SlowestDish names the slowest ticket by its first dish, with how many
	// more dishes were on it. Nil until a ticket is ready.
	SlowestDish *SlowDish
}

// SlowDish is the day's slowest ticket as a cook would name it.
type SlowDish struct {
	Name string
	More int64 // the other dishes on the same ticket
	Took time.Duration
}

// Board reads a station's tickets for its display.
func (s *Service) Board(ctx context.Context, venueID int64, station string) (Board, error) {
	v, err := loadVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return Board{}, err
	}
	q := s.db.Queries()
	since := s.clock.Now().UTC().Add(-RecallWindow)
	rows, err := q.StationBoard(ctx, gen.StationBoardParams{
		VenueID: venueID, Station: station, ReadySince: sql.NullTime{Time: since, Valid: true}, MaxTickets: maxBoard,
	})
	if err != nil {
		return Board{}, fmt.Errorf("reading the %q board: %w", station, err)
	}
	tables, err := q.TablesForVenue(ctx, venueID)
	if err != nil {
		return Board{}, fmt.Errorf("reading the tables: %w", err)
	}
	out := Board{Tickets: make([]BoardTicket, 0, len(rows)), Tables: make(map[int64]string, len(tables)), LateAfter: v.LateAfter}
	if out.Day.DayStats, err = s.stationDay(ctx, q, v, venueID, station); err != nil {
		return Board{}, err
	}
	for _, t := range tables {
		out.Tables[t.ID] = t.Name
	}
	orders := map[int64]order.State{}
	for _, r := range rows {
		state, ok := orders[r.OrderID]
		if !ok {
			if state, err = s.db.LoadOrder(ctx, r.OrderID, v.Taxes); err != nil {
				return Board{}, err
			}
			orders[r.OrderID] = state
		}
		for _, t := range state.Tickets {
			if t.Seq == r.TicketSeq {
				out.Tickets = append(out.Tickets, BoardTicket{OrderID: r.OrderID, PublicID: r.PublicID, Ticket: t, Order: state})
			}
		}
	}
	if err := s.coming(ctx, q, v, venueID, station, orders, &out); err != nil {
		return Board{}, err
	}
	if slow := out.Day.Slowest; slow != nil {
		state, ok := orders[slow.OrderID]
		if !ok {
			if state, err = s.db.LoadOrder(ctx, slow.OrderID, v.Taxes); err != nil {
				return Board{}, err
			}
		}
		out.Day.SlowestDish = slowDish(state, *slow)
	}
	return out, nil
}

// coming fills the board with what the station's open orders hold back for
// it. Every open order is projected to find out, as the open orders list
// does: a venue has tens of them, not thousands, and the snapshot makes each
// a short read.
func (s *Service) coming(ctx context.Context, q *gen.Queries, v venue, venueID int64, station string, orders map[int64]order.State, out *Board) error {
	rows, err := q.UnclosedOrdersForVenue(ctx, gen.UnclosedOrdersForVenueParams{VenueID: venueID, Limit: maxOpenTickets})
	if err != nil {
		return fmt.Errorf("listing the open orders of venue %d: %w", venueID, err)
	}
	for _, r := range rows {
		state, ok := orders[r.ID]
		if !ok {
			if state, err = s.db.LoadOrder(ctx, r.ID, v.Taxes); err != nil {
				return err
			}
			orders[r.ID] = state
		}
		for _, held := range order.Held(state, station) {
			if len(out.Coming) == maxComing {
				out.ComingMore++
				continue
			}
			out.Coming = append(out.Coming, ComingCourse{PublicID: r.PublicID, Order: state, Course: held.Course, LineIDs: held.LineIDs})
		}
	}
	return nil
}

// stationDay adds up the tickets a station was sent since the trading day
// began, from the ticket index alone.
func (s *Service) stationDay(ctx context.Context, q *gen.Queries, v venue, venueID int64, station string) (kitchen.DayStats, error) {
	dayStart, err := dayStartOf(ctx, q, venueID)
	if err != nil {
		return kitchen.DayStats{}, err
	}
	since, err := shift.DayStartAt(s.clock.Now(), v.Location, dayStart)
	if err != nil {
		return kitchen.DayStats{}, fmt.Errorf("the trading day of venue %d: %w", venueID, err)
	}
	// Timestamps are stored in UTC and compared as text.
	rows, err := q.StationDay(ctx, gen.StationDayParams{VenueID: venueID, Station: station, Since: since.UTC(), MaxTickets: maxDayTickets})
	if err != nil {
		return kitchen.DayStats{}, fmt.Errorf("reading the %q station's day: %w", station, err)
	}
	tickets := make([]kitchen.DayTicket, len(rows))
	for i, r := range rows {
		tickets[i] = kitchen.DayTicket{OrderID: r.OrderID, Seq: r.TicketSeq, SentAt: r.SentAt}
		if r.ReadyAt.Valid {
			tickets[i].ReadyAt = r.ReadyAt.Time
		}
	}
	return kitchen.Stats(tickets), nil
}

// slowDish names a ticket by the first dish on it that was not voided. A
// ticket whose every dish was voided is named by its first.
func slowDish(state order.State, slow kitchen.SlowTicket) *SlowDish {
	out := &SlowDish{Took: slow.Took}
	for _, t := range state.Tickets {
		if t.Seq != slow.Seq {
			continue
		}
		var first string
		for _, l := range state.Lines {
			if !slices.Contains(t.LineIDs, l.ID) {
				continue
			}
			if first == "" {
				first = l.Name
			}
			if l.Status == order.LineStatusVoided {
				continue
			}
			if out.Name == "" {
				out.Name = l.Name
			} else {
				out.More++
			}
		}
		if out.Name == "" {
			out.Name = first
		}
	}
	return out
}
