package printq

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/printjob"
)

// Job is something to print.
type Job struct {
	VenueID   int64
	PrinterID int64 // 0: the venue has no printer for this; the job is kept dead
	Kind      printjob.Kind
	Label     string // what staff read on the queue
	Payload   []byte // the ESC/POS bytes, from escpos
	OrderID   int64  // with EventSeq, what caused it; 0 when no order did
	EventSeq  int64
}

// NoPrinter is the error a job with no printer is kept with.
const NoPrinter = "no printer is set up for this; set one up and retry"

// ErrInvalidJob means a job that could never print: no bytes, an unknown
// kind, no label.
var ErrInvalidJob = errors.New("printq: invalid job")

// EnqueueTx writes a job inside a transaction the caller holds, so a job is
// written with the event that causes it or not at all (Task 5b). It returns
// the job's id. Call Wake once the transaction commits.
func EnqueueTx(ctx context.Context, tx *gen.Queries, j Job, now time.Time) (int64, error) {
	if err := j.check(); err != nil {
		return 0, err
	}
	now = now.UTC()
	params := gen.CreatePrintJobParams{
		VenueID: j.VenueID, Kind: string(j.Kind), Label: j.Label, Payload: j.Payload,
		State: string(printjob.Queued), NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if j.PrinterID != 0 {
		params.PrinterID = sql.NullInt64{Int64: j.PrinterID, Valid: true}
	} else {
		params.State, params.LastError = string(printjob.Dead), NoPrinter
	}
	if j.OrderID != 0 {
		params.OrderID = sql.NullInt64{Int64: j.OrderID, Valid: true}
		params.EventSeq = sql.NullInt64{Int64: j.EventSeq, Valid: true}
	}
	id, err := tx.CreatePrintJob(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("printq: queueing %q: %w", j.Label, err)
	}
	return id, nil
}

// Enqueue writes a job in a transaction of its own and wakes the worker:
// for a job no event causes, such as a test page.
func (q *Queue) Enqueue(ctx context.Context, j Job) (int64, error) {
	var id int64
	err := q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		var err error
		id, err = EnqueueTx(ctx, tx, j, q.clock.Now())
		return err
	})
	if err != nil {
		return 0, err
	}
	q.Wake()
	return id, nil
}

func (j Job) check() error {
	switch {
	case len(j.Payload) == 0:
		return fmt.Errorf("%w: nothing to print", ErrInvalidJob)
	case strings.TrimSpace(j.Label) == "":
		return fmt.Errorf("%w: no label", ErrInvalidJob)
	case j.VenueID == 0:
		return fmt.Errorf("%w: no venue", ErrInvalidJob)
	case (j.OrderID == 0) != (j.EventSeq == 0):
		return fmt.Errorf("%w: an order needs the event that caused the job", ErrInvalidJob)
	}
	for _, k := range printjob.Kinds() {
		if k == j.Kind {
			return nil
		}
	}
	return fmt.Errorf("%w: kind %q", ErrInvalidJob, j.Kind)
}
