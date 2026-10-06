package order

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// maxKeyLength matches the schema's check. A UUID is 36.
const maxKeyLength = 255

// checkKey accepts no key, or up to 255 printable ASCII characters.
func checkKey(key string) error {
	if len(key) > maxKeyLength {
		return fmt.Errorf("%w: idempotency key is %d characters, at most %d", ErrInvalidCommand, len(key), maxKeyLength)
	}
	if strings.IndexFunc(key, func(r rune) bool { return r > unicode.MaxASCII || !unicode.IsPrint(r) }) >= 0 {
		return fmt.Errorf("%w: idempotency key must be printable ASCII", ErrInvalidCommand)
	}
	return nil
}

// fingerprint identifies a request: the command, the order it is for, and
// its body. A key reused with a different fingerprint is refused.
func fingerprint(name string, orderID int64, cmd any) (string, error) {
	body, err := json.Marshal(cmd)
	if err != nil {
		return "", fmt.Errorf("%w: encoding %s: %v", ErrInvalidCommand, name, err)
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%d\x00", name, orderID)
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// priorResult is where an applied key's result is: a run of an order's
// events.
type priorResult struct {
	OrderID  int64
	FirstSeq int64
	LastSeq  int64
}

// System keys. A payment that lands from a gateway and a ticket opened for
// an order to collect are appended under keys of mez's own making, in the
// same table as the keys clients send, so that each happens once. These
// prefixes are the system's and a client may not send one: a client's key
// that collided would make the gateway's tender fail as a reuse, and the
// charge would stand open with nobody told why.
const (
	// ChargeKeyPrefix begins the key a gateway charge's tender is appended
	// under; the rest is the charge's public id.
	ChargeKeyPrefix = "charge:"
	// PickupTicketKeyPrefix begins the key an order to collect is opened
	// under; the rest is the round's public id.
	PickupTicketKeyPrefix = "pickup-ticket:"

	// MaxIdempotencyKeyLen bounds a client's key. A UUID is 36; nothing a
	// client has reason to send is near this.
	MaxIdempotencyKeyLen = 128
)

// ErrReservedIdempotencyKey is a client's key that is too long, not
// printable, or begins like one of the system's own.
var ErrReservedIdempotencyKey = errors.New("that idempotency key is not one a client may send")

// CheckClientIdempotencyKey says whether a key a client sent may be used.
// Empty is fine: it is no key at all.
func CheckClientIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}
	if len(key) > MaxIdempotencyKeyLen {
		return fmt.Errorf("%w: longer than %d characters", ErrReservedIdempotencyKey, MaxIdempotencyKeyLen)
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return fmt.Errorf("%w: it has a character that is not printable ASCII", ErrReservedIdempotencyKey)
		}
	}
	if strings.HasPrefix(key, ChargeKeyPrefix) || strings.HasPrefix(key, PickupTicketKeyPrefix) {
		return fmt.Errorf("%w: that prefix is the system's", ErrReservedIdempotencyKey)
	}
	return nil
}

// findPrior looks up the request's key inside the command's transaction. No
// key, or a key never used, is nil. A key used for another request is
// ErrIdempotencyKeyReused.
func findPrior(ctx context.Context, q *gen.Queries, meta Meta, fp string) (*priorResult, error) {
	if meta.IdempotencyKey == "" {
		return nil, nil
	}
	row, err := q.IdempotencyKey(ctx, gen.IdempotencyKeyParams{VenueID: meta.VenueID, Key: meta.IdempotencyKey})
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading idempotency key: %w", err)
	}
	if row.Fingerprint != fp {
		return nil, fmt.Errorf("%w: %q", ErrIdempotencyKeyReused, meta.IdempotencyKey)
	}
	return &priorResult{OrderID: row.OrderID, FirstSeq: row.FirstSeq, LastSeq: row.LastSeq}, nil
}

// saveKey records the request's key against the events it is appending, in
// the same transaction, so the key exists exactly when they do.
func saveKey(ctx context.Context, q *gen.Queries, meta Meta, fp string, orderID, first, last int64, at time.Time) error {
	if meta.IdempotencyKey == "" {
		return nil
	}
	err := q.SaveIdempotencyKey(ctx, gen.SaveIdempotencyKeyParams{
		VenueID: meta.VenueID, Key: meta.IdempotencyKey, Fingerprint: fp,
		OrderID: orderID, FirstSeq: first, LastSeq: last, CreatedAt: at,
	})
	if err != nil {
		return fmt.Errorf("saving idempotency key: %w", err)
	}
	return nil
}

// replay rebuilds the result a key's first request got: the events it
// appended, and the order projected up to the last of them.
//
// The projection uses the venue's tax regime as it is now. If the owner has
// changed it since, the replayed totals follow the change; the events, and
// any invoice's tax snapshot in them, do not.
func (s *Service) replay(ctx context.Context, v venue, p priorResult) (Result, error) {
	row, err := s.db.Queries().OrderByID(ctx, p.OrderID)
	if err != nil {
		return Result{}, fmt.Errorf("replaying order %d: %w", p.OrderID, err)
	}
	all, err := s.db.OrderEvents(ctx, p.OrderID, 0)
	if err != nil {
		return Result{}, err
	}
	if int64(len(all)) < p.LastSeq {
		return Result{}, fmt.Errorf("replaying order %d: key points at seq %d, the stream ends at %d", p.OrderID, p.LastSeq, len(all))
	}
	upTo := all[:p.LastSeq]
	state, err := order.Project(upTo, v.Taxes)
	if err != nil {
		return Result{}, fmt.Errorf("replaying order %d: %w", p.OrderID, err)
	}
	return Result{
		OrderID:  p.OrderID,
		PublicID: row.PublicID,
		State:    state,
		Events:   upTo[p.FirstSeq-1:],
		Replayed: true,
	}, nil
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
