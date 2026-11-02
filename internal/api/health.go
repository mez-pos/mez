package api

import (
	"net/http"
	"time"
)

// WithVenue names the venue this server serves, by its public id, for health
// to tell the venue's network (ADR-416). A tablet that finds a mez at a new
// address carries its pairing there only if that mez names the same venue.
// Empty before a venue is made; first run over HTTP names it as it makes it.
func WithVenue(publicID string) Option {
	return func(s *Server) { s.nameVenue(publicID) }
}

// nameVenue keeps the venue health names; an empty id names none.
func (s *Server) nameVenue(publicID string) {
	if publicID != "" {
		s.venue.Store(&publicID)
	}
}

// venueID is the venue health names, or "" before there is one.
func (s *Server) venueID() string {
	if v := s.venue.Load(); v != nil {
		return *v
	}
	return ""
}

// Check results. Every check is unknown until the subsystem that answers it
// exists: Phase 02 wires db and disk, Phase 07 the printer, Phase 11 the tunnel.
// The tunnel stays unknown on a venue with no relay: there is nothing to fail.
const (
	CheckOK      = "ok"
	CheckFailing = "failing"
	CheckUnknown = "unknown"
)

// BuildInfo is what the binary knows about itself. Phase 01 Task 5 stamps
// Commit and BuildDate through ldflags; until then they are empty and omitted.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// healthChecks is a struct rather than a map so the shape is fixed: Phase 04
// generates TypeScript from these types, and a map generates Record<string,
// string>, which tells a caller nothing about what it can expect.
type healthChecks struct {
	DB      string `json:"db"`
	Disk    string `json:"disk"`
	Printer string `json:"printer"`
	Tunnel  string `json:"tunnel"`
}

type healthResponse struct {
	// Status answers "is this process serving", which is the question a
	// watchdog asks. It is not a claim about the subsystems: those are in
	// checks, and an unknown check does not make the server unhealthy.
	Status string `json:"status"`

	// Venue is the venue's public id, for a tablet to know it has found its
	// own PC (ADR-416). Not told through the relay: nobody away needs it.
	Venue string `json:"venue,omitempty"`

	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`

	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`

	Checks healthChecks `json:"checks"`
}

// health reports liveness, identity and the state of each subsystem.
//
// It must not touch the database or the printer. A health endpoint that does
// real work is one more thing that can hang, and the thing asking is usually
// asking because something already has.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	venue := s.venueID()
	if relayed(r) {
		venue = ""
	}
	writeJSON(w, s.log, http.StatusOK, healthResponse{
		Status:    "ok",
		Venue:     venue,
		Version:   s.build.Version,
		Commit:    s.build.Commit,
		BuildDate: s.build.BuildDate,

		StartedAt: s.started.UTC(),
		// Seconds, not a duration string: a dashboard graphs a number.
		UptimeSeconds: int64(s.now().Sub(s.started).Seconds()),

		Checks: healthChecks{
			DB:      check(s.dbCheck),
			Disk:    CheckUnknown,
			Printer: CheckUnknown,
			Tunnel:  s.tunnelCheck(),
		},
	})
}

// check reads one subsystem's status, treating an absent check and an answer
// outside the vocabulary the same way: unknown.
//
// The three Check constants are the whole published vocabulary, and a
// dashboard switches on them. Letting an unexpected word through would be a
// silent change to the API contract, so it is clamped here rather than trusted
// from whatever wired the check up.
func check(status func() string) string {
	if status == nil {
		return CheckUnknown
	}

	switch got := status(); got {
	case CheckOK, CheckFailing, CheckUnknown:
		return got
	default:
		return CheckUnknown
	}
}
