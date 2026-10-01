package order

import "testing"

// A discount and a refund carry who let them through into the state
// (ADR-117), and an older one without it reads as not recorded.
func TestDiscountsAndRefundsKeepWhoAuthorisedThem(t *testing.T) {
	approved := DiscountApplied{Scope: ScopeOrder, Kind: DiscountPercent, BasisPoints: 1000, Reason: "regular", AuthorisedBy: 7}
	older := DiscountApplied{Scope: ScopeOrder, Kind: DiscountPercent, BasisPoints: 500, Reason: "staff"}
	s := mustProject(t, opened(), dish("Masala dosa", 1, 12000), approved, older)
	if len(s.Discounts) != 2 || s.Discounts[0].AuthorisedBy != 7 || s.Discounts[1].AuthorisedBy != 0 {
		t.Errorf("discounts = %+v", s.Discounts)
	}

	// Part paid, then the line voided: the order owes the guest, who is
	// refunded part of it.
	s = mustProject(t, opened(), dish("Masala dosa", 1, 12000), pay(PaymentCash, 10000),
		LineVoided{LineID: 2, ReasonCode: "out_of_stock", AuthorisedBy: 7},
		RefundIssued{Amount: inr(5000), AgainstPaymentID: 3, Reason: "cold food", AuthorisedBy: 9})
	if len(s.Refunds) != 1 || s.Refunds[0].AuthorisedBy != 9 {
		t.Errorf("refunds = %+v", s.Refunds)
	}
}
