package order

import (
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
)

var addingAt = time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC)

// TestAddingToNoTicket: the lines as the first on a ticket of their own,
// with tax where the venue adds it, and none added where prices hold it.
func TestAddingToNoTicket(t *testing.T) {
	tests := []struct {
		name  string
		taxes Taxes
		lines []LineAdded
		want  int64
	}{
		{"an untaxed venue", Taxes{}, []LineAdded{dish("Dosa", 2, 12000)}, 24000},
		{"GST on top", gstCafe(tax.Exclusive), []LineAdded{dish("Dosa", 2, 12000)}, 25200},
		{"GST in the price", gstCafe(tax.Inclusive), []LineAdded{dish("Dosa", 2, 12000)}, 24000},
		{"two lines", gstCafe(tax.Exclusive), []LineAdded{dish("Dosa", 1, 12000), dish("Coffee", 1, 4000)}, 16800},
		// 5% of 33.00 is 1.65, but GST is two halves of 0.825 each, and each
		// rounds half to even, to 0.82: 1.64. A second calculation here would
		// have said 34.65 and been a paisa off the bill, which is why there
		// is no second calculation.
		{"an amount whose tax does not come out even", gstCafe(tax.Exclusive), []LineAdded{dish("Tea", 1, 3300)}, 3464},
		{"a choice that costs more", gstCafe(tax.Exclusive), []LineAdded{{
			Name: "Flat white", Qty: 2, UnitPrice: inr(18000), Modifiers: []Modifier{{ModifierID: 1, Name: "Oat", Price: inr(4000)}},
			Station: "bar", TaxRate: 5000, HSN: "996331",
		}}, 46200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Adding(nil, tt.lines, tt.taxes, "INR", 4, addingAt)
			if err != nil {
				t.Fatal(err)
			}
			if got != inr(tt.want) {
				t.Errorf("adds %v, want %v", got, inr(tt.want))
			}
		})
	}
}

// TestAddingIsWhatTheBillRisesBy: for a ticket with lines, a discount and a
// part payment already on it, what Adding says is exactly the difference
// between the bill before and the bill after the lines are really added.
func TestAddingIsWhatTheBillRisesBy(t *testing.T) {
	for _, pricing := range []tax.Pricing{tax.Exclusive, tax.Inclusive} {
		taxes := gstCafe(pricing)
		prior := stream(
			opened(),
			dish("Dosa", 1, 12000),
			dish("Tea", 3, 3300),
			DiscountApplied{Scope: ScopeOrder, Kind: DiscountPercent, BasisPoints: 1000, Reason: "regular"},
			PaymentTendered{Method: PaymentCash, Amount: inr(5000), Tip: inr(0), TenderedBy: 7},
		)
		before, err := Project(prior, taxes)
		if err != nil {
			t.Fatal(err)
		}
		lines := []LineAdded{dish("Coffee", 2, 4700), dish("Vada", 1, 3300)}

		said, err := Adding(prior, lines, taxes, "INR", 4, addingAt)
		if err != nil {
			t.Fatal(err)
		}

		payloads := []Payload{}
		for _, l := range lines {
			payloads = append(payloads, l)
		}
		after, err := Project(append(prior, numbered(int64(len(prior)), payloads...)...), taxes)
		if err != nil {
			t.Fatal(err)
		}
		rise := after.Totals.Total.Minor - before.Totals.Total.Minor
		if said.Minor != rise || said.Currency != "INR" {
			t.Errorf("%s: Adding said %d, the bill rose by %d", pricing, said.Minor, rise)
		}
		if owed := after.Totals.BalanceDue.Minor - before.Totals.BalanceDue.Minor; owed != rise {
			t.Errorf("%s: the balance rose by %d and the total by %d", pricing, owed, rise)
		}
	}
}

// numbered is payloads as events following seq.
func numbered(after int64, payloads ...Payload) []Event {
	out := make([]Event, len(payloads))
	for i, p := range payloads {
		out[i] = NewEvent(after+int64(i)+1, addingAt, 0, 0, p)
	}
	return out
}

func TestAddingRefuses(t *testing.T) {
	settled := stream(opened(), dish("Dosa", 1, 12000),
		PaymentTendered{Method: PaymentCash, Amount: inr(12000), Tip: inr(0), TenderedBy: 7},
		InvoiceIssued{InvoiceNo: "INV/26-27/000001", Series: "INV", TaxSnapshot: []byte(`{}`)})
	tests := []struct {
		name  string
		prior []Event
		lines []LineAdded
		want  error
	}{
		{"nothing to add", nil, nil, ErrInvalidPayload},
		{"a line of none", nil, []LineAdded{dish("Dosa", 0, 12000)}, ErrInvalidPayload},
		{"a line in another currency", nil, []LineAdded{{Name: "Dosa", Qty: 1, UnitPrice: money.Amount{Minor: 100, Currency: "USD"}}}, ErrInvalidPayload},
		{"a ticket on hold", stream(opened(), OrderHeld{}), []LineAdded{dish("Dosa", 1, 12000)}, ErrOrderHeld},
		{"a bill already invoiced", settled, []LineAdded{dish("Dosa", 1, 12000)}, ErrOrderInvoiced},
		{"a bill closed", append(settled, numbered(int64(len(settled)), OrderClosed{ClosedBy: 7})...), []LineAdded{dish("Dosa", 1, 12000)}, ErrOrderClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Adding(tt.prior, tt.lines, Taxes{}, "INR", 4, addingAt)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
