package order

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
)

// A guest's phone's session for the sitting (ADR-262). A table's code says
// which table and never expires, so it is not enough to order to the
// kitchen on whoever sits there now or to read their bill. The phone holds
// a session as well:
//
//   - handed out by the landing while the table has no ticket, bound to no
//     ticket yet, and good only while the table still has none;
//   - bound to the ticket the round it was placed with went onto, when staff
//     accept that round or it is paid for;
//   - made for a phone that had none when staff accept its round, and told to
//     it by tracking that round, until it first presents it;
//   - over when its ticket closes, when the table's code is renewed or turned
//     off, or GuestSessionTTL after it was made.

// ErrGuestSession means a phone presented no session that is good for the
// table it asks about: none, one that was never made, another table's, or
// one that has ended, expired or belongs to a sitting that is over.
var ErrGuestSession = errors.New("no good guest session for this table")

const (
	// GuestSessionTTL is how long a session lasts at the most: a long
	// lunch and then some. A ticket that closes ends it sooner.
	GuestSessionTTL = 12 * time.Hour

	// guestSessionPrefix starts every session's token, so one pasted
	// somewhere says what it is.
	guestSessionPrefix = "gst_"

	// guestSessionBytes is a token's entropy: 256 bits.
	guestSessionBytes = 32

	// guestSessionIDPrefix is a session's public id, for the log: gss_...
	guestSessionIDPrefix = "gss"

	// guestSessionKeep is how long a lapsed session is kept before the
	// next one at its table clears it away.
	guestSessionKeep = 24 * time.Hour

	// maxSessionDraws bounds the redraw when a session's public id collides.
	maxSessionDraws = 4
)

// GuestSession is a phone's session as checked. OrderID is the ticket it is
// bound to, or 0 while it is bound to none.
type GuestSession struct {
	ID       int64
	PublicID string
	TableID  int64
	OrderID  int64
}

// drawnSession is a session's secrets, drawn before a transaction that may
// run more than once.
type drawnSession struct {
	token, hash, publicID string
}

// drawSession draws a token from crypto/rand, whatever the service's own
// source: a token anyone could predict is a bill anyone could read.
func drawSession() (drawnSession, error) {
	var b [guestSessionBytes]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return drawnSession{}, fmt.Errorf("drawing a guest session: %w", err)
	}
	token := guestSessionPrefix + base64.RawURLEncoding.EncodeToString(b[:])
	publicID, err := id.NewPublic(guestSessionIDPrefix)
	if err != nil {
		return drawnSession{}, fmt.Errorf("drawing a guest session's id: %w", err)
	}
	return drawnSession{token: token, hash: hashToken(token), publicID: publicID}, nil
}

// guestSessionHash checks a presented token's shape and returns its hash.
// One of the wrong shape never reaches the database.
func guestSessionHash(token string) (string, bool) {
	body, ok := strings.CutPrefix(token, guestSessionPrefix)
	if !ok {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) != guestSessionBytes {
		return "", false
	}
	return hashToken(token), true
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// OpenGuestSession hands a phone a session at a table with no ticket open,
// bound to none yet. At a table with a ticket it hands out nothing and
// answers false: whoever has the code is not thereby one of the party, and
// a phone that is joins by a round staff accept.
func (s *Service) OpenGuestSession(ctx context.Context, venueID, tableID int64) (string, bool, error) {
	if _, err := s.ticketOf(ctx, s.db.Queries(), venueID, tableID); err == nil {
		return "", false, nil
	} else if !isNoRows(err) {
		return "", false, err
	}
	at := s.clock.Now().UTC()
	for range maxSessionDraws {
		drawn, err := drawSession()
		if err != nil {
			return "", false, err
		}
		issued := false
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			// Looked at again inside: a ticket opened since the look is a
			// sitting this phone has not been let into.
			if _, err := s.ticketOf(ctx, q, venueID, tableID); err == nil {
				return nil
			} else if !isNoRows(err) {
				return err
			}
			if err := q.PruneGuestSessions(ctx, gen.PruneGuestSessionsParams{TableID: tableID, Before: at.Add(-guestSessionKeep)}); err != nil {
				return fmt.Errorf("clearing table %d's lapsed sessions: %w", tableID, err)
			}
			issued = true
			return createSession(ctx, q, venueID, tableID, 0, 0, drawn, false, at)
		})
		switch {
		case err == nil:
			return drawn.token, issued, nil
		case db.IsUniqueViolation(err):
			continue
		default:
			return "", false, err
		}
	}
	return "", false, fmt.Errorf("opening a guest session at table %d: every id drawn was taken", tableID)
}

// GuestSession checks a presented token for a table. It is good when it is
// that table's at that venue, has not ended or expired, and is either bound
// to no ticket while the table has none, or bound to the ticket open there
// now. A session a phone was handed by tracking stops being kept in clear
// the first time it is presented.
func (s *Service) GuestSession(ctx context.Context, venueID, tableID int64, token string) (GuestSession, error) {
	refuse := func(why string) (GuestSession, error) {
		return GuestSession{}, fmt.Errorf("%w: table %d: %s", ErrGuestSession, tableID, why)
	}
	hash, ok := guestSessionHash(token)
	if !ok {
		return refuse("not a session")
	}
	q := s.db.Queries()
	row, err := q.GuestSessionByTokenHash(ctx, hash)
	if isNoRows(err) {
		return refuse("no such session")
	}
	if err != nil {
		return GuestSession{}, fmt.Errorf("reading a guest session: %w", err)
	}
	if row.VenueID != venueID || row.TableID != tableID {
		return refuse("another table's session")
	}
	if row.EndedAt.Valid {
		return refuse("ended")
	}
	if !s.clock.Now().Before(row.ExpiresAt) {
		return refuse("expired")
	}
	ticket, err := s.ticketOf(ctx, q, venueID, tableID)
	if err != nil && !isNoRows(err) {
		return GuestSession{}, err
	}
	if ticket != row.OrderID {
		// Bound to none while the table has a ticket, or to a ticket that
		// is not the one open here now.
		return refuse("not this sitting's")
	}
	if row.Unclaimed == 1 {
		if err := s.db.WriteTx(ctx, func(q *gen.Queries) error { return q.ClaimGuestSession(ctx, row.ID) }); err != nil {
			return GuestSession{}, fmt.Errorf("claiming guest session %s: %w", row.PublicID, err)
		}
	}
	return GuestSession{ID: row.ID, PublicID: row.PublicID, TableID: row.TableID, OrderID: row.OrderID}, nil
}

// ticketOf is the ticket open at a table, or sql.ErrNoRows.
func (s *Service) ticketOf(ctx context.Context, q *gen.Queries, venueID, tableID int64) (int64, error) {
	orderID, err := q.UnclosedOrderAtTable(ctx, gen.UnclosedOrderAtTableParams{TableID: tableID, VenueID: venueID})
	if err != nil && !isNoRows(err) {
		return 0, fmt.Errorf("reading the ticket at table %d: %w", tableID, err)
	}
	return orderID, err
}

// seatSession lets a round's phone into the sitting its round has just
// joined: the session it was placed with is bound to the ticket, or when it
// had none, or that one can no longer be bound (ended, expired, another
// sitting's), the phone is made one bound to the ticket, kept in clear
// until it presents it. It runs in the transaction that puts the round on
// the ticket. fresh is drawn beforehand, and used only if needed.
func seatSession(ctx context.Context, q *gen.Queries, venueID, tableID, roundID, sessionID, orderID int64, fresh drawnSession, at time.Time) error {
	if sessionID != 0 {
		n, err := q.BindGuestSession(ctx, gen.BindGuestSessionParams{
			OrderID: sql.NullInt64{Int64: orderID, Valid: true}, ID: sessionID, VenueID: venueID, TableID: tableID, Now: at,
		})
		if err != nil {
			return fmt.Errorf("binding a guest session to order %d: %w", orderID, err)
		}
		if n == 1 {
			return nil
		}
	}
	return createSession(ctx, q, venueID, tableID, orderID, roundID, fresh, true, at)
}

// createSession writes a session. A handed-off one keeps its token in clear
// for the round that made it.
func createSession(ctx context.Context, q *gen.Queries, venueID, tableID, orderID, roundID int64, drawn drawnSession, handoff bool, at time.Time) error {
	err := q.CreateGuestSession(ctx, gen.CreateGuestSessionParams{
		PublicID: drawn.publicID, VenueID: venueID, TableID: tableID, OrderID: orderID, TokenHash: drawn.hash,
		RoundID: roundID, Handoff: sql.NullString{String: drawn.token, Valid: handoff},
		CreatedAt: at, ExpiresAt: at.Add(GuestSessionTTL),
	})
	if err != nil {
		return fmt.Errorf("making a guest session at table %d: %w", tableID, err)
	}
	return nil
}

// endSessions ends the sessions of a ticket that has closed, and those
// handed out at its tables and never bound: the sitting is over. It runs in
// the transaction that closes the ticket.
func endSessions(ctx context.Context, q *gen.Queries, orderID int64, tables []int64, at time.Time) error {
	ended := sql.NullTime{Time: at, Valid: true}
	if err := q.EndGuestSessionsOfOrder(ctx, gen.EndGuestSessionsOfOrderParams{EndedAt: ended, OrderID: sql.NullInt64{Int64: orderID, Valid: true}}); err != nil {
		return fmt.Errorf("ending order %d's guest sessions: %w", orderID, err)
	}
	for _, t := range tables {
		if err := q.EndUnboundGuestSessionsAtTable(ctx, gen.EndUnboundGuestSessionsAtTableParams{EndedAt: ended, TableID: t}); err != nil {
			return fmt.Errorf("ending table %d's guest sessions: %w", t, err)
		}
	}
	return nil
}

// handedSession is the session made for a round when it was accepted, in
// clear, while its phone has still to present it; empty otherwise.
func (s *Service) handedSession(ctx context.Context, roundID int64) (string, error) {
	token, err := s.db.Queries().GuestSessionHandoff(ctx, gen.GuestSessionHandoffParams{
		RoundID: sql.NullInt64{Int64: roundID, Valid: true}, Now: s.clock.Now().UTC(),
	})
	if isNoRows(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading the session handed to round %d: %w", roundID, err)
	}
	return token, nil
}
