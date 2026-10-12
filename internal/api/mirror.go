package api

import (
	"net/http"
	"strings"
	"time"
)

// A mirror is the back office over a copy of a venue's backup (ADR-312,
// ADR-326): what the owner reads when the venue's computer is off. It is
// this server with one option. Everything asked of it is taken as asked
// from away, so what can be reached is what remote.go lists, and of that
// nothing is changed: a request that is not a read is refused before any
// handler sees it.
const (
	// CodeMirrorReadOnly is a change asked of a mirror. Nothing is wrong
	// with the request: there is no venue behind this server to change.
	CodeMirrorReadOnly ErrorCode = "mirror_read_only"

	// HeaderMirror is on every answer a mirror gives: the moment of the
	// last write its copy holds, RFC 3339 to the nanosecond, UTC, so that a
	// newer copy never carries the time of the one before. A screen reads
	// it off whatever it last asked, so the time it shows is never older
	// than what it is showing.
	HeaderMirror = "X-Mez-Mirror"
)

// MirrorInfo is what a mirror knows of its copy.
type MirrorInfo struct {
	// AsOf is when the venue last wrote something this copy holds.
	AsOf time.Time
}

// WithMirror makes the server a mirror. info is asked on every request: a
// mirror that follows its backup grows newer as it runs.
func WithMirror(info func() MirrorInfo) Option {
	return func(s *Server) { s.mirror = info }
}

// mirrorWrites are the requests a mirror takes that are not reads, exactly
// as they are registered. A mirror has to let a browser in, and that is all
// it lets happen; a route added later is refused until it is put here.
var mirrorWrites = map[string]struct{}{
	// Letting someone in, and out.
	"POST /api/v1/devices/pair":       {},
	"POST /api/v1/sessions":           {},
	"POST /api/v1/sessions/cloud":     {},
	"DELETE /api/v1/sessions/current": {},

	// Reads sent as POST because they carry a draft: nothing is kept.
	"POST /api/v1/settings/receipt/preview":   {},
	"POST /api/v1/settings/numbering/preview": {},
}

// mirrorGate stands outside the relay's gate on a mirror. It marks every
// request as one from away, so that gate judges it; refuses the guest app,
// which that gate would pass; and refuses every write but the few named.
//
// Like the relay's gate it asks the mux which route the request would take
// and judges that, so what is judged is what would run.
func (s *Server) mirrorGate(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if at := s.mirror().AsOf; !at.IsZero() {
			w.Header().Set(HeaderMirror, at.UTC().Format(time.RFC3339Nano))
		} else {
			// A mirror all the same, of a copy that cannot say how old it is.
			w.Header().Set(HeaderMirror, "")
		}
		// Whoever is asking is not at the venue: there is no venue here.
		r.Header.Set(HeaderVia, ViaRelay)

		// Nobody orders from a copy.
		path := r.URL.Path
		if strings.HasPrefix(path, remoteGuest) || (s.guestMount != "" && strings.HasPrefix(path, s.guestMount)) {
			s.notFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if _, pattern := mux.Handler(r); !mirrorAllows(pattern) {
				w.Header().Set("Cache-Control", "no-store")
				writeError(w, s.log, http.StatusConflict, CodeMirrorReadOnly,
					"This is a copy of the venue from its backup, to read. Changes are made at the venue, or from away while its computer is on.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// mirrorAllows reports whether a mirror takes the write registered as
// pattern, "METHOD /path".
func mirrorAllows(pattern string) bool {
	_, ok := mirrorWrites[pattern]
	return ok
}
