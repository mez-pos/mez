package order

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/service/printing"
)

var (
	// ErrOrderNotFound means no order with that ID at the caller's venue. An
	// order at another venue is not found, rather than forbidden, so an ID
	// says nothing about venues the caller cannot see.
	ErrOrderNotFound = errors.New("order not found")

	// ErrInvalidCommand means the command is malformed before the order is
	// even consulted: a missing field, a bad idempotency key.
	ErrInvalidCommand = errors.New("invalid command")

	// ErrIdempotencyKeyReused means a key already applied to one request was
	// sent with a different one. Answering with the first result would tell
	// the client something happened that it did not ask for.
	ErrIdempotencyKeyReused = errors.New("idempotency key was already used for a different request")

	// ErrNoSuchTable means an order was opened at a table the venue does
	// not have in service: another venue's, a retired one, or none at all.
	ErrNoSuchTable = errors.New("no such table in service at this venue")
)

// Clock is where the service gets the time for the events it writes. The
// domain never reads a clock; this is the one place the time is decided.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Change is what a command did, as the hub receives it.
type Change struct {
	VenueID  int64
	OrderID  int64
	PublicID string
	Events   []order.Event
	State    order.State
}

// Publisher receives every change after it has committed. It must not block
// or fail the command: the change is on disk, and a device that misses it
// catches up by replay.
type Publisher interface {
	Publish(ctx context.Context, c Change)
}

// PublisherFunc lets a function be a Publisher.
type PublisherFunc func(ctx context.Context, c Change)

// Publish implements Publisher.
func (f PublisherFunc) Publish(ctx context.Context, c Change) { f(ctx, c) }

// Options configure a Service. DB is required.
type Options struct {
	DB        *db.DB
	Clock     Clock     // nil is the system clock
	Publisher Publisher // nil publishes nothing
	Log       *slog.Logger
	// Prints, when set, has the commands queue what their events print, in
	// the same transaction (ADR-081), and is woken once they commit. Nil
	// prints nothing, which is what most tests want.
	Prints PrintQueue
}

// PrintQueue is the print queue as the order service needs it: woken after
// a command queued jobs, so the receipt prints as the guest pays.
type PrintQueue interface {
	Wake()
}

// Service runs order commands.
type Service struct {
	db        *db.DB
	clock     Clock
	publisher Publisher
	log       *slog.Logger
	prints    PrintQueue
}

// New returns a Service.
func New(opts Options) (*Service, error) {
	if opts.DB == nil {
		return nil, errors.New("order service: no database")
	}
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{db: opts.DB, clock: opts.Clock, publisher: opts.Publisher, log: opts.Log, prints: opts.Prints}, nil
}

// Meta is who is asking, from where, and the idempotency key if they sent
// one. Authentication fills it in; the service trusts it.
type Meta struct {
	VenueID        int64
	ActorID        int64 // the staff member; 0 for a guest
	DeviceID       int64
	IdempotencyKey string
}

func (m Meta) validate() error {
	if m.VenueID < 1 {
		return fmt.Errorf("%w: no venue", ErrInvalidCommand)
	}
	if m.ActorID < 0 || m.DeviceID < 0 {
		return fmt.Errorf("%w: negative actor or device", ErrInvalidCommand)
	}
	return checkKey(m.IdempotencyKey)
}

// Result is an order after a command: its state, and the events the command
// appended. Replayed is true when an idempotency key matched and nothing was
// appended this time; the result is then the one the first request got.
type Result struct {
	OrderID  int64
	PublicID string
	State    order.State
	Events   []order.Event
	Replayed bool
}

// decider turns a command into its events, given the order as it stands.
type decider func(ctx context.Context, tx *db.OrderTx, v venue, at time.Time) ([]order.Payload, error)

// run is the path every command on an existing order takes.
func (s *Service) run(ctx context.Context, meta Meta, orderID int64, name string, cmd any, decide decider) (Result, error) {
	if err := meta.validate(); err != nil {
		return Result{}, err
	}
	v, err := loadVenue(ctx, s.db.Queries(), meta.VenueID)
	if err != nil {
		return Result{}, err
	}
	fp, err := fingerprint(name, orderID, cmd)
	if err != nil {
		return Result{}, err
	}
	at := s.clock.Now().UTC()

	var (
		publicID string
		prior    *priorResult
		queued   int
	)
	state, events, err := s.db.AppendOrder(ctx, orderID, v.Taxes, func(ctx context.Context, tx *db.OrderTx) ([]order.Event, error) {
		// Decide can run more than once, so everything it sets is reset.
		publicID, prior, queued = tx.PublicID, nil, 0
		if tx.VenueID != meta.VenueID {
			return nil, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
		}
		p, err := findPrior(ctx, tx.Queries(), meta, fp)
		if err != nil || p != nil {
			prior = p
			return nil, err
		}
		payloads, err := decide(ctx, tx, v, at)
		if err != nil {
			return nil, err
		}
		if len(payloads) == 0 {
			return nil, fmt.Errorf("order service: %s decided nothing", name)
		}
		first := tx.State.LastSeq + 1
		if err := saveKey(ctx, tx.Queries(), meta, fp, tx.OrderID, first, first+int64(len(payloads))-1, at); err != nil {
			return nil, err
		}
		if s.prints != nil {
			tx.OnAppended(func(ctx context.Context, q *gen.Queries, next order.State, appended []order.Event) error {
				n, err := printing.Plan(ctx, q, printing.Input{
					VenueID: meta.VenueID, OrderID: tx.OrderID, PublicID: tx.PublicID, Location: v.Location,
					State: next, Appended: appended,
					Stream: func(ctx context.Context) ([]order.Event, error) { return tx.Events(ctx, 0) },
				}, at)
				queued = n
				return err
			})
		}
		return stamp(payloads, meta, at), nil
	})
	if errors.Is(err, db.ErrOrderNotFound) {
		return Result{}, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
	}
	if err != nil {
		return Result{}, err
	}
	if prior != nil {
		return s.replay(ctx, v, *prior)
	}
	res := Result{OrderID: orderID, PublicID: publicID, State: state, Events: events}
	if queued > 0 {
		s.prints.Wake()
	}
	s.publish(ctx, meta.VenueID, res)
	return res, nil
}

// stamp turns payloads into events: now, by this actor, on this device. The
// store numbers them.
func stamp(payloads []order.Payload, meta Meta, at time.Time) []order.Event {
	events := make([]order.Event, len(payloads))
	for i, p := range payloads {
		events[i] = order.NewEvent(0, at, meta.ActorID, meta.DeviceID, p)
	}
	return events
}

func (s *Service) publish(ctx context.Context, venueID int64, res Result) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, Change{
		VenueID: venueID, OrderID: res.OrderID, PublicID: res.PublicID, Events: res.Events, State: res.State,
	})
}

// Resolve turns the public ID a client uses into the internal one the
// commands take. An order at another venue is not found, so an ID says
// nothing about venues the caller cannot see.
func (s *Service) Resolve(ctx context.Context, venueID int64, publicID string) (int64, error) {
	row, err := s.db.Queries().OrderByPublicID(ctx, publicID)
	if err != nil || row.VenueID != venueID {
		if err == nil || isNoRows(err) {
			return 0, fmt.Errorf("%w: %q", ErrOrderNotFound, publicID)
		}
		return 0, fmt.Errorf("reading order %q: %w", publicID, err)
	}
	return row.ID, nil
}

// Order is an order's current state, if it is at the caller's venue.
func (s *Service) Order(ctx context.Context, venueID, orderID int64) (Result, error) {
	row, err := s.db.Queries().OrderByID(ctx, orderID)
	if err != nil || row.VenueID != venueID {
		if err == nil || isNoRows(err) {
			return Result{}, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
		}
		return Result{}, fmt.Errorf("reading order %d: %w", orderID, err)
	}
	v, err := loadVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return Result{}, err
	}
	state, err := s.db.LoadOrder(ctx, orderID, v.Taxes)
	if err != nil {
		return Result{}, err
	}
	return Result{OrderID: orderID, PublicID: row.PublicID, State: state}, nil
}

// maxOpenTickets caps the open-ticket list. A venue with more than this
// many orders open at once has a problem the list will not solve.
const maxOpenTickets = 200

// OpenTickets is every order at a venue that is not closed, and how long
// one may run before the till shows it as late.
type OpenTickets struct {
	Orders    []Result
	LateAfter time.Duration
}

// OpenOrders lists the venue's unclosed orders, oldest first, each
// projected as a read of it would be.
func (s *Service) OpenOrders(ctx context.Context, venueID int64) (OpenTickets, error) {
	v, err := loadVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return OpenTickets{}, err
	}
	rows, err := s.db.Queries().UnclosedOrdersForVenue(ctx, gen.UnclosedOrdersForVenueParams{VenueID: venueID, Limit: maxOpenTickets})
	if err != nil {
		return OpenTickets{}, fmt.Errorf("listing the open orders of venue %d: %w", venueID, err)
	}
	out := OpenTickets{Orders: make([]Result, 0, len(rows)), LateAfter: v.LateAfter}
	for _, r := range rows {
		state, err := s.db.LoadOrder(ctx, r.ID, v.Taxes)
		if err != nil {
			return OpenTickets{}, err
		}
		out.Orders = append(out.Orders, Result{OrderID: r.ID, PublicID: r.PublicID, State: state})
	}
	return out, nil
}

// History is an order's events and the names they need: every staff member
// and device the venue has had, by internal id, and the venue's time zone
// to show the times in. It is what the till's order timeline reads.
type History struct {
	Events   []order.Event
	Staff    map[int64]string
	Devices  map[int64]string
	TimeZone string
}

// OrderHistory reads an order's whole stream, in order.
func (s *Service) OrderHistory(ctx context.Context, venueID, orderID int64) (History, error) {
	row, err := s.db.Queries().OrderByID(ctx, orderID)
	if err != nil || row.VenueID != venueID {
		if err == nil || isNoRows(err) {
			return History{}, fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
		}
		return History{}, fmt.Errorf("reading order %d: %w", orderID, err)
	}
	venue, err := s.db.Queries().VenueByID(ctx, venueID)
	if err != nil {
		return History{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	events, err := s.db.OrderEvents(ctx, orderID, 0)
	if err != nil {
		return History{}, err
	}
	staff, err := s.db.Queries().StaffNamesForVenue(ctx, venueID)
	if err != nil {
		return History{}, fmt.Errorf("reading the staff of venue %d: %w", venueID, err)
	}
	devices, err := s.db.Queries().DeviceNamesForVenue(ctx, venueID)
	if err != nil {
		return History{}, fmt.Errorf("reading the devices of venue %d: %w", venueID, err)
	}
	h := History{Events: events, Staff: make(map[int64]string, len(staff)), Devices: make(map[int64]string, len(devices)), TimeZone: venue.Timezone}
	for _, st := range staff {
		h.Staff[st.ID] = st.Name
	}
	for _, d := range devices {
		h.Devices[d.ID] = d.Name
	}
	return h, nil
}
