package charges

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/gateway"
)

func (w *world) letGo(t *testing.T, c Charge) Charge {
	t.Helper()
	got, err := w.s.LetGo(context.Background(), 1, 4, c.PublicID)
	if err != nil {
		t.Fatalf("letting go of %s: %v", c.PublicID, err)
	}
	return got
}

// TestLettingGoFreesTheBill: a guest looked at the page for the whole bill
// and came back. Until they let go nobody can take a share; after, they can.
func TestLettingGoFreesTheBill(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	whole := w.start(t)
	if _, err := w.s.StartShare(ctx, 1, 4, "phone-a", inr(8400), "", ""); !errors.Is(err, ErrOverClaim) {
		t.Fatalf("a share while the whole bill's page is open: %v", err)
	}

	got := w.letGo(t, whole)
	if !got.LetGo || got.Status != Open || got.PublicID != whole.PublicID {
		t.Fatalf("let go = %+v, want it open and let go", got)
	}
	// Twice is once.
	if again := w.letGo(t, whole); !again.LetGo || again.Status != Open {
		t.Errorf("let go again = %+v", again)
	}
	claims, err := w.s.Claims(ctx, 1, 4, "phone-a")
	if err != nil || claims != (Claims{Due: inr(16800), Claimed: inr(0), Unclaimed: inr(16800)}) {
		t.Errorf("claims after letting go = %+v, %v", claims, err)
	}
	a := w.share(t, "phone-a", 8400)
	b := w.share(t, "phone-b", 8400)

	// Paying online again is a new page, not the one given up.
	if _, err := w.s.Start(ctx, 1, 4, "", ""); !errors.Is(err, ErrOverClaim) {
		t.Errorf("the rest with both halves being paid: %v", err)
	}

	// The page given up can still be paid. It is, and so are both halves:
	// the first two to land fit the bill and the third is kept for a person.
	for i := range 3 {
		w.gw.Pay(gatewayID(i+1), gateway.UPI)
	}
	if got := w.settle(t, a); got.Status != Paid {
		t.Fatalf("phone-a's half: %+v", got)
	}
	if got := w.settle(t, b); got.Status != Paid {
		t.Fatalf("phone-b's half: %+v", got)
	}
	if got := w.settle(t, whole); got.Status != Unapplied || !got.LetGo {
		t.Fatalf("the page given up and then paid: %+v", got)
	}
	if st := w.state(t); len(st.Tenders) != 2 || !st.Totals.BalanceDue.IsZero() {
		t.Errorf("%d tenders and %v owed", len(st.Tenders), st.Totals.BalanceDue)
	}
}

// TestAskingAgainAfterLettingGoIsANewCharge: what was given up is not handed
// back, for the whole bill or for a share. The new charge is checked against
// the bill like any other.
func TestAskingAgainAfterLettingGoIsANewCharge(t *testing.T) {
	w := newWorld(t)
	whole := w.start(t)
	w.letGo(t, whole)
	again := w.start(t)
	if again.Replayed || again.PublicID == whole.PublicID || again.LetGo || again.Amount != inr(16800) {
		t.Fatalf("the whole bill again = %+v", again)
	}
	w.letGo(t, again)

	share := w.share(t, "phone-a", 5600)
	w.letGo(t, share)
	second := w.share(t, "phone-a", 5600)
	if second.Replayed || second.PublicID == share.PublicID || second.LetGo {
		t.Fatalf("the share again = %+v", second)
	}
	if third := w.share(t, "phone-a", 5600); !third.Replayed || third.PublicID != second.PublicID {
		t.Errorf("and once more = %+v, want %s again", third, second.PublicID)
	}
	if len(w.gw.Started) != 4 {
		t.Errorf("the gateway was asked %d times, want 4", len(w.gw.Started))
	}
}

// TestLettingGoOfWhatIsNotAClaim: a charge already paid says so and is not
// marked; the gateway is asked first, so a guest who paid and then tapped
// is told they paid. A charge that is not this table's bill's is not found.
func TestLettingGoOfWhatIsNotAClaim(t *testing.T) {
	ctx := context.Background()

	t.Run("paid a moment ago", func(t *testing.T) {
		w := newWorld(t)
		c := w.start(t)
		w.gw.Pay(gatewayID(1), gateway.Card)
		if got := w.letGo(t, c); got.Status != Paid || got.LetGo {
			t.Errorf("letting go of a page just paid = %+v", got)
		}
		if st := w.state(t); len(st.Tenders) != 1 {
			t.Errorf("%d tenders", len(st.Tenders))
		}
	})

	t.Run("failed", func(t *testing.T) {
		w := newWorld(t)
		c := w.start(t)
		w.gw.Fail(gatewayID(1))
		if got := w.letGo(t, c); got.Status != Failed || got.LetGo {
			t.Errorf("letting go of a failed page = %+v", got)
		}
	})

	t.Run("the gateway cannot be asked", func(t *testing.T) {
		w := newWorld(t)
		c := w.start(t)
		w.gw.Down(true)
		if got := w.letGo(t, c); got.Status != Open || !got.LetGo {
			t.Errorf("letting go with the gateway down = %+v, want it let go all the same", got)
		}
	})

	t.Run("another table's", func(t *testing.T) {
		w := newWorld(t)
		c := w.start(t)
		if _, err := w.s.LetGo(ctx, 1, 5, c.PublicID); !errors.Is(err, ErrChargeNotFound) {
			t.Errorf("from table 5: %v", err)
		}
		if _, err := w.s.LetGo(ctx, 2, 4, c.PublicID); !errors.Is(err, ErrChargeNotFound) {
			t.Errorf("from another venue: %v", err)
		}
		if _, err := w.s.LetGo(ctx, 1, 4, "chg_NOSUCH01"); !errors.Is(err, ErrChargeNotFound) {
			t.Errorf("one that is not there: %v", err)
		}
		if got, _ := w.s.Charge(ctx, 1, c.PublicID); got.LetGo {
			t.Error("it was let go by a table it is not at")
		}
	})

	t.Run("a round's", func(t *testing.T) {
		w := newWorld(t)
		w.hold(t, "rnd_HELD0001", 6)
		c := w.startRound(t, "rnd_HELD0001", 6)
		if _, err := w.s.LetGo(ctx, 1, 6, c.PublicID); !errors.Is(err, ErrChargeNotFound) {
			t.Errorf("a round's charge: %v", err)
		}
	})
}
