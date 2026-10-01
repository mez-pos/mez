package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/report"
	"github.com/byjoellobo/nofiltr-pos/internal/service/reports"
)

// Reports is what the API needs from the range reports (phase 10, Task 5a,
// ADR-118). *reports.Service is the implementation.
type Reports interface {
	Sales(ctx context.Context, venueID int64, q reports.Query) (reports.Sales, error)
}

// WithReports mounts the range reports. It needs WithDevices and WithStaff,
// and reports.view.
func WithReports(r Reports) Option {
	return func(s *Server) { s.reports = r }
}

func (s *Server) mountReports(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.reports == nil {
		return
	}
	mux.Handle("GET /api/v1/reports/sales", s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, http.HandlerFunc(s.readSales)))))
}

// readSales answers ?from=2026-10-01&to=2026-10-07, both trading days and
// required, with an optional every of day (the default), week or month.
// The reports are aggregates, bounded by the menu, the staff and the days
// asked for rather than by trade, so they come whole, not in pages
// (ADR-118).
func (s *Server) readSales(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.reports.Sales(r.Context(), Meta(r).VenueID, reports.Query{
		From: q.Get("from"), To: q.Get("to"), Every: report.Every(q.Get("every")),
	})
	switch {
	case errors.Is(err, reports.ErrBadRange):
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidFilter, err.Error())
	case err != nil:
		s.log.Error("reading the sales reports failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, s.log, http.StatusOK, res)
	}
}
