package zreport

import (
	"testing"

	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// Tips are tallied by who took the payment (the staff report's Tips column,
// D17). A payment voided takes its tip back from whoever took it.
//
//	Bill 1  Kabir (5) takes 126.00 cash with a 20.00 tip
//	Bill 2  Kabir (5) takes 126.00 card with a 10.00 tip
//	Bill 3  Meera (3) takes 126.00 cash with a 15.00 tip, voided: wrong bill;
//	        then 126.00 UPI with a 5.00 tip
//	Bill 4  Meera (3) takes 126.00 cash with no tip
func tippedDay(t *testing.T) Input {
	t.Helper()
	paid := func(no string, tenders ...func(b *bill)) []shift.StampedEvent {
		b := newBill(t, 1, order.ChannelCounter, 3)
		b.line(2, "Espresso", 1, 12000, gst5)
		for _, tender := range tenders {
			tender(b)
		}
		b.invoice(no)
		return b.events
	}
	take := func(by int64, method order.PaymentMethod, tip int64) func(b *bill) {
		return func(b *bill) {
			b.as(by, order.PaymentTendered{Method: method, Amount: inr(12600), Tip: inr(tip), TenderedBy: by})
		}
	}
	voided := func(by int64, tip int64) func(b *bill) {
		return func(b *bill) {
			id := b.as(by, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(12600), Tip: inr(tip), TenderedBy: by})
			b.as(2, order.PaymentVoided{PaymentID: id, Reason: "wrong bill", AuthorisedBy: 2})
		}
	}
	return Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Categories: menu, Orders: [][]shift.StampedEvent{
		paid("INV/26-27/000001", take(5, order.PaymentCash, 2000)),
		paid("INV/26-27/000002", take(5, order.PaymentCard, 1000)),
		paid("INV/26-27/000003", voided(3, 1500), take(3, order.PaymentUPI, 500)),
		paid("INV/26-27/000004", take(3, order.PaymentCash, 0)),
	}}
}

func TestTipsByWhoTookThePayment(t *testing.T) {
	r, err := Build(tippedDay(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.Tips != inr(3500) {
		t.Fatalf("tips = %s, want 35.00", r.Tips)
	}
	// The most first; a payment with no tip is not counted.
	want := []StaffTally{
		{StaffID: 5, Tally: Tally{Count: 2, Amount: inr(3000)}},
		{StaffID: 3, Tally: Tally{Count: 1, Amount: inr(500)}},
	}
	if !equalSlices(r.TipsBy, want) {
		t.Errorf("tips by = %+v\n   want %+v", r.TipsBy, want)
	}
	sum := int64(0)
	for _, s := range r.TipsBy {
		sum += s.Amount.Minor
	}
	if sum != r.Tips.Minor {
		t.Errorf("tips by person come to %d, the tips to %s", sum, r.Tips)
	}
}

func TestSomeoneWhoseOnlyTipWasVoidedIsNotListed(t *testing.T) {
	b := newBill(t, 1, order.ChannelCounter, 3)
	b.line(2, "Espresso", 1, 12000, gst5)
	id := b.as(3, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(12600), Tip: inr(1500), TenderedBy: 3})
	b.as(2, order.PaymentVoided{PaymentID: id, Reason: "wrong bill", AuthorisedBy: 2})
	b.as(5, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(12600), Tip: inr(0), TenderedBy: 5})
	b.invoice("INV/26-27/000001")
	r, err := Build(Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Categories: menu, Orders: [][]shift.StampedEvent{b.events}})
	if err != nil {
		t.Fatal(err)
	}
	if r.TipsBy == nil || len(r.TipsBy) != 0 {
		t.Errorf("tips by = %+v, want an empty list", r.TipsBy)
	}
}
