package order

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// The kitchen's side of an order (phase 08, Task 4, ADR-090): a station's
// tickets on the display, bumped to the pass and recalled.

// RecallWindow is how long a bumped ticket stays on the board to be
// recalled. After it, a bump is final as far as the display is concerned.
const RecallWindow = 30 * time.Minute

// maxBoard caps a station's board: a station with more tickets than this in
// the kitchen at once has a problem a longer list will not solve.
const maxBoard = 100

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
	return out, nil
}
