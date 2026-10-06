package order

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// prepayWorld is a venue where guests pay first, and can: the setting says
// so and a gateway is there. The venue is on GST, added on top.
type prepayWorld struct {
	s *Service
	d *db.DB
	g guestMenu
}

func newPrepayWorld(t *testing.T, canCharge bool, mode PayMode) *prepayWorld {
	t.Helper()
	_, d, _ := newService(t)
	ctx := context.Background()
	if mode != "" {
		err := d.WriteTx(ctx, func(q *gen.Queries) error {
			return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingPayMode, Value: string(mode), UpdatedAt: now})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(Options{DB: d, Clock: fixedClock{}, CanCharge: canCharge})
	if err != nil {
		t.Fatal(err)
	}
	return &prepayWorld{s: s, d: d, g: seedGuestMenu(t, d)}
}

func (w *prepayWorld) waitingForStaff(t *testing.T) int {
	t.Helper()
	list, err := w.s.WaitingRounds(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

// TestWherePayingComesFirstARoundWaitsForItsPayment: it is priced with its
// tax, kept, and is no order, nothing for the kitchen and nothing for staff
// to answer.
func TestWherePayingComesFirstARoundWaitsForItsPayment(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()

	round, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundUnpaid || round.OrderID != 0 || round.Replayed {
		t.Fatalf("round = %+v", round)
	}
	// 560.00 of dishes, and 5% GST on top: what the bill will rise by.
	if round.Subtotal != inr(56000) || round.Due != inr(58800) {
		t.Errorf("subtotal %v, due %v; want 560.00 and 588.00", round.Subtotal, round.Due)
	}
	if n := orderCount(t, w.d); n != 0 {
		t.Errorf("%d orders exist: an unpaid round is not an order", n)
	}
	if n := w.waitingForStaff(t); n != 0 {
		t.Errorf("%d requests in front of staff: an unpaid round is not one", n)
	}

	// Sent again with its key, it is the same round, still asking the same.
	again, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil || !again.Replayed || again.PublicID != round.PublicID || again.Status != RoundUnpaid || again.Due != inr(58800) {
		t.Errorf("again = %+v, %v", again, err)
	}

	// Staff cannot answer it: to them it is not there.
	if _, err := w.s.AcceptRound(ctx, cashier(""), round.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("accepting an unpaid round: %v", err)
	}
	if _, err := w.s.DeclineRound(ctx, cashier(""), round.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("declining an unpaid round: %v", err)
	}
}

// TestASeatedTableStillPaysFirst: staff opening the ticket does not let a
// round through unpaid, and what is asked is what the round adds to a bill
// that already has something on it.
func TestASeatedTableStillPaysFirst(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	opened := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))
	must(t)(w.s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))

	round, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	if round.Status != RoundUnpaid || round.Due != inr(58800) {
		t.Fatalf("round = %+v, want it unpaid for 588.00", round)
	}
	res, err := w.s.Order(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 1 || len(res.State.Tickets) != 0 {
		t.Errorf("the ticket has %d lines and %d kitchen tickets: an unpaid round reached it", len(res.State.Lines), len(res.State.Tickets))
	}
}

// TestPayingFirstNeedsBothTheSettingAndAWay: the setting with no gateway,
// or a gateway with no setting, is the venue as it was.
func TestPayingFirstNeedsBothTheSettingAndAWay(t *testing.T) {
	tests := []struct {
		name      string
		canCharge bool
		mode      PayMode
	}{
		{"the setting, and no gateway", false, PayFirst},
		{"a gateway, and the setting left alone", true, ""},
		{"a gateway, and pay at the table said", true, PayAtTable},
		{"a gateway, and a mode this build does not know", true, "tab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newPrepayWorld(t, tt.canCharge, tt.mode)
			round, err := party(t, w.s).PlaceRound(context.Background(), guest("k1"), w.g.twoOatAndABrownie(4))
			if err != nil {
				t.Fatal(err)
			}
			if round.Status != RoundWaiting || round.Due.Minor != 0 {
				t.Errorf("round = %+v, want a request for staff", round)
			}
			if n := w.waitingForStaff(t); n != 1 {
				t.Errorf("%d requests in front of staff, want 1", n)
			}
		})
	}
}

func TestAnUnpaidRoundIsRefused(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, w *prepayWorld)
		want    error
	}{
		{"with the venue shut", func(t *testing.T, w *prepayWorld) {
			ctx := context.Background()
			err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
				_, err := q.CloseShift(ctx, gen.CloseShiftParams{ID: 1, ClosedBy: 1, ClosedAt: sql.NullTime{Time: now, Valid: true}})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
		}, nil},
		{"onto a ticket on hold", func(t *testing.T, w *prepayWorld) {
			ctx := context.Background()
			opened := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
			must(t)(w.s.Hold(ctx, cashier(""), opened.OrderID))
		}, ErrTableBusy},
		{"onto a bill already invoiced", func(t *testing.T, w *prepayWorld) {
			ctx := context.Background()
			opened := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
			must(t)(w.s.AddLine(ctx, cashier(""), opened.OrderID, coffee(1)))
			must(t)(w.s.TenderPayment(ctx, cashier(""), opened.OrderID, TenderPayment{Method: order.PaymentCash, Amount: inr(4200)}))
		}, ErrTableBusy},
		{"past the unpaid rounds a table may have", func(t *testing.T, w *prepayWorld) {
			for i := range maxWaitingRounds {
				if _, err := party(t, w.s).PlaceRound(context.Background(), guest("u"+string(rune('a'+i))), w.g.twoOatAndABrownie(4)); err != nil {
					t.Fatalf("round %d: %v", i, err)
				}
			}
		}, ErrTooManyWaiting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newPrepayWorld(t, true, PayFirst)
			tt.prepare(t, w)
			_, err := party(t, w.s).PlaceRound(context.Background(), guest("k"), w.g.twoOatAndABrownie(4))
			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if _, err := w.d.Queries().GuestRoundByKey(context.Background(), gen.GuestRoundByKeyParams{VenueID: 1, IdempotencyKey: "k"}); !isNoRows(err) {
				t.Errorf("the refused round was kept: %v", err)
			}
		})
	}
}

// TestUnpaidRoundsDoNotCrowdOutRequests: the two are counted apart, so a
// table's unpaid rounds are not requests and its requests are not unpaid.
func TestUnpaidRoundsDoNotCrowdOutRequests(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	for i := range maxWaitingRounds {
		if _, err := party(t, w.s).PlaceRound(ctx, guest("u"+string(rune('a'+i))), w.g.twoOatAndABrownie(4)); err != nil {
			t.Fatal(err)
		}
	}
	// The owner turns paying first off. The table can still make requests.
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingPayMode, Value: string(PayAtTable), UpdatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}
	round, err := party(t, w.s).PlaceRound(ctx, guest("r1"), w.g.twoOatAndABrownie(4))
	if err != nil || round.Status != RoundWaiting {
		t.Fatalf("round = %+v, %v", round, err)
	}
	if n := w.waitingForStaff(t); n != 1 {
		t.Errorf("%d requests in front of staff, want the one", n)
	}
}
