package payment

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// A UPI pay link (phase 11, Task 4e1, ADR-215). UPI is India's bank-to-bank
// payment rail: a venue has an address, "cafe@okbank", and a phone's UPI app
// opens on a upi://pay link with the address and the amount filled in.
//
// The link moves no money and tells the venue nothing. The guest's bank pays
// the venue's bank, the venue sees it on its own phone, and a member of
// staff records the payment as they do one made at the counter. So building
// a link needs no network, no account and no secret.

var (
	// ErrNoUPIAddress means the venue has not said where it is paid.
	ErrNoUPIAddress = errors.New("the venue has no UPI address")
	// ErrUPICurrency means the amount is not in rupees: UPI moves nothing else.
	ErrUPICurrency = errors.New("UPI pays in INR only")
	// ErrUPIAmount means there is nothing to pay.
	ErrUPIAmount = errors.New("a UPI payment needs an amount above zero")
)

// MaxUPIPayee is the longest payee name a link carries.
const MaxUPIPayee = 60

// upiAddress is a virtual payment address: a handle, "@", and the bank's
// name. NPCI allows letters, digits, dot, hyphen and underscore before the
// "@", and letters and digits after it.
var upiAddress = regexp.MustCompile(`^[A-Za-z0-9._-]{2,256}@[A-Za-z][A-Za-z0-9]{1,63}$`)

// ValidUPIAddress reports whether s has the shape of a UPI address. It
// cannot say whether the address exists: only a payment finds that out.
func ValidUPIAddress(s string) bool { return upiAddress.MatchString(s) }

// UPIAccount is where a venue is paid by UPI.
type UPIAccount struct {
	Address string // "cafe@okbank"
	Payee   string // the name the guest's app shows: the venue's
}

// Set reports whether the venue has given an address.
func (a UPIAccount) Set() bool { return a.Address != "" }

// Link is the upi://pay link for amount, with note as what the guest's app
// shows the payment is for. The amount is written from minor units, "540.00",
// never through a float.
func (a UPIAccount) Link(amount money.Amount, note string) (string, error) {
	if !a.Set() {
		return "", ErrNoUPIAddress
	}
	if !ValidUPIAddress(a.Address) {
		return "", fmt.Errorf("%w: %q is not a UPI address", ErrNoUPIAddress, a.Address)
	}
	if amount.Currency != "INR" {
		return "", fmt.Errorf("%w: not %s", ErrUPICurrency, amount.Currency)
	}
	if amount.Minor <= 0 {
		return "", ErrUPIAmount
	}
	// "540.00 INR" without its currency: the link names the currency itself.
	figure := strings.TrimSuffix(amount.String(), " INR")

	q := url.Values{}
	q.Set("pa", a.Address)
	if payee := clip(a.Payee, MaxUPIPayee); payee != "" {
		q.Set("pn", payee)
	}
	q.Set("am", figure)
	q.Set("cu", "INR")
	if note = clip(note, 50); note != "" {
		q.Set("tn", note)
	}
	// A space as %20: some UPI apps show a "+" as it is written.
	return "upi://pay?" + strings.ReplaceAll(q.Encode(), "+", "%20"), nil
}

// clip trims s and holds it to n characters.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
