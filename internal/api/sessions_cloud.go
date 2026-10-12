package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/service/cloudsignin"
)

// The owner signs in from mez Cloud (ADR-323, ADR-324).
const (
	// CodeCloudSignInRefused is a sign-in on mez Cloud's word that the
	// venue does not accept. Why is not said: not paired, nobody for the
	// cloud to be, and an assertion that is not good all read alike.
	CodeCloudSignInRefused ErrorCode = "cloud_sign_in_refused"
	// CodeCloudClock is an assertion rightly signed for this venue and
	// outside its five minutes: the venue computer's clock is wrong.
	CodeCloudClock ErrorCode = "cloud_clock"

	// ByCloud is what the access log says vouched for a request served on a
	// session mez Cloud opened.
	ByCloud = "cloud"
)

// CloudSignIn signs the owner in on mez Cloud's word.
// *cloudsignin.Service is the implementation.
type CloudSignIn interface {
	SignIn(ctx context.Context, assertion string) (cloudsignin.SignedIn, error)
}

// WithCloudSignIn lets mez Cloud sign the account's owner in to the back
// office through the relay. It needs WithDevices and WithStaff.
func WithCloudSignIn(c CloudSignIn) Option {
	return func(s *Server) { s.cloudSignIn = c }
}

// cloudSignInLimit is what one address may ask in a minute. A signature is
// not something to be guessed, so this is not against guessing: it bounds
// what an outsider can make this computer spend on checking them.
var cloudSignInLimit = limit{requests: 30, per: time.Minute}

// cloudSessionResponse is a session mez Cloud opened: the session as any
// sign-in answers it, and the device it is on. The token is presented from
// then on as both the device's and the session's (ADR-324).
type cloudSessionResponse struct {
	Staff       staffResponse     `json:"staff"`
	Permissions []auth.Permission `json:"permissions"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Token       string            `json:"token"`
	Device      deviceResponse    `json:"device"`
}

// mountCloudSignIn adds the one route. It is not behind requireDevice: the
// assertion is what says who is asking.
func (s *Server) mountCloudSignIn(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil {
		return
	}
	mux.Handle("POST /api/v1/sessions/cloud", s.rateLimit(cloudSignInLimit, http.HandlerFunc(s.signInFromCloud)))
	mux.HandleFunc("/api/v1/sessions/cloud", s.methodNotAllowed)
}

// assertionOf reads "Authorization: Mez-Cloud <assertion>".
func assertionOf(r *http.Request) (string, bool) {
	scheme, presented, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, cloud.AssertionScheme) {
		return "", false
	}
	presented = strings.TrimSpace(presented)
	return presented, presented != ""
}

func (s *Server) signInFromCloud(w http.ResponseWriter, r *http.Request) {
	// Nothing here is to be kept by anything between the cloud and the venue.
	w.Header().Set("Cache-Control", "no-store")
	// mez Cloud reaches a venue through the relay and no other way. On the
	// venue's own network there is a till, and a PIN.
	if !relayed(r) {
		writeError(w, s.log, http.StatusForbidden, CodeCloudSignInRefused, "mez Cloud signs in through the relay, not at the venue.")
		return
	}
	presented, ok := assertionOf(r)
	if !ok || s.cloudSignIn == nil {
		s.refuseCloudSignIn(w)
		return
	}
	in, err := s.cloudSignIn.SignIn(r.Context(), presented)
	switch {
	case errors.Is(err, cloudsignin.ErrClock):
		w.Header().Set("WWW-Authenticate", cloud.AssertionScheme+` realm="mez"`)
		writeError(w, s.log, http.StatusUnauthorized, CodeCloudClock,
			"The venue's computer and mez Cloud disagree about the time. Check the clock on the venue's computer, then try again.")
		return
	case errors.Is(err, cloudsignin.ErrRefused):
		s.refuseCloudSignIn(w)
		return
	case err != nil:
		s.internalError(w, r, "signing in from mez Cloud failed", err)
		return
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID, info.deviceID = in.Device.VenueID, in.Device.ID
		info.staffID, info.by = in.Session.Staff.ID, ByCloud
	}
	res, err := s.toSessionResponse(r.Context(), in.Session)
	if err != nil {
		s.internalError(w, r, "reading permissions at a cloud sign-in failed", err)
		return
	}
	// The same rule a PIN from away meets: only the desk is used from
	// outside. The cloud's owner holds every permission unless someone at
	// the venue has moved them to a role that does not, which is that
	// someone's way of saying no.
	held, err := s.staff.Permissions(r.Context(), in.Session.Staff.RoleID)
	if err != nil {
		s.internalError(w, r, "reading permissions at a cloud sign-in failed", err)
		return
	}
	if !remoteStaff(held) {
		if err := s.staff.SignOut(r.Context(), in.Session); err != nil {
			s.internalError(w, r, "closing a refused cloud session failed", err)
			return
		}
		s.refuseCloudSignIn(w)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, cloudSessionResponse{
		Staff: res.Staff, Permissions: res.Permissions, ExpiresAt: res.ExpiresAt,
		Token: in.Token, Device: toDeviceResponse(in.Device),
	})
}

// refuseCloudSignIn answers every sign-in the venue does not accept in one
// voice. The log has the reason.
func (s *Server) refuseCloudSignIn(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", cloud.AssertionScheme+` realm="mez"`)
	writeError(w, s.log, http.StatusUnauthorized, CodeCloudSignInRefused, "This venue does not accept that sign-in from mez Cloud.")
}

// cloudDevice finds the cloud's device behind a bearer token that is a
// session's and not a device's (ADR-324). It reports false, having answered
// nothing, for any token that is not one: the caller then judges it as a
// device's token, as it always did. Only a relayed request is looked at:
// the cloud has no place on the venue's own network.
func (s *Server) cloudDevice(r *http.Request, token string) (auth.Device, bool) {
	if s.staff == nil || !relayed(r) {
		return auth.Device{}, false
	}
	dev, err := s.staff.CloudDevice(r.Context(), token)
	if err != nil {
		if !errors.Is(err, auth.ErrUnauthenticated) {
			// Not answered here: the device check that follows refuses the
			// token as one that opens nothing.
			s.log.Error("looking for the cloud's device failed", "request_id", RequestID(r.Context()), "error", err)
		}
		return auth.Device{}, false
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID, info.deviceID = dev.VenueID, dev.ID
	}
	return dev, true
}
