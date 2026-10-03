package escpos

import (
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/order"
)

func TestSampleReceiptLaysOutUnderEveryProfile(t *testing.T) {
	at := time.Date(2026, 10, 3, 13, 5, 0, 0, time.UTC)
	tests := []struct {
		profile string
		want    []string
		absent  []string
	}{
		{"gst_in", []string{"TAX INVOICE", "CGST 2.5%", "SGST 2.5%", "HSN/SAC 996331", "Rs.252.00", "Taxable value"}, nil},
		{"vat_uk", []string{"VAT RECEIPT", "VAT 5%", "Rs.252.00"}, []string{"HSN/SAC"}},
		{"sales_tax_us", []string{"INVOICE", "Sales tax 5%", "Rs.252.00"}, []string{"HSN/SAC"}},
		{"none", []string{"INVOICE", "Subtotal", "Rs.240.00"}, []string{"Taxable value", "HSN/SAC", "CGST"}},
	}
	for _, tt := range tests {
		for _, paper := range []Paper{Paper80, Paper58} {
			t.Run(tt.profile+"/"+paper80or58(paper), func(t *testing.T) {
				seller := order.InvoiceSeller{Name: "Cafe", LegalName: "Cafe Pvt Ltd", TaxProfile: tt.profile, Registration: "REG123"}
				data, err := SampleReceipt(seller, "INR", "INV/26-27/000007", at, Template{}).Layout(paper)
				if err != nil {
					t.Fatal(err)
				}
				p := decode(t, data)
				p.fits(t, paper)
				p.has(t, append(tt.want, "Cafe Pvt Ltd", "2 x Filter coffee", "Oat milk", "1 x Masala dosa", "No.   INV/26-27/000007", "Paid Cash", "Table 4")...)
				for _, s := range tt.absent {
					if strings.Contains(p.text(), s) {
						t.Errorf("%q is printed:\n%s", s, p.text())
					}
				}
			})
		}
	}
}

// The sample's figures add up, in a currency with two decimals and one with none.
func TestSampleReceiptAddsUp(t *testing.T) {
	for _, currency := range []string{"INR", "JPY", "USD"} {
		r := SampleReceipt(order.InvoiceSeller{Name: "C", LegalName: "C", TaxProfile: "gst_in"}, currency, "N", time.Time{}, Template{})
		p := r.Prices
		var gross, tax int64
		for _, l := range p.Lines {
			gross += l.Gross.Minor
		}
		for _, c := range r.Taxes {
			tax += c.Amount.Minor
		}
		if gross != p.Gross.Minor || p.Taxable.Minor != gross || p.Tax.Minor != tax || p.Total.Minor != gross+tax || r.Tenders[0].Amount != p.Total {
			t.Errorf("%s: gross %d, taxable %d, tax %d, total %d, paid %d", currency, gross, p.Taxable.Minor, p.Tax.Minor, p.Total.Minor, r.Tenders[0].Amount.Minor)
		}
		if p.Total.Currency != currency {
			t.Errorf("currency %q, want %q", p.Total.Currency, currency)
		}
	}
}

func TestSampleReceiptTakesTheTemplate(t *testing.T) {
	tpl := Template{Header: []string{"Unit 4, MG Road"}, Footer: []string{"Thank you!"}, HideBarcode: true, HideModifiers: true}
	data, err := SampleReceipt(order.InvoiceSeller{Name: "C", LegalName: "C", TaxProfile: "none"}, "INR", "INV/1", time.Time{}, tpl).Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.has(t, "Unit 4, MG Road", "Thank you!")
	if strings.Contains(p.text(), "Oat milk") || len(p.codes) != 0 {
		t.Errorf("the template's switches were ignored:\n%s%v", p.text(), p.codes)
	}
}
