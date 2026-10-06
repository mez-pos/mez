package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// A guest's request, answered (phase 11, Task 4b1, ADR-211). A round placed
// at a table with no ticket waits until a member of staff accepts or declines
// it. Each request is answered on its own: accepting one says someone is at
// the table, and it does not say every phone that asked is theirs.
//
// A request lasts as long as the shift it was made in. Nothing ends it at
// the close; one from a shift before is simply no longer listed, counted or
// acceptable.

var (
	// ErrRoundNotFound means no request with that id at the caller's venue.
	ErrRoundNotFound = errors.New("no such request")

	// ErrRoundDecided means the request was already answered the other way:
	// accepted when this would decline it, or declined when this would
	// accept it.
	ErrRoundDecided = errors.New("the request was already answered")

	// ErrRoundLapsed means the request was made in a shift that has closed.
	ErrRoundLapsed = errors.New("the request is from a shift that has closed")
)

// errRoundRaced unwinds an accept that found the request answered by
// someone else between the read and the write.
var errRoundRaced = errors.New("the request was answered by someone else first")

// Request is a round waiting for an answer, as staff see it.
type Request struct {
	Round
	TableName string
	PlacedAt  time.Time
}

// Decision is a request once answered. OrderPublicID is the ticket its
// lines went onto, empty for one declined. Replayed is true when it had
// been answered this way already and nothing was written.
type Decision struct {
	Round         Round
	OrderPublicID string
	Replayed      bool
}

// WaitingRounds is the requests the venue has still to answer, oldest
// first. With no shift open there are none.
func (s *Service) WaitingRounds(ctx context.Context, venueID int64) ([]Request, error) {
	q := s.db.Queries()
	since, err := takingRequestsSince(ctx, q, venueID)
	if errors.Is(err, shift.ErrNoOpenShift) {
		return []Request{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.WaitingGuestRounds(ctx, gen.WaitingGuestRoundsParams{VenueID: venueID, Since: since})
	if err != nil {
		return nil, fmt.Errorf("listing the requests of venue %d: %w", venueID, err)
	}
	out := make([]Request, len(rows))
	for i, r := range rows {
		lines, err := decodeLines(r.PublicID, r.Lines)
		if err != nil {
			return nil, err
		}
		out[i] = Request{
			Round: Round{
				PublicID: r.PublicID, Status: RoundWaiting, TableID: r.TableID, Lines: lines,
				Subtotal: money.Amount{Minor: r.SubtotalMinor, Currency: r.Currency},
			},
			TableName: r.TableName, PlacedAt: r.CreatedAt.UTC(),
		}
	}
	return out, nil
}

// AcceptRound puts a waiting request's lines on the table's ticket, opening
// one if there is none, and sends them to the kitchen. The lines are the
// ones priced when the guest asked, not rung up again.
//
// The ticket is opened and the lines appended in two commands. If the second
// never happens the table is left seated with an empty ticket and the request
// still waiting, and accepting it again finishes the job on that ticket.
//
// The events are the accepting staff member's: they are who let the order
// into the kitchen, and the timeline should name a person.
func (s *Service) AcceptRound(ctx context.Context, meta Meta, publicID string) (Decision, error) {
	// One at a time: the look for the table's ticket and the opening of one
	// are two steps, and this process is the only writer there is.
	s.accepting.Lock()
	defer s.accepting.Unlock()

	staff, row, err := s.deciding(ctx, meta, publicID)
	if err != nil {
		return Decision{}, err
	}
	switch RoundStatus(row.Status) {
	case RoundSent:
		return decided(row, true)
	case RoundDeclined:
		return Decision{}, fmt.Errorf("%w: %s was declined", ErrRoundDecided, publicID)
	}
	since, err := takingRequestsSince(ctx, s.db.Queries(), meta.VenueID)
	if err != nil {
		return Decision{}, err
	}
	if row.CreatedAt.Before(since) {
		return Decision{}, fmt.Errorf("%w: %s", ErrRoundLapsed, publicID)
	}
	lines, err := decodeLines(row.PublicID, row.Lines)
	if err != nil {
		return Decision{}, err
	}

	for range maxRoundAttempts {
		orderID, err := s.ticketAt(ctx, staff, row.TableID)
		if err != nil {
			return Decision{}, err
		}
		_, err = s.run(ctx, staff, orderID, "accept_round", publicID, func(ctx context.Context, tx *db.OrderTx, _ venue, at time.Time) ([]order.Payload, error) {
			if tx.State.Status == order.StatusClosed || !slices.Contains(tx.State.Tables(), row.TableID) {
				return nil, errTableChanged
			}
			n, err := tx.Queries().DecideGuestRound(ctx, gen.DecideGuestRoundParams{
				Status: string(RoundSent), OrderID: sql.NullInt64{Int64: tx.OrderID, Valid: true},
				DecidedAt: sql.NullTime{Time: at, Valid: true}, DecidedBy: sql.NullInt64{Int64: staff.ActorID, Valid: true},
				ID: row.ID, VenueID: meta.VenueID,
			})
			if err != nil {
				return nil, fmt.Errorf("accepting request %s: %w", publicID, err)
			}
			if n != 1 {
				return nil, errRoundRaced
			}
			if err := tx.Queries().BumpFloorVersion(ctx, meta.VenueID); err != nil {
				return nil, fmt.Errorf("accepting request %s: %w", publicID, err)
			}
			return roundPayloads(lines, tx.State.LastSeq+1), nil
		})
		switch {
		case errors.Is(err, errTableChanged):
			continue
		case errors.Is(err, errRoundRaced):
			// Another till answered it. Accepted there is accepted here.
			again, rerr := s.round(ctx, meta.VenueID, publicID)
			if rerr != nil {
				return Decision{}, rerr
			}
			if RoundStatus(again.Status) == RoundSent {
				return decided(again, true)
			}
			return Decision{}, fmt.Errorf("%w: %s was declined", ErrRoundDecided, publicID)
		case err != nil:
			return Decision{}, busy(err)
		}
		s.onChange(meta.VenueID)
		after, err := s.round(ctx, meta.VenueID, publicID)
		if err != nil {
			return Decision{}, err
		}
		return decided(after, false)
	}
	return Decision{}, fmt.Errorf("accepting request %s: table %d kept changing", publicID, row.TableID)
}

// ticketAt is the ticket open at a table, opened now if there is none: a
// guest's own, on the qr_table channel, by the staff member accepting.
func (s *Service) ticketAt(ctx context.Context, staff Meta, tableID int64) (int64, error) {
	orderID, err := s.db.Queries().UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: tableID, VenueID: staff.VenueID})
	if err == nil {
		return orderID, nil
	}
	if !isNoRows(err) {
		return 0, fmt.Errorf("reading the ticket at table %d: %w", tableID, err)
	}
	opened, err := s.Open(ctx, staff, OpenOrder{Channel: order.ChannelQRTable, TableID: tableID})
	if err != nil {
		return 0, err
	}
	return opened.OrderID, nil
}

// DeclineRound turns a waiting request down. Nothing was ever an order, so
// nothing is voided: the request is marked, with who and when.
func (s *Service) DeclineRound(ctx context.Context, meta Meta, publicID string) (Decision, error) {
	staff, row, err := s.deciding(ctx, meta, publicID)
	if err != nil {
		return Decision{}, err
	}
	if RoundStatus(row.Status) == RoundDeclined {
		return decided(row, true)
	}
	at := s.clock.Now().UTC()
	var changed int64
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.DecideGuestRound(ctx, gen.DecideGuestRoundParams{
			Status:    string(RoundDeclined),
			DecidedAt: sql.NullTime{Time: at, Valid: true}, DecidedBy: sql.NullInt64{Int64: staff.ActorID, Valid: true},
			ID: row.ID, VenueID: meta.VenueID,
		})
		if err != nil {
			return fmt.Errorf("declining request %s: %w", publicID, err)
		}
		if changed = n; n != 1 {
			return nil
		}
		return q.BumpFloorVersion(ctx, meta.VenueID)
	})
	if err != nil {
		return Decision{}, err
	}
	after, err := s.round(ctx, meta.VenueID, publicID)
	if err != nil {
		return Decision{}, err
	}
	if RoundStatus(after.Status) != RoundDeclined {
		return Decision{}, fmt.Errorf("%w: %s was accepted", ErrRoundDecided, publicID)
	}
	if changed == 1 {
		s.onChange(meta.VenueID)
	}
	return decided(after, changed != 1)
}

// deciding checks who is answering and reads the request. Only a member of
// staff answers one: the answer names them.
func (s *Service) deciding(ctx context.Context, meta Meta, publicID string) (Meta, gen.GuestRoundByPublicIDRow, error) {
	if err := meta.validate(); err != nil {
		return Meta{}, gen.GuestRoundByPublicIDRow{}, err
	}
	if meta.ActorID < 1 {
		return Meta{}, gen.GuestRoundByPublicIDRow{}, fmt.Errorf("%w: a request is answered by a member of staff", ErrInvalidCommand)
	}
	row, err := s.round(ctx, meta.VenueID, publicID)
	// One click answers a request and may open a ticket and append to it,
	// so no one of those commands may carry the request's key.
	staff := meta
	staff.IdempotencyKey = ""
	return staff, row, err
}

// round reads a request, if it is at the caller's venue.
func (s *Service) round(ctx context.Context, venueID int64, publicID string) (gen.GuestRoundByPublicIDRow, error) {
	row, err := s.db.Queries().GuestRoundByPublicID(ctx, publicID)
	// A round waiting to be paid is not a request: there is nothing for
	// staff to accept or decline, and to them it is not there.
	if isNoRows(err) || (err == nil && (row.VenueID != venueID || row.Prepay == 1)) {
		return gen.GuestRoundByPublicIDRow{}, fmt.Errorf("%w: %q", ErrRoundNotFound, publicID)
	}
	if err != nil {
		return gen.GuestRoundByPublicIDRow{}, fmt.Errorf("reading request %q: %w", publicID, err)
	}
	return row, nil
}

func decided(row gen.GuestRoundByPublicIDRow, replayed bool) (Decision, error) {
	lines, err := decodeLines(row.PublicID, row.Lines)
	if err != nil {
		return Decision{}, err
	}
	return Decision{
		Round: Round{
			PublicID: row.PublicID, Status: RoundStatus(row.Status), TableID: row.TableID, OrderID: row.OrderID.Int64,
			Lines: lines, Subtotal: money.Amount{Minor: row.SubtotalMinor, Currency: row.Currency},
		},
		OrderPublicID: row.OrderPublicID, Replayed: replayed,
	}, nil
}

func decodeLines(publicID, encoded string) ([]order.LineAdded, error) {
	var lines []order.LineAdded
	if err := json.Unmarshal([]byte(encoded), &lines); err != nil {
		return nil, fmt.Errorf("decoding round %s: %w", publicID, err)
	}
	return lines, nil
}
