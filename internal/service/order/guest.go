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
	"github.com/byjoellobo/mez/internal/domain/id"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// A guest's round (phase 11, Task 4a, ADR-210): one time a guest pressed
// Place Order at a table. Where it goes turns on the table:
//
//   - A ticket is open there, because staff seated the table or accepted an
//     earlier request: the round's lines go onto that ticket and to the
//     kitchen in one command, as a waiter's would. One table, one ticket,
//     one bill, whoever at it ordered.
//   - No ticket is open: the round is a request (ADR-209). It is kept, priced,
//     and is no order and no event until staff accept it.
//
// The guest is nobody: actor 0 on device 0. What they send is ids and
// counts; every name, price and tax fact is the menu's.

var (
	// ErrTableBusy means the table's ticket takes no more lines: its bill
	// is invoiced, or it is on hold.
	ErrTableBusy = errors.New("the table's ticket is not taking orders")

	// ErrTooManyWaiting means the table already has as many requests
	// waiting to be accepted as it may.
	ErrTooManyWaiting = errors.New("the table has too many orders waiting to be accepted")
)

const (
	// MaxRoundLines is the most lines one round may carry: the guest
	// cart's own bound (ADR-208).
	MaxRoundLines = 50

	// maxWaitingRounds is how many requests may wait at one table. A party
	// each ordering from their own phone is a few; past that it is someone
	// with a photograph of the sticker, and staff have enough to decline.
	maxWaitingRounds = 5

	// roundPrefix is the public ID prefix of a round: rnd_7K3M9QX2.
	roundPrefix = "rnd"

	// maxRoundAttempts bounds the retry when the table changes under a
	// round, or its public ID collides.
	maxRoundAttempts = 4
)

// RoundStatus is where a guest's round stands.
type RoundStatus string

// The statuses. The strings are what is stored and sent.
const (
	// RoundWaiting is a request staff have not answered.
	RoundWaiting RoundStatus = "waiting"
	// RoundSent is on a ticket and with the kitchen.
	RoundSent RoundStatus = "sent"
	// RoundDeclined is a request staff turned down.
	RoundDeclined RoundStatus = "declined"
)

// RoundStatuses is every status, for the generated vocabulary.
func RoundStatuses() []RoundStatus { return []RoundStatus{RoundWaiting, RoundSent, RoundDeclined} }

// PlaceRound is what a guest asks for at a table.
type PlaceRound struct {
	TableID int64       `json:"table_id"`
	Picks   []menu.Pick `json:"picks"`
}

// Round is a guest's round as it stands. OrderID is 0 until its lines are on
// a ticket. Replayed is true when the key had been sent before and nothing
// was written this time.
type Round struct {
	PublicID string
	Status   RoundStatus
	TableID  int64
	OrderID  int64
	Lines    []order.LineAdded
	Subtotal money.Amount
	Replayed bool
}

// errTableChanged unwinds a round whose table gained or lost its ticket
// between the look and the write. The round is tried again from the look.
var errTableChanged = errors.New("the table changed under the round")

// roundDraft is a round priced and ready to be written.
type roundDraft struct {
	tableID     int64
	lines       []order.LineAdded
	encoded     string
	subtotal    money.Amount
	key         string
	fingerprint string
}

// PlaceRound takes a guest's round. The idempotency key is required: a
// phone on a poor connection sends again, and a second round in the kitchen
// is food nobody asked for.
func (s *Service) PlaceRound(ctx context.Context, meta Meta, cmd PlaceRound) (Round, error) {
	if err := meta.validate(); err != nil {
		return Round{}, err
	}
	if meta.IdempotencyKey == "" {
		return Round{}, fmt.Errorf("%w: a guest's order needs an idempotency key", ErrInvalidCommand)
	}
	if n := len(cmd.Picks); n < 1 || n > MaxRoundLines {
		return Round{}, fmt.Errorf("%w: a round has 1 to %d lines, not %d", ErrInvalidCommand, MaxRoundLines, n)
	}
	fp, err := fingerprint("guest_round", 0, cmd)
	if err != nil {
		return Round{}, err
	}
	if prior, err := s.priorRound(ctx, meta, fp); err != nil || prior != nil {
		return deref(prior), err
	}

	v, err := loadVenue(ctx, s.db.Queries(), meta.VenueID)
	if err != nil {
		return Round{}, err
	}
	m, err := s.db.Menu(ctx, meta.VenueID)
	if err != nil {
		return Round{}, fmt.Errorf("reading the menu for a guest's round: %w", err)
	}
	draft, err := priceRound(m, v.Currency, cmd, s.clock.Now().UTC())
	if err != nil {
		return Round{}, err
	}
	draft.key, draft.fingerprint = meta.IdempotencyKey, fp

	for range maxRoundAttempts {
		round, err := s.writeRound(ctx, meta, draft)
		switch {
		case err == nil:
			return round, nil
		case errors.Is(err, errTableChanged):
			continue
		case db.IsUniqueViolation(err):
			// The same key from the same phone, twice at once: the other
			// one won, and this is its answer. Otherwise the public ID
			// collided, and another is drawn.
			if prior, perr := s.priorRound(ctx, meta, fp); perr != nil || prior != nil {
				return deref(prior), perr
			}
			continue
		default:
			return Round{}, err
		}
	}
	return Round{}, fmt.Errorf("placing a guest's round at table %d: it kept changing", cmd.TableID)
}

// priceRound rings every pick up from the menu and adds the round up.
func priceRound(m menu.Menu, currency string, cmd PlaceRound, at time.Time) (roundDraft, error) {
	draft := roundDraft{tableID: cmd.TableID, lines: make([]order.LineAdded, len(cmd.Picks))}
	values := make([]money.Amount, len(cmd.Picks))
	for i, p := range cmd.Picks {
		line, err := m.Ring(p, at)
		if err != nil {
			return roundDraft{}, err
		}
		if values[i], err = menu.LineValue(line); err != nil {
			return roundDraft{}, fmt.Errorf("pricing %s: %w", line.Name, err)
		}
		draft.lines[i] = line
	}
	var err error
	if draft.subtotal, err = money.Sum(currency, values...); err != nil {
		return roundDraft{}, fmt.Errorf("adding up a guest's round: %w", err)
	}
	encoded, err := json.Marshal(draft.lines)
	if err != nil {
		return roundDraft{}, fmt.Errorf("encoding a guest's round: %w", err)
	}
	draft.encoded = string(encoded)
	return draft, nil
}

// writeRound looks at the table and writes the round where it goes. The
// look is outside the write, so each path checks it again inside.
func (s *Service) writeRound(ctx context.Context, meta Meta, draft roundDraft) (Round, error) {
	publicID, err := id.NewPublic(roundPrefix)
	if err != nil {
		return Round{}, fmt.Errorf("drawing a round id: %w", err)
	}
	round := Round{PublicID: publicID, TableID: draft.tableID, Lines: draft.lines, Subtotal: draft.subtotal}

	orderID, err := s.db.Queries().UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: draft.tableID, VenueID: meta.VenueID})
	switch {
	case isNoRows(err):
		round.Status = RoundWaiting
		return round, s.requestRound(ctx, meta, draft, publicID)
	case err != nil:
		return Round{}, fmt.Errorf("reading the ticket at table %d: %w", draft.tableID, err)
	}
	round.Status, round.OrderID = RoundSent, orderID
	return round, s.sendRound(ctx, meta, orderID, draft, publicID)
}

// requestRound keeps a round as a request at a table with no ticket open.
// It is refused with the venue shut, as an order would be: a request nobody
// is there to accept is a guest waiting for nothing.
func (s *Service) requestRound(ctx context.Context, meta Meta, draft roundDraft, publicID string) error {
	at := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: draft.tableID, VenueID: meta.VenueID})
		if err == nil {
			return errTableChanged
		}
		if !isNoRows(err) {
			return fmt.Errorf("reading the ticket at table %d: %w", draft.tableID, err)
		}
		if err := tableInService(ctx, q, meta.VenueID, draft.tableID); err != nil {
			return err
		}
		if _, err := q.OpenShiftIDForVenue(ctx, meta.VenueID); isNoRows(err) {
			return fmt.Errorf("%w: a guest's order at venue %d", shift.ErrNoOpenShift, meta.VenueID)
		} else if err != nil {
			return fmt.Errorf("reading venue %d's open shift: %w", meta.VenueID, err)
		}
		waiting, err := q.CountWaitingGuestRounds(ctx, draft.tableID)
		if err != nil {
			return fmt.Errorf("counting the requests at table %d: %w", draft.tableID, err)
		}
		if waiting >= maxWaitingRounds {
			return fmt.Errorf("%w: table %d has %d", ErrTooManyWaiting, draft.tableID, waiting)
		}
		return saveRound(ctx, q, meta, draft, publicID, RoundWaiting, 0, at)
	})
	if err != nil {
		return err
	}
	s.log.Info("a guest's order is waiting to be accepted", "venue_id", meta.VenueID, "table_id", draft.tableID, "round", publicID)
	return nil
}

// sendRound puts a round's lines on the table's open ticket and sends them
// to their stations, in one command. Only the round's own lines are sent: a
// waiter's lines still being rung up on the same ticket stay where they are.
// A course is the waiter's to hold back, so a guest's lines go whatever
// course they are in.
func (s *Service) sendRound(ctx context.Context, meta Meta, orderID int64, draft roundDraft, publicID string) error {
	guest := Meta{VenueID: meta.VenueID}
	_, err := s.run(ctx, guest, orderID, "guest_round", draft.fingerprint, func(ctx context.Context, tx *db.OrderTx, _ venue, at time.Time) ([]order.Payload, error) {
		if tx.State.Status == order.StatusClosed || !slices.Contains(tx.State.Tables(), draft.tableID) {
			return nil, errTableChanged
		}
		payloads := make([]order.Payload, 0, len(draft.lines)+2)
		for _, l := range draft.lines {
			payloads = append(payloads, l)
		}
		for _, t := range roundTickets(draft.lines, tx.State.LastSeq+1) {
			payloads = append(payloads, t)
		}
		if err := saveRound(ctx, tx.Queries(), meta, draft, publicID, RoundSent, tx.OrderID, at); err != nil {
			return nil, err
		}
		return payloads, nil
	})
	if errors.Is(err, order.ErrOrderHeld) || errors.Is(err, order.ErrOrderInvoiced) {
		return fmt.Errorf("%w: %v", ErrTableBusy, err)
	}
	return err
}

// roundTickets is a ticket for each station with lines in the round,
// stations in the order their first line comes. The lines' ids are their
// events' seqs, which run from first. A line with no station goes nowhere.
func roundTickets(lines []order.LineAdded, first int64) []order.TicketSent {
	var out []order.TicketSent
	at := map[string]int{}
	for i, l := range lines {
		if l.Station == "" {
			continue
		}
		n, ok := at[l.Station]
		if !ok {
			n = len(out)
			at[l.Station] = n
			out = append(out, order.TicketSent{Station: l.Station})
		}
		out[n].LineIDs = append(out[n].LineIDs, first+int64(i))
	}
	return out
}

func saveRound(ctx context.Context, q *gen.Queries, meta Meta, draft roundDraft, publicID string, status RoundStatus, orderID int64, at time.Time) error {
	_, err := q.CreateGuestRound(ctx, gen.CreateGuestRoundParams{
		PublicID: publicID, VenueID: meta.VenueID, TableID: draft.tableID,
		Status: string(status), OrderID: sql.NullInt64{Int64: orderID, Valid: orderID != 0},
		Lines: draft.encoded, SubtotalMinor: draft.subtotal.Minor, Currency: draft.subtotal.Currency,
		IdempotencyKey: draft.key, Fingerprint: draft.fingerprint, CreatedAt: at,
	})
	if err != nil {
		return fmt.Errorf("saving a guest's round: %w", err)
	}
	return nil
}

// priorRound is the round a key was first sent with, as it stands now, or
// nil for a key never used. A key used for another round is refused.
func (s *Service) priorRound(ctx context.Context, meta Meta, fp string) (*Round, error) {
	row, err := s.db.Queries().GuestRoundByKey(ctx, gen.GuestRoundByKeyParams{VenueID: meta.VenueID, IdempotencyKey: meta.IdempotencyKey})
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading a guest's round by its key: %w", err)
	}
	if row.Fingerprint != fp {
		return nil, fmt.Errorf("%w: %q", ErrIdempotencyKeyReused, meta.IdempotencyKey)
	}
	var lines []order.LineAdded
	if err := json.Unmarshal([]byte(row.Lines), &lines); err != nil {
		return nil, fmt.Errorf("decoding round %s: %w", row.PublicID, err)
	}
	return &Round{
		PublicID: row.PublicID, Status: RoundStatus(row.Status), TableID: row.TableID, OrderID: row.OrderID.Int64,
		Lines: lines, Subtotal: money.Amount{Minor: row.SubtotalMinor, Currency: row.Currency}, Replayed: true,
	}, nil
}

func deref(r *Round) Round {
	if r == nil {
		return Round{}
	}
	return *r
}
