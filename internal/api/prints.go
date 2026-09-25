package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/device/printq"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/printjob"
)

// Prints is what the API needs from the print queue. *printq.Queue is the
// implementation.
type Prints interface {
	Jobs(ctx context.Context, venueID int64) ([]printq.JobView, error)
	Retry(ctx context.Context, venueID, jobID int64) error
	Cancel(ctx context.Context, venueID, jobID int64, by string) error
}

// WithPrints mounts the print queue endpoints (phase 07, Task 5b). They need
// WithDevices and WithStaff: a till's signed-in staff.
func WithPrints(p Prints) Option {
	return func(s *Server) { s.prints = p }
}

// The print queue error codes.
const (
	CodePrintJobNotFound ErrorCode = "print_job_not_found"
	CodePrintJobState    ErrorCode = "print_job_state"
	CodeNoPrinter        ErrorCode = "no_printer"
)

var printErrors = []struct {
	err    error
	status int
	code   ErrorCode
}{
	{printq.ErrJobNotFound, http.StatusNotFound, CodePrintJobNotFound},
	{printq.ErrJobState, http.StatusConflict, CodePrintJobState},
	{printq.ErrStillNoPrinter, http.StatusConflict, CodeNoPrinter},
}

func (s *Server) mountPrints(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.prints == nil {
		return
	}
	guard := func(p auth.Permission, h http.HandlerFunc) http.Handler {
		return s.requireDevice(s.requireStaff(s.requirePermission(p, h)))
	}
	mux.Handle("GET /api/v1/print-jobs", guard(auth.PermOrderOpen, s.listPrintJobs))
	mux.HandleFunc("/api/v1/print-jobs", s.methodNotAllowed)
	// Anyone working the till may retry a receipt; cancelling one, so that
	// it never prints, is an edit.
	mux.Handle("POST /api/v1/print-jobs/{id}/retry", guard(auth.PermOrderOpen, s.retryPrintJob))
	mux.HandleFunc("/api/v1/print-jobs/{id}/retry", s.methodNotAllowed)
	mux.Handle("POST /api/v1/print-jobs/{id}/cancel", guard(auth.PermOrderEdit, s.cancelPrintJob))
	mux.HandleFunc("/api/v1/print-jobs/{id}/cancel", s.methodNotAllowed)
}

// printJobResponse is a job on the queue screen. NextAttemptAt is set only
// while a failed job waits to be tried again.
type printJobResponse struct {
	ID            int64          `json:"id"`
	Kind          printjob.Kind  `json:"kind"`
	Label         string         `json:"label"`
	State         printjob.State `json:"state"`
	Printer       string         `json:"printer,omitempty"`
	Station       string         `json:"station,omitempty"`
	Attempts      int64          `json:"attempts"`
	MaxAttempts   int64          `json:"max_attempts"`
	NextAttemptAt *time.Time     `json:"next_attempt_at,omitempty"`
	LastError     string         `json:"last_error,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	DoneAt        *time.Time     `json:"done_at,omitempty"`
}

type printJobsResponse struct {
	Jobs []printJobResponse `json:"jobs"`
}

func (s *Server) listPrintJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.prints.Jobs(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writePrintError(w, r, err)
		return
	}
	out := printJobsResponse{Jobs: make([]printJobResponse, len(jobs))}
	for i, j := range jobs {
		out.Jobs[i] = printJobResponse{
			ID: j.ID, Kind: printjob.Kind(j.Kind), Label: j.Label, State: printjob.State(j.State),
			Printer: j.PrinterName, Station: j.Station, Attempts: j.Attempts, MaxAttempts: printjob.MaxAttempts,
			LastError: j.LastError, CreatedAt: j.CreatedAt.UTC(),
		}
		if out.Jobs[i].State == printjob.Failed {
			next := j.NextAttemptAt.UTC()
			out.Jobs[i].NextAttemptAt = &next
		}
		if j.DoneAt.Valid {
			done := j.DoneAt.Time.UTC()
			out.Jobs[i].DoneAt = &done
		}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) retryPrintJob(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.prints.Retry(r.Context(), Meta(r).VenueID, id); err != nil {
		s.writePrintError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cancelPrintJob(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	by := "staff"
	if session, ok := SessionFrom(r.Context()); ok {
		by = session.Staff.Name
	}
	if err := s.prints.Cancel(r.Context(), Meta(r).VenueID, id, by); err != nil {
		s.writePrintError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writePrintError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range printErrors {
		if errors.Is(err, m.err) {
			writeError(w, s.log, m.status, m.code, err.Error())
			return
		}
	}
	s.log.Error("a print queue request failed",
		"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
}
