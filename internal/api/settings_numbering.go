package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/settings"
)

type numberingRequest struct {
	InvoiceSeries           string `json:"invoice_series"`
	CreditSeries            string `json:"credit_series"`
	InvoicePrefix           string `json:"invoice_prefix"`
	CreditPrefix            string `json:"credit_prefix"`
	Template                string `json:"template"`
	FinancialYearStartMonth int    `json:"financial_year_start_month"`
	CashRoundingStep        int64  `json:"cash_rounding_step"`
}

// numberingResponse is how invoices and credit notes are numbered as they
// stand, and what each series would write next. Money is minor units, so
// cash_rounding_step is too; max_rounding_step is one whole unit of the
// currency.
type numberingResponse struct {
	numberingRequest
	Currency        string `json:"currency"`
	NextInvoiceNo   string `json:"next_invoice_no"`
	NextCreditNo    string `json:"next_credit_no"`
	NumberLimit     int    `json:"number_limit"`
	FYLocked        bool   `json:"fy_locked"`
	MaxRoundingStep int64  `json:"max_rounding_step"`
}

func (s *Server) mountNumberingSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/numbering", s.readNumbering)
	handle("PUT /api/v1/settings/numbering", s.updateNumbering)
	handle("POST /api/v1/settings/numbering/preview", s.previewNumbering)
	mux.HandleFunc("/api/v1/settings/numbering/preview", s.methodNotAllowed)
	mux.HandleFunc("/api/v1/settings/numbering", s.methodNotAllowed)
}

func (s *Server) readNumbering(w http.ResponseWriter, r *http.Request) {
	s.writeNumbering(w, r)
}

func (s *Server) updateNumbering(w http.ResponseWriter, r *http.Request) {
	var req numberingRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.settings.UpdateNumbering(r.Context(), Meta(r).VenueID, settings.NumberingInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeNumbering(w, r)
}

// previewNumbering answers with the numbering a save of the body would give,
// and what each series would write next, without saving anything: the screen's
// live preview. A body a save would refuse is refused the same way.
func (s *Server) previewNumbering(w http.ResponseWriter, r *http.Request) {
	var req numberingRequest
	if !s.decode(w, r, &req) {
		return
	}
	n, err := s.settings.PreviewNumbering(r.Context(), Meta(r).VenueID, settings.NumberingInput(req))
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeNumberingOf(w, n)
}

// writeNumbering answers with the numbering as it now stands.
func (s *Server) writeNumbering(w http.ResponseWriter, r *http.Request) {
	n, err := s.settings.Numbering(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeNumberingOf(w, n)
}

func (s *Server) writeNumberingOf(w http.ResponseWriter, n settings.Numbering) {
	writeJSON(w, s.log, http.StatusOK, numberingResponse{
		numberingRequest: numberingRequest(n.NumberingInput),
		Currency:         n.Currency, NextInvoiceNo: n.NextInvoiceNo, NextCreditNo: n.NextCreditNo,
		NumberLimit: n.NumberLimit, FYLocked: n.FYLocked, MaxRoundingStep: n.MaxRoundingStep,
	})
}
