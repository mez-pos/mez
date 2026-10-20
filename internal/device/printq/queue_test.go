package printq

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/printjob"
)

var start = time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)

// clock is a time the test moves by hand.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) add(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.now = c.now.Add(d) }

// printers is a fake transport: it records what each address was sent and
// fails an address while told to.
type printers struct {
	mu      sync.Mutex
	got     map[string][]string // address to the labels, in print order
	failing map[string]error
	byBytes map[string]string // payload to its label
}

func (p *printers) Send(_ context.Context, address string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.failing[address]; err != nil {
		return err
	}
	p.got[address] = append(p.got[address], p.byBytes[string(data)])
	return nil
}

func (p *printers) fail(address string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failing[address] = err
}

func (p *printers) printed(address string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.got[address]...)
}

type fixture struct {
	q     *Queue
	db    *db.DB
	clock *clock
	fake  *printers
}

// newFixture is a queue over a fresh database with one venue and two TCP
// printers, "bar" (id 1) and "kitchen" (id 2), and one spooler printer (3).
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_PRINT001", Name: "Filter Coffee House", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: start, UpdatedAt: start,
		}); err != nil {
			return err
		}
		for _, p := range []struct{ name, transport, addr string }{
			{"Bar", TCP, "bar"}, {"Kitchen", TCP, "kitchen"}, {"USB", Windows, "EPSON TM-T82"},
		} {
			if _, err := q.CreatePrinter(ctx, gen.CreatePrinterParams{
				VenueID: 1, Name: p.name, Transport: p.transport, Address: p.addr, PaperMm: 80, CreatedAt: start, UpdatedAt: start,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{now: start}
	fake := &printers{got: map[string][]string{}, failing: map[string]error{}, byBytes: map[string]string{}}
	q, err := New(Options{DB: d, Clock: c, Transports: map[string]Transport{TCP: fake, Windows: fake}})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{q: q, db: d, clock: c, fake: fake}
}

// add queues a job whose bytes are its label.
func (f *fixture) add(t *testing.T, printerID int64, label string) int64 {
	t.Helper()
	f.fake.mu.Lock()
	f.fake.byBytes[label] = label
	f.fake.mu.Unlock()
	id, err := f.q.Enqueue(context.Background(), Job{VenueID: 1, PrinterID: printerID, Kind: printjob.Receipt, Label: label, Payload: []byte(label)})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// pass runs the worker once and waits for every send it started.
func (f *fixture) pass(t *testing.T) {
	t.Helper()
	if err := f.q.dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.q.sends.Wait()
}

func (f *fixture) job(t *testing.T, id int64) gen.PrintJobByIDRow {
	t.Helper()
	j, err := f.db.Queries().PrintJobByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestPrints(t *testing.T) {
	f := newFixture(t)
	id := f.add(t, 1, "Receipt 1")
	f.pass(t)
	j := f.job(t, id)
	if j.State != "done" || j.Attempts != 1 || !j.DoneAt.Valid || j.LastError != "" {
		t.Errorf("job = %+v", j)
	}
	if got := f.fake.printed("bar"); len(got) != 1 || got[0] != "Receipt 1" {
		t.Errorf("bar printed %v", got)
	}
	f.pass(t)
	if got := f.fake.printed("bar"); len(got) != 1 {
		t.Errorf("a done job printed again: %v", got)
	}
}

// TestRetriesWithBackoff: a printer that is off keeps its job, tried again
// on the schedule, until it comes back.
func TestRetriesWithBackoff(t *testing.T) {
	f := newFixture(t)
	f.fake.fail("bar", errors.New("connection refused"))
	id := f.add(t, 1, "Receipt 1")

	f.pass(t)
	j := f.job(t, id)
	if j.State != "failed" || j.Attempts != 1 || j.LastError != Unreachable || !j.NextAttemptAt.Equal(start.Add(2*time.Second)) {
		t.Fatalf("after one failure: %+v", j)
	}
	// Not due yet: nothing is tried.
	f.clock.add(time.Second)
	f.pass(t)
	if j := f.job(t, id); j.Attempts != 1 {
		t.Fatalf("tried before its backoff: %d attempts", j.Attempts)
	}
	f.clock.add(time.Second)
	f.pass(t)
	if j := f.job(t, id); j.State != "failed" || j.Attempts != 2 || !j.NextAttemptAt.Equal(start.Add(2*time.Second+4*time.Second)) {
		t.Fatalf("after two failures: %+v", j)
	}

	f.fake.fail("bar", nil) // the paper is back in
	f.clock.add(4 * time.Second)
	f.pass(t)
	if j := f.job(t, id); j.State != "done" || j.Attempts != 3 {
		t.Errorf("after the printer came back: %+v", j)
	}
}

// TestGoesDead: tries run out, or no retry can help; either way the job
// stays, dead, with its reason.
func TestGoesDead(t *testing.T) {
	f := newFixture(t)
	f.fake.fail("bar", errors.New("paper out"))
	id := f.add(t, 1, "Receipt 1")
	for range printjob.MaxAttempts {
		f.pass(t)
		f.clock.add(10 * time.Minute)
	}
	if j := f.job(t, id); j.State != "dead" || j.Attempts != printjob.MaxAttempts || j.LastError != Unreachable {
		t.Errorf("after every try: %+v", j)
	}
	f.pass(t)
	if j := f.job(t, id); j.Attempts != printjob.MaxAttempts {
		t.Error("a dead job was tried again")
	}

	f.fake.fail("kitchen", fmt.Errorf("no such printer: %w", printjob.ErrPermanent))
	perm := f.add(t, 2, "Ticket 1")
	f.pass(t)
	if j := f.job(t, perm); j.State != "dead" || j.Attempts != 1 {
		t.Errorf("a permanent error: %+v", j)
	}
}

// TestOnePrinterAtATimeInOrder: a printer's jobs go in order, and a job
// failing on one printer holds back only that printer.
func TestOnePrinterAtATimeInOrder(t *testing.T) {
	f := newFixture(t)
	f.fake.fail("bar", errors.New("cover open"))
	first := f.add(t, 1, "Bar 1")
	second := f.add(t, 1, "Bar 2")
	kitchen := f.add(t, 2, "Kitchen 1")

	f.pass(t)
	if j := f.job(t, second); j.State != "queued" || j.Attempts != 0 {
		t.Errorf("a later bar job went past a failed one: %+v", j)
	}
	if j := f.job(t, kitchen); j.State != "done" {
		t.Errorf("the kitchen waited on the bar: %+v", j)
	}

	f.fake.fail("bar", nil)
	f.clock.add(2 * time.Second)
	f.pass(t) // the first bar job
	f.pass(t) // then the second
	if got := f.fake.printed("bar"); len(got) != 2 || got[0] != "Bar 1" || got[1] != "Bar 2" {
		t.Errorf("bar printed %v, want Bar 1 then Bar 2", got)
	}
	if f.job(t, first).State != "done" || f.job(t, second).State != "done" {
		t.Error("not every bar job is done")
	}
}

func TestRetiredPrinterAndMissingTransport(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.db.Writer().ExecContext(ctx, `UPDATE printers SET active = 0 WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	retired := f.add(t, 2, "Ticket 1")
	f.pass(t)
	if j := f.job(t, retired); j.State != "dead" || j.LastError != `printer "Kitchen" has been retired: will not succeed on a retry` {
		t.Errorf("a retired printer's job: %+v", j)
	}

	delete(f.q.transports, Windows)
	usb := f.add(t, 3, "Receipt 1")
	f.pass(t)
	if j := f.job(t, usb); j.State != "dead" || j.Attempts != 1 {
		t.Errorf("a transport this build lacks: %+v", j)
	}
}

// TestNoPrinter: a job with nowhere to go is kept, dead, never dropped.
func TestNoPrinter(t *testing.T) {
	f := newFixture(t)
	id := f.add(t, 0, "Receipt 1")
	f.pass(t)
	if j := f.job(t, id); j.State != "dead" || j.LastError != NoPrinter || j.Attempts != 0 || j.PrinterID.Valid {
		t.Errorf("job = %+v", j)
	}
}

// TestCrashRecovery: a job left printing by a stop goes back to the queue
// on the next start, and prints.
func TestCrashRecovery(t *testing.T) {
	f := newFixture(t)
	id := f.add(t, 1, "Receipt 1")
	due, err := f.db.Queries().DuePrintJobs(context.Background(), start)
	if err != nil || len(due) != 1 {
		t.Fatal(due, err)
	}
	if ok, err := f.q.claim(context.Background(), due[0]); !ok || err != nil {
		t.Fatal(ok, err)
	}
	// The process dies here, mid-send.

	q2, err := New(Options{DB: f.db, Clock: f.clock, Transports: map[string]Transport{TCP: f.fake}})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := q2.requeueStale(context.Background()); n != 1 || err != nil {
		t.Fatalf("requeued %d, %v", n, err)
	}
	if j := f.job(t, id); j.State != "queued" || j.LastError == "" {
		t.Fatalf("after the restart: %+v", j)
	}
	f.q = q2
	f.pass(t)
	if j := f.job(t, id); j.State != "done" || j.Attempts != 2 {
		t.Errorf("after printing: %+v", j)
	}
}

// TestRunningQueue: the worker started for real prints a job it is woken
// for, and Close waits for it.
func TestRunningQueue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.q.Start(ctx); err == nil {
		t.Error("started twice")
	}
	id := f.add(t, 1, "Receipt 1")
	deadline := time.Now().Add(5 * time.Second)
	for f.job(t, id).State != "done" {
		if time.Now().After(deadline) {
			t.Fatal("the running queue never printed the job")
		}
		time.Sleep(10 * time.Millisecond)
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := f.q.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
	if err := f.q.Close(closeCtx); err != nil {
		t.Errorf("a second Close: %v", err)
	}
}

func TestInvalidJobs(t *testing.T) {
	f := newFixture(t)
	base := Job{VenueID: 1, PrinterID: 1, Kind: printjob.Kitchen, Label: "Ticket", Payload: []byte("x")}
	tests := []struct {
		name string
		edit func(*Job)
	}{
		{"no bytes", func(j *Job) { j.Payload = nil }},
		{"no label", func(j *Job) { j.Label = " " }},
		{"no venue", func(j *Job) { j.VenueID = 0 }},
		{"an unknown kind", func(j *Job) { j.Kind = "label" }},
		{"an order without its event", func(j *Job) { j.OrderID = 5 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := base
			tt.edit(&j)
			if _, err := f.q.Enqueue(context.Background(), j); !errors.Is(err, ErrInvalidJob) {
				t.Errorf("err = %v, want ErrInvalidJob", err)
			}
		})
	}
	if _, err := New(Options{}); err == nil {
		t.Error("a queue with no database")
	}
}

// TestRetryCancelAndList: a person's retry and cancel, the list staff see,
// and the version that tells the tills.
func TestRetryCancelAndList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var changed []int64
	f.q.onChange = func(v int64) { changed = append(changed, v) }
	version := func() int64 {
		v, err := f.q.Version(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if version() != 0 {
		t.Fatal("a venue with no jobs has a version")
	}

	f.fake.fail("bar", errors.New("paper out"))
	stuck := f.add(t, 1, "Receipt 1")
	waiting := f.add(t, 1, "Receipt 2")
	v1 := version()
	f.pass(t)
	if version() <= v1 || len(changed) < 3 {
		t.Errorf("a send moved the version %d to %d, told %v", v1, version(), changed)
	}

	// A cancel stops a waiting job, and says who.
	if err := f.q.Cancel(ctx, 1, waiting, "Asha"); err != nil {
		t.Fatal(err)
	}
	if j := f.job(t, waiting); j.State != "dead" || j.LastError != "cancelled by Asha" {
		t.Errorf("cancelled job = %+v", j)
	}
	if err := f.q.Cancel(ctx, 1, waiting, "Asha"); !errors.Is(err, ErrJobState) {
		t.Errorf("a second cancel: %v", err)
	}

	// A retry restores the tries of a job that failed.
	f.fake.fail("bar", nil)
	if err := f.q.Retry(ctx, 1, stuck); err != nil {
		t.Fatal(err)
	}
	if j := f.job(t, stuck); j.State != "queued" || j.Attempts != 0 || j.LastError != "" {
		t.Errorf("retried job = %+v", j)
	}
	f.pass(t)
	if err := f.q.Retry(ctx, 1, stuck); !errors.Is(err, ErrJobState) {
		t.Errorf("a retry of a printed job: %v", err)
	}
	if err := f.q.Retry(ctx, 2, waiting); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("another venue's job: %v", err)
	}
	if err := f.q.Retry(ctx, 1, 999); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("no such job: %v", err)
	}

	list, err := f.q.Jobs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != waiting || list[1].ID != stuck || list[1].PrinterName != "Bar" {
		t.Errorf("list = %+v", list)
	}
	// A printed job leaves the list once it is old news.
	f.clock.add(ShownFor + time.Minute)
	if list, _ := f.q.Jobs(ctx, 1); len(list) != 1 || list[0].ID != waiting {
		t.Errorf("list later = %+v", list)
	}
}
