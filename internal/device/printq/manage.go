package printq

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/printjob"
)

// The refusals of a person's retry or cancel. The API maps them to codes.
var (
	ErrJobNotFound    = errors.New("no such print job at this venue")
	ErrJobState       = errors.New("the print job is not in a state that allows that")
	ErrStillNoPrinter = errors.New(NoPrinter)
)

// ShownFor is how long a printed job stays on the queue screen: long enough
// to see the bill that just went through, short enough that the list is
// about now.
const ShownFor = 2 * time.Hour

// maxShown bounds the list. A venue with more unfinished jobs than this has
// a printer problem the list will not fix.
const maxShown = 200

// JobView is a job as staff see it on the queue: no payload.
type JobView = gen.PrintJobsForVenueRow

// Jobs lists a venue's queue: every job not printed, and the printed ones
// from the last ShownFor, newest first.
func (q *Queue) Jobs(ctx context.Context, venueID int64) ([]JobView, error) {
	jobs, err := q.db.Queries().PrintJobsForVenue(ctx, gen.PrintJobsForVenueParams{
		VenueID: venueID, Since: q.clock.Now().UTC().Add(-ShownFor), MaxJobs: maxShown,
	})
	if err != nil {
		return nil, fmt.Errorf("printq: listing the jobs of venue %d: %w", venueID, err)
	}
	return jobs, nil
}

// Version is the venue's print version: it moves on every change to the
// venue's jobs, which is how the stream knows to tell the tills.
func (q *Queue) Version(ctx context.Context, venueID int64) (int64, error) {
	v, err := q.db.Queries().PrintVersion(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("printq: print version of venue %d: %w", venueID, err)
	}
	return v, nil
}

// Retry puts a failed or dead job back in the queue with its tries
// restored. A job that had no printer takes the one set up for it since:
// the receipt printer for a receipt, its station's for a kitchen ticket.
func (q *Queue) Retry(ctx context.Context, venueID, jobID int64) error {
	err := q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		job, err := venueJob(ctx, tx, venueID, jobID)
		if err != nil {
			return err
		}
		if _, err := printjob.Transition(printjob.State(job.State), printjob.Queued); err != nil {
			return fmt.Errorf("%w: it is %s", ErrJobState, job.State)
		}
		printer := job.PrinterID
		if !printer.Valid {
			id, err := printerFor(ctx, tx, venueID, printjob.Kind(job.Kind), job.Station)
			if err != nil {
				return err
			}
			printer = sql.NullInt64{Int64: id, Valid: true}
		}
		now := q.clock.Now().UTC()
		n, err := tx.RetryPrintJob(ctx, gen.RetryPrintJobParams{PrinterID: printer, Now: now, ID: jobID, VenueID: venueID})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: it moved on", ErrJobState)
		}
		return tx.BumpPrintVersion(ctx, venueID)
	})
	if err != nil {
		return err
	}
	q.Wake()
	q.changed(venueID)
	return nil
}

// Cancel stops a job that has not printed from being tried. It is kept,
// dead, with who cancelled it, and can be retried.
func (q *Queue) Cancel(ctx context.Context, venueID, jobID int64, by string) error {
	err := q.db.WriteTx(ctx, func(tx *gen.Queries) error {
		job, err := venueJob(ctx, tx, venueID, jobID)
		if err != nil {
			return err
		}
		// Only a job waiting its turn: one printing is already at the printer.
		if st := printjob.State(job.State); st != printjob.Queued && st != printjob.Failed {
			return fmt.Errorf("%w: it is %s", ErrJobState, job.State)
		}
		n, err := tx.CancelPrintJob(ctx, gen.CancelPrintJobParams{
			Reason: "cancelled by " + by, Now: q.clock.Now().UTC(), ID: jobID, VenueID: venueID,
		})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: it moved on", ErrJobState)
		}
		return tx.BumpPrintVersion(ctx, venueID)
	})
	if err != nil {
		return err
	}
	q.changed(venueID)
	return nil
}

func venueJob(ctx context.Context, tx *gen.Queries, venueID, jobID int64) (gen.PrintJobByIDRow, error) {
	job, err := tx.PrintJobByID(ctx, jobID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && job.VenueID != venueID) {
		return gen.PrintJobByIDRow{}, fmt.Errorf("%w: %d", ErrJobNotFound, jobID)
	}
	if err != nil {
		return gen.PrintJobByIDRow{}, fmt.Errorf("printq: reading job %d: %w", jobID, err)
	}
	return job, nil
}

// Route is where a job prints, and on what paper.
type Route struct {
	PrinterID int64 // 0: none set up
	PaperMM   int64
}

// RouteFor finds the printer for a kind of job at a venue: the receipt
// printer for receipts and Z-reports, the station's printer for a kitchen
// ticket. None set up is a zero Route, not an error: the job is kept dead.
func RouteFor(ctx context.Context, tx *gen.Queries, venueID int64, kind printjob.Kind, station string) (Route, error) {
	var (
		row gen.ReceiptPrinterRow
		err error
	)
	switch kind {
	case printjob.Kitchen:
		var s gen.StationPrinterRow
		s, err = tx.StationPrinter(ctx, gen.StationPrinterParams{VenueID: venueID, Code: station})
		row = gen.ReceiptPrinterRow(s)
	default:
		row, err = tx.ReceiptPrinter(ctx, venueID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Route{PaperMM: 80}, nil
	}
	if err != nil {
		return Route{}, fmt.Errorf("printq: finding the printer for a %s: %w", kind, err)
	}
	return Route{PrinterID: row.ID, PaperMM: row.PaperMm}, nil
}

func printerFor(ctx context.Context, tx *gen.Queries, venueID int64, kind printjob.Kind, station string) (int64, error) {
	r, err := RouteFor(ctx, tx, venueID, kind, station)
	if err != nil {
		return 0, err
	}
	if r.PrinterID == 0 {
		return 0, ErrStillNoPrinter
	}
	return r.PrinterID, nil
}
