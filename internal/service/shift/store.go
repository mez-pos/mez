package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
)

// EventType is a line in a shift's history.
type EventType string

// The shift's events, as shift_events stores them.
const (
	EventOpened  EventType = "opened"
	EventCashIn  EventType = EventType(shift.CashIn)
	EventCashOut EventType = EventType(shift.CashOut)
	EventClosed  EventType = "closed"
)

// EventTypes is every event type, for the generated vocabulary.
func EventTypes() []EventType {
	return []EventType{EventOpened, EventCashIn, EventCashOut, EventClosed}
}

func eventType(k shift.MovementKind) EventType { return EventType(k) }

// event is a row about to be appended, and what a retry is compared on.
type event struct {
	Type         EventType
	Amount       int64
	Reason       string
	AuthorisedBy int64
}

// venue is what the shift needs to know of the venue.
type venue struct {
	currency string
	location *time.Location
}

func venueOf(ctx context.Context, q *gen.Queries, venueID int64) (venue, error) {
	row, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return venue{}, fmt.Errorf("%w: no venue %d", ErrVenueConfig, venueID)
	}
	if err != nil {
		return venue{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return venue{}, fmt.Errorf("%w: timezone %q: %v", ErrVenueConfig, row.Timezone, err)
	}
	return venue{currency: row.Currency, location: loc}, nil
}

// isRetry reports whether the request's key already recorded an event in
// this shift. The same key for a different request is refused.
func isRetry(ctx context.Context, q *gen.Queries, shiftID int64, meta Meta, want event) (bool, error) {
	if meta.IdempotencyKey == "" {
		return false, nil
	}
	row, err := q.ShiftEventByKey(ctx, gen.ShiftEventByKeyParams{ShiftID: shiftID, IdempotencyKey: meta.IdempotencyKey})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading idempotency key: %w", err)
	}
	got := event{Type: EventType(row.Type), Amount: row.Amount, Reason: row.Reason, AuthorisedBy: row.AuthorisedBy}
	if got != want || row.ActorID != meta.ActorID {
		return false, fmt.Errorf("%w: %q", ErrIdempotencyKeyReused, meta.IdempotencyKey)
	}
	return true, nil
}

func appendEvent(ctx context.Context, q *gen.Queries, shiftID int64, meta Meta, e event, at time.Time) error {
	_, err := q.AppendShiftEvent(ctx, gen.AppendShiftEventParams{
		ShiftID: shiftID, Type: string(e.Type), Amount: e.Amount, Reason: e.Reason,
		ActorID: meta.ActorID, AuthorisedBy: e.AuthorisedBy, DeviceID: meta.DeviceID,
		IdempotencyKey: meta.IdempotencyKey, At: at,
	})
	if err != nil {
		return fmt.Errorf("recording %s: %w", e.Type, err)
	}
	return nil
}

func saveCount(ctx context.Context, q *gen.Queries, shiftID int64, c shift.Count, by int64, at time.Time) error {
	denominations := c.Denominations
	if denominations == nil {
		denominations = []shift.Denomination{}
	}
	body, err := json.Marshal(denominations)
	if err != nil {
		return fmt.Errorf("encoding the %s count: %w", c.Kind, err)
	}
	err = q.CreateDrawerCount(ctx, gen.CreateDrawerCountParams{
		ShiftID: shiftID, Kind: string(c.Kind), Denominations: string(body), Total: c.Total.Minor, CountedBy: by, CountedAt: at,
	})
	if err != nil {
		return fmt.Errorf("recording the %s count: %w", c.Kind, err)
	}
	return nil
}

// Event is a line of a shift's history, as the back office shows it.
type Event struct {
	ID           int64        `json:"id"`
	Type         EventType    `json:"type"`
	Amount       money.Amount `json:"amount"`
	Reason       string       `json:"reason"`
	ActorID      int64        `json:"actor_id"`
	AuthorisedBy int64        `json:"authorised_by"`
	DeviceID     int64        `json:"device_id"`
	At           time.Time    `json:"at"`
}
