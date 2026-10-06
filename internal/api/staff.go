package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

// HeaderStaffToken carries the staff session token. It is a header of its
// own rather than another Authorization scheme because both are sent at
// once: the device's token says which terminal, this says who is on it.
const HeaderStaffToken = "X-Staff-Token"

// Staff is what the API needs from staff authentication.
// *auth.StaffAuth is the implementation.
type Staff interface {
	ActiveStaff(ctx context.Context, venueID int64) ([]auth.Staff, error)
	WithPermission(ctx context.Context, venueID int64, p auth.Permission) ([]auth.Staff, error)
	SignIn(ctx context.Context, dev auth.Device, staffRef, pin string) (auth.SignedIn, error)
	Session(ctx context.Context, dev auth.Device, token string) (auth.Session, error)
	SignOut(ctx context.Context, s auth.Session) error
	Permissions(ctx context.Context, roleID int64) (auth.PermissionSet, error)
	Authorise(ctx context.Context, dev auth.Device, staffRef, pin string, p auth.Permission) (auth.Staff, error)
}

// WithStaff turns on staff sign-in. It needs WithDevices: a PIN is only
// accepted from a paired device.
func WithStaff(a Staff) Option {
	return func(s *Server) { s.staff = a }
}

// SessionFrom is the staff session behind the request, if there is one.
func SessionFrom(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(staffKey).(auth.Session)
	return s, ok
}

// mountStaff adds the staff list, sign-in and sign-out.
func (s *Server) mountStaff(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil {
		return
	}
	mux.Handle("GET /api/v1/staff", s.requireDevice(http.HandlerFunc(s.listStaff)))
	mux.HandleFunc("/api/v1/staff", s.methodNotAllowed)

	mux.Handle("POST /api/v1/sessions", s.requireDevice(http.HandlerFunc(s.signIn)))
	mux.HandleFunc("/api/v1/sessions", s.methodNotAllowed)

	current := s.requireDevice(s.requireStaff(http.HandlerFunc(s.currentSession)))
	mux.Handle("GET /api/v1/sessions/current", current)
	mux.Handle("DELETE /api/v1/sessions/current", s.requireDevice(s.requireStaff(http.HandlerFunc(s.signOut))))
	mux.HandleFunc("/api/v1/sessions/current", s.methodNotAllowed)
}

// requireStaff admits a request whose staff session token belongs to a live
// session on this device. It runs inside requireDevice, which has already
// established which device is asking.
func (s *Server) requireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dev, ok := DeviceFrom(r.Context())
		if !ok {
			// Only reachable by mounting this without requireDevice around
			// it, which would be a routing bug rather than a bad request.
			s.log.Error("staff check with no device", "request_id", RequestID(r.Context()), "path", s.loggedPath(r))
			writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
			return
		}
		session, err := s.staff.Session(r.Context(), dev, r.Header.Get(HeaderStaffToken))
		if errors.Is(err, auth.ErrNoSession) {
			s.unauthenticated(w, CodeSignInRequired, "sign in to do that")
			return
		}
		if err != nil {
			s.log.Error("checking a staff session failed", "request_id", RequestID(r.Context()), "error", err)
			writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
			return
		}
		if info := infoFrom(r.Context()); info != nil {
			info.staffID = session.Staff.ID
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), staffKey, session)))
	})
}

// staffResponse is a staff member as the API shows them.
type staffResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Role is the name of their role, shown under the name on the sign-in
	// screen. It is a label only: the permissions are on the session.
	Role string `json:"role"`
}

type staffListResponse struct {
	Staff []staffResponse `json:"staff"`
}

// signInRequest is the body of POST /api/v1/sessions.
type signInRequest struct {
	StaffID string `json:"staff_id"`
	PIN     string `json:"pin"`
}

// sessionResponse is a staff session. The token is only in the response that
// opens it. Permissions are what the staff member's role grants, so a screen
// can hide what would only be refused (ADR-105); every endpoint still checks.
// They are here and not on staffResponse, which any paired device can list.
type sessionResponse struct {
	Staff       staffResponse     `json:"staff"`
	Permissions []auth.Permission `json:"permissions"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Token       string            `json:"token,omitempty"`
}

func toStaffResponse(s auth.Staff) staffResponse {
	return staffResponse{ID: s.PublicID, Name: s.Name, Role: s.RoleName}
}

// listStaff is who can sign in here, for the terminal's name tiles.
func (s *Server) listStaff(w http.ResponseWriter, r *http.Request) {
	dev, _ := DeviceFrom(r.Context())
	staff, err := s.staff.ActiveStaff(r.Context(), dev.VenueID)
	if err != nil {
		s.log.Error("listing staff failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	out := make([]staffResponse, len(staff))
	for i, member := range staff {
		out[i] = toStaffResponse(member)
	}
	writeJSON(w, s.log, http.StatusOK, staffListResponse{Staff: out})
}

// signIn opens a staff session on this device.
func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	dev, _ := DeviceFrom(r.Context())

	in, err := s.staff.SignIn(r.Context(), dev, req.StaffID, req.PIN)
	var locked *auth.LockedError
	switch {
	case errors.As(err, &locked):
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(locked.RetryAfter)))
		writeError(w, s.log, http.StatusTooManyRequests, CodeLockedOut,
			"too many failed attempts; sign-in is locked for a few minutes")
		return
	case errors.Is(err, auth.ErrInvalidPIN):
		// Which of the three it was stays here. The client is told one
		// thing, so it cannot be used to find out who works here.
		s.unauthenticated(w, CodeInvalidPIN, "that staff member or PIN is not right")
		return
	case err != nil:
		s.log.Error("signing in failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	if info := infoFrom(r.Context()); info != nil {
		info.staffID = in.Session.Staff.ID
	}
	res, err := s.toSessionResponse(r.Context(), in.Session)
	if err != nil {
		s.log.Error("reading permissions at sign-in failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	res.Token = in.Token
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusCreated, res)
}

// currentSession says who is signed in on this device, and what they may do.
func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	res, err := s.toSessionResponse(r.Context(), session)
	if err != nil {
		s.log.Error("reading permissions failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	writeJSON(w, s.log, http.StatusOK, res)
}

// toSessionResponse is a session as the API shows it, without its token.
func (s *Server) toSessionResponse(ctx context.Context, session auth.Session) (sessionResponse, error) {
	held, err := s.staff.Permissions(ctx, session.Staff.RoleID)
	if err != nil {
		return sessionResponse{}, fmt.Errorf("permissions for role %d: %w", session.Staff.RoleID, err)
	}
	return sessionResponse{
		Staff:       toStaffResponse(session.Staff),
		Permissions: held.Keys(),
		ExpiresAt:   session.ExpiresAt.UTC(),
	}, nil
}

// signOut ends the session. The device stays paired.
func (s *Server) signOut(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFrom(r.Context())
	if err := s.staff.SignOut(r.Context(), session); err != nil {
		s.log.Error("signing out failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
