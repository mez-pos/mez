package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// A screen that failed to draw is caught where it failed, by the error
// boundary around that part of the screen, and told to the server here, so
// that it reaches the log the venue's installer reads instead of a browser
// console nobody at the venue opens (TECH-8, ADR-402).
//
// It is a log line and nothing else: no table, no event, nothing a report
// counts. A device sends it with its token and no signed-in person, since a
// lock screen can fail too.

// clientErrorRequest is one screen that failed to draw, as the app that
// drew it saw it.
type clientErrorRequest struct {
	// App is the bundle: terminal, kitchen or backoffice.
	App string `json:"app"`
	// Where names the boundary that caught it: "route", "rail", "pay",
	// "modal", "root".
	Where string `json:"where"`
	// Path is the screen's address, without its query or fragment.
	Path string `json:"path"`
	// Message and Stack are the error's own; ComponentStack is React's,
	// the components it was drawing.
	Message        string `json:"message"`
	Stack          string `json:"stack"`
	ComponentStack string `json:"component_stack"`
}

// clientErrorLimit is what one device may report: a screen that fails
// again on every Try again is a handful of lines a minute, not a flood. It
// is counted per device, after its token is judged, not per address: every
// request through the relay comes from the tunnel's one address, and a
// stranger's refused requests must not spend a venue's till's reports.
var clientErrorLimit = limit{requests: 30, per: time.Minute}

// The longest each field is kept. A stack is long and is cut, not refused:
// the top of it is what says where.
const (
	maxClientWhere = 40
	maxClientPath  = 200
	maxClientText  = 500
	maxClientStack = 4000
)

// clientApps are the bundles that report. The guest's app has no device
// token, and is not one of them.
var clientApps = map[string]struct{}{"terminal": {}, "kitchen": {}, "backoffice": {}}

func (s *Server) mountClientErrors(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/client-errors", s.requireDevice(http.HandlerFunc(s.clientError)))
}

func (s *Server) clientError(w http.ResponseWriter, r *http.Request) {
	meta := Meta(r)
	if ok, retryAfter := s.limiter.allow(clientErrorLimit.key("device:"+strconv.FormatInt(meta.DeviceID, 10)), clientErrorLimit); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
		writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, "too many requests; wait and try again")
		return
	}
	var req clientErrorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	if _, ok := clientApps[req.App]; !ok {
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidRequest, "app is terminal, kitchen or backoffice", "app")
		return
	}
	attrs := []any{
		"request_id", RequestID(r.Context()), "venue_id", meta.VenueID, "device_id", meta.DeviceID,
		"app", req.App, "where", clip(req.Where, maxClientWhere), "path", clip(req.Path, maxClientPath),
		"message", clip(req.Message, maxClientText),
		"stack", clip(req.Stack, maxClientStack), "component_stack", clip(req.ComponentStack, maxClientStack),
	}
	if dev, ok := DeviceFrom(r.Context()); ok {
		attrs = append(attrs, "device_kind", string(dev.Kind))
	}
	s.log.Error("a screen failed to draw", attrs...)
	w.WriteHeader(http.StatusNoContent)
}

// clip is s, trimmed, cut to at most n characters on a character's edge,
// with its control characters other than line breaks and tabs taken out: a
// stack keeps its lines, and nothing in it can move a terminal's cursor.
func clip(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}
