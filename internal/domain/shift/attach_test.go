package shift

import (
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// Every event type is decided here, by name, so a new type fails this test
// until someone says whether it needs a shift.
func TestWhichEventsNeedAShift(t *testing.T) {
	want := map[order.Type]bool{
		order.TypeOrderOpened:       true,
		order.TypeLineAdded:         true,
		order.TypeLineQtyChanged:    true,
		order.TypeLineModified:      true,
		order.TypeLineVoided:        true,
		order.TypeLineComped:        true,
		order.TypeDiscountApplied:   true,
		order.TypeNoteAdded:         true,
		order.TypeCourseFired:       true,
		order.TypeTicketSent:        true,
		order.TypePaymentTendered:   true,
		order.TypePaymentVoided:     true,
		order.TypeInvoiceIssued:     true,
		order.TypeRefundIssued:      true,
		order.TypeOrderClosed:       true,
		order.TypeOrderReopened:     true,
		order.TypeOrderHeld:         true,
		order.TypeOrderResumed:      true,
		order.TypeCreditNoteIssued:  true,
		order.TypeOrderTableChanged: true,
		order.TypeTablesJoined:      true,
		order.TypeTablesParted:      true,
		order.TypeInvoiceReprinted:  false,
		order.TypeTicketBumped:      false,
		order.TypeTicketRecalled:    false,
	}
	for _, typ := range order.Types() {
		expected, ok := want[typ]
		if !ok {
			t.Errorf("%s: not decided here; add it with whether it needs a shift", typ)
			continue
		}
		if got := NeedsShift(typ); got != expected {
			t.Errorf("NeedsShift(%s) = %v, want %v", typ, got, expected)
		}
	}
	if !NeedsShift("something.new") {
		t.Error("an unknown type must need a shift")
	}
}
