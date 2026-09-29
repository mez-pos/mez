package api

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/zreport"
	svcshift "github.com/byjoellobo/nofiltr-pos/internal/service/shift"
)

// ZReports is what the API needs of the venue's Z-reports (ADR-098).
// *shift.Service is the implementation.
type ZReports interface {
	ZReport(ctx context.Context, venueID, number int64) (svcshift.ZReport, error)
	ZReportForShift(ctx context.Context, venueID, shiftID int64) (svcshift.ZReport, error)
	ZReports(ctx context.Context, venueID, before int64) ([]svcshift.ZSummary, error)
	CorrectZReport(ctx context.Context, meta svcshift.Meta, shiftID int64, reason string) (svcshift.ZReport, error)
	PrintZReport(ctx context.Context, meta svcshift.Meta, number int64) error
}

// The Z-report error codes.
const (
	CodeZReportNotFound ErrorCode = "z_report_not_found"
	CodeShiftStillOpen  ErrorCode = "shift_still_open"
)

// zReportResponse is a Z-report as issued. Every figure is the stored
// document's; nothing here is worked out again.
type zReportResponse struct {
	Number      int64            `json:"number"`
	Corrects    int64            `json:"corrects,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	ShiftID     int64            `json:"shift_id"`
	Shift       svcshift.ZShift  `json:"shift"`
	Report      zreport.Report   `json:"report"`
	Drawer      svcshift.ZDrawer `json:"drawer"`
	Names       []zNameResponse  `json:"names"`
	GeneratedBy int64            `json:"generated_by"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// zNameResponse is a staff member the report mentions.
type zNameResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type zReportsResponse struct {
	Reports []svcshift.ZSummary `json:"reports"`
	// NextBefore is the cursor for the next page, absent on the last.
	NextBefore int64 `json:"next_before,omitempty"`
}

type correctRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) mountZReports(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.zreports == nil {
		return
	}
	handle := func(pattern string, p auth.Permission, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(p, h))))
	}
	handle("GET /api/v1/z-reports", auth.PermReportsView, s.listZReports)
	handle("GET /api/v1/z-reports/{number}", auth.PermReportsView, s.oneZReport)
	// A copy, marked COPY, on the report printer.
	handle("POST /api/v1/z-reports/{number}/print", auth.PermReportsView, s.printZReport)
	handle("GET /api/v1/shift/{id}/z-report", auth.PermReportsView, s.shiftZReport)
	handle("POST /api/v1/shift/{id}/z-report/corrections", auth.PermShiftManage, s.correctZReport)
}

func (s *Server) listZReports(w http.ResponseWriter, r *http.Request) {
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "before must be a report number", "before")
			return
		}
		before = n
	}
	list, err := s.zreports.ZReports(r.Context(), Meta(r).VenueID, before)
	if err != nil {
		s.writeZError(w, r, err)
		return
	}
	out := zReportsResponse{Reports: list}
	if len(list) == svcshift.ZPageSize {
		out.NextBefore = list[len(list)-1].Number
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) oneZReport(w http.ResponseWriter, r *http.Request) {
	number, ok := s.pathID(w, r, "number")
	if !ok {
		return
	}
	z, err := s.zreports.ZReport(r.Context(), Meta(r).VenueID, number)
	s.writeZ(w, r, z, err, http.StatusOK)
}

func (s *Server) printZReport(w http.ResponseWriter, r *http.Request) {
	number, ok := s.pathID(w, r, "number")
	if !ok {
		return
	}
	if err := s.zreports.PrintZReport(r.Context(), shiftMeta(r), number); err != nil {
		s.writeZError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) shiftZReport(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	z, err := s.zreports.ZReportForShift(r.Context(), Meta(r).VenueID, id)
	s.writeZ(w, r, z, err, http.StatusOK)
}

func (s *Server) correctZReport(w http.ResponseWriter, r *http.Request) {
	var req correctRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	z, err := s.zreports.CorrectZReport(r.Context(), shiftMeta(r), id, req.Reason)
	s.writeZ(w, r, z, err, http.StatusCreated)
}

func (s *Server) writeZ(w http.ResponseWriter, r *http.Request, z svcshift.ZReport, err error, status int) {
	if err != nil {
		s.writeZError(w, r, err)
		return
	}
	d := z.Document
	out := zReportResponse{
		Number: d.Number, Corrects: d.Corrects, Reason: d.Reason, ShiftID: z.ShiftID,
		Shift: d.Shift, Report: d.Report, Drawer: d.Drawer, Names: []zNameResponse{},
		GeneratedBy: d.GeneratedBy, GeneratedAt: d.GeneratedAt.UTC(),
	}
	for id, n := range d.Names {
		out.Names = append(out.Names, zNameResponse{ID: id, Name: n})
	}
	slices.SortFunc(out.Names, func(a, b zNameResponse) int { return cmp.Compare(a.ID, b.ID) })
	writeJSON(w, s.log, status, out)
}

func (s *Server) writeZError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, svcshift.ErrZReportNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeZReportNotFound, "No such Z-report.")
	case errors.Is(err, svcshift.ErrShiftStillOpen):
		writeError(w, s.log, http.StatusConflict, CodeShiftStillOpen, "The shift is still open: its Z-report comes with the close.")
	default:
		s.writeShiftError(w, r, err)
	}
}
