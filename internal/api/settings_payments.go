package api

import (
	"net/http"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/settings"
)

// paymentSettingsRequest is where the venue is paid when a guest pays from
// a phone (ADR-215): its UPI address, and the name a guest's app shows.
// Either may be empty: no address is no UPI on a guest's bill, and no payee
// is the venue's name.
type paymentSettingsRequest struct {
	UPIAddress string `json:"upi_address"`
	UPIPayee   string `json:"upi_payee"`
	// Mode is pay_at_table or prepay (ADR-225). Left out, it is not changed.
	Mode svcorder.PayMode `json:"mode,omitempty"`
}

// paymentSettingsResponse is the payment settings as they stand. venue_name
// is what an empty payee reads as; currency is the venue's, since UPI pays
// in INR only; max_payee is the longest name a form should take.
type paymentSettingsResponse struct {
	paymentSettingsRequest
	VenueName string `json:"venue_name"`
	Currency  string `json:"currency"`
	MaxPayee  int    `json:"max_payee"`
	// CanCharge is whether guests can pay online here: a payment gateway
	// is set up. Paying first is in effect only with it, whatever mode says.
	CanCharge bool `json:"can_charge"`
	// Online is how paying online stands, when a gateway is set up. It is
	// read only: a gateway and its keys are the venue's configuration and
	// not a setting (ADR-221, ADR-231). No key is ever in it.
	Online *onlinePaymentsResponse `json:"online,omitempty"`
}

// onlinePaymentsResponse is the gateway guests pay online through, for the
// owner to check. gateway is its name, "razorpay". webhooks is whether its
// calls are taken, so a payment lands at once; without them it lands when
// mez next asks. return_url is where a guest is sent back after paying, or
// empty when the venue's address is not set. last_paid_at is when a payment
// through it last landed on a bill, absent when none ever has.
type onlinePaymentsResponse struct {
	Gateway    string     `json:"gateway"`
	Webhooks   bool       `json:"webhooks"`
	ReturnURL  string     `json:"return_url"`
	LastPaidAt *time.Time `json:"last_paid_at,omitempty"`
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
	out := paymentSettingsResponse{
		paymentSettingsRequest: paymentSettingsRequest(p.PaymentsInput),
		VenueName:              p.VenueName, Currency: p.Currency, MaxPayee: p.MaxPayee,
		CanCharge: s.charges != nil,
	}
	if s.charges != nil {
		st, err := s.charges.Standing(r.Context(), Meta(r).VenueID)
		if err != nil {
			s.internalError(w, r, "reading how paying online stands failed", err, "venue_id", Meta(r).VenueID)
			return
		}
		out.Online = &onlinePaymentsResponse{Gateway: st.Gateway, Webhooks: st.Notices}
		if s.public() != "" {
			out.Online.ReturnURL = s.public() + s.guestMount
		}
		if !st.LastPaidAt.IsZero() {
			out.Online.LastPaidAt = &st.LastPaidAt
		}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}
