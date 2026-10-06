package order

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// partyPhone is a phone of the table's party: its rounds carry a session
// good for the table as it stands, which is what most tests of rounds want.
// A test of a phone with no session calls the service itself.
type partyPhone struct {
	t *testing.T
	s *Service
}

func party(t *testing.T, s *Service) partyPhone { return partyPhone{t: t, s: s} }

func (p partyPhone) PlaceRound(ctx context.Context, meta Meta, cmd PlaceRound) (Round, error) {
	if cmd.Session == "" {
		cmd.Session = partySession(p.t, p.s, meta.VenueID, cmd.TableID)
	}
	return p.s.PlaceRound(ctx, meta, cmd)
}

// partySession is a session for a phone of the table's party: the landing's
// at a table with no ticket, and at one with a ticket a session bound to it,
// as accepting the phone's first round would have made. Empty where there is
// no table to make one at: the round is then refused for what the test is
// about.
func partySession(t *testing.T, s *Service, venueID, tableID int64) string {
	t.Helper()
	ctx := context.Background()
	token, issued, err := s.OpenGuestSession(ctx, venueID, tableID)
	if err != nil {
		t.Logf("no session at table %d: %v", tableID, err)
		return ""
	}
	if issued {
		return token
	}
	return boundSession(t, s, venueID, tableID)
}

// boundSession makes a session bound to the ticket open at a table, behind
// the service's back.
func boundSession(t *testing.T, s *Service, venueID, tableID int64) string {
	t.Helper()
	ctx := context.Background()
	orderID, err := s.ticketOf(ctx, s.db.Queries(), venueID, tableID)
	if err != nil {
		t.Fatalf("no ticket at table %d to bind a session to: %v", tableID, err)
	}
	drawn, err := drawSession()
	if err != nil {
		t.Fatal(err)
	}
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		return createSession(ctx, q, venueID, tableID, orderID, 0, drawn, false, s.clock.Now().UTC())
	})
	if err != nil {
		t.Fatal(err)
	}
	return drawn.token
}

// landing is what the landing hands a phone at a table, and fails a test
// that expected one and got none.
func landing(t *testing.T, s *Service, tableID int64) string {
	t.Helper()
	token, issued, err := s.OpenGuestSession(context.Background(), 1, tableID)
	if err != nil || !issued {
		t.Fatalf("landing at table %d: issued %v, %v", tableID, issued, err)
	}
	return token
}

// good reports whether a session is good for a table now.
func good(s *Service, tableID int64, token string) bool {
	_, err := s.GuestSession(context.Background(), 1, tableID, token)
	return err == nil
}

func roundWith(g guestMenu, table int64, session string) PlaceRound {
	cmd := g.twoOatAndABrownie(table)
	cmd.Session = session
	return cmd
}

// TestASessionIsHandedOutOnlyAtAnEmptyTable: with no ticket the landing
// hands one out; with one open it does not, and a phone joins by a request.
func TestASessionIsHandedOutOnlyAtAnEmptyTable(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()

	token, issued, err := s.OpenGuestSession(ctx, 1, 4)
	if err != nil || !issued || len(token) != len(guestSessionPrefix)+43 || token[:4] != guestSessionPrefix {
		t.Fatalf("at an empty table: %q, %v, %v", token, issued, err)
	}
	if again, _, _ := s.OpenGuestSession(ctx, 1, 4); again == token {
		t.Error("two phones were handed the same session")
	}
	open(t, s) // table 4
	if token, issued, err := s.OpenGuestSession(ctx, 1, 4); err != nil || issued || token != "" {
		t.Errorf("at a seated table: %q, %v, %v", token, issued, err)
	}
	// The one handed out before the ticket opened is not of its party.
	if good(s, 4, token) {
		t.Error("a session from before the ticket opened is good for it")
	}
}

func TestASessionIsChecked(t *testing.T) {
	tests := []struct {
		name string
		// prepare returns the token to present at table 4.
		prepare func(t *testing.T, s *Service) string
		want    bool
	}{
		{"handed out at the empty table", func(t *testing.T, s *Service) string { return landing(t, s, 4) }, true},
		{"nothing at all", func(*testing.T, *Service) string { return "" }, false},
		{"not the shape of one", func(*testing.T, *Service) string { return "gst_short" }, false},
		{"a device's token", func(*testing.T, *Service) string { return "nfd_" + strings.Repeat("A", 43) }, false},
		{"the shape of one, never made", func(*testing.T, *Service) string {
			drawn, _ := drawSession()
			return drawn.token
		}, false},
		{"another table's", func(t *testing.T, s *Service) string { return landing(t, s, 5) }, false},
		{"bound to the ticket open here", func(t *testing.T, s *Service) string {
			open(t, s)
			return boundSession(t, s, 1, 4)
		}, true},
		{"handed out before the ticket opened", func(t *testing.T, s *Service) string {
			token := landing(t, s, 4)
			open(t, s)
			return token
		}, false},
		{"bound to a ticket that has closed", func(t *testing.T, s *Service) string {
			ctx := context.Background()
			id := open(t, s).OrderID
			token := boundSession(t, s, 1, 4)
			must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
			must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
			must(t)(s.Close(ctx, cashier(""), id))
			return token
		}, false},
		{"bound to another ticket at the table", func(t *testing.T, s *Service) string {
			open(t, s)
			token := boundSession(t, s, 1, 4)
			// The party moved to table 5, and another sat at 4.
			ctx := context.Background()
			id, _ := s.ticketOf(ctx, s.db.Queries(), 1, 4)
			must(t)(s.MoveTable(ctx, cashier(""), id, MoveTable{From: 4, To: 5}))
			open(t, s)
			return token
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newService(t)
			token := tt.prepare(t, s)
			got, err := s.GuestSession(context.Background(), 1, 4, token)
			if (err == nil) != tt.want {
				t.Fatalf("good = %v (%v), want %v", err == nil, err, tt.want)
			}
			if err != nil && !errors.Is(err, ErrGuestSession) {
				t.Errorf("err = %v, want ErrGuestSession", err)
			}
			if err == nil && (got.ID == 0 || got.TableID != 4 || got.PublicID == "") {
				t.Errorf("session = %+v", got)
			}
		})
	}
}

// TestASessionFromAnotherVenueIsRefused: the same table number elsewhere is
// another table.
func TestASessionFromAnotherVenueIsRefused(t *testing.T) {
	s, _, _ := newService(t)
	token := landing(t, s, 4)
	if _, err := s.GuestSession(context.Background(), 2, 4, token); !errors.Is(err, ErrGuestSession) {
		t.Errorf("err = %v, want ErrGuestSession", err)
	}
}

// TestASessionExpires: twelve hours, whatever the table does.
func TestASessionExpires(t *testing.T) {
	_, d, _ := newService(t)
	clock := &movedClock{at: now}
	s, err := New(Options{DB: d, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	token := landing(t, s, 4)
	clock.at = now.Add(GuestSessionTTL - time.Second)
	if !good(s, 4, token) {
		t.Fatal("a session is refused before it expires")
	}
	clock.at = now.Add(GuestSessionTTL)
	if good(s, 4, token) {
		t.Error("a session is good at its expiry")
	}
	// A lapsed session is cleared away by the next one handed out there,
	// once it has been lapsed a while.
	clock.at = now.Add(GuestSessionTTL + guestSessionKeep + time.Minute)
	landing(t, s, 4)
	hash, _ := guestSessionHash(token)
	if _, err := d.Queries().GuestSessionByTokenHash(context.Background(), hash); !isNoRows(err) {
		t.Errorf("the lapsed session is still kept: %v", err)
	}
}

// TestARoundBindsItsSession: the first phone's request opens the ticket
// when it is accepted, and its session is then the ticket's: its next round
// goes straight to the kitchen.
func TestARoundBindsItsSession(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	token := landing(t, w.s, 4)

	first, err := w.s.PlaceRound(ctx, guest("k1"), roundWith(w.g, 4, token))
	if err != nil || first.Status != RoundWaiting {
		t.Fatalf("first round = %+v, %v", first, err)
	}
	got, err := w.s.AcceptRound(ctx, cashier(""), first.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := w.s.GuestSession(ctx, 1, 4, token)
	if err != nil || session.OrderID != got.Round.OrderID {
		t.Fatalf("session = %+v, %v; want it bound to order %d", session, err, got.Round.OrderID)
	}
	// It was the phone's own session that was bound: none is handed over.
	if tracked, err := w.s.TrackRound(ctx, 1, 4, first.PublicID); err != nil || tracked.Session != "" {
		t.Errorf("tracked = %+v, %v; want no session handed over", tracked, err)
	}
	second, err := w.s.PlaceRound(ctx, guest("k2"), roundWith(w.g, 4, token))
	if err != nil || second.Status != RoundSent || second.OrderID != got.Round.OrderID {
		t.Errorf("second round = %+v, %v; want it sent on the ticket", second, err)
	}
}

// TestTwoPhonesBeforeTheTicketOpens: both are handed a session at the empty
// table, both ask, and each is bound when its own request is accepted.
func TestTwoPhonesBeforeTheTicketOpens(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	a, b := landing(t, w.s, 4), landing(t, w.s, 4)
	ra, err := w.s.PlaceRound(ctx, guest("a"), roundWith(w.g, 4, a))
	if err != nil {
		t.Fatal(err)
	}
	rb, err := w.s.PlaceRound(ctx, guest("b"), roundWith(w.g, 4, b))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.s.AcceptRound(ctx, cashier(""), ra.PublicID); err != nil {
		t.Fatal(err)
	}
	if !good(w.s, 4, a) || good(w.s, 4, b) {
		t.Fatalf("after the first is accepted: a good %v, b good %v; want only a", good(w.s, 4, a), good(w.s, 4, b))
	}
	// Waiting, b's request is listed as asking to join.
	if list, _ := w.s.WaitingRounds(ctx, 1); len(list) != 1 || !list[0].JoinsOpenTicket {
		t.Errorf("waiting = %+v, want b's asking to join", list)
	}
	if _, err := w.s.AcceptRound(ctx, cashier(""), rb.PublicID); err != nil {
		t.Fatal(err)
	}
	if !good(w.s, 4, a) || !good(w.s, 4, b) {
		t.Errorf("after both are accepted: a good %v, b good %v", good(w.s, 4, a), good(w.s, 4, b))
	}
}

// TestAPhoneWithNoSessionAsksToJoin: at a table with a ticket open, a phone
// with no session is not of the party. Its round is a request, the terminal
// says it asks to join, and accepting it hands the phone a session through
// tracking, once, until the phone presents it.
func TestAPhoneWithNoSessionAsksToJoin(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	seated := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))

	round, err := w.s.PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundWaiting || round.OrderID != 0 {
		t.Fatalf("round = %+v, want a request", round)
	}
	if res, _ := w.s.Order(ctx, 1, seated.OrderID); len(res.State.Lines) != 0 {
		t.Fatalf("%d lines reached the ticket before anyone accepted", len(res.State.Lines))
	}
	list, err := w.s.WaitingRounds(ctx, 1)
	if err != nil || len(list) != 1 || !list[0].JoinsOpenTicket {
		t.Fatalf("waiting = %+v, %v; want one asking to join", list, err)
	}
	if tracked, _ := w.s.TrackRound(ctx, 1, 4, round.PublicID); tracked.Session != "" {
		t.Error("a session was handed over before anyone accepted")
	}

	if _, err := w.s.AcceptRound(ctx, cashier(""), round.PublicID); err != nil {
		t.Fatal(err)
	}
	tracked, err := w.s.TrackRound(ctx, 1, 4, round.PublicID)
	if err != nil || tracked.Session == "" {
		t.Fatalf("tracked = %+v, %v; want a session handed over", tracked, err)
	}
	// Asked again before the phone uses it, it is told again: an answer
	// lost on the way is not a session lost.
	if again, _ := w.s.TrackRound(ctx, 1, 4, round.PublicID); again.Session != tracked.Session {
		t.Error("the session was told once and then forgotten before it was used")
	}
	session, err := w.s.GuestSession(ctx, 1, 4, tracked.Session)
	if err != nil || session.OrderID != seated.OrderID {
		t.Fatalf("session = %+v, %v; want it bound to the seated ticket", session, err)
	}
	if after, _ := w.s.TrackRound(ctx, 1, 4, round.PublicID); after.Session != "" {
		t.Error("the session is still told after the phone presented it")
	}
	next, err := w.s.PlaceRound(ctx, guest("k2"), roundWith(w.g, 4, tracked.Session))
	if err != nil || next.Status != RoundSent {
		t.Errorf("the phone's next round = %+v, %v; want it sent", next, err)
	}
}

// TestARoundWithNoSessionAtAnEmptyTableIsRefused: the landing hands one out
// there, so a phone without one is sent to it.
func TestARoundWithNoSessionAtAnEmptyTableIsRefused(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	tests := []struct {
		name    string
		session string
	}{
		{"none", ""},
		{"one never made", "gst_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"another table's", landing(t, s, 5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.PlaceRound(context.Background(), guest("k-"+tt.name), roundWith(g, 4, tt.session))
			if !errors.Is(err, ErrGuestSession) {
				t.Errorf("err = %v, want ErrGuestSession", err)
			}
		})
	}
	if n, _ := d.Queries().CountWaitingGuestRounds(context.Background(), gen.CountWaitingGuestRoundsParams{TableID: 4, Since: now.Add(-time.Hour)}); n != 0 {
		t.Errorf("%d refused rounds were kept", n)
	}
}

// TestAStaleSessionIsRefused: a phone from an earlier sitting presents the
// session it had. It is refused, and is no request either: the phone is
// told to scan again, and with none it would ask to join.
func TestAStaleSessionIsRefused(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()
	id := open(t, s).OrderID
	stale := boundSession(t, s, 1, 4)
	must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
	must(t)(s.Close(ctx, cashier(""), id))
	open(t, s) // the next party

	if _, err := s.PlaceRound(ctx, guest("k1"), roundWith(g, 4, stale)); !errors.Is(err, ErrGuestSession) {
		t.Errorf("err = %v, want ErrGuestSession", err)
	}
}

// TestClosingEndsTheSittingsSessions: the ticket's own, and those handed out
// at its table before it opened and never bound. Another table's go on.
func TestClosingEndsTheSittingsSessions(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	early := landing(t, s, 4)
	elsewhere := landing(t, s, 5)
	id := open(t, s).OrderID
	bound := boundSession(t, s, 1, 4)
	must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
	must(t)(s.Close(ctx, cashier(""), id))

	for name, token := range map[string]string{"bound": bound, "early": early} {
		hash, _ := guestSessionHash(token)
		row, err := d.Queries().GuestSessionByTokenHash(ctx, hash)
		if err != nil || !row.EndedAt.Valid || !row.EndedAt.Time.Equal(now) {
			t.Errorf("%s session: %+v, %v; want it ended at the close", name, row, err)
		}
		if good(s, 4, token) {
			t.Errorf("%s session is good after the close", name)
		}
	}
	if !good(s, 5, elsewhere) {
		t.Error("table 5's session ended with table 4's ticket")
	}
}

// TestPayingFirstStillBindsSessions: where guests pay first a round is held,
// with or without a session, and nobody is asked to accept it. Its payment
// binds the session it was placed with, or hands the phone one.
func TestPayingFirstStillBindsSessions(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	token := landing(t, w.s, 4)

	first, err := w.s.PlaceRound(ctx, guest("k1"), roundWith(w.g, 4, token))
	if err != nil || first.Status != RoundUnpaid {
		t.Fatalf("first = %+v, %v; want it held", first, err)
	}
	sent, err := w.s.SendPaidRound(ctx, 1, first.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if session, err := w.s.GuestSession(ctx, 1, 4, token); err != nil || session.OrderID != sent.OrderID {
		t.Fatalf("session = %+v, %v; want it bound to order %d", session, err, sent.OrderID)
	}

	// A second phone, with no session, at the ticket the first opened: its
	// round is held as any is, not made a request.
	joining, err := w.s.PlaceRound(ctx, guest("k2"), w.g.twoOatAndABrownie(4))
	if err != nil || joining.Status != RoundUnpaid {
		t.Fatalf("joining = %+v, %v; want it held", joining, err)
	}
	if n := w.waitingForStaff(t); n != 0 {
		t.Errorf("%d requests for staff where guests pay first", n)
	}
	if _, err := w.s.SendPaidRound(ctx, 1, joining.PublicID); err != nil {
		t.Fatal(err)
	}
	tracked, err := w.s.TrackRound(ctx, 1, 4, joining.PublicID)
	if err != nil || tracked.Session == "" || !good(w.s, 4, tracked.Session) {
		t.Errorf("tracked = %+v, %v; want a good session handed over", tracked, err)
	}
}
