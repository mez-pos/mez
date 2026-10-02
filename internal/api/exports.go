package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// Exports is what the API needs of a shift's export files (ADR-104).
// *shift.Service is the implementation.
type Exports interface {
	Exports(ctx context.Context, venueID, shiftID int64) ([]svcshift.ExportFile, error)
	Export(ctx context.Context, venueID, id int64) (svcshift.ExportBody, error)
}

// CodeExportNotFound is the export error code.
const CodeExportNotFound ErrorCode = "export_not_found"

type exportFileResponse struct {
	ID        int64               `json:"id"`
	Kind      svcshift.ExportKind `json:"kind"`
	Filename  string              `json:"filename"`
	Size      int64               `json:"size"`
	CreatedAt time.Time           `json:"created_at"`
}

type exportsResponse struct {
	Files []exportFileResponse `json:"files"`
}

// exportResponse is a file and its CSV text. It is JSON, not the file
// itself, so the back office reads it with the same signed-in client as
// everything else and saves it as the file.
type exportResponse struct {
	exportFileResponse
	ShiftID     int64  `json:"shift_id"`
	ContentType string `json:"content_type"`
	Text        string `json:"text"`
}

func (s *Server) mountExports(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.exports == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermReportsView, h))))
	}
	handle("GET /api/v1/shift/{id}/exports", s.listExports)
	handle("GET /api/v1/exports/{id}", s.oneExport)
}

func (s *Server) listExports(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	files, err := s.exports.Exports(r.Context(), Meta(r).VenueID, id)
	if err != nil {
		s.writeZError(w, r, err)
		return
	}
	out := exportsResponse{Files: make([]exportFileResponse, len(files))}
	for i, f := range files {
		out.Files[i] = toExportFile(f)
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) oneExport(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := s.exports.Export(r.Context(), Meta(r).VenueID, id)
	if errors.Is(err, svcshift.ErrExportNotFound) {
		writeError(w, s.log, http.StatusNotFound, CodeExportNotFound, "No such export.")
		return
	}
	if err != nil {
		s.writeZError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, exportResponse{
		exportFileResponse: toExportFile(f.ExportFile), ShiftID: f.ShiftID, ContentType: "text/csv; charset=utf-8", Text: f.Text,
	})
}

func toExportFile(f svcshift.ExportFile) exportFileResponse {
	return exportFileResponse{ID: f.ID, Kind: f.Kind, Filename: f.Filename, Size: f.Size, CreatedAt: f.CreatedAt.UTC()}
}
