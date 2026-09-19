package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes. These are contracts: a client switches on them, so they change
// only with the API version. The message is for a human and may be reworded at
// any time.
const (
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeInternal         = "internal"

	// CodeInvalidRequest is a body that is not the JSON the endpoint takes.
	CodeInvalidRequest = "invalid_request"

	// CodeUnauthenticated is a request with no device token, or one that is
	// not any device's.
	CodeUnauthenticated = "unauthenticated"

	// CodeDeviceRevoked is a token of a device the venue has unpaired. The
	// device should forget its token and show the pairing screen.
	CodeDeviceRevoked = "device_revoked"

	// CodeInvalidPairingCode is a pairing code that is unknown, expired or
	// used. Which of the three is deliberately not said.
	CodeInvalidPairingCode = "invalid_pairing_code"
)

// Error is one failure, in the shape 03-conventions.md fixes for every endpoint.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
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
func writeError(w http.ResponseWriter, log *slog.Logger, status int, code, message string) {
	writeJSON(w, log, status, errorBody{Error: Error{Code: code, Message: message}})
}

// writeFieldError sends the standard envelope naming the field at fault.
func writeFieldError(w http.ResponseWriter, log *slog.Logger, status int, code, message, field string) {
	writeJSON(w, log, status, errorBody{Error: Error{Code: code, Message: message, Field: field}})
}
