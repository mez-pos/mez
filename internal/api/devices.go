package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

// Devices is what the API needs from device authentication.
// *auth.Devices is the implementation.
type Devices interface {
	Authenticate(ctx context.Context, token string) (auth.Device, error)
	Redeem(ctx context.Context, code string) (auth.Paired, error)
	CreatePairingCode(ctx context.Context, n auth.NewCode) (auth.PairingCode, error)
	Roster(ctx context.Context, venueID int64) (auth.Roster, error)
	Rename(ctx context.Context, venueID int64, publicID, name string, by int64) error
	RevokeByPublicID(ctx context.Context, venueID int64, publicID string, actingID, by int64) error
}

// WithDevices turns on device authentication and the pairing endpoint.
// Without it the authenticated routes are not mounted at all, so a server
// built without it cannot serve them unauthenticated.
func WithDevices(d Devices) Option {
	return func(s *Server) { s.devices = d }
}

// DeviceFrom is the device that authenticated the request, if one did.
func DeviceFrom(ctx context.Context) (auth.Device, bool) {
	d, ok := ctx.Value(deviceKey).(auth.Device)
	return d, ok
}

// mountDevices adds the pairing endpoint and every route that needs a device.
func (s *Server) mountDevices(mux *http.ServeMux) {
	if s.devices == nil {
		return
	}
	// Pairing is the one endpoint anyone on the LAN may call, so it is the
	// one needing a limit per caller. Sign-in has its lockout instead.
	mux.Handle("POST /api/v1/devices/pair", s.rateLimit(pairLimit, http.HandlerFunc(s.pairDevice)))
	mux.HandleFunc("/api/v1/devices/pair", s.methodNotAllowed)

	mux.Handle("GET /api/v1/devices/me", s.requireDevice(http.HandlerFunc(s.currentDevice)))
	mux.HandleFunc("/api/v1/devices/me", s.methodNotAllowed)

	// Making a code is a back office action: a signed-in staff member who
	// may manage devices, on a device that is already paired.
	if s.staff != nil {
		mux.Handle("POST /api/v1/devices/codes", s.requireDevice(s.requireStaff(
			s.requirePermission(auth.PermDeviceManage, http.HandlerFunc(s.createPairingCode)))))
		mux.HandleFunc("/api/v1/devices/codes", s.methodNotAllowed)
		s.mountDeviceManagement(mux)
	}
}

// newCodeRequest is the body of POST /api/v1/devices/codes.
type newCodeRequest struct {
	Name string          `json:"name"`
	Kind auth.DeviceKind `json:"kind"`
}

// pairingCodeResponse is a code to read out to whoever is pairing.
type pairingCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// createPairingCode makes a code for one new device.
func (s *Server) createPairingCode(w http.ResponseWriter, r *http.Request) {
	var req newCodeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	dev, _ := DeviceFrom(r.Context())
	session, _ := SessionFrom(r.Context())

	code, err := s.devices.CreatePairingCode(r.Context(), auth.NewCode{
		VenueID: dev.VenueID, CreatedBy: session.Staff.ID, DeviceName: req.Name, DeviceKind: req.Kind,
	})
	var invalid *auth.InvalidDevice
	if errors.As(err, &invalid) {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, invalid.Message, invalid.Field)
		return
	}
	if err != nil {
		s.internalError(w, r, "making a pairing code failed", err)
		return
	}
	// The code pairs a device, so it is a secret for as long as it lives.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusCreated, pairingCodeResponse{Code: code.Code, ExpiresAt: code.ExpiresAt.UTC()})
}

// requireDevice admits a request carrying a paired, unrevoked device's token
// as "Authorization: Bearer <token>", and puts the device in its context.
func (s *Server) requireDevice(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearer(r)
		if !ok {
			s.unauthenticated(w, CodeUnauthenticated, "a device token is needed")
			return
		}
		dev, ok := s.authenticateDevice(w, r, token)
		if !ok {
			return
		}
		if !s.remoteDevice(w, r, dev) {
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceKey, dev)))
	})
}

// authenticateDevice turns a device token into the device, answering the
// request itself when it cannot: a revoked device, a token that opens
// nothing, or a store that failed. It is the one place a token is judged,
// for the routes behind requireDevice and for the stream, which reads its
// token from the subprotocol and cannot sit behind the middleware.
func (s *Server) authenticateDevice(w http.ResponseWriter, r *http.Request, token string) (auth.Device, bool) {
	dev, err := s.devices.Authenticate(r.Context(), token)
	switch {
	case errors.Is(err, auth.ErrDeviceRevoked):
		s.unauthenticated(w, CodeDeviceRevoked, "this device has been unpaired; pair it again")
		return auth.Device{}, false
	case errors.Is(err, auth.ErrUnauthenticated):
		s.unauthenticated(w, CodeUnauthenticated, "the device token is not valid")
		return auth.Device{}, false
	case err != nil:
		s.internalError(w, r, "authenticating a device failed", err)
		return auth.Device{}, false
	}
	if info := infoFrom(r.Context()); info != nil {
		info.venueID, info.deviceID = dev.VenueID, dev.ID
	}
	return dev, true
}

// bearer reads a bearer token. The scheme is case-insensitive, per RFC 9110.
func bearer(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func (s *Server) unauthenticated(w http.ResponseWriter, code ErrorCode, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="mez"`)
	writeError(w, s.log, http.StatusUnauthorized, code, message)
}

// pairRequest is the body of POST /api/v1/devices/pair.
type pairRequest struct {
	Code string `json:"code"`
}

// deviceResponse is a device as the API shows it: by public ID.
type deviceResponse struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Kind auth.DeviceKind `json:"kind"`
}

// pairResponse is a newly paired device and the token it keeps.
type pairResponse struct {
	Device deviceResponse `json:"device"`
	Token  string         `json:"token"`
}

func toDeviceResponse(d auth.Device) deviceResponse {
	return deviceResponse{ID: d.PublicID, Name: d.Name, Kind: d.Kind}
}

// pairDevice redeems a pairing code for a device token.
func (s *Server) pairDevice(w http.ResponseWriter, r *http.Request) {
	var req pairRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	paired, err := s.devices.Redeem(r.Context(), req.Code)
	if errors.Is(err, auth.ErrInvalidPairingCode) {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidPairingCode,
			"that code is not valid; ask for a new one", "code")
		return
	}
	if err != nil {
		s.internalError(w, r, "pairing a device failed", err)
		return
	}
	// The token appears in this response and nowhere else, ever.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusCreated, pairResponse{Device: toDeviceResponse(paired.Device), Token: paired.Token})
}

// currentDevice tells a device who it is paired as. A terminal calls it on
// start to find out whether its stored token still works.
func (s *Server) currentDevice(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	writeJSON(w, s.log, http.StatusOK, toDeviceResponse(dev))
}
