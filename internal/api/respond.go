package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorCode names a failure. A client switches on it, so the set is a type
// rather than a loose string, and it generates into a TypeScript union.
type ErrorCode string

// Error codes. These are contracts: a client switches on them, so they change
// only with the API version. The message is for a human and may be reworded at
// any time.
const (
	CodeNotFound         ErrorCode = "not_found"
	CodeMethodNotAllowed ErrorCode = "method_not_allowed"
	CodeInternal         ErrorCode = "internal"

	// CodeInvalidRequest is a body that is not the JSON the endpoint takes.
	CodeInvalidRequest ErrorCode = "invalid_request"

	// CodeUnauthenticated is a request with no device token, or one that is
	// not any device's.
	CodeUnauthenticated ErrorCode = "unauthenticated"

	// CodeNoVenue is a device token shown to a computer that has no venue
	// yet (ADR-339): a new install at the address a venue's computer had.
	// It says nothing about the token. The device keeps what it holds and
	// asks again; the venue may be on its way back.
	CodeNoVenue ErrorCode = "no_venue"

	// CodeDeviceRevoked is a token of a device the venue has unpaired. The
	// device should forget its token and show the pairing screen.
	CodeDeviceRevoked ErrorCode = "device_revoked"

	// CodeInvalidPairingCode is a pairing code that is unknown, expired or
	// used. Which of the three is deliberately not said.
	CodeInvalidPairingCode ErrorCode = "invalid_pairing_code"

	// CodeInvalidPIN is a staff member and PIN that did not sign in. An
	// unknown or deactivated staff member gets this too.
	CodeInvalidPIN ErrorCode = "invalid_pin"

	// CodeLockedOut is sign-in locked after repeated failures. The response
	// carries Retry-After.
	CodeLockedOut ErrorCode = "locked_out"

	// CodeSignInRequired is a request needing a staff member behind it,
	// with no session or a session that has ended. The terminal shows the
	// PIN pad.
	CodeSignInRequired ErrorCode = "sign_in_required"

	// CodeRateLimited is too many requests from one address. The response
	// carries Retry-After.
	CodeRateLimited ErrorCode = "rate_limited"

	// CodeForbidden is a staff member without the permission the endpoint
	// needs, or an authoriser without it. Field names the permission, or
	// the field carrying the authoriser.
	CodeForbidden ErrorCode = "forbidden"
)

// Error is one failure, in the shape 03-conventions.md fixes for every endpoint.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

// errorBody wraps Error so that the payload is {"error": {...}} rather than a
// bare object. A response is then self-describing: a client can tell success
// from failure without consulting the status code twice.
type errorBody struct {
	Error Error `json:"error"`
}

// writeJSON encodes body and sends it with status.
//
// The encode happens into a buffer first. Encoding straight into the
// ResponseWriter commits the status code on the first byte, so a value that
// fails halfway through leaves a 200 with truncated JSON on the wire, which is
// far worse to debug than a clean 500.
func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, body any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		log.Error("encoding response failed", "error", err, "status", status)
		http.Error(w, `{"error":{"code":"internal","message":"internal error"}}`,
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// The register runs on a LAN with printers and tablets on it. Nothing here
	// should ever be sniffed into being executable.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	if _, err := w.Write(buf.Bytes()); err != nil {
		// The client hung up mid-response. Worth a line, not worth a panic.
		log.Debug("writing response failed", "error", err, "status", status)
	}
}

// writeError sends the standard envelope.
func writeError(w http.ResponseWriter, log *slog.Logger, status int, code ErrorCode, message string) {
	writeJSON(w, log, status, errorBody{Error: Error{Code: code, Message: message}})
}

// writeFieldError sends the standard envelope naming the field at fault.
func writeFieldError(w http.ResponseWriter, log *slog.Logger, status int, code ErrorCode, message, field string) {
	writeJSON(w, log, status, errorBody{Error: Error{Code: code, Message: message, Field: field}})
}

// internalError answers a failure that is the server's own: an ERROR line
// with the request's thread through it, and a 500 that says no more than
// "internal error". A client that went away before the answer (a till that
// moved to another screen, a tab that closed) is not a failure of ours: its
// cancelled context is noted at Debug and nothing is written, since nobody
// is reading. Without this, every screen change mid-request was a 500 in
// the log.
func (s *Server) internalError(w http.ResponseWriter, r *http.Request, what string, err error, fields ...any) {
	base := []any{"request_id", RequestID(r.Context()), "method", r.Method, "path", s.loggedPath(r)}
	base = append(base, fields...)
	if r.Context().Err() != nil {
		s.log.Debug(what+": the client went away first", append(base, "error", err)...)
		return
	}
	s.log.Error(what, append(base, "error", err)...)
	writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
}
