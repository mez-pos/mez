package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/tax"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// The tax settings error codes (phase 10, Task 6b).
const (
	CodeTaxClassNotFound  ErrorCode = "tax_class_not_found"
	CodeTaxClassNameTaken ErrorCode = "tax_class_name_taken"
	CodeTaxClassInUse     ErrorCode = "tax_class_in_use"
	CodeTicketsOpen       ErrorCode = "tickets_open"
	CodeNumberingLocked   ErrorCode = "numbering_locked"
	CodeNumberTaken       ErrorCode = "number_taken"
)

// jurisdictionRate is one authority's rate under US sales tax, in thousandths
// of a percent.
type jurisdictionRate struct {
	Name string   `json:"name"`
	Rate tax.Rate `json:"rate"`
}

type taxSettingsRequest struct {
	Profile        string             `json:"profile"`
	RegistrationID string             `json:"registration_id"`
	Pricing        string             `json:"pricing"`
	StateCode      string             `json:"state_code"`
	Jurisdiction   []jurisdictionRate `json:"jurisdiction"`
}

type taxClassRequest struct {
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
}

type taxClassResponse struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
	Active bool     `json:"active"`
}

type itemRefResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// taxSettingsResponse is the venue's tax profile and tax classes as they stand.
// items_without_hsn is what the menu still owes a profile that prints HSN.
type taxSettingsResponse struct {
	Profile         string             `json:"profile"`
	RegistrationID  string             `json:"registration_id"`
	Pricing         string             `json:"pricing"`
	StateCode       string             `json:"state_code"`
	Jurisdiction    []jurisdictionRate `json:"jurisdiction"`
	NeedsHSN        bool               `json:"needs_hsn"`
	ItemsWithoutHSN []itemRefResponse  `json:"items_without_hsn"`
	Classes         []taxClassResponse `json:"classes"`
}

func (s *Server) mountTaxSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/tax", s.readTax)
	handle("PUT /api/v1/settings/tax", s.updateTax)
	mux.HandleFunc("/api/v1/settings/tax", s.methodNotAllowed)
	handle("POST /api/v1/settings/tax-classes", s.createTaxClass)
	mux.HandleFunc("/api/v1/settings/tax-classes", s.methodNotAllowed)
	handle("PUT /api/v1/settings/tax-classes/{id}", s.updateTaxClass)
	handle("POST /api/v1/settings/tax-classes/{id}/retire", s.activateTaxClass(false))
	handle("POST /api/v1/settings/tax-classes/{id}/restore", s.activateTaxClass(true))
}

func (s *Server) readTax(w http.ResponseWriter, r *http.Request) {
	s.writeTax(w, r, http.StatusOK)
}

func (s *Server) updateTax(w http.ResponseWriter, r *http.Request) {
	var req taxSettingsRequest
	if !s.decode(w, r, &req) {
		return
	}
	in := settings.TaxInput{
		Profile: req.Profile, RegistrationID: req.RegistrationID, Pricing: req.Pricing, StateCode: req.StateCode,
		Jurisdiction: make([]tax.Component, len(req.Jurisdiction)),
	}
	for i, c := range req.Jurisdiction {
		in.Jurisdiction[i] = tax.Component{Name: c.Name, Rate: c.Rate}
	}
	if err := s.settings.UpdateTax(r.Context(), Meta(r).VenueID, in); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeTax(w, r, http.StatusOK)
}

func (s *Server) createTaxClass(w http.ResponseWriter, r *http.Request) {
	var req taxClassRequest
	if !s.decode(w, r, &req) {
		return
	}
	if _, err := s.settings.CreateTaxClass(r.Context(), Meta(r).VenueID, settings.TaxClassInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeTax(w, r, http.StatusCreated)
}

func (s *Server) updateTaxClass(w http.ResponseWriter, r *http.Request) {
	var req taxClassRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	if err := s.settings.UpdateTaxClass(r.Context(), Meta(r).VenueID, id, settings.TaxClassInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeTax(w, r, http.StatusOK)
}

func (s *Server) activateTaxClass(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.pathID(w, r, "id")
		if !ok {
			return
		}
		if err := s.settings.SetTaxClassActive(r.Context(), Meta(r).VenueID, id, active); err != nil {
			s.writeSettingsError(w, r, err)
			return
		}
		s.writeTax(w, r, http.StatusOK)
	}
}

// writeTax answers with the tax settings as they now stand.
func (s *Server) writeTax(w http.ResponseWriter, r *http.Request, status int) {
	t, err := s.settings.Tax(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	out := taxSettingsResponse{
		Profile: t.Profile, RegistrationID: t.RegistrationID, Pricing: t.Pricing, StateCode: t.StateCode,
		NeedsHSN:        t.NeedsHSN,
		Jurisdiction:    make([]jurisdictionRate, len(t.Jurisdiction)),
		ItemsWithoutHSN: make([]itemRefResponse, len(t.ItemsWithoutHSN)),
		Classes:         make([]taxClassResponse, len(t.Classes)),
	}
	for i, c := range t.Jurisdiction {
		out.Jurisdiction[i] = jurisdictionRate{Name: c.Name, Rate: c.Rate}
	}
	for i, it := range t.ItemsWithoutHSN {
		out.ItemsWithoutHSN[i] = itemRefResponse(it)
	}
	for i, c := range t.Classes {
		out.Classes[i] = taxClassResponse(c)
	}
	writeJSON(w, s.log, status, out)
}

// writeSettingsRefusal maps the refusals that have a status and code of their
// own; ok is false for any other error.
func (s *Server) writeSettingsRefusal(w http.ResponseWriter, err error) bool {
	var inUse *settings.TaxClassInUse
	var invalid *settings.Invalid
	switch {
	case errors.Is(err, settings.ErrTaxClassName) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodeTaxClassNameTaken, invalid.Message, invalid.Field)
	case errors.Is(err, settings.ErrTaxClassNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeTaxClassNotFound, "No such tax class.")
	case errors.As(err, &inUse):
		writeError(w, s.log, http.StatusConflict, CodeTaxClassInUse,
			"Items still use this tax class: "+namesLine(inUse.Items, maxNamedItems)+". Move them to another class first.")
	case errors.Is(err, settings.ErrNumberingLocked) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodeNumberingLocked, invalid.Message, invalid.Field)
	case errors.Is(err, settings.ErrNumberTaken) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodeNumberTaken, invalid.Message, invalid.Field)
	case errors.Is(err, settings.ErrOpenOrders):
		writeError(w, s.log, http.StatusConflict, CodeTicketsOpen,
			"Orders are still open. Settle or void them before changing the tax profile.")
	default:
		return false
	}
	return true
}

// maxNamedItems is how many items a refusal names: enough to find them, few
// enough to read, on a menu of hundreds.
const maxNamedItems = 8

// namesLine is names as a sentence's list: all of them, or the first few and
// how many more.
func namesLine(names []string, max int) string {
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:max], ", "), len(names)-max)
}
