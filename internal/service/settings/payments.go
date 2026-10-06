package settings

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/domain/payment"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// PaymentsInput is where the venue is paid when a guest pays from a phone
// (phase 11, Task 4e1, ADR-215).
type PaymentsInput struct {
	// UPIAddress is the venue's UPI address, "cafe@okbank". Empty is none:
	// a guest's bill then offers no UPI link.
	UPIAddress string `json:"upi_address"`
	// UPIPayee is the name a guest's UPI app shows for the payment. Empty
	// takes the venue's name.
	UPIPayee string `json:"upi_payee"`
	// Mode is when a guest ordering from a phone pays: pay_at_table, or
	// prepay (ADR-225). Empty leaves it as it is, so a form that does not
	// show it cannot change it by saving.
	Mode svcorder.PayMode `json:"mode,omitempty"`
}

// Payments is the payment settings as they stand, and what the screen needs
// to edit them.
type Payments struct {
	PaymentsInput
	// VenueName is what an empty payee reads as.
	VenueName string
	// Currency is the venue's. UPI pays in INR, so a venue in another
	// currency can keep an address and no bill will offer it.
	Currency string
	// MaxPayee is the longest payee name a link carries.
	MaxPayee int
}

// Payments reads where the venue is paid.
func (s *Service) Payments(ctx context.Context, venueID int64) (Payments, error) {
	v, stored, err := rawVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return Payments{}, err
	}
	return Payments{
		PaymentsInput: PaymentsInput{
			UPIAddress: strings.TrimSpace(stored[svcorder.SettingUPIAddress]),
			UPIPayee:   strings.TrimSpace(stored[svcorder.SettingUPIPayee]),
			Mode:       modeOf(stored[svcorder.SettingPayMode]),
		},
		VenueName: v.Name, Currency: v.Currency, MaxPayee: payment.MaxUPIPayee,
	}, nil
}

// UpdatePayments saves where the venue is paid, or nothing. An address is
// checked for its shape only: nothing here can tell whether it exists, and
// the screen says so.
func (s *Service) UpdatePayments(ctx context.Context, venueID int64, in PaymentsInput) error {
	if _, _, err := rawVenue(ctx, s.db.Queries(), venueID); err != nil {
		return err
	}
	values := map[string]string{
		svcorder.SettingUPIAddress: in.UPIAddress,
		svcorder.SettingUPIPayee:   in.UPIPayee,
	}
	if in.Mode != "" {
		values[svcorder.SettingPayMode] = string(in.Mode)
	}
	err := s.Set(ctx, venueID, values)
	var inv *Invalid
	if !errors.As(err, &inv) {
		return err
	}
	switch inv.Field {
	case svcorder.SettingUPIAddress:
		return &Invalid{"upi_address", inv.Message}
	case svcorder.SettingUPIPayee:
		return &Invalid{"upi_payee", inv.Message}
	case svcorder.SettingPayMode:
		return &Invalid{"mode", "Choose pay at the table, or pay first."}
	}
	return err
}

func upiAddress(v string) string {
	if payment.ValidUPIAddress(v) {
		return ""
	}
	return "Enter a UPI address like cafe@okbank: a name, an @, and the bank's handle."
}

func upiPayee(v string) string {
	if utf8.RuneCountInString(v) > payment.MaxUPIPayee {
		return "Keep the name to 60 characters."
	}
	if strings.ContainsAny(v, "\r\n\t") {
		return "The name is one line."
	}
	return ""
}

// modeOf is the stored mode as the order service reads it: unset, or
// anything it does not know, is pay at the table.
func modeOf(stored string) svcorder.PayMode {
	if svcorder.PayMode(strings.TrimSpace(stored)) == svcorder.PayFirst {
		return svcorder.PayFirst
	}
	return svcorder.PayAtTable
}
