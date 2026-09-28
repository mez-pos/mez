// Package shift keeps a venue's shifts (phase 09, Task 1a): opening one
// with a counted float, cash moved in and out of the drawer with a reason
// and a manager's authorisation, and closing it. The rules are
// internal/domain/shift; this is where they are stored, each change in one
// transaction.
//
// One venue, one drawer, one open shift at a time. Orders and payments are
// attached to the open shift in Task 1b.
package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/id"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
)

// The refusals. The API maps them to codes.
var (
	ErrInvalidCommand       = errors.New("invalid shift command")
	ErrNoOpenShift          = shift.ErrNoOpenShift
	ErrShiftOpen            = errors.New("a shift is already open")
	ErrShiftNotFound        = errors.New("no such shift at this venue")
	ErrShiftClosed          = errors.New("the shift is closed")
	ErrIdempotencyKeyReused = errors.New("idempotency key already used for a different request")
	ErrVenueConfig          = errors.New("the venue is not configured for shifts")
)

// publicPrefix is what a shift's public ID starts with: shf_7K3M9QX2.
const publicPrefix = "shf"

// maxKeyLength matches the schema's check. A UUID is 36.
const maxKeyLength = 255

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options configure a Service. DB is required.
type Options struct {
	DB    *db.DB
	Clock Clock
	// DayStart is when the venue's trading day begins, local time. Zero
	// means shift.DefaultDayStart; there is no venue setting for it yet.
	DayStart time.Duration
	// OnChange is told a venue's shift changed, after it is committed.
	OnChange func(venueID int64)
}

// Service is a venue's shifts.
type Service struct {
	db       *db.DB
	clock    Clock
	dayStart time.Duration
	onChange func(venueID int64)
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil {
		return nil, errors.New("shift: a database is required")
	}
	if o.Clock == nil {
		o.Clock = systemClock{}
	}
	if o.DayStart == 0 {
		o.DayStart = shift.DefaultDayStart
	}
	if o.DayStart < 0 || o.DayStart >= 24*time.Hour {
		return nil, fmt.Errorf("shift: %w: %s", shift.ErrInvalidDayStart, o.DayStart)
	}
	if o.OnChange == nil {
		o.OnChange = func(int64) {}
	}
	return &Service{db: o.DB, clock: o.Clock, dayStart: o.DayStart, onChange: o.OnChange}, nil
}

// Meta is who is doing it, where, and the request's idempotency key.
type Meta struct {
	VenueID        int64
	ActorID        int64 // the signed-in staff member; a shift always has one
	DeviceID       int64
	IdempotencyKey string
}

func (m Meta) validate() error {
	switch {
	case m.VenueID < 1:
		return fmt.Errorf("%w: no venue", ErrInvalidCommand)
	case m.ActorID < 1:
		return fmt.Errorf("%w: a shift needs a staff member", ErrInvalidCommand)
	case m.DeviceID < 0:
		return fmt.Errorf("%w: negative device", ErrInvalidCommand)
	case len(m.IdempotencyKey) > maxKeyLength:
		return fmt.Errorf("%w: idempotency key is %d characters, at most %d", ErrInvalidCommand, len(m.IdempotencyKey), maxKeyLength)
	case strings.IndexFunc(m.IdempotencyKey, func(r rune) bool { return r > unicode.MaxASCII || !unicode.IsPrint(r) }) >= 0:
		return fmt.Errorf("%w: idempotency key must be printable ASCII", ErrInvalidCommand)
	}
	return nil
}

// Open opens the venue's shift with the float counted into the drawer.
// With a shift already open it is refused, unless this is a retry of the
// request that opened it, which gets that shift back.
func (s *Service) Open(ctx context.Context, meta Meta, float shift.Count) (Shift, error) {
	if err := meta.validate(); err != nil {
		return Shift{}, err
	}
	if float.Kind != shift.CountOpening {
		return Shift{}, fmt.Errorf("%w: a shift opens with an opening count, not a %s count", ErrInvalidCommand, float.Kind)
	}
	now := s.clock.Now().UTC()
	var shiftID int64
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		if err := float.Validate(v.currency); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
		}
		open, err := q.OpenShiftForVenue(ctx, meta.VenueID)
		switch {
		case err == nil:
			retry, err := isRetry(ctx, q, open.ID, meta, event{Type: EventOpened, Amount: float.Total.Minor})
			if err != nil {
				return err
			}
			if !retry {
				return fmt.Errorf("%w: %s since %s", ErrShiftOpen, open.PublicID, open.OpenedAt.UTC().Format(time.RFC3339))
			}
			shiftID = open.ID
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("reading the open shift: %w", err)
		}
		day, err := shift.TradingDay(now, v.location, s.dayStart)
		if err != nil {
			return err
		}
		if shiftID, err = createShift(ctx, q, meta, day, now); err != nil {
			return err
		}
		if err := saveCount(ctx, q, shiftID, float, meta.ActorID, now); err != nil {
			return err
		}
		return appendEvent(ctx, q, shiftID, meta, event{Type: EventOpened, Amount: float.Total.Minor}, now)
	})
	if err != nil {
		return Shift{}, fmt.Errorf("opening a shift: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.ByID(ctx, meta.VenueID, shiftID)
}

// createShift inserts the shift row, redrawing the public ID on the
// one-in-a-trillion collision. Losing the race for the one open shift is
// ErrShiftOpen.
func createShift(ctx context.Context, q *gen.Queries, meta Meta, day string, now time.Time) (int64, error) {
	for range 3 {
		publicID, err := id.NewPublic(publicPrefix)
		if err != nil {
			return 0, fmt.Errorf("drawing a public ID: %w", err)
		}
		shiftID, err := q.CreateShift(ctx, gen.CreateShiftParams{
			PublicID: publicID, VenueID: meta.VenueID, TradingDay: day, OpenedBy: meta.ActorID, OpenedAt: now,
		})
		if err == nil {
			return shiftID, nil
		}
		if !db.IsUniqueViolation(err) {
			return 0, fmt.Errorf("creating the shift: %w", err)
		}
		if _, err := q.OpenShiftForVenue(ctx, meta.VenueID); err == nil {
			return 0, ErrShiftOpen
		}
	}
	return 0, errors.New("creating the shift: three public ID collisions in a row")
}

// MoveCash records cash put into or taken out of an open shift's drawer.
// The movement names the manager who authorised it; checking that they can
// is the API's (Task 1b), as it is for a void.
func (s *Service) MoveCash(ctx context.Context, meta Meta, shiftID int64, m shift.Movement) (Shift, error) {
	if err := meta.validate(); err != nil {
		return Shift{}, err
	}
	now := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		if err := m.Validate(v.currency); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
		}
		e := event{Type: eventType(m.Kind), Amount: m.Amount.Minor, Reason: m.Reason, AuthorisedBy: m.AuthorisedBy}
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		if retry, err := isRetry(ctx, q, row.ID, meta, e); err != nil || retry {
			return err
		}
		if shift.Status(row.Status) != shift.StatusOpen {
			return fmt.Errorf("%w: %s", ErrShiftClosed, row.PublicID)
		}
		switch _, err := closingOf(ctx, q, row.ID); {
		case err == nil:
			return fmt.Errorf("%w: %s", ErrDrawerCounted, row.PublicID)
		case !errors.Is(err, ErrNotCounted):
			return err
		}
		return appendEvent(ctx, q, row.ID, meta, e, now)
	})
	if err != nil {
		return Shift{}, fmt.Errorf("moving cash: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.ByID(ctx, meta.VenueID, shiftID)
}

// Close closes an open shift. The drawer must be counted first, and a
// variance past the threshold explained (ADR-095).
func (s *Service) Close(ctx context.Context, meta Meta, shiftID int64) (Shift, error) {
	if err := meta.validate(); err != nil {
		return Shift{}, err
	}
	now := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		if retry, err := isRetry(ctx, q, row.ID, meta, event{Type: EventClosed}); err != nil || retry {
			return err
		}
		if _, err := shift.Transition(shift.Status(row.Status), shift.StatusClosed); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrShiftClosed, row.PublicID, err)
		}
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		sh, err := load(ctx, q, row, v.currency)
		if err != nil {
			return err
		}
		if err := sh.ReadyToClose(); err != nil {
			return err
		}
		n, err := q.CloseShift(ctx, gen.CloseShiftParams{ID: row.ID, ClosedBy: meta.ActorID, ClosedAt: sql.NullTime{Time: now, Valid: true}})
		if err != nil {
			return fmt.Errorf("closing: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("%w: %s", ErrShiftClosed, row.PublicID)
		}
		return appendEvent(ctx, q, row.ID, meta, event{Type: EventClosed}, now)
	})
	if err != nil {
		return Shift{}, fmt.Errorf("closing the shift: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.ByID(ctx, meta.VenueID, shiftID)
}

// shiftAt reads a shift inside a transaction, refusing one of another
// venue as if it did not exist.
func shiftAt(ctx context.Context, q *gen.Queries, venueID, shiftID int64) (gen.Shift, error) {
	row, err := q.ShiftByID(ctx, shiftID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.VenueID != venueID) {
		return gen.Shift{}, fmt.Errorf("%w: %d", ErrShiftNotFound, shiftID)
	}
	if err != nil {
		return gen.Shift{}, fmt.Errorf("reading shift %d: %w", shiftID, err)
	}
	return row, nil
}
