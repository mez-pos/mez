package api

import (
	"net/http"
	"strings"

	"github.com/mez-pos/mez/internal/hub"
)

// tokenProtocolPrefix is how a browser sends its device token. The
// WebSocket API cannot set headers, so the token rides in the subprotocol
// list: ["mez.v1", "token.nfd_..."]. It is never a query parameter,
// which would put it in the access log and in any proxy's history.
const tokenProtocolPrefix = "token."

// Stream is what the API needs from the hub. *hub.Hub implements it.
type Stream interface {
	Serve(w http.ResponseWriter, r *http.Request, c hub.Client)
}

// WithStream mounts the WebSocket endpoint. It needs WithDevices: a socket
// is authenticated as a paired device, like every other route.
func WithStream(s Stream) Option {
	return func(srv *Server) { srv.stream = s }
}

func (s *Server) mountStream(mux *http.ServeMux) {
	if s.devices == nil || s.stream == nil {
		return
	}
	mux.HandleFunc("GET /api/v1/stream", s.serveStream)
	mux.HandleFunc("/api/v1/stream", s.methodNotAllowed)
}

// serveStream authenticates the device and hands the connection to the hub.
//
// It does its own authentication rather than sitting behind requireDevice,
// because a browser cannot send an Authorization header on a WebSocket and
// the token arrives in the subprotocol instead. A native client may still
// use the header, and both are checked the same way afterwards.
func (s *Server) serveStream(w http.ResponseWriter, r *http.Request) {
	token, ok := bearer(r)
	if !ok {
		token, ok = protocolToken(r)
	}
	if !ok {
		s.unauthenticated(w, CodeUnauthenticated, "a device token is needed")
		return
	}
	dev, ok := s.authenticateDevice(w, r, token)
	if !ok {
		return
	}

	s.stream.Serve(w, r, hub.Client{
		VenueID: dev.VenueID, DeviceID: dev.ID, Name: dev.Name, Kind: dev.Kind,
	})
}

// protocolToken reads the device token out of the requested subprotocols.
func protocolToken(r *http.Request) (string, bool) {
	for _, offered := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(offered, ",") {
			if token, found := strings.CutPrefix(strings.TrimSpace(p), tokenProtocolPrefix); found && token != "" {
				return token, true
			}
		}
	}
	return "", false
}
