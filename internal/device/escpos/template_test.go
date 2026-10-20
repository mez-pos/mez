package escpos

import (
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

func TestUnprintable(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want rune // 0: prints as written or spelled in ASCII
	}{
		{"plain text", "Unit 4, MG Road. Tel 080 1234 5678", 0},
		{"a question mark typed", "Open daily? Yes!", 0},
		{"accents are spelled", "Café crème brûlée", 0},
		{"the rupee sign is spelled", "Min. ₹100", 0},
		{"a curly apostrophe is spelled", "We’re open", 0},
		{"Devanagari prints as ?", "स्वागत", 'स'},
		{"an emoji prints as ?", "Thanks \U0001F600", '\U0001F600'},
		{"a newline is not a character of a line", "one\ntwo", '\n'},
		{"a control byte", "bell\a", '\a'},
		{"a Chinese character", "谢谢", '谢'},
		{"nothing", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, bad := Unprintable(tt.in)
			if bad != (tt.want != 0) || got != tt.want {
				t.Errorf("Unprintable(%q) = %q, %v; want %q", tt.in, got, bad, tt.want)
			}
			// It agrees with Printable: what it passes prints with no "?" the
			// writer did not type.
			if !bad && strings.Count(Printable(tt.in), "?") != strings.Count(tt.in, "?") {
				t.Errorf("Printable(%q) = %q has a ? Unprintable missed", tt.in, Printable(tt.in))
			}
		})
	}
}

func templated() Receipt {
	r := receipt()
	r.Template = Template{
		Header: []string{"Unit 4, MG Road, Bengaluru", "Tel 080 1234 5678"},
		Footer: []string{"Thank you. Come back soon!", "Wi-Fi: filtercoffee / brewbrew"},
	}
	return r
}

func TestReceiptHeaderAndFooter(t *testing.T) {
	for _, paper := range []Paper{Paper80, Paper58} {
		t.Run(paper80or58(paper), func(t *testing.T) {
			data, err := templated().Layout(paper)
			if err != nil {
				t.Fatal(err)
			}
			p := decode(t, data)
			p.fits(t, paper)
			text := p.text()
			p.has(t, "Unit 4, MG Road", "Tel 080 1234 5678", "Thank you. Come back soon!", "Wi-Fi: filtercoffee")
			// The header sits under the registration and above the document's
			// title; the footer is last, after the barcode's place.
			order := []string{"GSTIN 29ABCDE1234F1Z5", "Unit 4, MG Road", "Tel 080 1234 5678", "TAX INVOICE", "TOTAL", "Thank you", "Wi-Fi"}
			last := -1
			for _, s := range order {
				i := strings.Index(text, s)
				if i <= last {
					t.Fatalf("%q is out of place in:\n%s", s, text)
				}
				last = i
			}
			if len(p.codes) != 1 {
				t.Errorf("codes = %v: the barcode stays", p.codes)
			}
		})
	}
}

func paper80or58(p Paper) string {
	if p.Columns == Paper58.Columns {
		return "58mm"
	}
	return "80mm"
}

func TestTemplateLinesWrapAndBlanksAreDropped(t *testing.T) {
	r := receipt()
	r.Template = Template{
		Header: []string{"", "  Opposite the old railway station, near the clock tower  ", ""},
		Footer: []string{"   ", "Follow us @filtercoffeehouse for the weekly specials"},
	}
	data, err := r.Layout(Paper58)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.fits(t, Paper58)
	p.has(t, "Opposite the old railway", "station, near the clock", "Follow us @filtercoffeehouse")
}

func TestReceiptOptionalParts(t *testing.T) {
	tests := []struct {
		name    string
		tpl     Template
		table   int64
		absent  []string
		present []string
		codes   int
	}{
		{"all shown", Template{}, 4, nil, []string{"Table 4  ord_7K2M9Q4X", "  Oat milk, Extra hot"}, 1},
		{"no barcode", Template{HideBarcode: true}, 4, nil, []string{"ord_7K2M9Q4X", "Oat milk"}, 0},
		{"no order reference, the table stays", Template{HideOrderRef: true}, 4, []string{"ord_7K2M9Q4X"}, []string{"Table 4", "Oat milk"}, 1},
		{"no order reference at the counter", Template{HideOrderRef: true}, 0, []string{"ord_7K2M9Q4X", "Table"}, []string{"Date  25/09/2026"}, 1},
		{"no modifiers", Template{HideModifiers: true}, 4, []string{"Oat milk", "Extra hot"}, []string{"2 x Filter coffee", "HSN/SAC 996331"}, 1},
		{"none of them", Template{HideBarcode: true, HideOrderRef: true, HideModifiers: true}, 4, []string{"ord_7K2M9Q4X", "Oat milk"}, []string{"Table 4", "TOTAL"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := receipt()
			r.Table, r.Template = tt.table, tt.tpl
			for _, paper := range []Paper{Paper80, Paper58} {
				data, err := r.Layout(paper)
				if err != nil {
					t.Fatal(err)
				}
				p := decode(t, data)
				p.fits(t, paper)
				p.has(t, tt.present...)
				for _, s := range tt.absent {
					if strings.Contains(p.text(), s) {
						t.Errorf("%q is printed:\n%s", s, p.text())
					}
				}
				if len(p.codes) != tt.codes {
					t.Errorf("codes = %v, want %d", p.codes, tt.codes)
				}
			}
		})
	}
}

// The tax profile's own fields are not the venue's to remove: every Template
// switched off still prints them.
func TestNoTemplateRemovesWhatTheTaxProfileRequires(t *testing.T) {
	r := receipt()
	r.Template = Template{HideBarcode: true, HideOrderRef: true, HideModifiers: true}
	data, err := r.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	decode(t, data).has(t, "GSTIN 29ABCDE1234F1Z5", "TAX INVOICE", "No.   INV/26-27/000042", "Date  25/09/2026 13:05",
		"Place of supply  29", "HSN/SAC 996331", "Taxable value", "CGST 2.5%", "SGST 2.5%", "TOTAL")
}

func TestABillCarriesTheTemplateToo(t *testing.T) {
	bill := templated()
	bill.InvoiceNo, bill.Tenders = "", nil
	data, err := bill.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.has(t, "BILL - NOT A TAX INVOICE", "Tel 080 1234 5678", "Pay at the counter to receive your tax invoice.", "Wi-Fi: filtercoffee")
	if len(p.codes) != 0 {
		t.Errorf("a bill carries a barcode: %v", p.codes)
	}
}

func TestCreditNoteHeaderAndFooter(t *testing.T) {
	c := CreditNote{
		CreditNo: "CN/26-27/000001", AgainstInvoiceNo: "INV/26-27/000042", IssuedAt: issued, OrderRef: "ord_7K2M9Q4X",
		Seller:  order.InvoiceSeller{Name: "Filter Coffee House", LegalName: "FCH Hospitality LLP", TaxProfile: "gst_in", Registration: "29ABCDE1234F1Z5"},
		Reason:  "one dosa was cold",
		Lines:   []order.CreditLine{{LineID: 2, Name: "Masala dosa", Qty: 1, Taxable: inr(12000), Tax: inr(600), Amount: inr(12600)}},
		Taxable: inr(12000), Tax: inr(600), Total: inr(12600),
		Components: []tax.ComponentTotal{{Name: "CGST", Rate: 2500, Taxable: inr(12000), Amount: inr(300)}},
		Refunds:    []CreditRefund{{Method: order.PaymentCash, Amount: money.Amount{Minor: 12600, Currency: "INR"}}},
		Header:     []string{"Unit 4, MG Road, Bengaluru"},
		Footer:     []string{"Sorry about the dosa."},
	}
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := c.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		text := p.text()
		p.has(t, "GSTIN 29ABCDE1234F1Z5", "Unit 4, MG Road", "CREDIT NOTE", "Sorry about the dosa.")
		if strings.Index(text, "Unit 4") < strings.Index(text, "GSTIN") || strings.Index(text, "Unit 4") > strings.Index(text, "CREDIT NOTE") ||
			strings.Index(text, "Sorry") < strings.Index(text, "Refunded to Cash") {
			t.Errorf("the header and footer are out of place:\n%s", text)
		}
		if len(p.codes) != 1 {
			t.Errorf("codes = %v: the credit note's barcode stays", p.codes)
		}
	}
}
