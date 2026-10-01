package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/service/audit"
)

// Audit is what the API needs from the trail of corrections (phase 10,
// Task 4c, ADR-117). *audit.Service is the implementation.
type Audit interface {
	Trail(ctx context.Context, venueID int64, f audit.Filter) (audit.Page, error)
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
		s.log.Error("reading the audit trail failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, s.log, http.StatusOK, page)
	}
}
