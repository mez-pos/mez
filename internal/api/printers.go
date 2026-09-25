package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/printjob"
	"github.com/byjoellobo/nofiltr-pos/internal/service/printers"
)

// Printers is what the API needs from the printer settings.
// *printers.Service is the implementation.
type Printers interface {
	List(ctx context.Context, venueID int64) (printers.Overview, error)
	Create(ctx context.Context, venueID int64, in printers.Settings) (int64, error)
	Update(ctx context.Context, venueID, id int64, in printers.Settings) error
	SetActive(ctx context.Context, venueID, id int64, active bool) error
	SetReceipt(ctx context.Context, venueID, id int64) error
	SetStation(ctx context.Context, venueID int64, code string, id int64) error
	TestPage(ctx context.Context, venueID, id int64) (int64, error)
}

// WithPrinters mounts the printer settings (phase 07, Task 5c). They need
// WithDevices and WithStaff, and settings.edit.
func WithPrinters(p Printers) Option {
	return func(s *Server) { s.printers = p }
}

// The printer settings error codes.
const (
	CodePrinterNotFound  ErrorCode = "printer_not_found"
	CodePrinterRetired   ErrorCode = "printer_retired"
	CodeStationNotFound  ErrorCode = "station_not_found"
	CodePrinterNameTaken ErrorCode = "printer_name_taken"
	CodeInvalidPrinter   ErrorCode = "invalid_printer"
)

type printerRequest = printers.Settings

// assignRequest names a printer for receipts or a station; null for none.
type assignRequest struct {
	PrinterID *int64 `json:"printer_id"`
}

type printerResponse struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	Transport printers.Transport `json:"transport"`
	Address   string             `json:"address"`
	PaperMM   int64              `json:"paper_mm"`
	Active    bool               `json:"active"`
	IsReceipt bool               `json:"is_receipt"`
	Stations  []string           `json:"stations"` // the codes of the stations it prints for
	LastJob   *lastJobResponse   `json:"last_job,omitempty"`
}

// lastJobResponse is a printer's newest job: its last result.
type lastJobResponse struct {
	ID    int64          `json:"id"`
	State printjob.State `json:"state"`
	Label string         `json:"label"`
	Error string         `json:"error,omitempty"`
	At    time.Time      `json:"at"`
}

type stationPrinterResponse struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	PrinterID *int64 `json:"printer_id,omitempty"`
}

type printersResponse struct {
	Printers []printerResponse        `json:"printers"`
	Stations []stationPrinterResponse `json:"stations"`
}

type testPageResponse struct {
	JobID int64 `json:"job_id"`
}

func (s *Server) mountPrinters(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.printers == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/printers", s.listPrinters)
	handle("POST /api/v1/printers", s.createPrinter)
	mux.HandleFunc("/api/v1/printers", s.methodNotAllowed)
	// The literal path wins over {id}: "receipt" is never a printer's id.
	handle("PUT /api/v1/printers/receipt", s.setReceiptPrinter)
	handle("PUT /api/v1/printers/{id}", s.updatePrinter)
	handle("POST /api/v1/printers/{id}/retire", s.activate(false))
	handle("POST /api/v1/printers/{id}/restore", s.activate(true))
	handle("POST /api/v1/printers/{id}/test", s.testPage)
	handle("PUT /api/v1/stations/{code}/printer", s.setStationPrinter)
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	s.writePrinters(w, r, http.StatusOK)
}

func (s *Server) createPrinter(w http.ResponseWriter, r *http.Request) {
	var req printerRequest
	if !s.decode(w, r, &req) {
		return
	}
	if _, err := s.printers.Create(r.Context(), Meta(r).VenueID, req); err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	s.writePrinters(w, r, http.StatusCreated)
}

func (s *Server) updatePrinter(w http.ResponseWriter, r *http.Request) {
	var req printerRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	if err := s.printers.Update(r.Context(), Meta(r).VenueID, id, req); err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	s.writePrinters(w, r, http.StatusOK)
}

func (s *Server) activate(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		if err := s.printers.SetActive(r.Context(), Meta(r).VenueID, id, active); err != nil {
			s.writePrinterError(w, r, err)
			return
		}
		s.writePrinters(w, r, http.StatusOK)
	}
}

func (s *Server) setReceiptPrinter(w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.printers.SetReceipt(r.Context(), Meta(r).VenueID, idOrZero(req.PrinterID)); err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	s.writePrinters(w, r, http.StatusOK)
}

func (s *Server) setStationPrinter(w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.printers.SetStation(r.Context(), Meta(r).VenueID, r.PathValue("code"), idOrZero(req.PrinterID)); err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	s.writePrinters(w, r, http.StatusOK)
}

func (s *Server) testPage(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	job, err := s.printers.TestPage(r.Context(), Meta(r).VenueID, id)
	if err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusAccepted, testPageResponse{JobID: job})
}

// writePrinters answers with the settings as they now stand, so a screen
// redraws from the server after every change.
func (s *Server) writePrinters(w http.ResponseWriter, r *http.Request, status int) {
	o, err := s.printers.List(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writePrinterError(w, r, err)
		return
	}
	out := printersResponse{Printers: make([]printerResponse, len(o.Printers)), Stations: make([]stationPrinterResponse, len(o.Stations))}
	byPrinter := map[int64][]string{}
	for i, st := range o.Stations {
		out.Stations[i] = stationPrinterResponse{Code: st.Code, Name: st.Name}
		if st.PrinterID.Valid {
			id := st.PrinterID.Int64
			out.Stations[i].PrinterID = &id
			byPrinter[id] = append(byPrinter[id], st.Code)
		}
	}
	for i, p := range o.Printers {
		out.Printers[i] = printerResponse{
			ID: p.ID, Name: p.Name, Transport: printers.Transport(p.Transport), Address: p.Address, PaperMM: p.PaperMm,
			Active: p.Active, IsReceipt: p.IsReceipt, Stations: byPrinter[p.ID],
		}
		if out.Printers[i].Stations == nil {
			out.Printers[i].Stations = []string{}
		}
		if p.LastJobID != 0 {
			out.Printers[i].LastJob = &lastJobResponse{
				ID: p.LastJobID, State: printjob.State(p.LastJobState), Label: p.LastJobLabel, Error: p.LastJobError,
				At: p.LastJobAt.Time.UTC(),
			}
		}
	}
	writeJSON(w, s.log, status, out)
}

func (s *Server) writePrinterError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *printers.Invalid
	switch {
	case errors.Is(err, printers.ErrDuplicate) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodePrinterNameTaken, invalid.Message, invalid.Field)
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidPrinter, invalid.Message, invalid.Field)
	case errors.Is(err, printers.ErrNotFound):
		writeError(w, s.log, http.StatusNotFound, CodePrinterNotFound, "No such printer.")
	case errors.Is(err, printers.ErrNoStation):
		writeError(w, s.log, http.StatusNotFound, CodeStationNotFound, "No such station.")
	case errors.Is(err, printers.ErrRetired):
		writeError(w, s.log, http.StatusConflict, CodePrinterRetired, "That printer is retired. Bring it back first.")
	default:
		s.log.Error("a printer settings request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}

func idOrZero(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
