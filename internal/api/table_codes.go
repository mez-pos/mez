package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/qr"
	"github.com/byjoellobo/mez/internal/service/tablecodes"
)

// The codes on a venue's tables, for whoever runs the venue (phase 11, Task
// 6a, ADR-243):
//
//	GET  /api/v1/table-codes               every table on the floor with its code
//	POST /api/v1/table-codes               give each table without a code one
//	POST /api/v1/tables/{id}/code/renew    a new code; the old one stops working
//	POST /api/v1/tables/{id}/code/revoke   no code: the sticker leads nowhere
//	GET  /api/v1/tables/{id}/code/qr       the table's address as a QR code
//	GET  /api/v1/table-codes/sheet         every table's, to print (ADR-245)
//
// A code is a guest's only credential (ADR-206), so all of this is for
// someone who may change the venue's settings and for nobody else.

// CodeNoPublicURL: the venue has not said where guests reach it, so there is
// no address to put on a table.
const CodeNoPublicURL ErrorCode = "no_public_url"

// TableCodes is what the API needs of the table codes. *tablecodes.Service
// is it.
type TableCodes interface {
	List(ctx context.Context, venueID int64) ([]tablecodes.Coded, error)
	Ensure(ctx context.Context, venueID int64) ([]tablecodes.Coded, error)
	Renew(ctx context.Context, venueID, tableID int64) (string, error)
	Revoke(ctx context.Context, venueID, tableID int64) error
}

// WithTableCodes lets the back office manage the codes on the tables. It
// needs WithGuests, whose mount is where a code leads. publicURL is the
// address guests reach this venue at, from the venue's own configuration, or
// empty: it is what a printed code says, so it is never taken from a request.
func WithTableCodes(c TableCodes, publicURL string) Option {
	return func(s *Server) {
		s.tableCodes = c
		s.publicURL = strings.TrimRight(publicURL, "/")
	}
}

// tableCodeResponse is a table and the code on it. Code and URL are empty
// for a table with no code, which guests cannot order from.
type tableCodeResponse struct {
	TableID   int64  `json:"table_id"`
	TableName string `json:"table_name"`
	Code      string `json:"code"`
	// URL is the address the code's sticker says, at the venue's public
	// address. Empty when the venue has not set one.
	URL string `json:"url"`
}

// tableCodesResponse is every table on the floor. Address is where guests
// reach the venue, with the code to come after it, or empty when the venue
// has not said: then no table has a URL, and nothing can be printed.
type tableCodesResponse struct {
	Address string              `json:"address"`
	Tables  []tableCodeResponse `json:"tables"`
}

// tableCodeQRResponse is a table's address drawn as a QR code. The picture
// is an SVG in a field, as a report's file is its text in one (ADR-119): the
// staff's API is JSON throughout, and the back office reads it as it reads
// everything else. A screen shows it as an image and never as markup.
type tableCodeQRResponse struct {
	TableID   int64  `json:"table_id"`
	TableName string `json:"table_name"`
	URL       string `json:"url"`
	SVG       string `json:"svg"`
}

func (s *Server) mountTableCodes(mux *http.ServeMux) {
	if s.tableCodes == nil || s.guests == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/table-codes", s.listTableCodes)
	handle("POST /api/v1/table-codes", s.ensureTableCodes)
	mux.HandleFunc("/api/v1/table-codes", s.methodNotAllowed)
	handle("GET /api/v1/table-codes/sheet", s.tableCodeSheet)
	handle("POST /api/v1/tables/{id}/code/renew", s.renewTableCode)
	handle("POST /api/v1/tables/{id}/code/revoke", s.revokeTableCode)
	handle("GET /api/v1/tables/{id}/code/qr", s.tableCodeQR)
}

// tableAddress is where a table's code leads at the venue's public address,
// before the code, or empty when the venue has set none.
func (s *Server) tableAddress() string {
	if s.publicURL == "" {
		return ""
	}
	return s.publicURL + s.guestMount + "t/"
}

func (s *Server) writeTableCodes(w http.ResponseWriter, list []tablecodes.Coded) {
	address := s.tableAddress()
	out := tableCodesResponse{Address: address, Tables: make([]tableCodeResponse, len(list))}
	for i, c := range list {
		out.Tables[i] = tableCodeResponse{TableID: c.TableID, TableName: c.TableName, Code: c.Code}
		if address != "" && c.Code != "" {
			out.Tables[i].URL = address + c.Code
		}
	}
	// Credentials, and they change under a renew: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) listTableCodes(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	list, err := s.tableCodes.List(r.Context(), dev.VenueID)
	if err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	s.writeTableCodes(w, list)
}

func (s *Server) ensureTableCodes(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	list, err := s.tableCodes.Ensure(r.Context(), dev.VenueID)
	if err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	s.log.Info("codes were made for the tables without one", "venue_id", dev.VenueID, "actor_id", session.Staff.ID, "device_id", dev.ID)
	s.writeTableCodes(w, list)
}

// tableIDFrom reads the table in the path, and has answered when ok is false.
func (s *Server) tableIDFrom(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, s.log, http.StatusNotFound, CodeTableNotFound, "No such table on the floor.")
		return 0, false
	}
	return id, true
}

func (s *Server) renewTableCode(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tableIDFrom(w, r)
	if !ok {
		return
	}
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	if _, err := s.tableCodes.Renew(r.Context(), dev.VenueID, id); err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	// The code is not logged: it is the credential.
	s.log.Info("a table's code was renewed", "venue_id", dev.VenueID, "actor_id", session.Staff.ID, "device_id", dev.ID, "table_id", id)
	s.listTableCodes(w, r)
}

func (s *Server) revokeTableCode(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tableIDFrom(w, r)
	if !ok {
		return
	}
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	if err := s.tableCodes.Revoke(r.Context(), dev.VenueID, id); err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	s.log.Info("a table's code was turned off", "venue_id", dev.VenueID, "actor_id", session.Staff.ID, "device_id", dev.ID, "table_id", id)
	s.listTableCodes(w, r)
}

// tableCodeQR draws a table's address as a QR code: what is printed and
// stuck to the table. It is the venue's public address and no other, since a
// sticker saying an address only this network can reach is a sticker that
// works for nobody on mobile data.
func (s *Server) tableCodeQR(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tableIDFrom(w, r)
	if !ok {
		return
	}
	dev, _ := DeviceFrom(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	address := s.tableAddress()
	if address == "" {
		writeError(w, s.log, http.StatusConflict, CodeNoPublicURL,
			"The venue has not said where guests reach it. Set public_url in the venue's configuration, then print the codes.")
		return
	}
	list, err := s.tableCodes.List(r.Context(), dev.VenueID)
	if err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	code, name := "", ""
	for _, c := range list {
		if c.TableID == id {
			code, name = c.Code, c.TableName
		}
	}
	if code == "" {
		writeError(w, s.log, http.StatusNotFound, CodeTableNotFound, "That table has no code to draw.")
		return
	}
	drawn, err := qr.Encode([]byte(address + code))
	if err != nil {
		s.log.Error("drawing a table's address as a QR failed", "request_id", RequestID(r.Context()), "venue_id", dev.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	writeJSON(w, s.log, http.StatusOK, tableCodeQRResponse{TableID: id, TableName: name, URL: address + code, SVG: drawn.SVG(qrQuiet)})
}

// tableCodeSheetResponse is every table that has a code, each with its
// address drawn, for the sheet an owner prints and cuts up. A table with no
// code is not on it: there is nothing to stick on that table.
type tableCodeSheetResponse struct {
	Tables []tableCodeQRResponse `json:"tables"`
}

// tableCodeSheet draws them all in one answer: a sheet of thirty is one
// request and one moment, so every code on the paper was live together.
func (s *Server) tableCodeSheet(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	address := s.tableAddress()
	if address == "" {
		writeError(w, s.log, http.StatusConflict, CodeNoPublicURL,
			"The venue has not said where guests reach it. Set public_url in the venue's configuration, then print the codes.")
		return
	}
	list, err := s.tableCodes.List(r.Context(), dev.VenueID)
	if err != nil {
		s.writeTableCodeError(w, r, err)
		return
	}
	out := tableCodeSheetResponse{Tables: []tableCodeQRResponse{}}
	for _, c := range list {
		if c.Code == "" {
			continue
		}
		drawn, err := qr.Encode([]byte(address + c.Code))
		if err != nil {
			s.log.Error("drawing a table's address as a QR failed", "request_id", RequestID(r.Context()), "venue_id", dev.VenueID, "error", err)
			writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
			return
		}
		out.Tables = append(out.Tables, tableCodeQRResponse{TableID: c.TableID, TableName: c.TableName, URL: address + c.Code, SVG: drawn.SVG(qrQuiet)})
	}
	// Every code the venue has leaves the screen at once: worth a line.
	s.log.Info("the tables' codes were drawn to print", "venue_id", dev.VenueID, "actor_id", session.Staff.ID, "device_id", dev.ID, "tables", len(out.Tables))
	writeJSON(w, s.log, http.StatusOK, out)
}

func (s *Server) writeTableCodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, tablecodes.ErrNoTable) {
		writeError(w, s.log, http.StatusNotFound, CodeTableNotFound, "No such table on the floor.")
		return
	}
	s.log.Error("a table code request failed", "request_id", RequestID(r.Context()), "path", r.URL.Path, "error", err)
	writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
}
