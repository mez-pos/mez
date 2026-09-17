package api

import (
	"net/http"
	"time"
)

// Check results. Every check is unknown until the subsystem that answers it
// exists: Phase 02 wires db and disk, Phase 07 the printer, Phase 11 the tunnel.
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
	writeJSON(w, s.log, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   s.build.Version,
		Commit:    s.build.Commit,
		BuildDate: s.build.BuildDate,

		StartedAt: s.started.UTC(),
		// Seconds, not a duration string: a dashboard graphs a number.
		UptimeSeconds: int64(s.now().Sub(s.started).Seconds()),

		Checks: healthChecks{
			DB:      CheckUnknown,
			Disk:    CheckUnknown,
			Printer: CheckUnknown,
			Tunnel:  CheckUnknown,
		},
	})
}
