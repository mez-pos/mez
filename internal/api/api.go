package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/config"
)

// Server holds what the handlers need and builds the routing tree once.
type Server struct {
	cfg      config.Config
	log      *slog.Logger
	build    BuildInfo
	now      func() time.Time
	started  time.Time
	bundles  map[string]fs.FS
	dbCheck  func() string
	devices  Devices
	staff    Staff
	orders   Orders
	menus    Menus
	stream   Stream
	prints   Prints
	printers Printers
	floor    Floor
	shifts   Shifts
	zreports ZReports
	tips     Tips
	limiter  *limiter
	handler  http.Handler
}

// Option adjusts a Server at construction.
type Option func(*Server)

// WithClock replaces the clock. Uptime is the only thing here that moves on its
// own, and a test that cannot control it can only assert that it is a number.
func WithClock(now func() time.Time) Option {
	return func(s *Server) { s.now = now }
}

// WithDBCheck supplies the database's status for the health endpoint.
//
// It is a function returning one of the Check constants rather than a database
// handle, so this package still knows nothing about storage, and so that what
// is behind it is obliged to be a cached read. The handler must be able to
// answer while the database is hung.
func WithDBCheck(check func() string) Option {
	return func(s *Server) { s.dbCheck = check }
}

// WithStatic mounts built SPA bundles, keyed by the path each is served at.
// The server is usable without them: an API-only process answers the documented
// 404 everywhere else, which is what the tests in this package run against.
func WithStatic(bundles map[string]fs.FS) Option {
	return func(s *Server) { s.bundles = bundles }
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
	s.limiter = newLimiter(s.now)
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

	s.mountDevices(mux)
	s.mountStaff(mux)
	s.mountOrders(mux)
	s.mountMenu(mux)
	s.mountStream(mux)
	s.mountPrints(mux)
	s.mountPrinters(mux)
	s.mountFloor(mux)
	s.mountKitchen(mux)
	s.mountShifts(mux)
	s.mountZReports(mux)
	s.mountTips(mux)

	// Anything else under /api keeps the JSON envelope. Without this, an
	// unknown endpoint would fall through to the terminal bundle mounted at /
	// and answer a client expecting JSON with a page of HTML and a 200.
	mux.HandleFunc("/api/", s.notFound)

	s.mountStatic(mux)

	// With no bundle at the root, an unknown path still answers in the
	// documented envelope rather than net/http's plain text.
	if _, mounted := s.bundles["/"]; !mounted {
		mux.HandleFunc("/", s.notFound)
	}

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
