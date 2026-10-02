// Package export is phase 09, Task 5a: the files a bookkeeper takes away
// from a closed shift. The tax register, filled column by column from the
// venue's tax profile, so a VAT venue gets VAT columns and a US venue its
// authorities with no code here knowing which regime it is; a Tally
// voucher file; and the day's sales, one row per invoice.
//
// Like the Z-report (ADR-097), a file counts the invoices and credit notes
// issued in the shift, measured as each bill stood at that moment. Nothing
// is recomputed from prices: every figure is an invoice's own tax
// snapshot or a credit note's.
package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// ErrExport means the shift's documents cannot be written out as they are.
var ErrExport = errors.New("export")

// Input is what the files are made from.
type Input struct {
	ShiftID  int64
	Currency string
	Taxes    order.Taxes // the venue's profile and place of supply
	Location *time.Location
	Orders   [][]shift.StampedEvent // the whole streams of the orders touched in the shift
}

// Table is a file's header and rows, every cell already text.
type Table struct {
	Header []string
	Rows   [][]string
}

// CSV writes the table as RFC 4180 CSV, CRLF line endings, which is what
// a spreadsheet and Tally's import both read.
func (t Table) CSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(t.Header); err != nil {
		return nil, err
	}
	if err := w.WriteAll(t.Rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Files are the shift's exports.
type Files struct {
	Register Table // the tax register, by the profile's columns
	Tally    Table // sales and credit note vouchers, for Tally's import
	Sales    Table // the day's sales, one row per invoice or credit note
}

// document is an invoice or credit note issued in the shift, as it stood.
type document struct {
	Number  string
	At      time.Time // local
	Credit  bool
	Against string        // a credit note's invoice
	Detail  tax.TaxResult // per line; a credit note's amounts are negative
	State   order.State   // the bill as it stood when the document was issued
	Paid    []entry       // what the guest paid by, or was refunded to
}

// entry is an amount under a ledger or payment method.
type entry struct {
	Name   string
	Amount money.Amount
}

// Build writes a closed shift's files.
func Build(in Input) (Files, error) {
	if in.Location == nil {
		return Files{}, fmt.Errorf("%w: no timezone", ErrExport)
	}
	if _, err := money.New(0, in.Currency); err != nil {
		return Files{}, fmt.Errorf("%w: %w", ErrExport, err)
	}
	docs, err := documents(in)
	if err != nil {
		return Files{}, err
	}
	var f Files
	if f.Register, err = register(in, docs); err != nil {
		return Files{}, err
	}
	if f.Tally, err = tally(in, docs); err != nil {
		return Files{}, err
	}
	f.Sales = sales(docs)
	return f, nil
}

// documents folds each order's stream and keeps the invoices and credit
// notes issued in the shift, in the order they were issued.
func documents(in Input) ([]document, error) {
	var docs []document
	for _, events := range in.Orders {
		var st order.State
		for i, se := range events {
			before := st
			next, err := order.Resume(st, []order.Event{se.Event}, in.Taxes)
			if err != nil {
				return nil, fmt.Errorf("%w: projecting: %w", ErrExport, err)
			}
			st = next
			if se.ShiftID != in.ShiftID {
				continue
			}
			switch p := se.Event.Payload.(type) {
			case order.InvoiceIssued:
				d, err := invoiceDoc(in, se.Event, p, st)
				if err != nil {
					return nil, err
				}
				docs = append(docs, d)
			case order.CreditNoteIssued:
				docs = append(docs, creditDoc(in, se.Event, p, before, events[i+1:]))
			}
		}
	}
	return docs, nil
}

func invoiceDoc(in Input, e order.Event, p order.InvoiceIssued, st order.State) (document, error) {
	d := document{Number: p.InvoiceNo, At: e.At.In(in.Location), State: st, Detail: st.TaxDetail}
	if len(p.TaxSnapshot) > 0 && string(p.TaxSnapshot) != "{}" {
		if err := json.Unmarshal(p.TaxSnapshot, &d.Detail); err != nil {
			return document{}, fmt.Errorf("%w: invoice %s's tax snapshot: %w", ErrExport, p.InvoiceNo, err)
		}
	}
	if len(d.Detail.Lines) == 0 {
		d.Detail = untaxed(st)
	}
	paid := map[order.PaymentMethod]int64{}
	for _, t := range st.Tenders {
		if t.Status == order.PaymentStatusCaptured || t.Status == order.PaymentStatusRefunded {
			paid[t.Method] += t.Amount.Minor - t.Refunded.Minor
		}
	}
	d.Paid = byMethod(paid, in.Currency, 1)
	return d, nil
}

// creditDoc is a credit note with its lines negative, their HSN and rate
// from the bill, and the refunds that went with it.
func creditDoc(in Input, e order.Event, p order.CreditNoteIssued, before order.State, after []shift.StampedEvent) document {
	d := document{Number: p.CreditNo, At: e.At.In(in.Location), Credit: true, Against: p.AgainstInvoiceNo, State: before}
	d.Detail = tax.TaxResult{Taxable: neg(p.Taxable), Tax: neg(p.Tax), Total: neg(p.Total)}
	for _, cl := range p.Lines {
		lt := tax.LineTax{LineID: cl.LineID, Taxable: neg(cl.Taxable), Tax: neg(cl.Tax), Total: neg(cl.Amount)}
		for _, l := range before.Lines {
			if l.ID == cl.LineID {
				lt.HSN, lt.Rate = l.HSN, l.TaxRate
			}
		}
		for _, c := range cl.Components {
			lt.Components = append(lt.Components, tax.ComponentAmount{Name: c.Name, Rate: c.Rate, Amount: neg(c.Amount)})
		}
		d.Detail.Lines = append(d.Detail.Lines, lt)
	}
	// The command that issues a credit note appends its refunds right
	// after it (ADR-083): those are where the money went.
	back := map[order.PaymentMethod]int64{}
	for _, se := range after {
		r, ok := se.Event.Payload.(order.RefundIssued)
		if !ok {
			break
		}
		for _, t := range before.Tenders {
			if t.ID == r.AgainstPaymentID {
				back[t.Method] += r.Amount.Minor
			}
		}
	}
	d.Paid = byMethod(back, in.Currency, -1)
	return d
}

// untaxed is an untaxed venue's bill as a tax result: every line at its
// due, no tax, so the files still list what was sold.
func untaxed(st order.State) tax.TaxResult {
	zero := money.Amount{Currency: st.Currency}
	r := tax.TaxResult{Taxable: st.Totals.Total, Tax: zero, Total: st.Totals.Total}
	for _, l := range st.Lines {
		if l.Status == order.LineStatusActive {
			r.Lines = append(r.Lines, tax.LineTax{LineID: l.ID, HSN: l.HSN, Taxable: l.Due, Tax: zero, Total: l.Due})
		}
	}
	return r
}

var methodLedgers = []struct {
	m    order.PaymentMethod
	name string
}{{order.PaymentCash, "Cash"}, {order.PaymentCard, "Card"}, {order.PaymentUPI, "UPI"}, {order.PaymentOther, "Other payments"}}

func byMethod(sums map[order.PaymentMethod]int64, currency string, sign int64) []entry {
	var out []entry
	for _, ml := range methodLedgers {
		if v := sums[ml.m]; v != 0 {
			out = append(out, entry{Name: ml.name, Amount: money.Amount{Minor: sign * v, Currency: currency}})
		}
	}
	return out
}

func neg(a money.Amount) money.Amount { return money.Amount{Minor: -a.Minor, Currency: a.Currency} }

// Decimal is an amount as a plain decimal, "-285.30", with no symbol or
// grouping: what a spreadsheet and Tally read as a number.
func Decimal(a money.Amount) string {
	exp := money.Exponent(a.Currency)
	minor := a.Minor
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	if exp == 0 {
		return fmt.Sprintf("%s%d", sign, minor)
	}
	scale := int64(1)
	for range exp {
		scale *= 10
	}
	return fmt.Sprintf("%s%d.%0*d", sign, minor/scale, exp, minor%scale)
}

// Text is a cell of text someone typed (an item's name, a reason) made safe
// for a spreadsheet, which runs a cell starting with =, +, - or @ as a
// formula: such a cell gets a leading apostrophe, which a spreadsheet reads
// as "this is text". Amounts are never passed through it: "-50.00" is a
// number.
func Text(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// rateText is a tax rate as the register prints it: "5%".
func rateText(r tax.Rate) string { return strings.TrimSpace(r.String()) }
