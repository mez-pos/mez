package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/config"
)

// Server holds what the handlers need and builds the routing tree once.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	build   BuildInfo
	now     func() time.Time
	started time.Time
	handler http.Handler
}

// Option adjusts a Server at construction.
type Option func(*Server)

// WithClock replaces the clock. Uptime is the only thing here that moves on its
// own, and a test that cannot control it can only assert that it is a number.
func WithClock(now func() time.Time) Option {
	return func(s *Server) { s.now = now }
}

// New builds the server. It does no IO: the caller owns the listener, so the
// lifecycle stays in cmd/nofiltr where it can be shut down in order.
func New(cfg config.Config, log *slog.Logger, build BuildInfo, opts ...Option) *Server {
	s := &Server{
		cfg:   cfg,
		log:   log,
		build: build,
		now:   time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.started = s.now()
	s.handler = s.routes()
	return s
}

// Handler is the whole API, middleware included.
func (s *Server) Handler() http.Handler { return s.handler }

// routes builds the tree and wraps it.
//
// The order of the chain is the point, outermost first:
//
//   - requestID first, so every line logged below it and every panic reported
//     below it carries the same id.
//   - accessLog next, so it observes the status recoverPanic produced rather
//     than the nothing a panicking handler wrote.
//   - recoverPanic inside those two, so a broken handler is a 500 in the log
//     with a stack, not a dead process mid-service.
//   - cors innermost, because a preflight needs an id and a log line like any
//     other request, but must be answered before routing.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Go 1.22 method patterns. The specific pattern wins, so GET reaches the
	// handler and every other method falls to the second registration, which
	// is how a 405 keeps the JSON error shape instead of net/http's plain text.
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("/api/v1/health", s.methodNotAllowed)

	// The catch-all exists for the same reason: an unknown path should answer
	// in the documented envelope, not "404 page not found".
	mux.HandleFunc("/", s.notFound)

	return chain(mux, s.middleware()...)
}

// middleware is the chain, outermost first. It is a method rather than a
// literal inside routes so that a test can wrap a deliberately broken handler
// in the same order production uses, instead of restating the order and proving
// only that the test agrees with itself.
func (s *Server) middleware() []middleware {
	return []middleware{
		s.requestID,
		s.accessLog,
		s.recoverPanic,
		s.cors,
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, s.log, http.StatusNotFound, CodeNotFound, "no such endpoint")
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, s.log, http.StatusMethodNotAllowed, CodeMethodNotAllowed,
		r.Method+" is not allowed on this endpoint")
}
