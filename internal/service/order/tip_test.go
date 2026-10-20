package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
)

// setTipCeiling sets venue 1's tip ceiling (ADR-261).
func setTipCeiling(t *testing.T, s *Service, value string) {
	t.Helper()
	ctx := context.Background()
	if err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingTipMaxPercent, Value: value, UpdatedAt: now})
	}); err != nil {
		t.Fatal(err)
	}
}

// A tip over the venue's ceiling is refused unless a manager let it through,
// and the manager is on the payment for as long as the order lasts (ADR-261).
// The bill is one dosa, 126.00 with its tax; the payments are part of it.
func TestATipOverTheCeilingNeedsAManager(t *testing.T) {
	tests := []struct {
		name       string
		ceiling    string // "" leaves the default, 50
		amount     int64
		tip        int64
		authorised int64
		refused    bool
	}{
		{"no tip", "", 10000, 0, 0, false},
		{"under the default ceiling", "", 10000, 4999, 0, false},
		{"at the default ceiling", "", 10000, 5000, 0, false},
		{"a paisa over the default ceiling", "", 10000, 5001, 0, true},
		{"over it, with a manager", "", 10000, 5001, 9, false},
		{"5000 rupees on 100", "", 10000, 500000, 0, true},
		{"5000 rupees on 100, approved", "", 10000, 500000, 9, false},
		{"the ceiling rounds up: 50% of 1.01 is 0.51", "", 101, 51, 0, false},
		{"and a paisa over that is refused", "", 101, 52, 0, true},
		{"a ceiling of 0 refuses any tip", "0", 10000, 1, 0, true},
		{"a ceiling of 0 with a manager", "0", 10000, 1, 9, false},
		{"a ceiling of 0 with no tip", "0", 10000, 0, 0, false},
		{"a ceiling of 1000", "1000", 10000, 100000, 0, false},
		{"over a ceiling of 1000", "1000", 10000, 100001, 0, true},
		{"a stored ceiling nothing could read is the default", "lots", 10000, 5001, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newService(t)
			if tt.ceiling != "" {
				setTipCeiling(t, s, tt.ceiling)
			}
			ctx := context.Background()
			id := open(t, s).OrderID
			must(t)(s.AddLine(ctx, cashier(""), id, dosa(1)))

			res, err := s.TenderPayment(ctx, cashier(""), id, TenderPayment{
				Method: order.PaymentCard, Amount: inr(tt.amount), Tip: inr(tt.tip), AuthorisedBy: tt.authorised,
			})
			if tt.refused {
				if !errors.Is(err, ErrTipNeedsApproval) {
					t.Fatalf("want ErrTipNeedsApproval, got %v", err)
				}
				got, err := s.Order(ctx, 1, id)
				if err != nil || len(got.State.Tenders) != 0 {
					t.Fatalf("a refused tip left a payment: %+v, %v", got.State.Tenders, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tenders := res.State.Tenders
			if len(tenders) != 1 || tenders[0].Tip != inr(tt.tip) || tenders[0].AuthorisedBy != tt.authorised {
				t.Fatalf("tenders = %+v", tenders)
			}
			var tendered *order.PaymentTendered
			for _, e := range res.Events {
				if p, ok := e.Payload.(order.PaymentTendered); ok {
					tendered = &p
				}
			}
			if tendered == nil || tendered.AuthorisedBy != tt.authorised || tendered.TenderedBy != 7 {
				t.Fatalf("the event = %+v", tendered)
			}
		})
	}
}

// A tender the projection refuses for itself is refused for that, not for
// its tip: a zero amount has a zero ceiling, and is no payment at all.
func TestATenderOfNothingIsRefusedForItsAmountNotItsTip(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, dosa(1)))
	_, err := s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(0), Tip: inr(100)})
	if err == nil || errors.Is(err, ErrTipNeedsApproval) {
		t.Fatalf("want the projection's refusal, got %v", err)
	}
}
