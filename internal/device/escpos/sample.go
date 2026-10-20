package escpos

import (
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// SampleReceipt is a made-up paid bill under a venue's own details, to show
// what a Template does to a receipt before a real one is printed: two items,
// one with a choice, tax under the venue's profile, paid in cash. The figures
// are invented and add up; nothing here is a sale. The venue's real next
// invoice number is passed in so the preview shows its real format.
func SampleReceipt(seller order.InvoiceSeller, currency, invoiceNo string, at time.Time, tpl Template) Receipt {
	unit := int64(1)
	for i := 0; i < money.Exponent(currency); i++ {
		unit *= 10
	}
	amt := func(major int64) money.Amount { return money.Amount{Minor: major * unit, Currency: currency} }

	// 240 of food, and under a regime that charges tax, 5% on top of it.
	hsn := ""
	var taxes []tax.ComponentTotal
	taxAmount := int64(0)
	switch seller.TaxProfile {
	case "gst_in":
		hsn = "996331"
		taxes = []tax.ComponentTotal{{Name: "CGST", Rate: 2500, Taxable: amt(240), Amount: amt(6)}, {Name: "SGST", Rate: 2500, Taxable: amt(240), Amount: amt(6)}}
		taxAmount = 12
	case "vat_uk":
		taxes = []tax.ComponentTotal{{Name: "VAT", Rate: 5000, Taxable: amt(240), Amount: amt(12)}}
		taxAmount = 12
	case "sales_tax_us":
		taxes = []tax.ComponentTotal{{Name: "Sales tax", Rate: 5000, Taxable: amt(240), Amount: amt(12)}}
		taxAmount = 12
	}
	total := amt(240 + taxAmount)

	return Receipt{
		InvoiceNo: invoiceNo, IssuedAt: at, OrderRef: "ord_SAMPLE01", Table: 4,
		Prices: order.InvoicePrices{
			Seller: seller,
			Lines: []order.InvoiceLine{
				{LineID: 2, Name: "Filter coffee", Qty: 2, UnitPrice: amt(60), HSN: hsn,
					Modifiers: []order.Modifier{{Name: "Oat milk"}}, Gross: amt(120), Discount: amt(0), Due: amt(120)},
				{LineID: 3, Name: "Masala dosa", Qty: 1, UnitPrice: amt(120), HSN: hsn, Gross: amt(120), Discount: amt(0), Due: amt(120)},
			},
			Gross: amt(240), LineDiscounts: amt(0), OrderDiscounts: amt(0), Subtotal: amt(240),
			Taxable: amt(240), Tax: amt(taxAmount), Total: total,
		},
		Taxes:    taxes,
		Tenders:  []order.Tender{{Method: order.PaymentCash, Amount: total, Tip: amt(0), Status: order.PaymentStatusCaptured}},
		Template: tpl,
	}
}
