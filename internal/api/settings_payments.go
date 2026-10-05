package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// paymentSettingsRequest is where the venue is paid when a guest pays from
// a phone (ADR-215): its UPI address, and the name a guest's app shows.
// Either may be empty: no address is no UPI on a guest's bill, and no payee
// is the venue's name.
type paymentSettingsRequest struct {
	UPIAddress string `json:"upi_address"`
	UPIPayee   string `json:"upi_payee"`
}

// paymentSettingsResponse is the payment settings as they stand. venue_name
// is what an empty payee reads as; currency is the venue's, since UPI pays
// in INR only; max_payee is the longest name a form should take.
type paymentSettingsResponse struct {
	paymentSettingsRequest
	VenueName string `json:"venue_name"`
	Currency  string `json:"currency"`
	MaxPayee  int    `json:"max_payee"`
}

func (s *Server) mountPaymentSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/payments", s.writePaymentSettings)
	handle("PUT /api/v1/settings/payments", s.updatePaymentSettings)
	mux.HandleFunc("/api/v1/settings/payments", s.methodNotAllowed)
}

func (s *Server) updatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	var req paymentSettingsRequest
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.settings.UpdatePayments(r.Context(), Meta(r).VenueID, settings.PaymentsInput(req)); err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	s.writePaymentSettings(w, r)
}

// writePaymentSettings answers with the payment settings as they now stand.
func (s *Server) writePaymentSettings(w http.ResponseWriter, r *http.Request) {
	p, err := s.settings.Payments(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeSettingsError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, paymentSettingsResponse{
		paymentSettingsRequest: paymentSettingsRequest(p.PaymentsInput),
		VenueName:              p.VenueName, Currency: p.Currency, MaxPayee: p.MaxPayee,
	})
}
