package order

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
)

// takeUPI gives venue 1 a UPI address, without which there is nothing a
// guest could have paid from a phone.
func (w *requestWorld) takeUPI(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingUPIAddress, Value: "cafe@okbank", UpdatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ticketWithCoffee seats table 4 and rings up one coffee: 40.00 plus 5%.
func (w *requestWorld) ticketWithCoffee(t *testing.T) Result {
	t.Helper()
	opened := must(t)(w.s.Open(context.Background(), cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))
	return must(t)(w.s.AddLine(context.Background(), cashier(""), opened.OrderID, coffee(1)))
}

// TestTellingPaidIsAMessageAndNotAPayment: the notice reaches the
// terminals, and the bill is exactly as it was.
func TestTellingPaidIsAMessageAndNotAPayment(t *testing.T) {
	w := newRequestWorld(t)
	w.takeUPI(t)
	ctx := context.Background()
	ticket := w.ticketWithCoffee(t)
	version, nudges := w.floorVersion(t), w.nudges.Load()

	notice, err := w.s.TellPaid(ctx, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if notice.PublicID == "" || notice.Replayed || notice.TableID != 4 || notice.OrderID != ticket.OrderID ||
		notice.OrderPublicID != ticket.PublicID || notice.Amount != inr(4200) || !notice.ToldAt.Equal(now) {
		t.Errorf("notice = %+v", notice)
	}
	if w.floorVersion(t) != version+1 || w.nudges.Load() != nudges+1 {
		t.Error("telling did not tell the terminals")
	}

	// Nothing on the ticket moved: no event, nothing paid, the same owed.
	after, err := w.s.Order(ctx, 1, ticket.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State.LastSeq != ticket.State.LastSeq || len(after.State.Tenders) != 0 || after.State.Totals.BalanceDue != inr(4200) {
		t.Errorf("the ticket changed: last seq %d, tenders %d, due %v", after.State.LastSeq, len(after.State.Tenders), after.State.Totals.BalanceDue)
	}

	list, err := w.s.PaidNotices(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].PublicID != notice.PublicID || list[0].TableName != "4" || list[0].Amount != inr(4200) || list[0].Due != inr(4200) {
		t.Errorf("list = %+v", list)
	}
	if told, err := w.s.PaidTold(ctx, ticket.OrderID); err != nil || !told {
		t.Errorf("told = %v, %v", told, err)
	}
}

// TestTellingPaidFortyTimesIsOnce: one open notice a ticket, however often
// and however fast it is said.
func TestTellingPaidFortyTimesIsOnce(t *testing.T) {
	w := newRequestWorld(t)
	w.takeUPI(t)
	ctx := context.Background()
	w.ticketWithCoffee(t)

	const taps = 40
	got := make([]PaidNotice, taps)
	errs := make([]error, taps)
	var wg sync.WaitGroup
	for i := range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.TellPaid(ctx, 1, 4)
		}()
	}
	wg.Wait()
	fresh := 0
	for i := range taps {
		if errs[i] != nil {
			t.Fatalf("tap %d: %v", i, errs[i])
		}
		if got[i].PublicID != got[0].PublicID {
			t.Errorf("tap %d made notice %s, tap 0 made %s", i, got[i].PublicID, got[0].PublicID)
		}
		if !got[i].Replayed {
			fresh++
		}
	}
	if fresh != 1 {
		t.Errorf("%d taps wrote a notice, want 1", fresh)
	}
	if list, _ := w.s.PaidNotices(ctx, 1); len(list) != 1 {
		t.Errorf("%d notices listed, want 1", len(list))
	}
	if w.nudges.Load() != 1 {
		t.Errorf("the terminals were told %d times, want once", w.nudges.Load())
	}
}

// TestTheAmountIsWhatWasOwedWhenSaid: a round added after the guest said so
// shows as more left than they could have paid.
func TestTheAmountIsWhatWasOwedWhenSaid(t *testing.T) {
	w := newRequestWorld(t)
	w.takeUPI(t)
	ctx := context.Background()
	ticket := w.ticketWithCoffee(t)
	if _, err := w.s.TellPaid(ctx, 1, 4); err != nil {
		t.Fatal(err)
	}
	must(t)(w.s.AddLine(ctx, cashier(""), ticket.OrderID, coffee(1)))

	list, err := w.s.PaidNotices(ctx, 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list[0].Amount != inr(4200) || list[0].Due != inr(8400) {
		t.Errorf("said of %v, now owing %v; want 42.00 and 84.00", list[0].Amount, list[0].Due)
	}
	// Said again, it is still the first notice, with its first amount.
	again, err := w.s.TellPaid(ctx, 1, 4)
	if err != nil || !again.Replayed || again.Amount != inr(4200) {
		t.Errorf("again = %+v, %v", again, err)
	}
}

// TestRecordingThePaymentPutsTheNoticeAway: once staff record the money,
// nothing is left to tell them, with no one having to dismiss it.
func TestRecordingThePaymentPutsTheNoticeAway(t *testing.T) {
	w := newRequestWorld(t)
	w.takeUPI(t)
	ctx := context.Background()
	ticket := w.ticketWithCoffee(t)
	if _, err := w.s.TellPaid(ctx, 1, 4); err != nil {
		t.Fatal(err)
	}

	// Part of it: still owing, still listed.
	must(t)(w.s.TenderPayment(ctx, cashier(""), ticket.OrderID, TenderPayment{Method: order.PaymentUPI, Amount: inr(2000), Reference: "UTR 1"}))
	if list, _ := w.s.PaidNotices(ctx, 1); len(list) != 1 || list[0].Due != inr(2200) {
		t.Errorf("after a part payment: %+v", list)
	}
	// The rest: settled, and the notice is gone from the list.
	must(t)(w.s.TenderPayment(ctx, cashier(""), ticket.OrderID, TenderPayment{Method: order.PaymentUPI, Amount: inr(2200), Reference: "UTR 2"}))
	if list, _ := w.s.PaidNotices(ctx, 1); len(list) != 0 {
		t.Errorf("after the bill is settled: %+v", list)
	}
	must(t)(w.s.Close(ctx, cashier(""), ticket.OrderID))
	if list, _ := w.s.PaidNotices(ctx, 1); len(list) != 0 {
		t.Errorf("after the bill is closed: %+v", list)
	}
	// And a settled bill has nothing more to pay.
	if _, err := w.s.TellPaid(ctx, 1, 4); !errors.Is(err, ErrNothingToPay) {
		t.Errorf("telling on a table with no ticket: %v", err)
	}
}

// TestPuttingANoticeAway: staff looked and found no payment. The notice
// goes, the bill is untouched, and the guest may say so again.
func TestPuttingANoticeAway(t *testing.T) {
	w := newRequestWorld(t)
	w.takeUPI(t)
	ctx := context.Background()
	ticket := w.ticketWithCoffee(t)
	first, err := w.s.TellPaid(ctx, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	version, nudges := w.floorVersion(t), w.nudges.Load()

	if err := w.s.SeePaidNotice(ctx, cashier(""), first.PublicID); err != nil {
		t.Fatal(err)
	}
	if w.floorVersion(t) != version+1 || w.nudges.Load() != nudges+1 {
		t.Error("putting it away did not tell the terminals")
	}
	if list, _ := w.s.PaidNotices(ctx, 1); len(list) != 0 {
		t.Errorf("still listed: %+v", list)
	}
	if told, _ := w.s.PaidTold(ctx, ticket.OrderID); told {
		t.Error("the bill still says staff have been told")
	}
	if after, _ := w.s.Order(ctx, 1, ticket.OrderID); after.State.Totals.BalanceDue != inr(4200) {
		t.Errorf("putting a notice away changed the bill: %v", after.State.Totals.BalanceDue)
	}
	// Twice is once.
	if err := w.s.SeePaidNotice(ctx, cashier(""), first.PublicID); err != nil || w.floorVersion(t) != version+1 {
		t.Errorf("put away twice: %v", err)
	}
	// Said again, it is a new notice.
	second, err := w.s.TellPaid(ctx, 1, 4)
	if err != nil || second.Replayed || second.PublicID == first.PublicID {
		t.Errorf("said again = %+v, %v", second, err)
	}
}

func TestPaidNoticesAreRefused(t *testing.T) {
	tests := []struct {
		name string
		do   func(t *testing.T, w *requestWorld) error
		want error
	}{
		{"telling at a table with no ticket", func(_ *testing.T, w *requestWorld) error {
			_, err := w.s.TellPaid(context.Background(), 1, 4)
			return err
		}, ErrNothingToPay},
		{"telling with nothing on the ticket", func(t *testing.T, w *requestWorld) error {
			must(t)(w.s.Open(context.Background(), cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
			_, err := w.s.TellPaid(context.Background(), 1, 4)
			return err
		}, ErrNothingToPay},
		{"telling a request nobody has accepted", func(t *testing.T, w *requestWorld) error {
			w.ask(t, "k1", 4)
			_, err := w.s.TellPaid(context.Background(), 1, 4)
			return err
		}, ErrNothingToPay},
		{"putting away a notice that does not exist", func(_ *testing.T, w *requestWorld) error {
			return w.s.SeePaidNotice(context.Background(), cashier(""), "pdn_NOPE0000")
		}, ErrNoticeNotFound},
		{"putting away another venue's notice", func(t *testing.T, w *requestWorld) error {
			w.ticketWithCoffee(t)
			n, err := w.s.TellPaid(context.Background(), 1, 4)
			if err != nil {
				t.Fatal(err)
			}
			return w.s.SeePaidNotice(context.Background(), Meta{VenueID: 2, ActorID: 7}, n.PublicID)
		}, ErrNoticeNotFound},
		{"putting one away as nobody", func(t *testing.T, w *requestWorld) error {
			w.ticketWithCoffee(t)
			n, err := w.s.TellPaid(context.Background(), 1, 4)
			if err != nil {
				t.Fatal(err)
			}
			return w.s.SeePaidNotice(context.Background(), guest(""), n.PublicID)
		}, ErrInvalidCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newRequestWorld(t)
			w.takeUPI(t)
			if err := tt.do(t, w); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	// A venue that takes no UPI: there was nothing to pay from a phone.
	w := newRequestWorld(t)
	w.ticketWithCoffee(t)
	if _, err := w.s.TellPaid(context.Background(), 1, 4); !errors.Is(err, ErrNothingToPay) {
		t.Errorf("with no UPI address: %v", err)
	}
}
