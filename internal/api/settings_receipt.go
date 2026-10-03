package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// receiptRequest is what a venue adds to its receipts and credit notes:
// header lines under its name, footer lines at the bottom, and three parts it
// may leave off, each hidden when true. Lines are plain text the printer can
// print (ASCII, with accents spelled plainly); blank ones are dropped.
type receiptRequest struct {
	Header        []string `json:"header"`
	Footer        []string `json:"footer"`
	HideBarcode   bool     `json:"hide_barcode"`
	HideOrderRef  bool     `json:"hide_order_ref"`
	HideModifiers bool     `json:"hide_modifiers"`
}

// receiptResponse is the template as it stands, with the limits a form
// should hold to: at most max_lines lines each of header and footer, each
// max_line_chars long.
type receiptResponse struct {
	receiptRequest
	MaxLines     int `json:"max_lines"`
	MaxLineChars int `json:"max_line_chars"`
}

func (s *Server) mountReceiptSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/receipt", s.readReceipt)
	handle("PUT /api/v1/settings/receipt", s.updateReceipt)
	mux.HandleFunc("/api/v1/settings/receipt", s.methodNotAllowed)
}

func (s *Server) readReceipt(w http.ResponseWriter, r *http.Request) {
	s.writeReceipt(w, r)
}

func (s *Server) updateReceipt(w http.ResponseWriter, r *http.Request) {
	var req receiptRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.settings.UpdateReceipt(r.Context(), Meta(r).VenueID, settings.ReceiptInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeReceipt(w, r)
}

// writeReceipt answers with the template as it now stands.
func (s *Server) writeReceipt(w http.ResponseWriter, r *http.Request) {
	t, err := s.settings.Receipt(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, receiptResponse{
		receiptRequest: receiptRequest(t.ReceiptInput), MaxLines: t.MaxLines, MaxLineChars: t.MaxLineChars,
	})
}
