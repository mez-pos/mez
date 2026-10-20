package shift

import (
	"reflect"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

// by is an order event recorded by actor at seq, so many minutes after
// when, stamped with shift.
func by(actor, shift, seq int64, p order.Payload) StampedEvent {
	at := when.Add(time.Duration(seq) * time.Minute)
	return StampedEvent{Event: order.NewEvent(seq, at, actor, 1, p), ShiftID: shift}
}

func opened(by, table int64) order.OrderOpened {
	return order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", TableID: table, Covers: 2, OpenedBy: by}
}

func took(by int64, method order.PaymentMethod, amount, tip int64) order.PaymentTendered {
	p := tender(method, amount, tip)
	p.TenderedBy = by
	return p
}

func TestMineOf(t *testing.T) {
	const kabir, meera = 5, 3
	first := func(seq int64) *time.Time {
		at := when.Add(time.Duration(seq) * time.Minute)
		return &at
	}
	tests := []struct {
		name   string
		orders [][]StampedEvent
		want   Mine
	}{
		{
			name:   "nothing done yet",
			orders: nil,
			want:   Mine{Sales: inr(0), Tips: inr(0)},
		},
		{
			name: "orders opened, the ones at a table, and what their payments settled",
			orders: [][]StampedEvent{
				{by(kabir, 1, 1, opened(kabir, 4)), by(kabir, 1, 2, took(kabir, order.PaymentUPI, 81500, 4000))},
				{by(kabir, 1, 3, opened(kabir, 0)), by(kabir, 1, 4, took(kabir, order.PaymentCash, 12000, 0))},
			},
			want: Mine{Orders: 2, Tables: 1, Sales: inr(93500), Tips: inr(4000), ByTable: []TableTips{{TableID: 4, Tips: inr(4000)}}, First: first(1)},
		},
		{
			name: "the orders and payments of someone else are theirs",
			orders: [][]StampedEvent{
				{by(meera, 1, 1, opened(meera, 2)), by(meera, 1, 2, took(meera, order.PaymentCash, 30000, 1000))},
			},
			want: Mine{Sales: inr(0), Tips: inr(0)},
		},
		{
			name: "a table one opened and another settled: the order is one's, the money the other's",
			orders: [][]StampedEvent{
				{by(meera, 1, 1, opened(meera, 7)), by(kabir, 1, 2, took(kabir, order.PaymentCard, 50000, 2500))},
			},
			want: Mine{Sales: inr(50000), Tips: inr(2500), ByTable: []TableTips{{TableID: 7, Tips: inr(2500)}}, First: first(2)},
		},
		{
			name: "a payment voided in the shift is taken off, with its tip",
			orders: [][]StampedEvent{{
				by(kabir, 1, 1, opened(kabir, 4)),
				by(kabir, 1, 2, took(kabir, order.PaymentCash, 20000, 1000)),
				by(meera, 1, 3, order.PaymentVoided{PaymentID: 2, Reason: "wrong bill", AuthorisedBy: 2}),
				by(kabir, 1, 4, took(kabir, order.PaymentCard, 20000, 0)),
			}},
			want: Mine{Orders: 1, Tables: 1, Sales: inr(20000), Tips: inr(0), First: first(1)},
		},
		{
			name: "the work of another shift is not this one's",
			orders: [][]StampedEvent{{
				by(kabir, 2, 1, opened(kabir, 4)),
				by(kabir, 2, 2, took(kabir, order.PaymentCash, 10000, 500)),
				by(kabir, 1, 3, took(kabir, order.PaymentCash, 5000, 0)),
			}},
			want: Mine{Sales: inr(5000), Tips: inr(0), First: first(3)},
		},
		{
			name: "tips follow the party to the table it moved to, largest first",
			orders: [][]StampedEvent{
				{by(kabir, 1, 1, opened(kabir, 4)), by(kabir, 1, 2, order.OrderTableChanged{From: 4, To: 9}), by(kabir, 1, 3, took(kabir, order.PaymentUPI, 10000, 1000))},
				{by(kabir, 1, 4, opened(kabir, 2)), by(kabir, 1, 5, took(kabir, order.PaymentUPI, 10000, 3000))},
				{by(kabir, 1, 6, opened(kabir, 0)), by(kabir, 1, 7, took(kabir, order.PaymentCash, 10000, 500))},
				{by(kabir, 1, 8, opened(kabir, 2)), by(kabir, 1, 9, took(kabir, order.PaymentCash, 10000, 200))},
			},
			want: Mine{
				Orders: 4, Tables: 3, Sales: inr(40000), Tips: inr(4700),
				ByTable: []TableTips{{TableID: 2, Tips: inr(3200)}, {TableID: 9, Tips: inr(1000)}, {TableID: 0, Tips: inr(500)}},
				First:   first(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MineOf("INR", 1, kabir, tt.orders)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MineOf\n got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestMineOfRefusesAnotherCurrency(t *testing.T) {
	orders := [][]StampedEvent{{
		by(5, 1, 1, order.PaymentTendered{Method: order.PaymentCash, Amount: money.Amount{Minor: 100, Currency: "USD"}, TenderedBy: 5}),
	}}
	if _, err := MineOf("INR", 1, 5, orders); err == nil {
		t.Fatal("a tender in another currency was added to rupees")
	}
}
