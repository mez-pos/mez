package order

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/payment"
)

// oddBill opens an order with two lines: 2, a ₹10.01 biscuit, and 3, a
// filter coffee, both with 5% GST.
func oddBill(t *testing.T, s *Service) (int64, order.State) {
	t.Helper()
	ctx := context.Background()
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), id, AddLine{
		ItemID: 9, Name: "Biscuit", Qty: 1, UnitPrice: inr(1001), Station: "bar", TaxRate: 5000, HSN: "996331",
	}))
	res := must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	return id, res.State
}

func amounts(sp payment.Split) []int64 {
	out := make([]int64, len(sp.Shares))
	for i, sh := range sp.Shares {
		out[i] = sh.Amount.Minor
	}
	return out
}

func TestSplitEvenlyOddTotal(t *testing.T) {
	s, _, _ := newService(t)
	id := open(t, s).OrderID
	res := must(t)(s.AddLine(context.Background(), cashier(""), id, AddLine{
		ItemID: 9, Name: "Biscuit", Qty: 1, UnitPrice: inr(1001), Station: "bar", TaxRate: 5000, HSN: "996331",
	}))
	total := res.State.Totals.Total
	if total.Minor%3 == 0 {
		t.Fatalf("total %d divides by three; the test needs an odd one", total.Minor)
	}

	sp, err := s.Split(context.Background(), 1, id, SplitRequest{Kind: SplitByEvenly, Ways: 3})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	q, r := total.Minor/3, total.Minor%3
	want := []int64{q, q, q}
	for i := range r {
		want[i]++ // the earliest payers carry the odd paisa
	}
	if got := amounts(sp); !reflect.DeepEqual(got, want) || sp.Total != total {
		t.Fatalf("shares %v of %d, want %v of %d", got, sp.Total.Minor, want, total.Minor)
	}
}

func TestSplitKinds(t *testing.T) {
	s, _, _ := newService(t)
	id, st := oddBill(t, s)
	total := st.Totals.Total.Minor
	biscuit, coffeeDue := st.Lines[0].Due.Minor, st.Lines[1].Due.Minor

	tests := []struct {
		name string
		req  SplitRequest
		want []int64
		ids  [][]int64
	}{
		{"by amount, the rest is one more payer", SplitRequest{Kind: SplitByAmount, Amounts: []money.Amount{inr(1000)}},
			[]int64{1000, total - 1000}, [][]int64{nil, nil}},
		{"by item, one line shared", SplitRequest{Kind: SplitByItem, Ways: 2, Items: []SplitItem{
			{LineID: 2, Payers: []int{0}}, {LineID: 3, Payers: []int{0, 1}},
		}}, []int64{biscuit + (coffeeDue+1)/2, coffeeDue / 2}, [][]int64{{2, 3}, {3}}},
		{"by guest, one line at the table", SplitRequest{Kind: SplitByGuest, Ways: 2, Seats: []SplitSeat{
			{LineID: 2, Guest: 2}, {LineID: 3, Guest: 0},
		}}, []int64{(coffeeDue + 1) / 2, biscuit + coffeeDue/2}, [][]int64{{3}, {2, 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := s.Split(context.Background(), 1, id, tt.req)
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if got := amounts(sp); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shares %v, want %v", got, tt.want)
			}
			for i, sh := range sp.Shares {
				if !reflect.DeepEqual(sh.LineIDs, tt.ids[i]) {
					t.Errorf("payer %d covers %v, want %v", i, sh.LineIDs, tt.ids[i])
				}
			}
		})
	}
}

func TestSplitRefusals(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id, _ := oddBill(t, s)
	// A voided line costs nothing and needs no payer.
	must(t)(s.AddLine(ctx, cashier(""), id, dosa(1)))
	must(t)(s.RemoveLine(ctx, cashier(""), id, 4))

	tests := []struct {
		name string
		req  SplitRequest
		want error
	}{
		{"no ways", SplitRequest{Kind: SplitByEvenly}, payment.ErrInvalidSplit},
		{"too many ways", SplitRequest{Kind: SplitByEvenly, Ways: maxPayers + 1}, payment.ErrInvalidSplit},
		{"no amounts", SplitRequest{Kind: SplitByAmount}, payment.ErrInvalidSplit},
		{"amounts over the bill", SplitRequest{Kind: SplitByAmount, Amounts: []money.Amount{inr(999999)}}, payment.ErrOverAllocated},
		{"an unknown kind", SplitRequest{Kind: "seat", Ways: 2}, payment.ErrInvalidSplit},
		{"a line without a payer", SplitRequest{Kind: SplitByItem, Ways: 2, Items: []SplitItem{{LineID: 2, Payers: []int{0}}}}, payment.ErrUnassigned},
		{"a line not on the bill", SplitRequest{Kind: SplitByItem, Ways: 1, Items: []SplitItem{
			{LineID: 2, Payers: []int{0}}, {LineID: 3, Payers: []int{0}}, {LineID: 4, Payers: []int{0}},
		}}, payment.ErrUnassigned},
		{"a line listed twice", SplitRequest{Kind: SplitByItem, Ways: 1, Items: []SplitItem{
			{LineID: 2, Payers: []int{0}}, {LineID: 2, Payers: []int{0}},
		}}, payment.ErrUnassigned},
		{"a payer out of range", SplitRequest{Kind: SplitByItem, Ways: 1, Items: []SplitItem{
			{LineID: 2, Payers: []int{0}}, {LineID: 3, Payers: []int{1}},
		}}, payment.ErrUnassigned},
		{"a seat out of range", SplitRequest{Kind: SplitByGuest, Ways: 2, Seats: []SplitSeat{
			{LineID: 2, Guest: 1}, {LineID: 3, Guest: 3},
		}}, payment.ErrUnassigned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Split(ctx, 1, id, tt.req); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestSplitAfterPayment: evenly divides what is left; by item cannot, since
// nothing says which lines the first payment was for.
func TestSplitAfterPayment(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id, st := oddBill(t, s)
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: inr(1000)}))
	left := paid.State.Totals.BalanceDue
	if left.Minor != st.Totals.Total.Minor-1000 {
		t.Fatalf("balance %d", left.Minor)
	}

	sp, err := s.Split(ctx, 1, id, SplitRequest{Kind: SplitByEvenly, Ways: 2})
	if err != nil || sp.Total != left {
		t.Fatalf("Split = %+v, %v; want the balance %d divided", sp, err, left.Minor)
	}
	_, err = s.Split(ctx, 1, id, SplitRequest{Kind: SplitByItem, Ways: 1, Items: []SplitItem{
		{LineID: 2, Payers: []int{0}}, {LineID: 3, Payers: []int{0}},
	}})
	if !errors.Is(err, ErrSplitAfterPayment) {
		t.Errorf("by item after a payment: %v, want ErrSplitAfterPayment", err)
	}

	must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCard, Amount: left}))
	if _, err := s.Split(ctx, 1, id, SplitRequest{Kind: SplitByEvenly, Ways: 2}); !errors.Is(err, payment.ErrInvalidSplit) {
		t.Errorf("nothing owed: %v, want ErrInvalidSplit", err)
	}
}

func TestSplitHeldAndMissing(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id, _ := oddBill(t, s)
	must(t)(s.Hold(ctx, cashier(""), id))
	if _, err := s.Split(ctx, 1, id, SplitRequest{Kind: SplitByEvenly, Ways: 2}); !errors.Is(err, order.ErrOrderHeld) {
		t.Errorf("held: %v", err)
	}
	if _, err := s.Split(ctx, 2, id, SplitRequest{Kind: SplitByEvenly, Ways: 2}); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("another venue: %v", err)
	}
}
