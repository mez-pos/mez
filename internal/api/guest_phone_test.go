package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A guest's phone in these tests is one of the table's party (ADR-262): it
// sends a session good for the table as it stands. A test of a phone with
// no session, or the wrong one, calls send itself.

// phoneSend asks as a phone of the party at the table whose code is in the
// path does.
func (g *guestWorld) phoneSend(method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	h := map[string]string{}
	for k, v := range header {
		h[k] = v
	}
	if g.sessionFor != nil {
		if code := tableCodeIn(path); code != "" {
			if token := g.sessionFor(code); token != "" {
				h[HeaderGuestSession] = token
			}
		}
	}
	return send(g.handler, method, path, body, h)
}

// tableCodeIn is the table's code in a guest path, or empty.
func tableCodeIn(path string) string {
	rest, ok := strings.CutPrefix(path, "/order/api/t/")
	if !ok {
		return ""
	}
	code, _, _ := strings.Cut(rest, "/")
	code, _, _ = strings.Cut(code, "?")
	return code
}

// withPhones makes the world's phones of the party: each table's phone
// keeps a session, and when it is no longer good gets another, from the
// landing at a table with no ticket, or bound to the ticket open there, as
// accepting its first order would have made.
func withPhones(t *testing.T, o *orderingWorld) *orderingWorld {
	sessions := map[string]string{}
	o.sessionFor = func(code string) string {
		t.Helper()
		ctx := context.Background()
		table, err := o.codes.Resolve(ctx, code)
		if err != nil {
			return ""
		}
		if token := sessions[code]; token != "" {
			if _, err := o.orders.GuestSession(ctx, table.VenueID, table.TableID, token); err == nil {
				return token
			}
		}
		token, issued, err := o.orders.OpenGuestSession(ctx, table.VenueID, table.TableID)
		if err != nil {
			t.Fatalf("a session at table %d: %v", table.TableID, err)
		}
		if !issued {
			res, open, err := o.orders.TableTicket(ctx, table.VenueID, table.TableID)
			if err != nil || !open {
				t.Fatalf("no ticket at table %d to bind a session to: %v", table.TableID, err)
			}
			token = boundGuestSession(t, o.db, table.VenueID, table.TableID, res.OrderID)
		}
		sessions[code] = token
		return token
	}
	return o
}

// boundGuestSession writes a session bound to a ticket, behind the API's
// back, and returns its token.
func boundGuestSession(t *testing.T, d *db.DB, venueID, tableID, orderID int64) string {
	t.Helper()
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	token := "gst_" + base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(token))
	publicID, err := id.NewPublic("gss")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	at := time.Now().UTC()
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.CreateGuestSession(ctx, gen.CreateGuestSessionParams{
			PublicID: publicID, VenueID: venueID, TableID: tableID, OrderID: orderID, TokenHash: hex.EncodeToString(sum[:]),
			CreatedAt: at, ExpiresAt: at.Add(svcorder.GuestSessionTTL),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return token
}
