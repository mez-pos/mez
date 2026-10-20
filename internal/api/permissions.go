package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/mez-pos/mez/internal/auth"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// HeaderIdempotencyKey is honoured on every mutating endpoint.
const HeaderIdempotencyKey = "Idempotency-Key"

// requirePermission admits a request whose signed-in staff member holds p.
// It runs inside requireStaff, which has established who they are.
//
// A denial is a 403 with a code the client can act on, never a 500: a
// cashier tapping a button they do not have is an ordinary event in a cafe,
// not an error.
func (s *Server) requirePermission(p auth.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFrom(r.Context())
		if !ok {
			// Only reachable by mounting this outside requireStaff, which
			// would be a routing bug rather than a bad request.
			s.log.Error("permission check with no staff session", "request_id", RequestID(r.Context()), "path", s.loggedPath(r))
			writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
			return
		}
		held, err := s.staff.Permissions(r.Context(), session.Staff.RoleID)
		if err != nil {
			s.internalError(w, r, "reading permissions failed", err)
			return
		}
		if !held.Has(p) {
			writeFieldError(w, s.log, http.StatusForbidden, CodeForbidden,
				"you do not have permission to do that", string(p))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Override is the manager's authorisation a void, comp, discount, payment
// void or reopen carries. Request types that need one embed it.
//
// The staff member named here is the one whose permission and PIN are
// checked, not the cashier who is signed in: the whole point is that the
// person at the till cannot do it alone.
type Override struct {
	AuthorisedBy string `json:"authorised_by"`
	PIN          string `json:"pin"`
}

// authorise checks an override for p and returns who authorised it. The
// caller puts the returned ID on the command as AuthorisedBy.
//
// It answers the client itself and reports whether the request is finished:
// the errors are the same shapes sign-in uses, so a terminal already knows
// how to show them.
func (s *Server) authorise(w http.ResponseWriter, r *http.Request, o Override, p auth.Permission) (auth.Staff, bool) {
	dev, ok := DeviceFrom(r.Context())
	if !ok {
		s.log.Error("override with no device", "request_id", RequestID(r.Context()), "path", s.loggedPath(r))
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return auth.Staff{}, false
	}
	by, err := s.staff.Authorise(r.Context(), dev, o.AuthorisedBy, o.PIN, p)
	var locked *auth.LockedError
	switch {
	case errors.As(err, &locked):
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(locked.RetryAfter)))
		writeError(w, s.log, http.StatusTooManyRequests, CodeLockedOut,
			"too many failed attempts; sign-in is locked for a few minutes")
		return auth.Staff{}, false
	case errors.Is(err, auth.ErrForbidden):
		writeFieldError(w, s.log, http.StatusForbidden, CodeForbidden,
			"that staff member cannot authorise this", "authorised_by")
		return auth.Staff{}, false
	case errors.Is(err, auth.ErrInvalidPIN):
		s.unauthenticated(w, CodeInvalidPIN, "that staff member or PIN is not right")
		return auth.Staff{}, false
	case err != nil:
		s.internalError(w, r, "authorising failed", err)
		return auth.Staff{}, false
	}
	return by, true
}

// clientIdempotencyKey refuses a request whose Idempotency-Key is not one a
// client may send: too long, not printable, or in the namespace the system
// appends its own under (a gateway's tender, an order to collect). It runs
// for every request so that no handler, present or future, can take one
// unchecked.
func (s *Server) clientIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := svcorder.CheckClientIdempotencyKey(r.Header.Get(HeaderIdempotencyKey)); err != nil {
			writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest,
				"Idempotency-Key must be up to 128 printable characters and not begin with a system prefix.", HeaderIdempotencyKey)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Meta is who a service command is from, taken from the request: the venue
// and device authentication established, the signed-in staff member, and the
// idempotency key the client sent.
//
// Handlers build it with this rather than reading the context themselves, so
// no handler can pass a venue or an actor the request did not prove.
func Meta(r *http.Request) svcorder.Meta {
	m := svcorder.Meta{IdempotencyKey: r.Header.Get(HeaderIdempotencyKey)}
	if dev, ok := DeviceFrom(r.Context()); ok {
		m.VenueID, m.DeviceID = dev.VenueID, dev.ID
	}
	if session, ok := SessionFrom(r.Context()); ok {
		m.ActorID = session.Staff.ID
	}
	return m
}
