package shift

import (
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

var when = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

// in is an order event at seq, stamped with shift.
func in(shift, seq int64, p order.Payload) StampedEvent {
	return StampedEvent{Event: order.NewEvent(seq, when, 1, 1, p), ShiftID: shift}
}

func tender(method order.PaymentMethod, amount, tip int64) order.PaymentTendered {
	return order.PaymentTendered{Method: method, Amount: inr(amount), Tip: inr(tip)}
}

func TestCashFromOrders(t *testing.T) {
	tests := []struct {
		name   string
		shift  int64 // 0 means shift 1
		orders [][]StampedEvent
		want   CashSales
	}{
		{
			name: "cash and its tip, not card or UPI",
			orders: [][]StampedEvent{{
				in(1, 2, tender(order.PaymentCash, 45000, 5000)),
				in(1, 3, tender(order.PaymentCard, 30000, 2000)),
				in(1, 4, tender(order.PaymentUPI, 10000, 0)),
			}},
			want: CashSales{Taken: inr(50000), Voided: inr(0), Refunded: inr(0)},
		},
		{
			name: "a tender with no tip at all",
			orders: [][]StampedEvent{{
				in(1, 2, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(12000)}),
			}},
			want: CashSales{Taken: inr(12000), Voided: inr(0), Refunded: inr(0)},
		},
		{
			name: "a cash tender voided hands back the bill and the tip",
			orders: [][]StampedEvent{{
				in(1, 2, tender(order.PaymentCash, 45000, 5000)),
				in(1, 3, order.PaymentVoided{PaymentID: 2, Reason: "wrong bill", AuthorisedBy: 2}),
				in(1, 4, order.PaymentVoided{PaymentID: 9, Reason: "not a tender", AuthorisedBy: 2}),
			}},
			want: CashSales{Taken: inr(50000), Voided: inr(50000), Refunded: inr(0)},
		},
		{
			name: "a card void or refund leaves the drawer alone",
			orders: [][]StampedEvent{{
				in(1, 2, tender(order.PaymentCard, 45000, 0)),
				in(1, 3, order.PaymentVoided{PaymentID: 2}),
				in(1, 4, tender(order.PaymentCard, 45000, 0)),
				in(1, 5, order.RefundIssued{Amount: inr(5000), AgainstPaymentID: 4}),
			}},
			want: CashSales{Taken: inr(0), Voided: inr(0), Refunded: inr(0)},
		},
		{
			name:  "yesterday's cash bill refunded today leaves today's drawer",
			shift: 2,
			orders: [][]StampedEvent{{
				in(1, 2, tender(order.PaymentCash, 45000, 0)),
				in(2, 3, order.RefundIssued{Amount: inr(15000), AgainstPaymentID: 2}),
			}},
			want: CashSales{Taken: inr(0), Voided: inr(0), Refunded: inr(15000)},
		},
		{
			name:  "a tab paid across two shifts counts only this shift's part",
			shift: 2,
			orders: [][]StampedEvent{{
				in(1, 2, tender(order.PaymentCash, 20000, 0)),
				in(2, 3, tender(order.PaymentCash, 25000, 1000)),
			}},
			want: CashSales{Taken: inr(26000), Voided: inr(0), Refunded: inr(0)},
		},
		{
			name: "no shift and other shifts are not this drawer",
			orders: [][]StampedEvent{
				{in(0, 2, tender(order.PaymentCash, 100, 0))},
				{in(3, 2, tender(order.PaymentCash, 100, 0))},
			},
			want: CashSales{Taken: inr(0), Voided: inr(0), Refunded: inr(0)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shiftID := max(tt.shift, 1)
			got, err := CashFromOrders("INR", shiftID, tt.orders)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("CashFromOrders = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCashFromOrdersRefusals(t *testing.T) {
	if _, err := CashFromOrders("inr", 1, nil); err == nil {
		t.Error("a malformed currency: want an error")
	}
	dollars := [][]StampedEvent{{in(1, 2, order.PaymentTendered{Method: order.PaymentCash, Amount: money.Amount{Minor: 1, Currency: "USD"}})}}
	if _, err := CashFromOrders("INR", 1, dollars); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("a dollar tender: err = %v, want ErrCurrencyMismatch", err)
	}
}

func TestExpected(t *testing.T) {
	float := Count{Kind: CountOpening, Total: inr(200000)}
	moves := []Movement{{Kind: CashOut, Amount: inr(12050)}, {Kind: CashIn, Amount: inr(5000)}}
	sales := CashSales{Taken: inr(90000), Voided: inr(10000), Refunded: inr(2500)}
	got, err := Expected(float, moves, sales)
	if err != nil {
		t.Fatal(err)
	}
	// 2000 - 120.50 + 50 + 900 - 100 - 25 = 2704.50
	if got != inr(270450) {
		t.Errorf("Expected = %s, want 2704.50", got)
	}
	if _, err := Expected(Count{Kind: CountClosing, Total: inr(0)}, nil, sales); err == nil {
		t.Error("a closing count as the float: want an error")
	}
}

func TestReconcile(t *testing.T) {
	closing := func(minor int64) Count { return Count{Kind: CountClosing, Total: inr(minor)} }
	tests := []struct {
		name        string
		counted     int64
		threshold   int64
		variance    int64
		over, short bool
		needsReason bool
	}{
		{"to the paisa", 270450, 0, 0, false, false, false},
		{"a rupee over, strict", 270550, 0, 100, true, false, true},
		{"a rupee short, strict", 270350, 0, -100, false, true, true},
		{"short, within the threshold", 270350, 500, -100, false, true, false},
		{"exactly at the threshold", 269950, 500, -500, false, true, false},
		{"a paisa past it", 269949, 500, -501, false, true, true},
		{"over past it", 271000, 500, 550, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Reconcile(inr(270450), closing(tt.counted), inr(tt.threshold))
			if err != nil {
				t.Fatal(err)
			}
			if r.Variance != inr(tt.variance) || r.Counted != inr(tt.counted) || r.Expected != inr(270450) {
				t.Errorf("Reconcile = %+v, want variance %d", r, tt.variance)
			}
			if r.Over() != tt.over || r.Short() != tt.short || r.NeedsReason() != tt.needsReason {
				t.Errorf("over %v short %v reason %v; want %v %v %v", r.Over(), r.Short(), r.NeedsReason(), tt.over, tt.short, tt.needsReason)
			}
		})
	}
}

func TestReconcileRefusals(t *testing.T) {
	tests := []struct {
		name      string
		count     Count
		threshold money.Amount
	}{
		{"the opening count", Count{Kind: CountOpening, Total: inr(0)}, inr(0)},
		{"a negative threshold", Count{Kind: CountClosing, Total: inr(0)}, inr(-1)},
		{"a threshold in dollars", Count{Kind: CountClosing, Total: inr(0)}, money.Amount{Currency: "USD"}},
		{"a count in dollars", Count{Kind: CountClosing, Total: money.Amount{Currency: "USD"}}, inr(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Reconcile(inr(100), tt.count, tt.threshold); err == nil {
				t.Error("want an error")
			}
		})
	}
}
