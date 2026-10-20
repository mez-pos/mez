// Package tablecodes makes and reads the codes on a venue's tables (phase 11,
// ADR-206). A code is what a table's QR sticker says and what a guest's phone
// presents: it leads to the venue and the table. It is not enough alone to
// order or read a bill: a phone holds a session for the sitting (ADR-262).
package tablecodes

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
)

// ErrUnknown is a code that leads nowhere: never made, revoked, or for a
// table since taken off the floor. A guest is told the same thing for all
// three.
var ErrUnknown = errors.New("tablecodes: no table has this code")

const (
	// Length is how many characters a code has.
	Length = 12

	// alphabet is what a code is drawn from: lower case letters and digits,
	// without the ones read as each other (0 and o, 1 and l and i). A code
	// is scanned far more often than it is typed, but a guest whose camera
	// will not focus reads it off the sticker, and staff read it out.
	alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
)

// Table is what a code leads to.
type Table struct {
	VenueID   int64
	VenueName string
	Currency  string
	TableID   int64
	TableName string
}

// Coded is a table on the floor with its live code, which is empty when it
// has none.
type Coded struct {
	TableID   int64
	TableName string
	Code      string
}

// Service makes and reads table codes.
type Service struct {
	db     *db.DB
	now    func() time.Time
	random io.Reader
}

// New returns a Service. now and random are the clock and the source of
// codes; nil means time.Now and crypto/rand.
func New(d *db.DB, now func() time.Time, random io.Reader) (*Service, error) {
	if d == nil {
		return nil, errors.New("tablecodes: a database is required")
	}
	if now == nil {
		now = time.Now
	}
	if random == nil {
		random = rand.Reader
	}
	return &Service{db: d, now: now, random: random}, nil
}

// Resolve reads what a code leads to.
func (s *Service) Resolve(ctx context.Context, code string) (Table, error) {
	// Nothing that is not a code reaches the database: the code is a
	// stranger's input, and most wrong ones are not even the right shape.
	if !WellFormed(code) {
		return Table{}, ErrUnknown
	}
	row, err := s.db.Queries().TableByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return Table{}, ErrUnknown
	}
	if err != nil {
		return Table{}, fmt.Errorf("tablecodes: reading a code: %w", err)
	}
	return Table(row), nil
}

// List is every table on the venue's floor with its live code.
func (s *Service) List(ctx context.Context, venueID int64) ([]Coded, error) {
	rows, err := s.db.Queries().TableCodesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("tablecodes: listing codes: %w", err)
	}
	out := make([]Coded, len(rows))
	for i, r := range rows {
		out[i] = Coded(r)
	}
	return out, nil
}

// ErrNoTable is a table that is not on the venue's floor: not there, another
// venue's, or taken off the floor.
var ErrNoTable = errors.New("tablecodes: no such table on this venue's floor")

// onFloor refuses a table that is not the venue's own and in service. A code
// for another venue's table would lead a guest of this one to that one.
func onFloor(ctx context.Context, q *gen.Queries, venueID, tableID int64) error {
	t, err := q.TableByID(ctx, tableID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (t.VenueID != venueID || !t.Active)) {
		return fmt.Errorf("%w: table %d", ErrNoTable, tableID)
	}
	return err
}

// Renew gives a table a new code and turns off the one it had, in one
// transaction: there is never a moment with two live codes, or with none
// after there was one.
func (s *Service) Renew(ctx context.Context, venueID, tableID int64) (string, error) {
	at := s.now().UTC()
	for range maxPickupDraws {
		code, err := s.draw()
		if err != nil {
			return "", err
		}
		taken := false
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			if err := onFloor(ctx, q, venueID, tableID); err != nil {
				return err
			}
			// Never a string that is, or was, any table's or a venue's
			// pickup code (ADR-247): what a revoked code said must go on
			// leading nowhere. With 59 bits to a code it does not happen;
			// if it does, the next draw is another code, as RenewPickup
			// does.
			tables, err := q.CodeIsATables(ctx, code)
			if err != nil {
				return err
			}
			pickups, err := q.CodeIsAPickups(ctx, code)
			if err != nil {
				return err
			}
			if tables+pickups > 0 {
				taken = true
				return nil
			}
			if err := revoke(ctx, q, venueID, tableID, at); err != nil {
				return err
			}
			return q.CreateTableCode(ctx, gen.CreateTableCodeParams{VenueID: venueID, TableID: tableID, Code: code, CreatedAt: at})
		})
		if err != nil {
			return "", fmt.Errorf("tablecodes: renewing table %d's code: %w", tableID, err)
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("tablecodes: renewing table %d's code: every code drawn was already used", tableID)
}

// Revoke turns off a table's code and gives it no other (phase 11, Task 6a,
// ADR-243): the sticker on the table leads nowhere, and so does every photo
// of it, until the table is given a code again. A table with no code is left
// as it is.
func (s *Service) Revoke(ctx context.Context, venueID, tableID int64) error {
	at := s.now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		if err := onFloor(ctx, q, venueID, tableID); err != nil {
			return err
		}
		return revoke(ctx, q, venueID, tableID, at)
	})
	if err != nil {
		return fmt.Errorf("tablecodes: turning off table %d's code: %w", tableID, err)
	}
	return nil
}

// revoke turns off a table's code and ends every guest session it gave
// (ADR-262): a code is renewed or turned off because it got out, and a
// session drawn with it is as good as the code was.
func revoke(ctx context.Context, q *gen.Queries, venueID, tableID int64, at time.Time) error {
	if err := q.RevokeTableCodes(ctx, gen.RevokeTableCodesParams{
		RevokedAt: sql.NullTime{Time: at, Valid: true}, VenueID: venueID, TableID: tableID,
	}); err != nil {
		return err
	}
	return q.EndGuestSessionsAtTable(ctx, gen.EndGuestSessionsAtTableParams{
		EndedAt: sql.NullTime{Time: at, Valid: true}, VenueID: venueID, TableID: tableID,
	})
}

// Ensure gives every table on the floor that has no code one, and returns
// the list as it then stands. Tables that have a code keep it: a sticker
// already on a table goes on working.
func (s *Service) Ensure(ctx context.Context, venueID int64) ([]Coded, error) {
	tables, err := s.List(ctx, venueID)
	if err != nil {
		return nil, err
	}
	out := make([]Coded, len(tables))
	for i, t := range tables {
		if t.Code == "" {
			if t.Code, err = s.Renew(ctx, venueID, t.TableID); err != nil {
				return nil, err
			}
		}
		out[i] = t
	}
	return out, nil
}

// draw makes a code. Twelve characters from thirty-one is about 59 bits,
// drawn without bias: a guess at a code is a request to the venue, and the
// venue limits those.
func (s *Service) draw() (string, error) {
	out := make([]byte, 0, Length)
	buf := make([]byte, 1)
	// The largest multiple of the alphabet that fits a byte. A byte at or
	// over it is thrown away, or the first few letters would come up more
	// often than the rest.
	limit := byte(256 - 256%len(alphabet))
	for len(out) < Length {
		if _, err := io.ReadFull(s.random, buf); err != nil {
			return "", fmt.Errorf("tablecodes: drawing a code: %w", err)
		}
		if buf[0] >= limit {
			continue
		}
		out = append(out, alphabet[int(buf[0])%len(alphabet)])
	}
	return string(out), nil
}

// WellFormed reports whether s has the shape of a code: the right length,
// from the alphabet.
func WellFormed(s string) bool {
	if len(s) != Length {
		return false
	}
	for i := range len(s) {
		if !inAlphabet(s[i]) {
			return false
		}
	}
	return true
}

func inAlphabet(c byte) bool {
	for i := range len(alphabet) {
		if alphabet[i] == c {
			return true
		}
	}
	return false
}
