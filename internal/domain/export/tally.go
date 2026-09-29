package export

import (
	"fmt"
	"strings"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// tallyHeader is the voucher file's columns: one row per ledger entry,
// the rows of a voucher sharing its date, type and number, as TallyPrime's
// import from a spreadsheet maps them.
var tallyHeader = []string{"Date", "Voucher Type", "Voucher No", "Reference", "Ledger", "Debit", "Credit"}

// customerCredit balances a credit note whose money did not all go back
// in this shift: what is left is owed to the guest.
const customerCredit = "Customer credit"

// tally writes a Sales voucher per invoice and a Credit Note voucher per
// credit note. The payment ledgers are debited, the sales ledgers by rate
// and the tax ledgers by component and rate are credited; a credit note is
// the same reversed. Every voucher balances, or the export fails.
func tally(in Input, docs []document) (Table, error) {
	t := Table{Header: tallyHeader}
	for _, d := range docs {
		debits := d.Paid
		credits := salesLedgers(in.Currency, d.Detail)
		dr, cr := total(debits), total(credits)
		if dr != cr {
			if !d.Credit {
				return Table{}, fmt.Errorf("%w: invoice %s's payments come to %d, its sales and tax to %d", ErrExport, d.Number, dr, cr)
			}
			debits = append(debits, entry{Name: customerCredit, Amount: money.Amount{Minor: cr - dr, Currency: in.Currency}})
		}
		kind, ref := "Sales", ""
		if d.Credit {
			kind, ref = "Credit Note", d.Against
		}
		date := d.At.Format("02-01-2006")
		for _, e := range debits {
			t.Rows = append(t.Rows, side(date, kind, d.Number, ref, e, true))
		}
		for _, e := range credits {
			t.Rows = append(t.Rows, side(date, kind, d.Number, ref, e, false))
		}
	}
	return t, nil
}

// salesLedgers is a document's credit side: its value by rate, then its
// tax by component and rate, in the order they first appear.
func salesLedgers(currency string, detail tax.TaxResult) []entry {
	var names []string
	sums := map[string]int64{}
	add := func(name string, v int64) {
		if _, ok := sums[name]; !ok {
			names = append(names, name)
		}
		sums[name] += v
	}
	for _, lt := range detail.Lines {
		switch {
		case lt.Rate == 0 && len(lt.Components) == 0 && lt.Tax.IsZero() && detail.Profile == "":
			add("Sales", lt.Taxable.Minor)
		case len(lt.Components) == 0:
			add("Sales, exempt", lt.Taxable.Minor)
		default:
			add("Sales @ "+rateText(lt.Rate), lt.Taxable.Minor)
		}
	}
	for _, lt := range detail.Lines {
		for _, c := range lt.Components {
			add(strings.ToUpper(c.Name)+" @ "+rateText(c.Rate), c.Amount.Minor)
		}
	}
	out := make([]entry, 0, len(names))
	for _, n := range names {
		if sums[n] != 0 {
			out = append(out, entry{Name: n, Amount: money.Amount{Minor: sums[n], Currency: currency}})
		}
	}
	return out
}

// side is a ledger row. A negative amount belongs on the other side.
func side(date, kind, number, ref string, e entry, debit bool) []string {
	amount := e.Amount
	if amount.Minor < 0 {
		amount, debit = neg(amount), !debit
	}
	row := []string{date, kind, number, ref, e.Name, "", ""}
	if debit {
		row[5] = decimal(amount)
	} else {
		row[6] = decimal(amount)
	}
	return row
}

func total(es []entry) int64 {
	var sum int64
	for _, e := range es {
		sum += e.Amount.Minor
	}
	return sum
}

// salesHeader is the day's sales: one row per invoice or credit note.
var salesHeader = []string{
	"Date", "Time", "Type", "Number", "Against", "Channel", "Opened by",
	"Gross", "Discounts", "Taxable", "Tax", "Total", "Cash", "Card", "UPI", "Other",
}

// sales writes the day's sales. A credit note's figures are negative, and
// its gross and discounts are blank: it credits value, not items as priced.
func sales(docs []document) Table {
	t := Table{Header: salesHeader}
	for _, d := range docs {
		st := d.State
		row := []string{
			d.At.Format("2006-01-02"), d.At.Format("15:04"), "Invoice", d.Number, d.Against,
			string(st.Channel), fmt.Sprint(st.OpenedBy), "", "",
			decimal(d.Detail.Taxable), decimal(d.Detail.Tax), decimal(d.Detail.Total),
		}
		if d.Credit {
			row[2] = "Credit note"
		} else {
			var gross, discounts int64
			for _, l := range st.Lines {
				if l.Status == order.LineStatusActive {
					gross += l.Gross.Minor
					discounts += l.Discount.Minor + l.OrderDiscount.Minor
				}
			}
			row[7] = decimal(money.Amount{Minor: gross, Currency: d.Detail.Total.Currency})
			row[8] = decimal(money.Amount{Minor: discounts, Currency: d.Detail.Total.Currency})
		}
		for _, ml := range methodLedgers {
			cell := ""
			for _, e := range d.Paid {
				if e.Name == ml.name {
					cell = decimal(e.Amount)
				}
			}
			row = append(row, cell)
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}
