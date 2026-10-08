// Package relay is the optional cloud relay: the public address a guest's
// phone can reach when the venue's own machine has none.
//
// A venue's tunnel client dials GET /agent with its venue key and holds the
// WebSocket open. A guest request is framed, sent down that socket and
// answered from it. The venue is named by the Host header, <venue>.<domain>,
// when RELAY_VENUE_DOMAIN is set (ADR-203), which is how a shared relay runs:
// each venue is its own browser origin. Without a domain the venue is the
// first path segment, /v/<venue>/..., and every venue shares one origin,
// which is for a relay serving one venue or for development. The relay
// keeps a map of venue id to socket
// in memory and nothing else: no database, no disk, no table of venues. Kill
// it and start it again and every venue is back within its client's backoff.
// See ADR-008 and ADR-201.
//
// Imports: internal/relay/wire. Never internal/db or internal/api, so the
// binary stays small and has no SQLite in it.
package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

// defaultHeadTimeout is how long a guest waits for the venue to start
// answering before being told it did not.
const defaultHeadTimeout = 30 * time.Second

// Options configures a Server.
type Options struct {
	// Keys checks venue keys. Required.
	Keys *Keys

	// Limit is each venue's request allowance. Zero means DefaultLimit.
	Limit Limit

	// IPLimit is each client address's request allowance, across every
	// venue, so one client cannot spend a venue's Limit for everyone.
	// Zero means DefaultIPLimit.
	IPLimit Limit

	// TrustProxy says a proxy this operator runs sits in front and sets
	// X-Forwarded-For and X-Forwarded-Proto. Without it those headers are a
	// guest's own claim about itself and are ignored.
	TrustProxy bool

	// VenueDomain, when set, gives each venue a hostname: <venue>.<domain>.
	// Venues are then reached only that way, and /v/<venue>/ is not served,
	// since a path every venue shares is the thing a hostname each replaces.
	// Empty means path routing, which is right for one venue or venues that
	// trust each other.
	VenueDomain string

	// Version is reported by the health endpoint.
	Version string

	// Log receives one line per request and per venue connection. Required.
	Log *slog.Logger

	// Now is the clock. Nil means time.Now.
	Now func() time.Time

	// HeadTimeout overrides defaultHeadTimeout. Tests shorten it.
	HeadTimeout time.Duration
}

// Server is the relay. It owns every venue socket and closes them in Close.
type Server struct {
	keys        *Keys
	limit       Limit
	ips         *ipTable
	trustProxy  bool
	venueDomain string
	version     string
	log         *slog.Logger
	now         func() time.Time
	headTimeout time.Duration

	mu     sync.Mutex
	agents map[string]*agent
	closed bool

	// wg counts the goroutines serving venue sockets, and the ones closing
	// a socket that was replaced.
	wg sync.WaitGroup
}

// New builds a relay.
func New(opts Options) (*Server, error) {
	if opts.Keys == nil {
		return nil, errors.New("relay: Options.Keys is required")
	}
	if opts.Log == nil {
		return nil, errors.New("relay: Options.Log is required")
	}
	if opts.Limit == (Limit{}) {
		opts.Limit = DefaultLimit
	}
	if opts.Limit.PerMinute < 1 || opts.Limit.Burst < 1 {
		return nil, errors.New("relay: Options.Limit needs a positive rate and burst")
	}
	if opts.IPLimit == (Limit{}) {
		opts.IPLimit = DefaultIPLimit
	}
	if opts.IPLimit.PerMinute < 1 || opts.IPLimit.Burst < 1 {
		return nil, errors.New("relay: Options.IPLimit needs a positive rate and burst")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.HeadTimeout <= 0 {
		opts.HeadTimeout = defaultHeadTimeout
	}
	domain := strings.Trim(strings.ToLower(strings.TrimSpace(opts.VenueDomain)), ".")
	if domain != "" && !validDomain(domain) {
		return nil, fmt.Errorf("relay: venue domain %q is not a domain name", opts.VenueDomain)
	}
	return &Server{
		venueDomain: domain,
		keys:        opts.Keys,
		limit:       opts.Limit,
		ips:         newIPTable(opts.IPLimit, opts.Now()),
		trustProxy:  opts.TrustProxy,
		version:     opts.Version,
		log:         opts.Log,
		now:         opts.Now,
		headTimeout: opts.HeadTimeout,
		agents:      map[string]*agent{},
	}, nil
}

// Handler is the relay's whole HTTP surface.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /agent", s.handleAgent)
	if s.venueDomain == "" {
		mux.HandleFunc(guestPrefix+"{venue}", s.handleBareVenue)
		mux.HandleFunc(guestPrefix+"{venue}/{rest...}", s.handleGuest)
	}
	return s.requestID(s.accessLog(s.byHost(mux)))
}

// validDomain accepts a plain dotted name: labels of letters, digits and
// hyphens. It is compared with Host headers, so it is held to what one of
// those can be.
func validDomain(domain string) bool {
	if len(domain) > 253 || !strings.Contains(domain, ".") {
		return false
	}
	for label := range strings.SplitSeq(domain, ".") {
		if !ValidVenueID(label) || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

// Close disconnects every venue and waits for their goroutines. Guest
// requests in flight end with it; http.Server.Shutdown is what waits for
// those.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	agents := make([]*agent, 0, len(s.agents))
	for _, a := range s.agents {
		agents = append(agents, a)
	}
	s.mu.Unlock()

	// Together, not in turn: a close waits for the venue to answer it, and
	// one venue with a dead line must not cost every other its goodbye.
	var closing sync.WaitGroup
	for _, a := range agents {
		closing.Add(1)
		go func() {
			defer closing.Done()
			a.close(websocket.StatusGoingAway, "relay shutting down")
		}()
	}
	closing.Wait()
	s.wg.Wait()
}

// agentFor is the venue's socket, or nil when the venue is not connected.
func (s *Server) agentFor(venueID string) *agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agents[venueID]
}

// healthResponse is what GET /healthz answers. It says the process is up and
// what it is, and nothing about who is connected: the endpoint is public.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.version})
}

// handleAgent is a venue connecting. It returns when the socket ends.
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		s.writeError(w, http.StatusUnauthorized, codeUnauthorized, "send the venue key as a bearer token")
		return
	}
	claims, err := s.keys.Check(strings.TrimSpace(key))
	if err != nil {
		// One answer for every failure. Which it was is for the operator.
		s.log.Warn("relay: venue key refused", "reason", err.Error(), "remote", s.clientIP(r))
		s.writeError(w, http.StatusUnauthorized, codeUnauthorized, "venue key refused")
		return
	}
	venueID := claims.VenueID
	noteVenue(r, venueID)

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		// Accept has already answered the client.
		s.log.Warn("relay: agent upgrade failed", "venue_id", venueID, "error", err.Error())
		return
	}
	ws.SetReadLimit(wire.MaxFrame)

	a := newAgent(claims, ws, s.limit, s.now(), s.log)
	if !s.register(a) {
		a.close(websocket.StatusGoingAway, "relay shutting down")
		return
	}
	defer s.wg.Done()

	a.log.Info("relay: venue connected", "remote", s.clientIP(r), "back_office", claims.Remote)
	connected := s.now()

	// The prober is this handler's own goroutine to start and to wait for.
	probing := make(chan struct{})
	go func() {
		defer close(probing)
		a.ping()
	}()

	readErr := a.read()
	a.close(websocket.StatusNormalClosure, "")
	<-probing
	s.unregister(a)

	a.log.Info("relay: venue disconnected",
		"reason", readErr.Error(),
		"connected_s", int64(s.now().Sub(connected).Seconds()),
		"in_flight", a.inFlight())
}

// register makes a the venue's socket, displacing an older one. It reports
// false when the relay is closing.
func (s *Server) register(a *agent) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	old := s.agents[a.venueID]
	s.agents[a.venueID] = a
	s.wg.Add(1)
	if old != nil {
		// The newest connection wins. A venue reconnecting after its line
		// dropped is the common case, and the old socket is the dead one,
		// which is why its close is not waited for here: a dead socket takes
		// seconds to give up, and the new one has guests to serve now.
		old.log.Info("relay: venue connection replaced")
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			old.close(wire.CloseReplaced, "replaced by a newer connection")
		}()
	}
	s.mu.Unlock()
	return true
}

// unregister forgets a, unless a newer socket has already taken its place.
func (s *Server) unregister(a *agent) {
	s.mu.Lock()
	if s.agents[a.venueID] == a {
		delete(s.agents, a.venueID)
	}
	s.mu.Unlock()
}

// The error codes the relay answers with. Stable, machine-readable, and
// distinct from the venue's own so a client can tell who refused it.
const (
	codeUnauthorized  = "relay_unauthorized"
	codeNotFound      = "relay_not_found"
	codeVenueOffline  = "venue_offline"
	codeVenueBusy     = "venue_busy"
	codeRateLimited   = "relay_rate_limited"
	codeBadGateway    = "venue_bad_response"
	codeTimeout       = "venue_timeout"
	codeTooLarge      = "request_too_large"
	codeHeaderTooLong = "request_header_too_large"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Debug("relay: writing a response", "error", err.Error())
	}
}
