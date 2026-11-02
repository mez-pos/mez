package api

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"regexp"

	"github.com/mez-pos/mez/internal/auth"
)

// DeviceProver proves that this server holds a device's pairing (ADR-417):
// *auth.Devices is one. A Devices that is not one serves no proof, and a
// tablet then carries no pairing to this server's address.
type DeviceProver interface {
	Prove(ctx context.Context, publicID string, message []byte) ([]byte, error)
}

// proofMessage is what a proof is made over: the version, the device, the
// tablet's nonce and the address it asked. The app builds the same bytes
// (`proofMessage()` in the app's Proof.kt).
func proofMessage(device, nonce, at string) []byte {
	return []byte("mez-proof-v1\n" + device + "\n" + nonce + "\n" + at)
}

type proofResponse struct {
	// Proof is HMAC-SHA256 over proofMessage, in hex.
	Proof string `json:"proof"`
}

// nonceShape is a nonce as the app draws one: lowercase hex, 16 to 64 bytes.
var nonceShape = regexp.MustCompile(`^[0-9a-f]{32,128}$`)

// mountDeviceProof serves the proof where the devices can make one. It is
// not among the relay's routes (ADR-308): a tablet asks it on the venue's
// network.
func (s *Server) mountDeviceProof(mux *http.ServeMux) {
	prover, ok := s.devices.(DeviceProver)
	if !ok {
		if s.devices != nil {
			s.log.Info("these devices cannot prove a pairing, so a tablet carries none to this server (ADR-417)")
		}
		return
	}
	mux.HandleFunc("GET /api/v1/devices/{id}/proof", func(w http.ResponseWriter, r *http.Request) {
		s.deviceProof(w, r, prover)
	})
}

// deviceProof answers a tablet about to carry a device's pairing to this
// address (ADR-417), for the address it asked and no other.
func (s *Server) deviceProof(w http.ResponseWriter, r *http.Request, prover DeviceProver) {
	id, nonce, at := r.PathValue("id"), r.URL.Query().Get("nonce"), r.URL.Query().Get("at")
	if !nonceShape.MatchString(nonce) {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "the nonce must be 32 to 128 lowercase hex digits", "nonce")
		return
	}
	if !reachedAt(r, at) {
		writeFieldError(w, s.log, http.StatusMisdirectedRequest, CodeInvalidRequest, "this is not the address the request reached", "at")
		return
	}
	proof, err := prover.Prove(r.Context(), id, proofMessage(id, nonce, at))
	switch {
	case errors.Is(err, auth.ErrNoProof):
		writeError(w, s.log, http.StatusNotFound, CodeNotFound, "no such device is paired here")
		return
	case err != nil:
		s.internalError(w, r, "proving a pairing", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, proofResponse{Proof: hex.EncodeToString(proof)})
}

// reachedAt reports whether at, an address and port as the tablet wrote it,
// is this server's own end of the request's connection. A device that
// passes the tablet's question on reaches the server at the server's
// address, not the one the tablet asked. A name is not an address here.
func reachedAt(r *http.Request, at string) bool {
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return false
	}
	want, err := netip.ParseAddrPort(at)
	if err != nil {
		return false
	}
	got, err := netip.ParseAddrPort(local.String())
	if err != nil {
		return false
	}
	return want.Addr().Unmap() == got.Addr().Unmap() && want.Port() == got.Port()
}
