package payment

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
)

func rupees(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func TestValidUPIAddress(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"cafe@okbank", true},
		{"filter.coffee-house_1@ybl", true},
		{"9876543210@upi", true},
		{"ab@ok", true},
		{"", false},
		{"cafe", false},
		{"@okbank", false},
		{"a@okbank", false},
		{"cafe@", false},
		{"cafe@1bank", false},
		{"cafe@ok bank", false},
		{"cafe @okbank", false},
		{"cafe@ok@bank", false},
		{"cafe@okbank&am=1", false},
		{"cafe@okbank\n", false},
		{"upi://pay?pa=cafe@okbank", false},
		{"café@okbank", false},
	}
	for _, tt := range tests {
		if got := ValidUPIAddress(tt.address); got != tt.want {
			t.Errorf("ValidUPIAddress(%q) = %v, want %v", tt.address, got, tt.want)
		}
	}
}

func TestUPILink(t *testing.T) {
	account := UPIAccount{Address: "cafe@okbank", Payee: "Filter Coffee House"}
	link, err := account.Link(rupees(54000), "Table T5")
	if err != nil {
		t.Fatal(err)
	}
	want := "upi://pay?am=540.00&cu=INR&pa=cafe%40okbank&pn=Filter%20Coffee%20House&tn=Table%20T5"
	if link != want {
		t.Errorf("link = %s\nwant   %s", link, want)
	}

	// What a UPI app reads back out of it.
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "upi" || u.Host != "pay" || q.Get("pa") != "cafe@okbank" || q.Get("pn") != "Filter Coffee House" ||
		q.Get("am") != "540.00" || q.Get("cu") != "INR" || q.Get("tn") != "Table T5" {
		t.Errorf("parsed = %s %s %v", u.Scheme, u.Host, q)
	}
}

func TestUPILinkAmounts(t *testing.T) {
	account := UPIAccount{Address: "cafe@okbank"}
	tests := []struct {
		minor int64
		want  string
	}{
		{1, "0.01"},
		{5, "0.05"},
		{100, "1.00"},
		{52550, "525.50"},
		{123456789, "1234567.89"},
	}
	for _, tt := range tests {
		link, err := account.Link(rupees(tt.minor), "")
		if err != nil {
			t.Fatalf("%d: %v", tt.minor, err)
		}
		if !strings.Contains(link, "am="+tt.want+"&") {
			t.Errorf("%d paise is written %s, want am=%s", tt.minor, link, tt.want)
		}
		if strings.Contains(link, "pn=") || strings.Contains(link, "tn=") {
			t.Errorf("an empty payee or note is in the link: %s", link)
		}
	}
}

func TestUPILinkRefuses(t *testing.T) {
	tests := []struct {
		name    string
		account UPIAccount
		amount  money.Amount
		want    error
	}{
		{"a venue with no address", UPIAccount{Payee: "Cafe"}, rupees(100), ErrNoUPIAddress},
		{"an address that is not one", UPIAccount{Address: "cafe@ok&am=1"}, rupees(100), ErrNoUPIAddress},
		{"nothing to pay", UPIAccount{Address: "cafe@okbank"}, rupees(0), ErrUPIAmount},
		{"a bill in credit", UPIAccount{Address: "cafe@okbank"}, rupees(-100), ErrUPIAmount},
		{"dollars", UPIAccount{Address: "cafe@okbank"}, money.Amount{Minor: 100, Currency: "USD"}, ErrUPICurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.account.Link(tt.amount, "Table 4"); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestUPILinkCannotBeSteered: nothing in a payee's name or a note can add a
// parameter, change the address or the amount.
func TestUPILinkCannotBeSteered(t *testing.T) {
	account := UPIAccount{Address: "cafe@okbank", Payee: "Cafe & Bar&pa=thief@bank"}
	link, err := account.Link(rupees(54000), "T5&am=1.00#x")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if len(q["pa"]) != 1 || q.Get("pa") != "cafe@okbank" || len(q["am"]) != 1 || q.Get("am") != "540.00" || u.Fragment != "" {
		t.Errorf("the link was steered: %s", link)
	}
	if q.Get("pn") != "Cafe & Bar&pa=thief@bank" || q.Get("tn") != "T5&am=1.00#x" {
		t.Errorf("the words did not survive: %v", q)
	}
}

func TestUPILinkClipsLongWords(t *testing.T) {
	account := UPIAccount{Address: "cafe@okbank", Payee: strings.Repeat("é", 200)}
	link, err := account.Link(rupees(100), strings.Repeat("n", 200))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	if n := len([]rune(u.Query().Get("pn"))); n != MaxUPIPayee {
		t.Errorf("payee is %d characters, want %d", n, MaxUPIPayee)
	}
	if n := len(u.Query().Get("tn")); n != 50 {
		t.Errorf("note is %d characters, want 50", n)
	}
}
