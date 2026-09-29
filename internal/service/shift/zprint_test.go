package shift

import (
	"bytes"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/device/printq"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/printjob"
)

type waker struct{ woken int }

func (w *waker) Wake() { w.woken++ }

type zJob struct {
	kind, label, state, lastError string
	payload                       []byte
}

func zJobs(t *testing.T, f *fixture) []zJob {
	t.Helper()
	rows, err := f.db.Reader().QueryContext(ctx,
		"SELECT kind, label, state, last_error, payload FROM print_jobs WHERE kind = ? ORDER BY id", string(printjob.ZReport))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []zJob
	for rows.Next() {
		var j zJob
		if err := rows.Scan(&j.kind, &j.label, &j.state, &j.lastError, &j.payload); err != nil {
			t.Fatal(err)
		}
		out = append(out, j)
	}
	return out
}

func withPrinter(t *testing.T, f *fixture) {
	t.Helper()
	_, err := f.db.Writer().ExecContext(ctx, `INSERT INTO printers (venue_id, name, transport, address, paper_mm, active, is_receipt, created_at, updated_at)
		VALUES (1, 'Counter', 'tcp', '192.168.1.50', 80, 1, 1, '2026-09-27 18:00:00+00:00', '2026-09-27 18:00:00+00:00')`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheCloseWithNoPrinterKeepsTheReportOnTheQueue(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	closeExact(t, f, cashier, open.ID)
	jobs := zJobs(t, f)
	if len(jobs) != 1 || jobs[0].label != "Z-report 1" || jobs[0].state != string(printjob.Dead) || jobs[0].lastError != printq.NoPrinter {
		t.Errorf("jobs = %+v, want Z-report 1 kept dead for want of a printer", jobs)
	}
}

func TestZReportsPrintOnCloseCorrectionAndRequest(t *testing.T) {
	f := newFixture(t)
	withStaff(t, f)
	withPrinter(t, f)
	q := &waker{}
	svc, err := New(Options{DB: f.db, Clock: f.clock, Prints: q})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 200000)))
	w.sale(45000, cash(45000, 5000))
	closeExact(t, f, Meta{VenueID: 1, ActorID: 2, DeviceID: 7}, open.ID)

	manager := Meta{VenueID: 1, ActorID: 2}
	must(f.svc.CorrectZReport(ctx, manager, open.ID, "a late category fix"))
	if err := f.svc.PrintZReport(ctx, manager, 1); err != nil {
		t.Fatal(err)
	}

	jobs := zJobs(t, f)
	if len(jobs) != 3 {
		t.Fatalf("jobs = %+v, want the close, the correction and a copy", jobs)
	}
	wants := []struct {
		label string
		has   []string
	}{
		{"Z-report 1", []string{"Z-REPORT 1", "Priya", "2026-09-27", "BY CATEGORY", "Uncategorised", "Cash (1)", "450.00", "Tips, apart from sales", "50.00", "Float", "2,000.00", "Counted", "2,500.00"}},
		{"Z-report 2", []string{"Z-REPORT 2", "CORRECTS Z-REPORT 1", "a late category fix"}},
		{"Z-report 1 (copy)", []string{"Z-REPORT 1", "COPY"}},
	}
	for i, want := range wants {
		j := jobs[i]
		if j.label != want.label || j.state != string(printjob.Queued) {
			t.Errorf("job %d = %q %s, want %q queued", i, j.label, j.state, want.label)
		}
		for _, s := range want.has {
			if !bytes.Contains(j.payload, []byte(s)) {
				t.Errorf("%s does not say %q", j.label, s)
			}
		}
	}
	if bytes.Contains(jobs[0].payload, []byte("COPY")) {
		t.Error("the report printed with the close says COPY")
	}
	if q.woken < 3 {
		t.Errorf("the queue was woken %d times, want once for each", q.woken)
	}
	if err := f.svc.PrintZReport(ctx, manager, 99); err == nil {
		t.Error("printing Z 99: want an error")
	}
}

// The till's print badge follows the queue through the stream: a
// correction and a copy each say the venue changed.
func TestPrintingAReportTellsTheStream(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	closeExact(t, f, cashier, open.ID)
	before := len(f.changed)
	must(f.svc.CorrectZReport(ctx, cashier, open.ID, "fix"))
	if err := f.svc.PrintZReport(ctx, cashier, 1); err != nil {
		t.Fatal(err)
	}
	if got := len(f.changed) - before; got != 2 {
		t.Errorf("told of %d changes, want one for the correction and one for the copy", got)
	}
}
