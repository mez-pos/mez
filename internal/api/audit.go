package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/service/audit"
)

// Audit is what the API needs from the trail of corrections (phase 10,
// Task 4c, ADR-117). *audit.Service is the implementation.
type Audit interface {
	Trail(ctx context.Context, venueID int64, f audit.Filter) (audit.Page, error)
	Table(ctx context.Context, venueID int64, f audit.Filter) (export.Table, error)
	Analysis(ctx context.Context, venueID int64, f audit.Filter) (audit.Analysis, error)
	AnalysisTable(ctx context.Context, venueID int64, f audit.Filter, sheet audit.AnalysisSheet) (export.Table, error)
}

// WithAudit mounts the trail. It needs WithDevices and WithStaff, and
// reports.view: it is a report on the day's money, read by whoever reads
// the Z-report.
func WithAudit(a Audit) Option {
	return func(s *Server) { s.audit = a }
}

// CodeInvalidFilter is a filter the trail cannot read: a day that is not
// one, a range backwards or over a year, a kind it does not have.
const CodeInvalidFilter ErrorCode = "invalid_filter"

func (s *Server) mountAudit(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.audit == nil {
		return
	}
	mux.Handle("GET /api/v1/audit", s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, http.HandlerFunc(s.readTrail)))))
	mux.Handle("GET /api/v1/audit/csv", s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, http.HandlerFunc(s.readTrailCSV)))))
	mux.Handle("GET /api/v1/audit/analysis", s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, http.HandlerFunc(s.readAnalysis)))))
	mux.Handle("GET /api/v1/audit/analysis/csv", s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, http.HandlerFunc(s.readAnalysisCSV)))))
}

// readTrail answers ?from=2026-10-01&to=2026-10-07, both trading days and
// required, with optional staff (a public id), kind, before and limit.
func (s *Server) readTrail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := audit.Filter{From: q.Get("from"), To: q.Get("to"), StaffID: q.Get("staff"), Kind: audit.Kind(q.Get("kind"))}
	for _, p := range []struct {
		name string
		set  func(int64)
	}{
		{"before", func(n int64) { f.Before = n }},
		{"limit", func(n int64) { f.Limit = int(min(n, audit.MaxPage)) }},
	} {
		if v := q.Get(p.name); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, p.name+" must be a whole number, 0 or more", p.name)
				return
			}
			p.set(n)
		}
	}
	page, err := s.audit.Trail(r.Context(), Meta(r).VenueID, f)
	switch {
	case errors.Is(err, audit.ErrBadFilter):
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
	case errors.Is(err, audit.ErrStaffNotFound):
		writeFieldError(w, s.log, http.StatusNotFound, CodeStaffNotFound, "No such person.", "staff")
	case err != nil:
		s.internalError(w, r, "reading the audit trail failed", err)
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, s.log, http.StatusOK, page)
	}
}

// readTrailCSV answers the same from, to, staff and kind as readTrail with
// everything they find as one CSV file: a download is not paged.
func (s *Server) readTrailCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := audit.Filter{From: q.Get("from"), To: q.Get("to"), StaffID: q.Get("staff"), Kind: audit.Kind(q.Get("kind"))}
	t, err := s.audit.Table(r.Context(), Meta(r).VenueID, f)
	switch {
	case errors.Is(err, audit.ErrBadFilter):
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
	case errors.Is(err, audit.ErrStaffNotFound):
		writeFieldError(w, s.log, http.StatusNotFound, CodeStaffNotFound, "No such person.", "staff")
	case err != nil:
		s.internalError(w, r, "reading the audit trail as CSV failed", err)
	default:
		// The filter was read, so from and to are days and safe in a name.
		s.writeCSV(w, r, "audit-"+f.From+"-to-"+f.To+".csv", t)
	}
}

// readAnalysis answers the same from, to, staff and kind as readTrail with
// everything they find added up: by kind, by reason, by who did it and by
// who authorised it (phase 10, Task 5e1, ADR-122). Aggregates, so whole,
// not paged.
func (s *Server) readAnalysis(w http.ResponseWriter, r *http.Request) {
	a, err := s.audit.Analysis(r.Context(), Meta(r).VenueID, trailFilter(r))
	if s.auditRefused(w, r, err, "adding up the corrections") {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, a)
}

// readAnalysisCSV answers one grouping of the analysis as a CSV file:
// ?by= names it (reason, by or authorised_by).
func (s *Server) readAnalysisCSV(w http.ResponseWriter, r *http.Request) {
	f := trailFilter(r)
	sheet := audit.AnalysisSheet(r.URL.Query().Get("by"))
	t, err := s.audit.AnalysisTable(r.Context(), Meta(r).VenueID, f, sheet)
	if errors.Is(err, audit.ErrNoSheet) {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error(), "by")
		return
	}
	if s.auditRefused(w, r, err, "adding up the corrections as CSV") {
		return
	}
	// The filter was read, so from and to are days and safe in a name.
	s.writeCSV(w, r, "corrections-"+string(sheet)+"-"+f.From+"-to-"+f.To+".csv", t)
}

// trailFilter is the filter a whole-trail route reads: no cursor, no limit.
func trailFilter(r *http.Request) audit.Filter {
	q := r.URL.Query()
	return audit.Filter{From: q.Get("from"), To: q.Get("to"), StaffID: q.Get("staff"), Kind: audit.Kind(q.Get("kind"))}
}

// auditRefused writes the answer to a failed read of the trail and says
// whether it did.
func (s *Server) auditRefused(w http.ResponseWriter, r *http.Request, err error, doing string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, audit.ErrBadFilter):
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
	case errors.Is(err, audit.ErrStaffNotFound):
		writeFieldError(w, s.log, http.StatusNotFound, CodeStaffNotFound, "No such person.", "staff")
	default:
		s.log.Error(doing+" failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
	return true
}
