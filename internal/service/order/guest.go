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
	// RoundUnpaid is a round at a venue where guests pay first, not yet
	// paid for: it is not sent, and it is nothing staff answer. It is
	// stored as a waiting round marked prepay.
	RoundUnpaid RoundStatus = "unpaid"
)

// RoundStatuses is every status, for the generated vocabulary.
func RoundStatuses() []RoundStatus {
	return []RoundStatus{RoundWaiting, RoundSent, RoundDeclined, RoundUnpaid}
}

// unpaidFor is how long an unpaid round counts against its table. One
// nobody paid in this time has been walked away from.
const unpaidFor = 30 * time.Minute

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
	// Due is what an unpaid round asks the guest to pay: what its lines
	// add to the table's bill, tax and all. Zero unless unpaid.
	Due      money.Amount
	Replayed bool
	// Progress is how far a sent round has come in the kitchen. Only
	// TrackRound fills it; it is empty for a round that is not sent, or
	// was sent before rounds knew their lines (ADR-236).
	Progress order.Progress
	// Bill is the public id of the ticket a sent round is on. Only
	// TrackRound fills it.
	Bill string
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
		round, err := s.writeRound(ctx, meta, v, draft)
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
func (s *Service) writeRound(ctx context.Context, meta Meta, v venue, draft roundDraft) (Round, error) {
	publicID, err := id.NewPublic(roundPrefix)
	if err != nil {
		return Round{}, fmt.Errorf("drawing a round id: %w", err)
	}
	round := Round{PublicID: publicID, TableID: draft.tableID, Lines: draft.lines, Subtotal: draft.subtotal}

	orderID, err := s.db.Queries().UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: draft.tableID, VenueID: meta.VenueID})
	if err != nil && !isNoRows(err) {
		return Round{}, fmt.Errorf("reading the ticket at table %d: %w", draft.tableID, err)
	}
	// Where guests pay first, a round waits for its payment whatever the
	// table: seated or not, first round or fifth.
	if s.paysFirst(v) {
		ticket := int64(0)
		if err == nil {
			ticket = orderID
		}
		round.Status = RoundUnpaid
		round.Due, err = s.holdRound(ctx, meta, v, ticket, draft, publicID)
		return round, err
	}
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
		since, err := takingRequestsSince(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		waiting, err := q.CountWaitingGuestRounds(ctx, gen.CountWaitingGuestRoundsParams{TableID: draft.tableID, Since: since})
		if err != nil {
			return fmt.Errorf("counting the requests at table %d: %w", draft.tableID, err)
		}
		if waiting >= maxWaitingRounds {
			return fmt.Errorf("%w: table %d has %d", ErrTooManyWaiting, draft.tableID, waiting)
		}
		if err := saveRound(ctx, q, meta, draft, publicID, RoundWaiting, 0, money.Amount{}, 0, at); err != nil {
			return err
		}
		// A table wanting to order is the floor changed, with no order
		// event to say so: the version is what tells the terminals.
		return q.BumpFloorVersion(ctx, meta.VenueID)
	})
	if err != nil {
		return err
	}
	s.onChange(meta.VenueID)
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
		first := tx.State.LastSeq + 1
		if err := saveRound(ctx, tx.Queries(), meta, draft, publicID, RoundSent, tx.OrderID, money.Amount{}, first, at); err != nil {
			return nil, err
		}
		return roundPayloads(draft.lines, first), nil
	})
	return busy(err)
}

// busy says a ticket on hold or invoiced in the round's own terms.
func busy(err error) error {
	if errors.Is(err, order.ErrOrderHeld) || errors.Is(err, order.ErrOrderInvoiced) {
		return fmt.Errorf("%w: %v", ErrTableBusy, err)
	}
	return err
}

// roundPayloads is what a round appends: its lines, then their tickets. The
// first line's event is at seq first.
func roundPayloads(lines []order.LineAdded, first int64) []order.Payload {
	payloads := make([]order.Payload, 0, len(lines)+2)
	for _, l := range lines {
		payloads = append(payloads, l)
	}
	for _, t := range roundTickets(lines, first) {
		payloads = append(payloads, t)
	}
	return payloads
}

// takingRequestsSince is when the venue's open shift opened: a request is
// taken only while one is open, and one from before it has lapsed.
func takingRequestsSince(ctx context.Context, q *gen.Queries, venueID int64) (time.Time, error) {
	if _, err := q.OpenShiftIDForVenue(ctx, venueID); isNoRows(err) {
		return time.Time{}, fmt.Errorf("%w: a guest's order at venue %d", shift.ErrNoOpenShift, venueID)
	} else if err != nil {
		return time.Time{}, fmt.Errorf("reading venue %d's open shift: %w", venueID, err)
	}
	open, err := q.OpenShiftForVenue(ctx, venueID)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading venue %d's open shift: %w", venueID, err)
	}
	return open.OpenedAt.UTC(), nil
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

// saveRound writes a round. due is what an unpaid round asks for, and zero
// for any other: an unpaid round is stored waiting, marked prepay. first is
// the seq of a sent round's first line on its ticket, and zero for a round
// that is on none.
func saveRound(ctx context.Context, q *gen.Queries, meta Meta, draft roundDraft, publicID string, status RoundStatus, orderID int64, due money.Amount, first int64, at time.Time) error {
	stored, prepay := status, int64(0)
	if status == RoundUnpaid {
		stored, prepay = RoundWaiting, 1
	}
	_, err := q.CreateGuestRound(ctx, gen.CreateGuestRoundParams{
		PublicID: publicID, VenueID: meta.VenueID, TableID: draft.tableID,
		Status: string(stored), OrderID: sql.NullInt64{Int64: orderID, Valid: orderID != 0},
		Lines: draft.encoded, SubtotalMinor: draft.subtotal.Minor, Currency: draft.subtotal.Currency,
		IdempotencyKey: draft.key, Fingerprint: draft.fingerprint, CreatedAt: at,
		Prepay: prepay, DueMinor: sql.NullInt64{Int64: due.Minor, Valid: prepay == 1},
		FirstSeq: sql.NullInt64{Int64: first, Valid: first != 0},
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
		PublicID: row.PublicID, Status: statusOf(row.Status, row.Prepay), TableID: row.TableID, OrderID: row.OrderID.Int64,
		Lines: lines, Subtotal: money.Amount{Minor: row.SubtotalMinor, Currency: row.Currency},
		Due: dueOf(row.Status, row.Prepay, row.DueMinor, row.Currency), Replayed: true,
	}, nil
}

func deref(r *Round) Round {
	if r == nil {
		return Round{}
	}
	return *r
}

// TableTicket is the ticket open at a table, as it stands, or false when
// the table has none: what a guest's phone shows as the table's bill
// (ADR-214). A table's code is a table's bill; whose phone ordered which
// line is not kept, and a paper bill would not say either.
func (s *Service) TableTicket(ctx context.Context, venueID, tableID int64) (Result, bool, error) {
	orderID, err := s.db.Queries().UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: tableID, VenueID: venueID})
	if isNoRows(err) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, fmt.Errorf("reading the ticket at table %d: %w", tableID, err)
	}
	res, err := s.Order(ctx, venueID, orderID)
	if errors.Is(err, ErrOrderNotFound) {
		return Result{}, false, nil
	}
	return res, err == nil, err
}

// statusOf is a stored round's status as it is told: a waiting round marked
// prepay is unpaid.
func statusOf(stored string, prepay int64) RoundStatus {
	if RoundStatus(stored) == RoundWaiting && prepay == 1 {
		return RoundUnpaid
	}
	return RoundStatus(stored)
}

// dueOf is what an unpaid round asks for, and zero for any other.
func dueOf(stored string, prepay int64, due sql.NullInt64, currency string) money.Amount {
	if statusOf(stored, prepay) != RoundUnpaid || !due.Valid {
		return money.Amount{}
	}
	return money.Amount{Minor: due.Int64, Currency: currency}
}

// paysFirst reports whether a guest's round waits for its payment here: the
// venue says guests pay first, and there is a way for them to.
func (s *Service) paysFirst(v venue) bool {
	return s.canCharge && v.PayMode == PayFirst
}

// holdRound keeps a round until it is paid for, and answers what it asks
// the guest to pay: what its lines add to the table's bill, tax and all, by
// the bill's own projection (order.Adding). It is no order and no event,
// nothing reaches the kitchen, and no terminal is told: staff have nothing
// to answer.
//
// orderID is the ticket open at the table, or 0. A ticket that would not
// take the lines, on hold or invoiced, refuses the round now, before
// anyone is asked for money.
func (s *Service) holdRound(ctx context.Context, meta Meta, v venue, orderID int64, draft roundDraft, publicID string) (money.Amount, error) {
	at := s.clock.Now().UTC()
	var prior []order.Event
	if orderID != 0 {
		var err error
		if prior, err = s.db.OrderEvents(ctx, orderID, 0); err != nil {
			return money.Amount{}, err
		}
	}
	due, err := order.Adding(prior, draft.lines, v.Taxes, v.Currency, draft.tableID, at)
	if err != nil {
		if errors.Is(err, order.ErrOrderClosed) {
			return money.Amount{}, errTableChanged
		}
		return money.Amount{}, busy(err)
	}
	if due.Minor <= 0 {
		// Dishes that come to nothing have nothing to pay first.
		return money.Amount{}, fmt.Errorf("%w: the round comes to nothing", ErrInvalidCommand)
	}
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		if err := tableInService(ctx, q, meta.VenueID, draft.tableID); err != nil {
			return err
		}
		if _, err := takingRequestsSince(ctx, q, meta.VenueID); err != nil {
			return err
		}
		unpaid, err := q.CountUnpaidGuestRounds(ctx, gen.CountUnpaidGuestRoundsParams{TableID: draft.tableID, Since: at.Add(-unpaidFor)})
		if err != nil {
			return fmt.Errorf("counting the unpaid rounds at table %d: %w", draft.tableID, err)
		}
		if unpaid >= maxWaitingRounds {
			return fmt.Errorf("%w: table %d has %d unpaid", ErrTooManyWaiting, draft.tableID, unpaid)
		}
		return saveRound(ctx, q, meta, draft, publicID, RoundUnpaid, 0, due, 0, at)
	})
	if err != nil {
		return money.Amount{}, err
	}
	return due, nil
}
