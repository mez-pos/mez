package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/payment"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

func TestPaymentsOfANewVenueAreUnset(t *testing.T) {
	s, _ := newService(t)
	got, err := s.Payments(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.UPIAddress != "" || got.UPIPayee != "" || got.VenueName == "" || got.Currency != "INR" || got.MaxPayee != payment.MaxUPIPayee {
		t.Errorf("payments = %+v", got)
	}
	if _, err := s.Payments(context.Background(), 99); !errors.Is(err, ErrNoVenue) {
		t.Errorf("an unknown venue: %v", err)
	}
}

func TestUpdatePaymentsSavesAndReadsBack(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "  cafe@okbank ", UPIPayee: " Filter Coffee "}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Payments(ctx, 1)
	if err != nil || got.UPIAddress != "cafe@okbank" || got.UPIPayee != "Filter Coffee" {
		t.Fatalf("got %+v, %v", got.PaymentsInput, err)
	}

	// Stored under the keys the order service reads.
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]string{}
	for _, e := range o.Entries {
		if e.Set {
			stored[e.Key] = e.Value
		}
	}
	if stored[svcorder.SettingUPIAddress] != "cafe@okbank" || stored[svcorder.SettingUPIPayee] != "Filter Coffee" {
		t.Errorf("stored = %v", stored)
	}

	// Emptied, the venue takes no UPI again and the keys are gone.
	if err := s.UpdatePayments(ctx, 1, PaymentsInput{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Payments(ctx, 1); got.UPIAddress != "" || got.UPIPayee != "" {
		t.Errorf("after clearing: %+v", got.PaymentsInput)
	}
}

func TestUpdatePaymentsRefuses(t *testing.T) {
	tests := []struct {
		name  string
		in    PaymentsInput
		field string
	}{
		{"an address with no bank", PaymentsInput{UPIAddress: "cafe"}, "upi_address"},
		{"an address that carries a parameter", PaymentsInput{UPIAddress: "cafe@okbank&am=1"}, "upi_address"},
		{"a pay link pasted whole", PaymentsInput{UPIAddress: "upi://pay?pa=cafe@okbank"}, "upi_address"},
		{"a phone number alone", PaymentsInput{UPIAddress: "9876543210"}, "upi_address"},
		{"a name too long", PaymentsInput{UPIAddress: "cafe@okbank", UPIPayee: strings.Repeat("n", payment.MaxUPIPayee+1)}, "upi_payee"},
		{"a name on two lines", PaymentsInput{UPIAddress: "cafe@okbank", UPIPayee: "Filter\tCoffee"}, "upi_payee"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			ctx := context.Background()
			if err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "old@okbank"}); err != nil {
				t.Fatal(err)
			}
			err := s.UpdatePayments(ctx, 1, tt.in)
			var inv *Invalid
			if !errors.As(err, &inv) || inv.Field != tt.field || inv.Message == "" {
				t.Fatalf("err = %v, want it to name %s", err, tt.field)
			}
			// Refused whole: what was there is still there.
			if got, _ := s.Payments(ctx, 1); got.UPIAddress != "old@okbank" || got.UPIPayee != "" {
				t.Errorf("a refused update changed the settings: %+v", got.PaymentsInput)
			}
		})
	}
	s, _ := newService(t)
	if err := s.UpdatePayments(context.Background(), 99, PaymentsInput{}); !errors.Is(err, ErrNoVenue) {
		t.Errorf("an unknown venue: %v", err)
	}
}

// TestPayMode: pay at the table until the owner says otherwise; a save that
// names no mode leaves it as it is; and a mode there is not is refused.
func TestPayMode(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if got, _ := s.Payments(ctx, 1); got.Mode != svcorder.PayAtTable {
		t.Fatalf("a new venue's mode is %q", got.Mode)
	}
	if err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "cafe@okbank", Mode: svcorder.PayFirst}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Payments(ctx, 1); got.Mode != svcorder.PayFirst {
		t.Fatalf("mode = %q, want prepay", got.Mode)
	}
	// A form that does not show the mode saves the address and no mode.
	if err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "other@okbank"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Payments(ctx, 1); got.Mode != svcorder.PayFirst || got.UPIAddress != "other@okbank" {
		t.Errorf("after a save with no mode: %+v", got.PaymentsInput)
	}
	err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "other@okbank", Mode: "tab"})
	var inv *Invalid
	if !errors.As(err, &inv) || inv.Field != "mode" {
		t.Errorf("a mode there is not: %v", err)
	}
	if got, _ := s.Payments(ctx, 1); got.Mode != svcorder.PayFirst {
		t.Errorf("a refused update changed the mode to %q", got.Mode)
	}
	if err := s.UpdatePayments(ctx, 1, PaymentsInput{UPIAddress: "other@okbank", Mode: svcorder.PayAtTable}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Payments(ctx, 1); got.Mode != svcorder.PayAtTable {
		t.Errorf("mode = %q, want pay at the table", got.Mode)
	}
}
