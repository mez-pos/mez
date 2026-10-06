// Package printq is the durable print queue (phase 07, Task 5, ADR-080):
// jobs in SQLite, a worker that sends each to its printer, and the retries,
// backoff and dead letters printjob decides.
//
// A job survives anything short of losing the database: a printer that is
// off, a jam, a restart mid-send. What it prints is fixed when it is queued,
// as ESC/POS bytes, so a retry at 21:10 prints what 19:40 would have.
package printq

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/printjob"
)

// Clock is where the queue gets the time. Tests fix it.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options configure a Queue. DB is required.
type Options struct {
	DB  *db.DB
	Log *slog.Logger
	// Clock defaults to the system clock.
	Clock Clock
	// Transports send bytes, by printers.transport. Defaults to TCP and, on
	// Windows, the spooler.
	Transports map[string]Transport
	// Poll is how often the worker looks for due jobs when nothing wakes
	// it: the backoff's resolution. Defaults to one second.
	Poll time.Duration
	// SendTimeout bounds one send. Defaults to fifteen seconds.
	SendTimeout time.Duration
	// OnChange is told a venue's jobs moved, after the change commits: the
	// stream's nudge (hub.Notify). Optional.
	OnChange func(venueID int64)
}

// Queue is the print queue and its worker. Start it once; Close stops it
// and waits for sends in flight.
type Queue struct {
	db          *db.DB
	log         *slog.Logger
	clock       Clock
	transports  map[string]Transport
	poll        time.Duration
	sendTimeout time.Duration
	onChange    func(venueID int64)

	wake     chan struct{}
	cancel   context.CancelFunc
	loopDone chan struct{}
	sends    sync.WaitGroup
	mu       sync.Mutex
	started  bool
	closed   bool
}

// New builds a queue. It does not start the worker.
func New(o Options) (*Queue, error) {
	if o.DB == nil {
		return nil, errors.New("printq: no database")
	}
	q := &Queue{
		db: o.DB, log: o.Log, clock: o.Clock, transports: o.Transports,
		poll: o.Poll, sendTimeout: o.SendTimeout, onChange: o.OnChange, wake: make(chan struct{}, 1),
	}
	if q.log == nil {
		q.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if q.clock == nil {
		q.clock = systemClock{}
	}
	if q.transports == nil {
		q.transports = DefaultTransports()
	}
	if q.poll <= 0 {
		q.poll = time.Second
	}
	if q.sendTimeout <= 0 {
		q.sendTimeout = 15 * time.Second
	}
	return q, nil
}

// Start puts back any job a crash left printing, then runs the worker until
// Close.
func (q *Queue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started || q.closed {
		return errors.New("printq: already started or closed")
	}
	n, err := q.requeueStale(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		q.log.Warn("print jobs requeued after a stop mid-send", "jobs", n)
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	q.cancel, q.loopDone, q.started = cancel, make(chan struct{}), true
	go q.loop(loopCtx)
	return nil
}

// Close stops the worker and waits for sends in flight, or for ctx. A send
// cut short leaves its job printing, which the next Start puts back.
func (q *Queue) Close(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	started := q.started
	q.mu.Unlock()
	if !started {
		return nil
	}
	q.cancel()
	done := make(chan struct{})
	go func() {
		<-q.loopDone
		q.sends.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("printq: closing: %w", ctx.Err())
	}
}

// Wake asks the worker to look for due jobs now rather than at its next
// poll: after an enqueue, so a receipt prints as the guest pays.
func (q *Queue) Wake() {
	select {
	case q.wake <- struct{}{}:
	default: // a wake is already pending
	}
}

func (q *Queue) loop(ctx context.Context) {
	defer close(q.loopDone)
	ticker := time.NewTicker(q.poll)
	defer ticker.Stop()
	for {
		if err := q.dispatch(ctx); err != nil && ctx.Err() == nil {
			q.log.Error("print queue: reading due jobs", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-q.wake:
		}
	}
}

// dispatch claims every due job, at most one per printer, and sends each in
// its own goroutine: one jammed printer does not hold up another.
func (q *Queue) dispatch(ctx context.Context) error {
	due, err := q.db.Queries().DuePrintJobs(ctx, q.clock.Now().UTC())
	if err != nil {
		return err
	}
	for _, job := range due {
		claimed, err := q.claim(ctx, job)
		if err != nil {
			return err
		}
		if !claimed {
			continue
		}
		q.sends.Add(1)
		go func() {
			defer q.sends.Done()
			q.send(ctx, job)
		}()
	}
	return nil
}

func (q *Queue) claim(ctx context.Context, job gen.DuePrintJobsRow) (bool, error) {
	if _, err := printjob.Transition(printjob.State(job.State), printjob.Printing); err != nil {
		return false, err
	}
	var n int64
	err := q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		var err error
		n, err = tx.ClaimPrintJob(ctx, gen.ClaimPrintJobParams{Now: q.clock.Now().UTC(), ID: job.ID, FromState: job.State})
		if err != nil || n != 1 {
			return err
		}
		return tx.BumpPrintVersion(ctx, job.VenueID)
	})
	if err != nil {
		return false, fmt.Errorf("claiming print job %d: %w", job.ID, err)
	}
	if n == 1 {
		q.changed(job.VenueID)
	}
	return n == 1, nil
}

// changed tells OnChange a venue's jobs moved.
func (q *Queue) changed(venueID int64) {
	if q.onChange != nil {
		q.onChange(venueID)
	}
}

// send makes one attempt and records how it went.
func (q *Queue) send(ctx context.Context, job gen.DuePrintJobsRow) {
	attempt := int(job.Attempts) + 1 // claiming counted it
	err := q.transmit(ctx, job)
	if err != nil && ctx.Err() != nil {
		return // cut short by shutdown: the job stays printing, and Start puts it back
	}
	// Recorded even if shutdown began after the bytes went: a printed job
	// left printing would print again on the next start.
	record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	out := printjob.After(attempt, err, q.clock.Now().UTC())
	if err != nil {
		out.Error = Reason(err) // the detail goes to the log below, not to the screens
	}
	if err := q.finish(record, job.VenueID, job.ID, out); err != nil {
		q.log.Error("print queue: recording a send", "job_id", job.ID, "error", err)
		return
	}
	q.changed(job.VenueID)
	attrs := []any{"job_id", job.ID, "venue_id", job.VenueID, "printer", job.PrinterName, "kind", job.Kind, "attempt", attempt}
	switch out.State {
	case printjob.Failed:
		q.log.Warn("print job failed, will retry", append(attrs, "error", err, "retry_at", out.Next)...)
	case printjob.Dead:
		q.log.Error("print job dead, needs a person", append(attrs, "error", err)...)
	default:
		q.log.Info("printed", attrs...)
		// The printer's next job may be waiting behind this one.
		q.Wake()
	}
}

func (q *Queue) transmit(ctx context.Context, job gen.DuePrintJobsRow) error {
	if !job.PrinterActive {
		return shown(fmt.Errorf("printer %q has been retired: %w", job.PrinterName, printjob.ErrPermanent))
	}
	t, ok := q.transports[job.Transport]
	if !ok {
		return shown(fmt.Errorf("no %q transport in this build: %w", job.Transport, printjob.ErrPermanent))
	}
	sendCtx, cancel := context.WithTimeout(ctx, q.sendTimeout)
	defer cancel()
	return t.Send(sendCtx, job.Address, job.Payload)
}

func (q *Queue) finish(ctx context.Context, venueID, id int64, out printjob.Outcome) error {
	if _, err := printjob.Transition(printjob.Printing, out.State); err != nil {
		return err
	}
	now := q.clock.Now().UTC()
	params := gen.FinishPrintJobParams{State: string(out.State), NextAttemptAt: now, LastError: out.Error, Now: now, ID: id}
	if out.State == printjob.Failed {
		params.NextAttemptAt = out.Next
	}
	if out.State == printjob.Done {
		params.DoneAt = sql.NullTime{Time: now, Valid: true}
	}
	return q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		n, err := tx.FinishPrintJob(ctx, params)
		if err == nil && n != 1 {
			err = fmt.Errorf("print job %d was no longer printing", id)
		}
		if err != nil {
			return err
		}
		return tx.BumpPrintVersion(ctx, venueID)
	})
}

func (q *Queue) requeueStale(ctx context.Context) (int64, error) {
	var n int64
	err := q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		var err error
		n, err = tx.RequeueStalePrintJobs(ctx, q.clock.Now().UTC())
		if err != nil || n == 0 {
			return err
		}
		return tx.BumpAllPrintVersions(ctx)
	})
	if err != nil {
		return 0, fmt.Errorf("printq: requeueing jobs left printing: %w", err)
	}
	return n, nil
}
