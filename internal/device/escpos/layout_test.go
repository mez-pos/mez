package escpos

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// printed is a byte stream as paper: its lines of text, each with the width
// multiplier it was printed at, and markers for what is not text.
type printed struct {
	lines []printedLine
	cuts  []string // "partial" or "full"
	kicks int
	codes []string // "barcode:DATA", "qr:DATA"
}

type printedLine struct {
	text  string
	width int // character width multiplier
	bold  bool
}

// decode replays a stream the way a printer would (Replay), failing the test
// on a command the package does not know it sent. A barcode is a code, not a
// line of text, so it is left out of the lines.
func decode(t *testing.T, data []byte) printed {
	t.Helper()
	page, err := Replay(data)
	if err != nil {
		t.Fatal(err)
	}
	p := printed{cuts: page.Cuts, kicks: page.Kicks, codes: page.Codes}
	for _, l := range page.Lines {
		if !l.Barcode {
			p.lines = append(p.lines, printedLine{l.Text, l.Width, l.Bold})
		}
	}
	return p
}

// fits fails the test for any line wider than the paper at its size.
func (p printed) fits(t *testing.T, paper Paper) {
	t.Helper()
	for _, l := range p.lines {
		if len(l.text)*l.width > paper.Columns {
			t.Errorf("%q is %d columns at width %d, over %d", l.text, len(l.text), l.width, paper.Columns)
		}
	}
}

func (p printed) text() string {
	var b strings.Builder
	for _, l := range p.lines {
		b.WriteString(l.text + "\n")
	}
	return b.String()
}

func (p printed) has(t *testing.T, want ...string) {
	t.Helper()
	all := p.text()
	for _, w := range want {
		if !strings.Contains(all, w) {
			t.Errorf("no %q in:\n%s", w, all)
		}
	}
}

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

var issued = time.Date(2026, 9, 25, 13, 5, 0, 0, time.FixedZone("IST", 5*3600+1800))

// receipt is a GST cafe bill: two coffees, one with oat milk and a long
// name, a discount on the second, paid by card with a tip.
func receipt() Receipt {
	return Receipt{
		InvoiceNo: "INV/26-27/000042", IssuedAt: issued, OrderRef: "ord_7K2M9Q4X", Table: 4,
		Prices: order.InvoicePrices{
			Seller: order.InvoiceSeller{Name: "Filter Coffee House", LegalName: "FCH Hospitality LLP", TaxProfile: "gst_in",
				Registration: "29ABCDE1234F1Z5", StateCode: "29"},
			Lines: []order.InvoiceLine{
				{LineID: 2, Name: "Filter coffee", Qty: 2, UnitPrice: inr(6000), HSN: "996331",
					Modifiers: []order.Modifier{{Name: "Oat milk"}, {Name: "Extra hot"}},
					Gross:     inr(12000), Discount: inr(0), Due: inr(12600)},
				{LineID: 3, Name: "Masala dosa with extra chutney and podi on the side", Qty: 1, UnitPrice: inr(1200000),
					HSN: "996331", Gross: inr(1200000), Discount: inr(10000), Due: inr(1249500)},
			},
			Gross: inr(1212000), LineDiscounts: inr(10000), OrderDiscounts: inr(0), Subtotal: inr(1202000),
			Taxable: inr(1202000), Tax: inr(60100), Total: inr(1262100),
		},
		Taxes: []tax.ComponentTotal{{Name: "CGST", Rate: 2500, Amount: inr(30050)}, {Name: "SGST", Rate: 2500, Amount: inr(30050)}},
		Tenders: []order.Tender{
			{Method: order.PaymentCard, Amount: inr(100), Tip: inr(0), Status: order.PaymentStatusVoided},
			{Method: order.PaymentCard, Amount: inr(1262100), Tip: inr(5000), Reference: "AUTH 1234", Status: order.PaymentStatusCaptured},
		},
		Template: Template{Footer: []string{"Thank you. Come back soon!"}},
	}
}

func TestReceipt(t *testing.T) {
	for _, paper := range []Paper{Paper80, Paper58} {
		t.Run(fmt.Sprintf("%d columns", paper.Columns), func(t *testing.T) {
			data, err := receipt().Layout(paper)
			if err != nil {
				t.Fatal(err)
			}
			p := decode(t, data)
			p.fits(t, paper)
			p.has(t, "FCH Hospitality LLP", "Filter Coffee House", "GSTIN 29ABCDE1234F1Z5", "TAX INVOICE",
				"No.   INV/26-27/000042", "Date  25/09/2026 13:05", "Place of supply  29", "Table 4  ord_7K2M9Q4X",
				"2 x Filter coffee", "  Oat milk, Extra hot", "  HSN/SAC 996331", "  Discount", "-100.00",
				"Taxable value", "CGST 2.5%", "300.50", "TOTAL", "Rs.12,621.00",
				"Paid Card AUTH 1234", "12,621.00", "Tip, apart from the bill", "Thank you. Come back soon!")
			if strings.Contains(p.text(), "DUPLICATE") || strings.Count(p.text(), "Paid Card") != 1 {
				t.Errorf("an original shows DUPLICATE, or the voided tender:\n%s", p.text())
			}
			if len(p.cuts) != 1 || p.cuts[0] != "partial" || p.kicks != 0 {
				t.Errorf("cuts %v, kicks %d: want one partial cut and no drawer", p.cuts, p.kicks)
			}
			if len(p.codes) != 1 || p.codes[0] != "barcode:INV/26-27/000042" {
				t.Errorf("codes = %v", p.codes)
			}
			// Every amount lines up flush right.
			for _, l := range p.lines {
				if strings.HasSuffix(l.text, ".00") && l.width == 1 && len(l.text) != paper.Columns {
					t.Errorf("amount line %q is not flush right", l.text)
				}
			}
		})
	}
}

func TestReceiptCopyAndBill(t *testing.T) {
	cp := receipt()
	cp.Copy = true
	data, err := cp.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	decode(t, data).has(t, "TAX INVOICE - DUPLICATE", "INV/26-27/000042")

	cash := receipt()
	cash.Kick = true
	data, err = cash.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	if p := decode(t, data); p.kicks != 1 {
		t.Errorf("a cash receipt kicked the drawer %d times", p.kicks)
	}

	bill := receipt()
	bill.InvoiceNo, bill.Tenders = "", nil
	data, err = bill.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.has(t, "BILL - NOT A TAX INVOICE", "Pay at the counter to receive your tax invoice.")
	if strings.Contains(p.text(), "No. ") || len(p.codes) != 0 {
		t.Errorf("a bill carries a number or a barcode:\n%s%v", p.text(), p.codes)
	}
}

func TestKitchenTicket(t *testing.T) {
	k := KitchenTicket{
		Station: "Kitchen", OrderRef: "ord_7K2M9Q4X", Table: 12, Covers: 4, Course: 2, At: issued, By: "Asha",
		Lines: []KitchenLine{
			{Qty: 2, Name: "Masala dosa", Modifiers: []string{"Extra chutney"}, Note: "no onion"},
			{Qty: 1, Name: "Paneer tikka with a very long name that has to wrap"},
		},
		Note: "Birthday table",
	}
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := k.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "TABLE 12", "Kitchen", "ord_7K2M9Q4X  4 covers", "13:05  Asha", "COURSE 2",
			"2 x Masala dosa", "  + Extra chutney", "  NOTE: no onion", "ORDER NOTE: Birthday table")
		if strings.Contains(p.text(), ".00") {
			t.Errorf("a kitchen ticket shows a price:\n%s", p.text())
		}
		if len(p.cuts) != 1 || p.cuts[0] != "full" {
			t.Errorf("cuts = %v, want one full cut", p.cuts)
		}
	}

	k.Reprint, k.Table = true, 0
	data, _ := k.Layout(Paper80)
	decode(t, data).has(t, "REPRINT - ALREADY SENT", "COUNTER")
}

func TestZReport(t *testing.T) {
	counted, diff := inr(1195000), inr(-5000)
	z := ZReport{
		Final: true, Number: 118, Venue: "Filter Coffee House", Device: "Counter 1",
		From: issued.Add(-10 * time.Hour), To: issued, By: "Ravi",
		Orders: 212, Gross: inr(4500000), Discounts: inr(50000), Taxable: inr(4238095),
		Taxes: []ZTax{{Name: "CGST", Amount: inr(105953)}, {Name: "SGST", Amount: inr(105952)}}, Total: inr(4450000),
		Tenders: []ZTender{
			{Method: order.PaymentCash, Count: 80, Amount: inr(1200000)},
			{Method: order.PaymentCard, Count: 60, Amount: inr(1800000)},
			{Method: order.PaymentUPI, Count: 72, Amount: inr(1450000)},
		},
		Tips: inr(62000), Refunds: ZCount{Count: 1, Amount: inr(22000)}, Voids: ZCount{Count: 3, Amount: inr(54000)},
		Comps: ZCount{Count: 2, Amount: inr(12000)}, FirstInvoice: "INV/26-27/001201", LastInvoice: "INV/26-27/001412",
		Invoices: 212, Reprints: 2, CashExpected: inr(1200000), CashCounted: &counted, CashDifference: &diff,
	}
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := z.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "Z-REPORT 118", "Filter Coffee House", "Counter 1", "Ravi", "Orders", "212", "45,000.00", "-500.00",
			"CGST", "44,500.00", "Cash (80)", "Card (60)", "UPI (72)", "Tips, apart from sales", "620.00",
			"Refunds (1)", "Voids (3)", "Comps (2)", "Reprints", "INV/26-27/001201", "INV/26-27/001412",
			"Expected", "12,000.00", "Counted", "11,950.00", "Difference", "-50.00")
	}

	x := ZReport{Venue: "Filter Coffee House", CashExpected: inr(0)}
	data, err := x.Layout(Paper80)
	if err != nil {
		t.Fatal(err)
	}
	p := decode(t, data)
	p.has(t, "X-REPORT", "Not a close: nothing was reset", "not yet")
	if strings.Contains(p.text(), "First") {
		t.Errorf("no invoices, but a first and last are printed:\n%s", p.text())
	}
}

func TestTestPage(t *testing.T) {
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := TestPage(paper, issued, "v1.2.3", true)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "MEZ", "Printer test", "mez v1.2.3", ruler(paper.Columns), "bold", "double height",
			"Cash drawer: kicked")
		if p.kicks != 1 || len(p.cuts) != 1 || len(p.codes) != 2 || p.codes[1] != "qr:"+testQR {
			t.Errorf("kicks %d, cuts %v, codes %v", p.kicks, p.cuts, p.codes)
		}
		if len(ruler(paper.Columns)) != paper.Columns {
			t.Errorf("ruler is %d wide", len(ruler(paper.Columns)))
		}
	}
	data, _ := TestPage(Paper80, issued, "", false)
	if p := decode(t, data); p.kicks != 0 {
		t.Error("the drawer was kicked without -drawer")
	}
}

func TestCreditNote(t *testing.T) {
	c := CreditNote{
		CreditNo: "CN/26-27/000001", AgainstInvoiceNo: "INV/26-27/000042", IssuedAt: issued, OrderRef: "ord_7K2M9Q4X",
		Seller: order.InvoiceSeller{Name: "Filter Coffee House", LegalName: "FCH Hospitality LLP", TaxProfile: "gst_in", Registration: "29ABCDE1234F1Z5", StateCode: "29"},
		Reason: "one dosa was cold",
		Lines: []order.CreditLine{
			{LineID: 2, Name: "Masala dosa", Qty: 1, Taxable: inr(12000), Tax: inr(600), Amount: inr(12600)},
			{LineID: 3, Name: "Filter coffee", Taxable: inr(952), Tax: inr(48), Amount: inr(1000)},
		},
		Taxable: inr(12952), Tax: inr(648), Total: inr(13600),
		Components: []tax.ComponentTotal{{Name: "CGST", Rate: 2500, Taxable: inr(12952), Amount: inr(324)}, {Name: "SGST", Rate: 2500, Taxable: inr(12952), Amount: inr(324)}},
		Refunds:    []CreditRefund{{Method: order.PaymentCash, Amount: inr(13600)}},
		Kick:       true,
	}
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := c.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "CREDIT NOTE", "No.   CN/26-27/000001", "Against invoice INV/26-27/000042", "GSTIN 29ABCDE1234F1Z5",
			"Reason: one dosa was cold", "1 x Masala dosa", "126.00", "Filter coffee (part)", "10.00",
			"Taxable value", "CGST 2.5%", "3.24", "CREDIT", "Rs.136.00", "Refunded to Cash")
		if p.kicks != 1 || len(p.cuts) != 1 || len(p.codes) != 1 || p.codes[0] != "barcode:CN/26-27/000001" {
			t.Errorf("kicks %d, cuts %v, codes %v", p.kicks, p.cuts, p.codes)
		}
	}
}

func TestZReportPhase09Sections(t *testing.T) {
	float, net, counted, diff, paid := inr(200000), inr(61080), inr(252000), inr(-950), inr(5000)
	z := ZReport{
		Final: true, Copy: true, Number: 3, Corrects: 1, Reason: "The cakes were filed under coffee",
		Venue: "Filter Coffee House", Device: "Counter 1", TradingDay: "2026-09-28", By: "Priya",
		Orders: 3, Gross: inr(79000), Discounts: inr(3500), Discounted: ZCount{Count: 2, Amount: inr(3500)},
		Taxable: inr(75500), Total: inr(81030), NetOfCredits: &net,
		Taxes:       []ZTax{{Name: "CGST 2.5%", Amount: inr(1075)}, {Name: "CGST 9%", Amount: inr(1215)}},
		Categories:  []ZLine{{Name: "Coffee", Count: 5, Amount: inr(65100)}, {Name: "Cakes", Count: 1, Amount: inr(15930)}},
		Channels:    []ZLine{{Name: "Counter", Count: 2, Amount: inr(41130)}},
		Staff:       []ZLine{{Name: "Ravi", Count: 2, Amount: inr(41130)}},
		CreditNotes: ZCount{Count: 1, Amount: inr(19950)}, Removed: ZCount{Count: 1, Amount: inr(12000)},
		Unbalanced: &paid, CashFloat: &float, CashIn: inr(0), CashOut: inr(12050), CashSales: inr(28530),
		CashExpected: inr(252950), CashCounted: &counted, CashDifference: &diff, CashReason: "Change given twice to table 4",
	}
	for _, paper := range []Paper{Paper80, Paper58} {
		data, err := z.Layout(paper)
		if err != nil {
			t.Fatal(err)
		}
		p := decode(t, data)
		p.fits(t, paper)
		p.has(t, "Z-REPORT 3", "COPY", "CORRECTS Z-REPORT 1", "The cakes were", "Trading day", "2026-09-28",
			"Discounts (2)", "CGST 2.5%", "CGST 9%", "Less credit notes", "-199.50", "Net sales", "610.80",
			"BY CATEGORY", "Coffee (5)", "651.00", "Cakes (1)", "BY CHANNEL", "Counter (2)", "BY STAFF", "Ravi (2)",
			"Paid for other shifts", "50.00", "Credit notes (1)", "Removed in error (1)",
			"Float", "2,000.00", "Cash out", "-120.50", "Cash sales, net", "285.30", "Expected", "2,529.50",
			"Difference", "-9.50", "Why: Change given twice")
	}
}

// A close says what it carried into the next shift (ADR-384); a report
// issued before that was counted prints no line for it.
func TestZReportOpenOrdersCarried(t *testing.T) {
	carried := ZCount{Count: 2, Amount: inr(45000)}
	for _, tc := range []struct {
		name    string
		carried *ZCount
		want    bool
	}{
		{"two carried", &carried, true},
		{"none carried", &ZCount{Amount: inr(0)}, true},
		{"before it was counted", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			z := ZReport{Final: true, Number: 4, Venue: "Filter Coffee House", CashExpected: inr(0), Carried: tc.carried}
			for _, paper := range []Paper{Paper80, Paper58} {
				data, err := z.Layout(paper)
				if err != nil {
					t.Fatal(err)
				}
				p := decode(t, data)
				p.fits(t, paper)
				if got := strings.Contains(p.text(), "Open orders carried"); got != tc.want {
					t.Fatalf("on %d columns the line printed = %v, want %v:\n%s", paper.Columns, got, tc.want, p.text())
				}
				if tc.carried == &carried {
					p.has(t, "Open orders carried (2)", "450.00")
				}
			}
		})
	}
}
