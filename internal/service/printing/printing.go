// Package printing decides what an order's events print (phase 07, Task
// 5b, ADR-081): a receipt when an invoice is issued, a DUPLICATE when one is
// reprinted, a kitchen ticket for each ticket sent. It lays each out with
// escpos, finds its printer, and queues it with printq, all inside the
// transaction that appends the events. So a job exists exactly when its
// event does: a payment that commits has its receipt queued, and one that
// rolls back has none.
package printing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/device/escpos"
	"github.com/byjoellobo/nofiltr-pos/internal/device/printq"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/printjob"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// Input is one append: the order after it, and the events it added.
type Input struct {
	VenueID  int64
	OrderID  int64
	PublicID string
	Location *time.Location // the venue's zone, for the times printed
	State    order.State    // after the append
	Appended []order.Event
	// Stream reads the order's whole stream in the transaction: a reprint
	// prints from the original invoice.issued, which is earlier.
	Stream func(ctx context.Context) ([]order.Event, error)
}

// Plan queues the jobs the appended events call for and returns how many.
// An error means the transaction must roll back; a job that cannot be laid
// out is not an error but a dead job with the reason.
func Plan(ctx context.Context, tx *gen.Queries, in Input, now time.Time) (int, error) {
	var jobs []printq.Job
	for _, e := range in.Appended {
		var (
			j   printq.Job
			err error
			ok  = true
		)
		switch pl := e.Payload.(type) {
		case order.InvoiceIssued:
			j, err = receipt(ctx, tx, in, e, pl, false)
		case order.InvoiceReprinted:
			j, err = duplicate(ctx, tx, in, e, pl)
		case order.TicketSent:
			j, err = kitchen(ctx, tx, in, e, pl)
		case order.CreditNoteIssued:
			j, err = creditNote(ctx, tx, in, e, pl)
		default:
			ok = false
		}
		if err != nil {
			return 0, err
		}
		if ok {
			jobs = append(jobs, j)
		}
	}
	for _, j := range jobs {
		if _, err := printq.EnqueueTx(ctx, tx, j, now); err != nil {
			return 0, err
		}
	}
	return len(jobs), nil
}

func receipt(ctx context.Context, tx *gen.Queries, in Input, e order.Event, pl order.InvoiceIssued, dup bool) (printq.Job, error) {
	route, err := printq.RouteFor(ctx, tx, in.VenueID, printjob.Receipt, "")
	if err != nil {
		return printq.Job{}, err
	}
	var detail tax.TaxResult
	if len(pl.TaxSnapshot) > 0 {
		if err := json.Unmarshal(pl.TaxSnapshot, &detail); err != nil {
			return printq.Job{}, fmt.Errorf("printing: the tax snapshot of %s: %w", pl.InvoiceNo, err)
		}
	}
	r := escpos.Receipt{
		InvoiceNo: pl.InvoiceNo, IssuedAt: e.At.In(in.Location), OrderRef: in.PublicID, Table: in.State.TableID,
		Taxes: detail.Components, Tenders: in.State.Tenders, Copy: dup,
		// The drawer opens with the original, when cash was taken; a copy
		// is paper only.
		Kick: !dup && tookCash(in.State),
	}
	if pl.Prices != nil {
		r.Prices = *pl.Prices
	}
	label := "Receipt " + pl.InvoiceNo
	if dup {
		label = "Duplicate " + pl.InvoiceNo
	}
	data, layoutErr := r.Layout(paper(route))
	return job(in, e, route, printjob.Receipt, label, "", data, layoutErr), nil
}

func duplicate(ctx context.Context, tx *gen.Queries, in Input, e order.Event, pl order.InvoiceReprinted) (printq.Job, error) {
	stream, err := in.Stream(ctx)
	if err != nil {
		return printq.Job{}, err
	}
	for _, orig := range stream {
		if issued, ok := orig.Payload.(order.InvoiceIssued); ok && issued.InvoiceNo == pl.InvoiceNo {
			// The copy shows when the invoice was issued, not when it was
			// copied: it is the same invoice.
			j, err := receipt(ctx, tx, in, orig, issued, true)
			j.EventSeq = e.Seq
			return j, err
		}
	}
	return printq.Job{}, fmt.Errorf("printing: order %d has no invoice %s to reprint", in.OrderID, pl.InvoiceNo)
}

func kitchen(ctx context.Context, tx *gen.Queries, in Input, e order.Event, pl order.TicketSent) (printq.Job, error) {
	route, err := printq.RouteFor(ctx, tx, in.VenueID, printjob.Kitchen, pl.Station)
	if err != nil {
		return printq.Job{}, err
	}
	k := escpos.KitchenTicket{
		Station: title(pl.Station), OrderRef: in.PublicID, Table: in.State.TableID, Covers: in.State.Covers,
		At: e.At.In(in.Location), Note: orderNotes(in.State),
	}
	courses := map[int64]bool{}
	for _, id := range pl.LineIDs {
		l, ok := line(in.State, id)
		if !ok {
			continue
		}
		mods := make([]string, len(l.Modifiers))
		for i, m := range l.Modifiers {
			mods[i] = m.Name
		}
		k.Lines = append(k.Lines, escpos.KitchenLine{Qty: l.Qty, Name: l.Name, Modifiers: mods, Note: lineNotes(in.State, id)})
		courses[l.Course] = true
	}
	if len(courses) == 1 {
		for c := range courses {
			k.Course = c
		}
	}
	where := "counter " + in.PublicID
	if in.State.TableID > 0 {
		where = fmt.Sprintf("table %d", in.State.TableID)
	}
	label := fmt.Sprintf("%s ticket, %s", title(pl.Station), where)
	data, layoutErr := k.Layout(paper(route))
	return job(in, e, route, printjob.Kitchen, label, pl.Station, data, layoutErr), nil
}

// job is a queued job, or, when the layout failed, a dead one that says so.
func job(in Input, e order.Event, route printq.Route, kind printjob.Kind, label, station string, data []byte, layoutErr error) printq.Job {
	j := printq.Job{
		VenueID: in.VenueID, PrinterID: route.PrinterID, Kind: kind, Label: label, Payload: data,
		OrderID: in.OrderID, EventSeq: e.Seq, Station: station,
	}
	if layoutErr != nil {
		j.Payload = []byte(label)
		j.Fail = "could not be laid out: " + layoutErr.Error()
	}
	return j
}

func paper(r printq.Route) escpos.Paper {
	if r.PaperMM == 58 {
		return escpos.Paper58
	}
	return escpos.Paper80
}

func tookCash(s order.State) bool {
	for _, t := range s.Tenders {
		if t.Method == order.PaymentCash && t.Status == order.PaymentStatusCaptured {
			return true
		}
	}
	return false
}

func line(s order.State, id int64) (order.Line, bool) {
	for _, l := range s.Lines {
		if l.ID == id {
			return l, true
		}
	}
	return order.Line{}, false
}

func lineNotes(s order.State, id int64) string {
	var notes []string
	for _, n := range s.Notes {
		if n.Scope == order.ScopeLine && n.LineID == id {
			notes = append(notes, n.Text)
		}
	}
	return strings.Join(notes, "; ")
}

func orderNotes(s order.State) string {
	var notes []string
	for _, n := range s.Notes {
		if n.Scope == order.ScopeOrder {
			notes = append(notes, n.Text)
		}
	}
	return strings.Join(notes, "; ")
}

// title is a station code as a word: "kitchen" is "Kitchen".
func title(code string) string {
	if code == "" {
		return "Kitchen"
	}
	return words(code)
}

// words is a code as the timeline says it: "cold_or_late" is "Cold or
// late". Free text keeps its words, with its first letter raised.
func words(code string) string {
	first, n := utf8.DecodeRuneInString(code)
	if n == 0 {
		return ""
	}
	return string(unicode.ToUpper(first)) + strings.ReplaceAll(code[n:], "_", " ")
}

// creditNote lays out a credit note (ADR-083) for the receipt printer, with
// the refunds appended with it. The drawer opens first when any of the
// money goes back in cash.
func creditNote(ctx context.Context, tx *gen.Queries, in Input, e order.Event, pl order.CreditNoteIssued) (printq.Job, error) {
	route, err := printq.RouteFor(ctx, tx, in.VenueID, printjob.Receipt, "")
	if err != nil {
		return printq.Job{}, err
	}
	c := escpos.CreditNote{
		CreditNo: pl.CreditNo, AgainstInvoiceNo: pl.AgainstInvoiceNo, IssuedAt: e.At.In(in.Location), OrderRef: in.PublicID,
		Reason: words(pl.Reason), Lines: pl.Lines, Taxable: pl.Taxable, Tax: pl.Tax, Total: pl.Total, Components: pl.Components,
	}
	if pl.Seller != nil {
		c.Seller = *pl.Seller
	}
	for _, later := range in.Appended {
		r, ok := later.Payload.(order.RefundIssued)
		if !ok || later.Seq < e.Seq {
			continue
		}
		for _, t := range in.State.Tenders {
			if t.ID != r.AgainstPaymentID {
				continue
			}
			c.Refunds = append(c.Refunds, escpos.CreditRefund{Method: t.Method, Reference: t.Reference, Amount: r.Amount})
			if t.Method == order.PaymentCash {
				c.Kick = true
			}
		}
	}
	data, layoutErr := c.Layout(paper(route))
	return job(in, e, route, printjob.Receipt, "Credit note "+pl.CreditNo, "", data, layoutErr), nil
}
