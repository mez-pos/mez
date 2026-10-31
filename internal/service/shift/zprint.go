package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/device/escpos"
	"github.com/mez-pos/mez/internal/device/printq"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/printjob"
	"github.com/mez-pos/mez/internal/domain/zreport"
)

// PrintQueue is the print queue as the shift needs it: woken after a
// transaction that queued a Z-report commits. Nil prints nothing.
type PrintQueue interface {
	Wake()
}

// channelWords are the channels as a report prints them.
var channelWords = map[string]string{
	"counter": "Counter", "qr_table": "QR at the table", "web": "Web", "kiosk": "Kiosk", "aggregator": "Delivery apps",
}

// layoutZ is a stored report as the printer lays it out. Every figure is
// the document's; nothing is added up here.
func layoutZ(doc ZDocument, venueName, device string, loc *time.Location, copy bool) escpos.ZReport {
	r := doc.Report
	d := doc.Drawer
	counted, diff, float, net := d.Counted, d.Variance, d.Float, r.Sales.NetOfCredits
	z := escpos.ZReport{
		Final: true, Copy: copy, Venue: venueName, Device: device, Number: doc.Number,
		Corrects: doc.Corrects, Reason: doc.Reason, TradingDay: doc.Shift.TradingDay,
		From: doc.Shift.OpenedAt.In(loc), To: doc.Shift.ClosedAt.In(loc), By: staffName(doc.Names, doc.Shift.ClosedBy),
		Orders: r.Sales.Bills, Gross: r.Sales.Gross, Discounts: r.Sales.Discounts, Taxable: r.Sales.Taxable,
		Total: r.Sales.Total, Tips: r.Tips,
		Discounted:  escpos.ZCount(r.Discounts),
		Refunds:     escpos.ZCount(r.Refunds),
		CreditNotes: escpos.ZCount(r.CreditNotes),
		Voids:       escpos.ZCount(r.Voids),
		Comps:       escpos.ZCount(r.Comps),
		Removed:     removed(r.Removed, r.Currency),
		Reprints:    r.Reprints,
		Invoices:    r.Invoices.Count, FirstInvoice: r.Invoices.First, LastInvoice: r.Invoices.Last,
		CashFloat: &float, CashIn: d.CashIn, CashOut: d.CashOut, CashExpected: d.Expected,
		CashCounted: &counted, CashDifference: &diff, CashReason: d.Reason,
	}
	if cash, err := d.Sales.Net(); err == nil {
		z.CashSales = cash
	}
	if r.CreditNotes.Count > 0 {
		z.NetOfCredits = &net
	}
	if !r.Balance.Difference.IsZero() {
		diff := r.Balance.Difference
		z.Unbalanced = &diff
	}
	if doc.Carried != nil {
		c := escpos.ZCount(*doc.Carried)
		z.Carried = &c
	}
	for _, t := range r.Taxes {
		z.Taxes = append(z.Taxes, escpos.ZTax{Name: t.Name + " " + t.Rate.String(), Amount: t.Amount})
	}
	for _, p := range r.Payments {
		z.Tenders = append(z.Tenders, escpos.ZTender{Method: p.Method, Count: p.Count, Amount: p.Net})
	}
	z.Categories = zLines(r.ByCategory, nil)
	z.Channels = zLines(r.ByChannel, func(g zreport.Group) string {
		if w, ok := channelWords[g.Key]; ok {
			return w
		}
		return g.Name
	})
	z.Staff = zLines(r.ByStaff, nil)
	return z
}

func zLines(gs []zreport.Group, name func(zreport.Group) string) []escpos.ZLine {
	out := make([]escpos.ZLine, len(gs))
	for i, g := range gs {
		n := g.Name
		if name != nil {
			n = name(g)
		}
		out[i] = escpos.ZLine{Name: n, Count: g.Qty, Amount: g.Total}
	}
	return out
}

func removed(ts []zreport.StaffTally, currency string) escpos.ZCount {
	out := escpos.ZCount{Amount: money.Amount{Currency: currency}}
	for _, t := range ts {
		out.Count += t.Count
		out.Amount.Minor += t.Amount.Minor
	}
	return out
}

// queueZ lays a report out and queues it on the venue's report printer, in
// the caller's transaction. With no printer it is kept dead on the queue
// with the reason, as any job is, so it can be printed once one is set up.
func queueZ(ctx context.Context, q *gen.Queries, venueID, deviceID int64, v venue, doc ZDocument, copy bool, now time.Time) error {
	route, err := printq.RouteFor(ctx, q, venueID, printjob.ZReport, "")
	if err != nil {
		return err
	}
	device := ""
	if deviceID > 0 {
		switch row, err := q.DeviceByID(ctx, deviceID); {
		case err == nil && row.VenueID == venueID:
			device = row.Name
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("reading device %d: %w", deviceID, err)
		}
	}
	paper := escpos.Paper80
	if route.PaperMM == 58 {
		paper = escpos.Paper58
	}
	label := fmt.Sprintf("Z-report %d", doc.Number)
	if copy {
		label += " (copy)"
	}
	j := printq.Job{VenueID: venueID, PrinterID: route.PrinterID, Kind: printjob.ZReport, Label: label}
	data, err := layoutZ(doc, v.name, device, v.location, copy).Layout(paper)
	if err != nil {
		j.Payload, j.Fail = []byte(label), "could not be laid out: "+err.Error()
	} else {
		j.Payload = data
	}
	if _, err := printq.EnqueueTx(ctx, q, j, now); err != nil {
		return err
	}
	return nil
}

// PrintZReport prints a copy of a report, marked COPY, on the venue's
// report printer.
func (s *Service) PrintZReport(ctx context.Context, meta Meta, number int64) error {
	if err := meta.validate(); err != nil {
		return err
	}
	z, err := s.ZReport(ctx, meta.VenueID, number)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		return queueZ(ctx, q, meta.VenueID, meta.DeviceID, v, z.Document, true, now)
	})
	if err != nil {
		return fmt.Errorf("printing Z-report %d: %w", number, err)
	}
	s.wake()
	s.onChange(meta.VenueID)
	return nil
}

func (s *Service) wake() {
	if s.prints != nil {
		s.prints.Wake()
	}
}

// staffName is who a report names for an ID, or a plain placeholder.
func staffName(names map[int64]string, id int64) string {
	if n := strings.TrimSpace(names[id]); n != "" {
		return n
	}
	return fmt.Sprintf("Staff %d", id)
}
