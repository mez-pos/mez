package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/byjoellobo/mez/internal/hub"
)

// servedStream records what the hub was handed.
type servedStream struct {
	mu      sync.Mutex
	clients []hub.Client
}

func (s *servedStream) Serve(w http.ResponseWriter, _ *http.Request, c hub.Client) {
	s.mu.Lock()
	s.clients = append(s.clients, c)
	s.mu.Unlock()
	// The real hub upgrades here. Answering plainly is enough to assert who
	// reached it; internal/hub tests the socket itself.
	w.WriteHeader(http.StatusOK)
}

func (s *servedStream) last() (hub.Client, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) == 0 {
		return hub.Client{}, 0
	}
	return s.clients[len(s.clients)-1], len(s.clients)
}

// TestTheStreamIsAuthenticatedLikeEverythingElse: a socket is a paired
// device, whether the token arrives in a header or, as a browser must send
// it, in the subprotocol.
func TestTheStreamIsAuthenticatedLikeEverythingElse(t *testing.T) {
	w := newTill(t)
	stream := &servedStream{}
	w.rebuild(t, WithStream(stream))

	t.Run("a native client with the header", func(t *testing.T) {
		rec := send(w.handler, http.MethodGet, "/api/v1/stream", "", w.asDevice())
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		client, n := stream.last()
		if n != 1 || client.VenueID != 1 || client.DeviceID != 1 || client.Kind != "terminal" {
			t.Errorf("the hub was handed %+v", client)
		}
	})

	t.Run("a browser with the subprotocol", func(t *testing.T) {
		rec := send(w.handler, http.MethodGet, "/api/v1/stream", "", map[string]string{
			"Sec-WebSocket-Protocol": hub.Subprotocol + ", token." + w.deviceToken,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		if _, n := stream.last(); n != 2 {
			t.Errorf("%d connections reached the hub, want 2", n)
		}
	})

	tests := []struct {
		name   string
		header map[string]string
		code   ErrorCode
	}{
		{"no token at all", nil, CodeUnauthenticated},
		{"only the subprotocol, no token", map[string]string{"Sec-WebSocket-Protocol": hub.Subprotocol}, CodeUnauthenticated},
		{"a token of nothing", map[string]string{"Sec-WebSocket-Protocol": "token.nfd_" + strings.Repeat("A", 43)}, CodeUnauthenticated},
		{"a header token of nothing", bearerOf("nfd_" + strings.Repeat("A", 43)), CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodGet, "/api/v1/stream", "", tt.header)
			if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != tt.code {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if _, n := stream.last(); n != 2 {
				t.Errorf("an unauthenticated request reached the hub")
			}
		})
	}

	t.Run("a revoked device", func(t *testing.T) {
		paired := pairOK(t, w.handler, newCode(t, w.devices))
		revoke(t, w.devices, paired)
		rec := send(w.handler, http.MethodGet, "/api/v1/stream", "", bearerOf(paired.Token))
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeDeviceRevoked {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("a method the endpoint does not take", func(t *testing.T) {
		rec := send(w.handler, http.MethodPost, "/api/v1/stream", "", w.asDevice())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
}

func TestTheStreamLogsWhoConnected(t *testing.T) {
	w := newTill(t)
	w.rebuild(t, WithStream(&servedStream{}))
	w.log.Reset()
	send(w.handler, http.MethodGet, "/api/v1/stream", "", w.asDevice())

	lines := requestLines(t, w.log)
	if len(lines) != 1 || lines[0]["venue_id"] != float64(1) || lines[0]["device_id"] != float64(1) {
		t.Errorf("logged as %v", lines)
	}
}

// TestTheStreamFailsClosed covers the two ways it can be misconfigured or
// broken rather than misused.
func TestTheStreamFailsClosed(t *testing.T) {
	t.Run("without a hub the route does not exist", func(t *testing.T) {
		w := newTill(t)
		rec := send(w.handler, http.MethodGet, "/api/v1/stream", "", w.asDevice())
		if rec.Code != http.StatusNotFound {
			t.Errorf("%d, want 404", rec.Code)
		}
	})

	t.Run("a device layer that cannot answer", func(t *testing.T) {
		srv := newTestServer(t, testConfig(), WithDevices(brokenDevices{}), WithStream(&servedStream{}))
		rec := send(srv.Handler(), http.MethodGet, "/api/v1/stream", "", bearerOf("nfd_anything"))
		if rec.Code != http.StatusInternalServerError || errorOf(t, rec).Code != CodeInternal {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
}

func TestProtocolToken(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"one header, two protocols", []string{hub.Subprotocol + ", token.abc"}, "abc"},
		{"two headers", []string{hub.Subprotocol, "token.abc"}, "abc"},
		{"spaces around it", []string{"  token.abc  "}, "abc"},
		{"no token", []string{hub.Subprotocol}, ""},
		{"an empty token", []string{"token."}, ""},
		{"nothing at all", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
			for _, v := range tt.values {
				r.Header.Add("Sec-WebSocket-Protocol", v)
			}
			got, ok := protocolToken(r)
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("protocolToken = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}

}
