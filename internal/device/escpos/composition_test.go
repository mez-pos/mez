package escpos

import (
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// composition is a venue under the GST composition scheme (ADR-401): a
// GSTIN, and no place of supply, since nothing is split.
var composition = order.InvoiceSeller{Name: "Filter Coffee House", LegalName: "FCH Hospitality LLP",
	TaxProfile: tax.CodeGSTComposition, Registration: "29ABCDE1234F1Z5"}

// TestABillOfSupply: a composition venue's bill is a bill of supply, with its
// GSTIN and, under the title, the rule's words (CGST Rule 5(1)(g)); no tax
// lines. Its duplicate says the same; a bill before payment says it is not
// one yet, and carries no declaration; a credit note calls the number GSTIN.
func TestABillOfSupply(t *testing.T) {
	// The declaration wraps on narrow paper, so it is looked for in the text
	// with its line breaks taken out.
	flat := func(p printed) string { return strings.Join(strings.Fields(p.text()), " ") }

	for _, paper := range []Paper{Paper80, Paper58} {
		bill := SampleReceipt(composition, "INR", "INV/26-27/000042", issued, Template{})
		data, err := bill.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "GSTIN 29ABCDE1234F1Z5", "BILL OF SUPPLY", "No.   INV/26-27/000042", "TOTAL", "Rs.240.00")
		if !strings.Contains(flat(p), tax.CompositionDeclaration) {
			t.Errorf("%d columns: no declaration in:\n%s", paper.Columns, p.text())
		}
		for _, never := range []string{"TAX INVOICE", "Taxable value", "CGST", "SGST", "Place of supply"} {
			if strings.Contains(p.text(), never) {
				t.Errorf("%d columns: a bill of supply says %q:\n%s", paper.Columns, never, p.text())
			}
		}
		title := strings.Index(p.text(), "BILL OF SUPPLY")
		if at := strings.Index(p.text(), "Composition taxable person"); at < title || at > strings.Index(p.text(), "No.   ") {
			t.Errorf("%d columns: the declaration is not under the title, before the number:\n%s", paper.Columns, p.text())
		}
	}

	dup := SampleReceipt(composition, "INR", "INV/26-27/000042", issued, Template{})
	dup.Copy = true
	data, err := dup.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	if p := decode(t, data); !strings.Contains(p.text(), "BILL OF SUPPLY - DUPLICATE") || !strings.Contains(flat(p), tax.CompositionDeclaration) {
		t.Errorf("the duplicate:\n%s", p.text())
	}

	before := SampleReceipt(composition, "INR", "", issued, Template{})
	before.Tenders = nil
	data, err = before.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.has(t, "BILL - NOT A BILL OF SUPPLY")
	if !strings.Contains(flat(p), "Pay at the counter to receive your bill of supply.") {
		t.Errorf("a bill before payment does not say what payment brings:\n%s", p.text())
	}
	if strings.Contains(p.text(), "Composition taxable person") {
		t.Errorf("a bill before payment carries the declaration:\n%s", p.text())
	}

	note := CreditNote{CreditNo: "CN/26-27/000001", AgainstInvoiceNo: "INV/26-27/000042", IssuedAt: issued,
		OrderRef: "ord_7K2M9Q4X", Seller: composition, Reason: "cold",
		Lines:   []order.CreditLine{{LineID: 2, Name: "Masala dosa", Qty: 1, Taxable: inr(12000), Amount: inr(12000)}},
		Taxable: inr(12000), Total: inr(12000)}
	data, err = note.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	decode(t, data).has(t, "CREDIT NOTE", "GSTIN 29ABCDE1234F1Z5")
}
