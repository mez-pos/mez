package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/invoice"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// SnapshotEvery is how many events apart an order's snapshot is retaken. A
// cafe order is ten to thirty events, so most never get one; a bar tab open
// all night does, and loading it replays at most this many.
const SnapshotEvery = 50

// maxAppendAttempts bounds the retry on a seq conflict. The stream is read
// inside the write transaction, so under one writer a conflict needs a second
// process on the same file; three attempts is generous for that.
const maxAppendAttempts = 3

var (
	// ErrOrderNotFound means no order has the ID asked for.
	ErrOrderNotFound = errors.New("db: no such order")

	// ErrSeqConflict means another writer appended to the order between
	// this command reading it and writing. AppendOrder retries it; it only
	// reaches a caller when every attempt conflicted.
	ErrSeqConflict = errors.New("db: the order was appended to by someone else first")

	// errStaleSnapshot means a command was refused on top of a snapshot that
	// a replay of the events disagrees with. AppendOrder retries from the
	// events, and the retry overwrites the snapshot.
	errStaleSnapshot = errors.New("db: the order snapshot disagrees with its events")

	// ErrPublicIDTaken means a new order drew a public ID already in use.
	// ADR-034: the caller draws another and tries again.
	ErrPublicIDTaken = errors.New("db: public id is already taken")
)

// NewOrder is an order about to be opened: its public ID, its venue and the
// order.opened event that starts its stream at seq 1.
type NewOrder struct {
	PublicID string
	VenueID  int64
	Opened   order.Event
}

// OpenOrder writes a new order and its first event in one transaction and
// returns its ID and projection. The event is projected before anything is
// written, so an order.opened the domain refuses leaves no row behind.
func (d *DB) OpenOrder(ctx context.Context, n NewOrder, taxes order.Taxes) (int64, order.State, error) {
	if _, ok := n.Opened.Payload.(order.OrderOpened); !ok {
		return 0, order.State{}, fmt.Errorf("db: an order must open with order.opened, not %T", n.Opened.Payload)
	}
	first := n.Opened
	first.Seq = 1
	state, err := order.Project([]order.Event{first}, taxes)
	if err != nil {
		return 0, order.State{}, err
	}

	var orderID int64
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		orderID, err = q.CreateOrder(ctx, gen.CreateOrderParams{
			PublicID:  n.PublicID,
			VenueID:   n.VenueID,
			Channel:   string(state.Channel),
			TableID:   state.TableID,
			Status:    string(state.Status),
			LastSeq:   state.LastSeq,
			OpenedAt:  state.OpenedAt,
			ClosedAt:  nullTime(state.ClosedAt),
			UpdatedAt: state.UpdatedAt,
		})
		if isUniqueViolation(err) && strings.Contains(err.Error(), "orders.public_id") {
			return fmt.Errorf("%w: %q", ErrPublicIDTaken, n.PublicID)
		}
		if err != nil {
			return fmt.Errorf("creating order %q: %w", n.PublicID, err)
		}
		return appendEvents(ctx, q, orderID, []order.Event{first})
	})
	if err != nil {
		return 0, order.State{}, err
	}
	return orderID, state, nil
}

// OrderTx is what a command sees while it decides: the order as it stands,
// and the transaction its events will be appended in. Anything it writes
// through Queries, or numbers it takes from Invoices, commits with the
// events or not at all.
type OrderTx struct {
	OrderID int64
	VenueID int64
	State   order.State

	q *gen.Queries
}

// Queries is the generated query set bound to the appending transaction.
func (o *OrderTx) Queries() *gen.Queries { return o.q }

// Invoices allocates invoice numbers inside the appending transaction, so a
// command that fails after allocating gives its number back.
func (o *OrderTx) Invoices() invoice.Allocator { return NewInvoiceAllocator(o.q) }

// RecordInvoice writes an issued number to the invoice register against this
// order. Its invoice.issued event is the record; this is the index over it.
func (o *OrderTx) RecordInvoice(ctx context.Context, in invoice.Issued, at time.Time) error {
	return recordInvoice(ctx, o.q, o.VenueID, o.OrderID, in, at)
}

// Decide is a command: given the order as it stands, the events to append.
// The events' Seq is ignored and assigned in order after State.LastSeq.
//
// Decide may run more than once if the append conflicts, each time in a fresh
// transaction against fresh state, so it must do nothing outside the OrderTx
// it is given: no printing, no hub publish, no card terminal. Those happen
// after AppendOrder returns.
type Decide func(ctx context.Context, tx *OrderTx) ([]order.Event, error)

// AppendOrder runs a command against an order and appends what it decides,
// in one write transaction: read the stream, decide, project the result,
// append the events, bring the orders row up to date and retake the snapshot
// when it is due.
//
// Projection is the check. If the events could not have happened, the
// *order.ProjectionError comes back and nothing is written. A seq conflict
// is retried from the read; any other error is returned as it is.
//
// It returns the new state and the events as appended, numbered. A Decide
// that returns no events appends nothing and returns the state unchanged.
func (d *DB) AppendOrder(ctx context.Context, orderID int64, taxes order.Taxes, decide Decide) (order.State, []order.Event, error) {
	var err error
	useSnapshot := true
	for range maxAppendAttempts {
		var (
			state    order.State
			appended []order.Event
		)
		err = d.Tx(ctx, func(tx *sql.Tx) error {
			var err error
			state, appended, err = d.appendOnce(ctx, gen.New(tx), orderID, taxes, decide, useSnapshot)
			return err
		})
		switch {
		case err == nil:
			return state, appended, nil
		case errors.Is(err, errStaleSnapshot):
			useSnapshot = false
		case !errors.Is(err, ErrSeqConflict):
			return order.State{}, nil, err
		}
		d.log.Warn("order append failed, retrying", "order_id", orderID, "error", err)
	}
	return order.State{}, nil, err
}

func (d *DB) appendOnce(ctx context.Context, q *gen.Queries, orderID int64, taxes order.Taxes, decide Decide, useSnapshot bool) (order.State, []order.Event, error) {
	row, err := q.OrderByID(ctx, orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return order.State{}, nil, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
	}
	if err != nil {
		return order.State{}, nil, fmt.Errorf("reading order %d: %w", orderID, err)
	}
	current, fromSnapshot, err := d.load(ctx, q, orderID, taxes, useSnapshot)
	if err != nil {
		return order.State{}, nil, err
	}
	if current.LastSeq != row.LastSeq {
		return order.State{}, nil, fmt.Errorf("order %d: the orders row is at seq %d but its events reach %d", orderID, row.LastSeq, current.LastSeq)
	}

	decided, err := decide(ctx, &OrderTx{OrderID: orderID, VenueID: row.VenueID, State: current, q: q})
	if err != nil {
		return order.State{}, nil, err
	}
	if len(decided) == 0 {
		return current, nil, nil
	}
	events := make([]order.Event, len(decided))
	for i, e := range decided {
		e.Seq = current.LastSeq + int64(i+1)
		events[i] = e
	}

	next, err := order.Resume(current, events, taxes)
	if err != nil {
		if fromSnapshot {
			return order.State{}, nil, d.blameSnapshot(ctx, q, orderID, taxes, current, err)
		}
		return order.State{}, nil, err
	}
	if err := appendEvents(ctx, q, orderID, events); err != nil {
		return order.State{}, nil, err
	}
	moved, err := q.UpdateOrderIndex(ctx, gen.UpdateOrderIndexParams{
		TableID:   next.TableID,
		Status:    string(next.Status),
		LastSeq:   next.LastSeq,
		ClosedAt:  nullTime(next.ClosedAt),
		UpdatedAt: next.UpdatedAt,
		ID:        orderID,
		PrevSeq:   current.LastSeq,
	})
	if err != nil {
		return order.State{}, nil, fmt.Errorf("updating order %d: %w", orderID, err)
	}
	if moved != 1 {
		return order.State{}, nil, fmt.Errorf("%w: order %d is no longer at seq %d", ErrSeqConflict, orderID, current.LastSeq)
	}
	crossed := current.LastSeq/SnapshotEvery != next.LastSeq/SnapshotEvery
	if crossed || (!useSnapshot && next.LastSeq >= SnapshotEvery) {
		if err := saveSnapshot(ctx, q, orderID, next, taxes); err != nil {
			return order.State{}, nil, err
		}
	}
	return next, events, nil
}

// LoadOrder is an order's current state: its snapshot, if one was taken
// under the same basis, and the events since.
//
// It reads without a transaction, which is safe because events are only
// ever appended: a snapshot and a tail read a moment apart still fold to a
// state the order was really in.
func (d *DB) LoadOrder(ctx context.Context, orderID int64, taxes order.Taxes) (order.State, error) {
	return d.loadState(ctx, d.Queries(), orderID, taxes, true)
}

// RebuildOrder is an order's state from its events alone, ignoring any
// snapshot. It is the definition LoadOrder is an optimisation of.
func (d *DB) RebuildOrder(ctx context.Context, orderID int64, taxes order.Taxes) (order.State, error) {
	return d.loadState(ctx, d.Queries(), orderID, taxes, false)
}

// OrderEvents is an order's stream after afterSeq, in order. Zero reads it
// all.
func (d *DB) OrderEvents(ctx context.Context, orderID, afterSeq int64) ([]order.Event, error) {
	return orderEvents(ctx, d.Queries(), orderID, afterSeq)
}

func (d *DB) loadState(ctx context.Context, q *gen.Queries, orderID int64, taxes order.Taxes, useSnapshot bool) (order.State, error) {
	state, _, err := d.load(ctx, q, orderID, taxes, useSnapshot)
	return state, err
}

// load folds an order, from its snapshot when useSnapshot allows and one
// fits, and reports whether the snapshot was used.
func (d *DB) load(ctx context.Context, q *gen.Queries, orderID int64, taxes order.Taxes, useSnapshot bool) (order.State, bool, error) {
	var from order.State
	if useSnapshot {
		snap, err := snapshotFor(ctx, q, orderID, taxes)
		if err != nil {
			return order.State{}, false, err
		}
		from = snap
	}
	tail, err := orderEvents(ctx, q, orderID, from.LastSeq)
	if err != nil {
		return order.State{}, false, err
	}
	if from.LastSeq == 0 && len(tail) == 0 {
		return order.State{}, false, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
	}
	state, err := order.Resume(from, tail, taxes)
	if err != nil && from.LastSeq > 0 {
		// The events are the truth. A snapshot the tail will not fold onto
		// is a bad snapshot, so the order is replayed without it, and the
		// replay's answer, success or failure, is the one returned.
		d.log.Warn("order snapshot did not fold, replaying from events",
			"order_id", orderID, "snapshot_seq", from.LastSeq, "error", err)
		return d.load(ctx, q, orderID, taxes, false)
	}
	return state, from.LastSeq > 0, err
}

// blameSnapshot decides whether a command refused on top of a snapshot was
// refused by the order or by the snapshot. It replays the events; if they
// agree with the snapshot the refusal stands, and if not the snapshot is at
// fault and the append is retried without it. This runs only on a refusal,
// so the replay costs nothing on the path that sells.
func (d *DB) blameSnapshot(ctx context.Context, q *gen.Queries, orderID int64, taxes order.Taxes, current order.State, refusal error) error {
	replayed, _, err := d.load(ctx, q, orderID, taxes, false)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(replayed, current) {
		return refusal
	}
	d.log.Warn("order snapshot disagrees with its events, retrying from events",
		"order_id", orderID, "snapshot_seq", current.LastSeq, "refusal", refusal)
	return fmt.Errorf("%w: order %d", errStaleSnapshot, orderID)
}

// snapshotFor returns the order's snapshot if it was taken under the same
// basis, and a zero State otherwise.
func snapshotFor(ctx context.Context, q *gen.Queries, orderID int64, taxes order.Taxes) (order.State, error) {
	snap, err := q.OrderSnapshot(ctx, orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return order.State{}, nil
	}
	if err != nil {
		return order.State{}, fmt.Errorf("reading the snapshot of order %d: %w", orderID, err)
	}
	basis, err := snapshotBasis(taxes)
	if err != nil {
		return order.State{}, err
	}
	if snap.Basis != basis {
		return order.State{}, nil
	}
	var state order.State
	if err := json.Unmarshal([]byte(snap.State), &state); err != nil || state.LastSeq != snap.Seq {
		return order.State{}, nil
	}
	return state, nil
}

func saveSnapshot(ctx context.Context, q *gen.Queries, orderID int64, state order.State, taxes order.Taxes) error {
	basis, err := snapshotBasis(taxes)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encoding the snapshot of order %d: %w", orderID, err)
	}
	err = q.SaveOrderSnapshot(ctx, gen.SaveOrderSnapshotParams{
		OrderID: orderID, Seq: state.LastSeq, Basis: basis, State: string(raw),
	})
	if err != nil {
		return fmt.Errorf("saving the snapshot of order %d: %w", orderID, err)
	}
	return nil
}

// snapshotBasis names what a projection depends on besides its events: the
// projection's own rules and the venue's tax regime. A snapshot is only
// reused under the same basis, so a venue changing its GST registration, or
// a build changing the fold, never serves a state its events would not give.
func snapshotBasis(taxes order.Taxes) (string, error) {
	profile := "none"
	if taxes.Profile != nil {
		profile = taxes.Profile.Name()
	}
	venue, err := json.Marshal(taxes.Venue)
	if err != nil {
		return "", fmt.Errorf("encoding the tax context: %w", err)
	}
	return fmt.Sprintf("v%d|%s|%s", order.ProjectionVersion, profile, venue), nil
}

func appendEvents(ctx context.Context, q *gen.Queries, orderID int64, events []order.Event) error {
	for _, e := range events {
		if e.Payload == nil {
			return fmt.Errorf("order %d: event at seq %d has no payload", orderID, e.Seq)
		}
		payload, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("order %d: encoding %s at seq %d: %w", orderID, e.Type, e.Seq, err)
		}
		err = q.AppendOrderEvent(ctx, gen.AppendOrderEventParams{
			OrderID:  orderID,
			Seq:      e.Seq,
			Type:     string(e.Type),
			Payload:  string(payload),
			ActorID:  e.ActorID,
			DeviceID: e.DeviceID,
			At:       e.At.UTC(),
		})
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: order %d already has seq %d", ErrSeqConflict, orderID, e.Seq)
		}
		if err != nil {
			return fmt.Errorf("order %d: appending %s at seq %d: %w", orderID, e.Type, e.Seq, err)
		}
	}
	return nil
}

func orderEvents(ctx context.Context, q *gen.Queries, orderID, afterSeq int64) ([]order.Event, error) {
	rows, err := q.OrderEventsAfter(ctx, gen.OrderEventsAfterParams{OrderID: orderID, Seq: afterSeq})
	if err != nil {
		return nil, fmt.Errorf("reading the events of order %d: %w", orderID, err)
	}
	events := make([]order.Event, len(rows))
	for i, r := range rows {
		p, err := order.DecodePayload(order.Type(r.Type), json.RawMessage(r.Payload))
		if err != nil {
			return nil, fmt.Errorf("order %d, seq %d: %w", orderID, r.Seq, err)
		}
		events[i] = order.Event{
			Seq:      r.Seq,
			Type:     order.Type(r.Type),
			At:       r.At.UTC(),
			ActorID:  r.ActorID,
			DeviceID: r.DeviceID,
			Payload:  p,
		}
	}
	return events, nil
}

// nullTime maps the domain's zero time, meaning not yet, to NULL.
func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

// isUniqueViolation reports whether err is SQLite refusing a duplicate key.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}
