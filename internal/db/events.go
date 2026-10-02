package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// maxStreamBatch bounds one read of the venue's stream. A device that has
// been off all day catches up in several passes rather than one that holds
// the whole day in memory.
const maxStreamBatch = 500

// StreamEvent is one event as the hub streams it: the event itself, its
// position in the venue's stream, and the little about its order that
// routing needs.
type StreamEvent struct {
	// ID is the position in the venue's stream. It only grows, so a client
	// that remembers the last one it saw can ask for everything after it.
	ID int64

	OrderID       int64
	OrderPublicID string
	OrderStatus   string
	TableID       int64

	Event order.Event
}

// VenueEventsAfter reads the venue's events after a position, oldest first.
// A limit above maxStreamBatch is capped rather than refused.
func (d *DB) VenueEventsAfter(ctx context.Context, venueID, afterID int64, limit int) ([]StreamEvent, error) {
	if limit <= 0 || limit > maxStreamBatch {
		limit = maxStreamBatch
	}
	rows, err := d.Queries().VenueEventsAfter(ctx, gen.VenueEventsAfterParams{
		VenueID: venueID, ID: afterID, Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("db: reading the stream of venue %d: %w", venueID, err)
	}
	out := make([]StreamEvent, len(rows))
	for i, r := range rows {
		p, err := order.DecodePayload(order.Type(r.Type), json.RawMessage(r.Payload))
		if err != nil {
			return nil, fmt.Errorf("db: event %d of order %d: %w", r.ID, r.OrderID, err)
		}
		out[i] = StreamEvent{
			ID: r.ID, OrderID: r.OrderID, OrderPublicID: r.OrderPublicID,
			OrderStatus: r.OrderStatus, TableID: r.TableID,
			Event: order.Event{
				Seq: r.Seq, Type: order.Type(r.Type), At: r.At.UTC(),
				ActorID: r.ActorID, DeviceID: r.DeviceID, Payload: p,
			},
		}
	}
	return out, nil
}

// VenueLastEventID is where the venue's stream has reached. A client with no
// position of its own starts here and sees only what happens next.
func (d *DB) VenueLastEventID(ctx context.Context, venueID int64) (int64, error) {
	last, err := d.Queries().VenueLastEventID(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("db: reading the stream position of venue %d: %w", venueID, err)
	}
	return last, nil
}
