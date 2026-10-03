package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// Settings is what the API needs from the venue settings.
// *settings.Service is the implementation.
type Settings interface {
	Read(ctx context.Context, venueID int64) (settings.Overview, error)
	VenueName(ctx context.Context, venueID int64) (string, error)
	UpdateDetails(ctx context.Context, venueID int64, in settings.Details) error
	Tax(ctx context.Context, venueID int64) (settings.TaxSetup, error)
	UpdateTax(ctx context.Context, venueID int64, in settings.TaxInput) error
	Numbering(ctx context.Context, venueID int64) (settings.Numbering, error)
	UpdateNumbering(ctx context.Context, venueID int64, in settings.NumberingInput) error
	PreviewNumbering(ctx context.Context, venueID int64, in settings.NumberingInput) (settings.Numbering, error)
	Rules(ctx context.Context, venueID int64) (settings.Rules, error)
	UpdateRules(ctx context.Context, venueID int64, in settings.RulesInput) error
	Receipt(ctx context.Context, venueID int64) (settings.Receipt, error)
	UpdateReceipt(ctx context.Context, venueID int64, in settings.ReceiptInput) error
	PreviewReceipt(ctx context.Context, venueID int64, in settings.ReceiptInput, paperMM int) (settings.ReceiptPreview, error)
	CreateTaxClass(ctx context.Context, venueID int64, in settings.TaxClassInput) (int64, error)
	UpdateTaxClass(ctx context.Context, venueID, id int64, in settings.TaxClassInput) error
	SetTaxClassActive(ctx context.Context, venueID, id int64, active bool) error
}

// WithSettings mounts the venue settings (phase 10, Task 6a). Reading them
// needs WithDevices and WithStaff and settings.edit; the venue's name needs
// only a paired device, for the lock screen.
func WithSettings(v Settings) Option {
	return func(s *Server) { s.settings = v }
}

// The venue settings error codes.
const (
	CodeInvalidSetting ErrorCode = "invalid_setting"
	CodeVenueNotFound  ErrorCode = "venue_not_found"
)

type venueDetailsRequest struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Timezone  string `json:"timezone"`
}

// venueNameResponse is what a paired device may know of the venue.
type venueNameResponse struct {
	Name string `json:"name"`
}

type settingsVenueResponse struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Currency  string `json:"currency"`
	Timezone  string `json:"timezone"`
}

// settingEntryResponse is one key: what it is set to, and what it is when
// it is not. Value is empty when not set.
type settingEntryResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Set     bool   `json:"set"`
	Default string `json:"default"`
}

type settingsResponse struct {
	Venue    settingsVenueResponse  `json:"venue"`
	Settings []settingEntryResponse `json:"settings"`
}

func (s *Server) mountSettings(mux *http.ServeMux) {
	if s.devices == nil || s.settings == nil {
		return
	}
	mux.Handle("GET /api/v1/venue", s.requireDevice(http.HandlerFunc(s.readVenueName)))
	mux.HandleFunc("/api/v1/venue", s.methodNotAllowed)

	if s.staff == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings", s.readSettings)
	mux.HandleFunc("/api/v1/settings", s.methodNotAllowed)
	handle("PUT /api/v1/settings/venue", s.updateVenueDetails)
	mux.HandleFunc("/api/v1/settings/venue", s.methodNotAllowed)
	s.mountTaxSettings(mux)
	s.mountNumberingSettings(mux)
	s.mountRulesSettings(mux)
	s.mountReceiptSettings(mux)
}

func (s *Server) readVenueName(w http.ResponseWriter, r *http.Request) {
	name, err := s.settings.VenueName(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, venueNameResponse{Name: name})
}

func (s *Server) readSettings(w http.ResponseWriter, r *http.Request) {
	s.writeSettings(w, r, http.StatusOK)
}

func (s *Server) updateVenueDetails(w http.ResponseWriter, r *http.Request) {
	var req venueDetailsRequest
	if !s.decode(w, r, &req) {
		return
	}
	err := s.settings.UpdateDetails(r.Context(), Meta(r).VenueID, settings.Details(req))
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writeSettings(w, r, http.StatusOK)
}

// writeSettings answers with the settings as they now stand, so a screen
// redraws from the server after every save.
func (s *Server) writeSettings(w http.ResponseWriter, r *http.Request, status int) {
	o, err := s.settings.Read(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	out := settingsResponse{
		Venue:    settingsVenueResponse(o.Venue),
		Settings: make([]settingEntryResponse, len(o.Entries)),
	}
	for i, e := range o.Entries {
		out.Settings[i] = settingEntryResponse(e)
	}
	writeJSON(w, s.log, status, out)
}

func (s *Server) writeSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *settings.Invalid
	if s.writeSettingsRefusal(w, err) {
		return
	}
	switch {
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidSetting, invalid.Message, invalid.Field)
	case errors.Is(err, settings.ErrNoVenue):
		writeError(w, s.log, http.StatusNotFound, CodeVenueNotFound, "No such venue.")
	default:
		s.log.Error("a venue settings request failed",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}
