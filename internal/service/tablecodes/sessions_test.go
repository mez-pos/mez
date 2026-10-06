package tablecodes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// seedSession writes a guest session at a table of venue 1, bound to no
// ticket, and returns its hash.
func seedSession(t *testing.T, d *db.DB, tableID int64, mark string) string {
	t.Helper()
	ctx := context.Background()
	hash := strings.Repeat(mark, 64)
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.CreateGuestSession(ctx, gen.CreateGuestSessionParams{
			PublicID: "gss_" + strings.ToUpper(strings.Repeat(mark, 8)), VenueID: 1, TableID: tableID, TokenHash: hash,
			CreatedAt: seededAt, ExpiresAt: seededAt.Add(12 * time.Hour),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func sessionEnded(t *testing.T, d *db.DB, hash string) bool {
	t.Helper()
	row, err := d.Queries().GuestSessionByTokenHash(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	return row.EndedAt.Valid
}

// TestANewCodeEndsTheTablesSessions: a code is renewed or turned off because
// it got out, and a session drawn with it is as good as the code was
// (ADR-262). Another table's sessions go on.
func TestANewCodeEndsTheTablesSessions(t *testing.T) {
	tests := []struct {
		name   string
		change func(s *Service) error
	}{
		{"renewed", func(s *Service) error { _, err := s.Renew(context.Background(), 1, 1); return err }},
		{"turned off", func(s *Service) error { return s.Revoke(context.Background(), 1, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := openDB(t)
			s := newService(t, d)
			if _, err := s.Renew(context.Background(), 1, 1); err != nil {
				t.Fatal(err)
			}
			here, there := seedSession(t, d, 1, "a"), seedSession(t, d, 2, "b")
			if err := tt.change(s); err != nil {
				t.Fatal(err)
			}
			if !sessionEnded(t, d, here) {
				t.Error("the table's session goes on with its code changed")
			}
			if sessionEnded(t, d, there) {
				t.Error("another table's session ended with this one's code")
			}
		})
	}
}
