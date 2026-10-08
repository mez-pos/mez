package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
)

// HeaderRequestID carries the correlation id, in and out.
const HeaderRequestID = "X-Request-Id"

// maxRequestIDLen bounds an id we accept from a client. Long enough for a UUID
// with dashes, short enough that nobody can pad a log file through it.
const maxRequestIDLen = 64

// middleware is the shape of every layer below.
type middleware func(http.Handler) http.Handler

// chain wraps h so that the first middleware listed is the outermost one, which
// is the order they are written in and therefore the order they read in.
func chain(h http.Handler, layers ...middleware) http.Handler {
	for i := len(layers) - 1; i >= 0; i-- {
		h = layers[i](h)
	}
	return h
}

type contextKey int

const (
	requestIDKey contextKey = iota
	deviceKey
	staffKey
	infoKey
)

// requestInfo is what the layers below the access log learn about a request
// and it does not: who was behind it. It is a pointer in the context because
// the log line is written by an outer layer after an inner one has filled it
// in. Only the request's own goroutine touches it.
type requestInfo struct {
	venueID  int64
	deviceID int64
	staffID  int64
	// via is how the request arrived when that was not the venue's own
	// network: "relay", or empty.
	via string
}

// infoFrom is the holder for this request, or nil outside one.
func infoFrom(ctx context.Context) *requestInfo {
	info, _ := ctx.Value(infoKey).(*requestInfo)
	return info
}

// RequestID returns the correlation id for a request, or "" outside one.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// requestID gives every request an id, puts it in the context and echoes it in
// the response.
//
// An inbound id is honoured, so a trace through the guest app and the relay
// stays one thread, but only after it is checked: it ends up in a log file that
// a human reads, and an unvalidated header is the cheapest way to write
// whatever you like into one.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !usableRequestID(id) {
			id = newRequestID()
		}

		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// usableRequestID accepts printable ASCII without spaces, which covers UUIDs,
// hex and the ids other systems tend to hand out, and excludes the newlines
// that would let a header forge a second log line.
func usableRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := range len(id) {
		if id[i] <= ' ' || id[i] > '~' {
			return false
		}
	}
	return true
}

func newRequestID() string {
	// crypto/rand.Read never returns an error (Go 1.24+): it stops the
	// program instead, which for a source of randomness is the right call.
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// accessLog records one line per request, after the response is written.
//
// It sits outside the panic recovery on purpose. Recovery turns a panic into a
// 500 first, so the line below reports what the client actually received. The
// other way round, the panic unwinds past this defer before anything is
// written, and the line claims 200 for a request that failed, which is worse
// than no line at all.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := s.now()
		rec := &recorder{ResponseWriter: w}
		info := &requestInfo{}
		r = r.WithContext(context.WithValue(r.Context(), infoKey, info))

		defer func() {
			fields := []any{
				"request_id", RequestID(r.Context()),
				"method", r.Method,
				"path", s.loggedPath(r),
				"status", rec.statusCode(),
				"bytes", rec.written,
				"duration_ms", s.now().Sub(started).Milliseconds(),
				"remote", remoteHost(r),
			}
			// Who it was, once authentication got far enough to know. An
			// unauthenticated request carries none of these rather than
			// three zeroes.
			for _, f := range []struct {
				key string
				val int64
			}{{"venue_id", info.venueID}, {"device_id", info.deviceID}, {"actor_id", info.staffID}} {
				if f.val != 0 {
					fields = append(fields, f.key, f.val)
				}
			}
			// Only when it did not come from the venue's own network, which
			// is nearly every line: a request from outside is the one to find.
			if info.via != "" {
				fields = append(fields, "via", info.via)
			}
			s.log.Info("request", fields...)
		}()

		next.ServeHTTP(rec, r)
	})
}

// remoteHost drops the ephemeral port, which is noise, and keeps the address,
// which tells you which till it was.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// recoverPanic turns a panic into a 500 and a logged stack.
//
// 03-conventions.md bans panics in request paths, which is exactly why this is
// here: the ban will be broken eventually, by a nil map or an index, and a
// register that dies mid-service because one handler was wrong is not
// acceptable. The client is told nothing but "internal error"; the stack goes
// to the log.
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Whether the handler got as far as a status line. After that a
		// 500 cannot be written: the client has its headers, and on a
		// hijacked connection (the stream) there is no response to write to.
		rec := &recorder{ResponseWriter: w}
		w = rec
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			// net/http uses this to abort a response deliberately. Passing it
			// through is the documented contract.
			if errors.Is(toError(recovered), http.ErrAbortHandler) {
				panic(recovered)
			}

			s.log.Error("handler panicked",
				"request_id", RequestID(r.Context()),
				"method", r.Method,
				"path", s.loggedPath(r),
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)
			if rec.status == 0 {
				writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func toError(recovered any) error {
	if err, ok := recovered.(error); ok {
		return err
	}
	return fmt.Errorf("%v", recovered)
}

// cors allows exactly one configured origin, and only when one is configured.
//
// A shipped binary has no dev_origin, so it sends no CORS headers at all and a
// page the venue did not open cannot read the till. See ADR-023.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := s.cfg.CORSEnabled() && origin == s.cfg.DevOrigin

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			// One origin is echoed back rather than reflected blindly, but a
			// cache in between still needs telling that the answer varies.
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if allowed {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, "+
					HeaderStaffToken+", "+HeaderGuestSession+", "+HeaderRequestID+", "+HeaderSetupToken)
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			// A preflight is answered here either way. Routing it would only
			// produce a 404 that the browser reads as a denial anyway.
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// recorder remembers what the handler did to the response so accessLog can
// report it.
type recorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK // what net/http implies on a first write
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// statusCode is the status the client saw. A handler that returned without
// writing anything still produced a 200.
func (r *recorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// Unwrap lets http.ResponseController reach the real writer. Phase 04 upgrades
// connections to WebSocket through this chain, and a wrapper that hides the
// underlying writer is how that silently stops working.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack: %T does not support it", r.ResponseWriter)
	}
	return h.Hijack()
}

// loggedPath is the request path as the log may carry it. A guest's path
// holds the table's code, and a pickup path the venue's, and a code is the
// credential (ADR-205): whoever reads the log must not come away holding
// one. The matched route pattern says which segment it is, and that
// segment is written as [code]. A path that matched no guest pattern, a
// mistyped one say, is read by its shape instead: whatever follows the
// guest API's t/ or c/ is masked, pattern or not, since a real code with a
// wrong path after it is a real code all the same.
func (s *Server) loggedPath(r *http.Request) string {
	pattern := r.Pattern
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:]
	}
	pathSegs := strings.Split(r.URL.Path, "/")
	masked := make([]string, len(pathSegs))
	copy(masked, pathSegs)
	changed := false
	if strings.Contains(pattern, "{code}") {
		for i, seg := range strings.Split(pattern, "/") {
			if seg == "{code}" && i < len(masked) {
				masked[i], changed = "[code]", true
			}
		}
	}
	if !changed && s.guestMount != "" && strings.HasPrefix(r.URL.Path, s.guestMount+"api/") {
		// The segment after t/ or c/ is the code, whatever follows it.
		for i := 0; i+1 < len(masked); i++ {
			if (masked[i] == "t" || masked[i] == "c") && masked[i+1] != "" {
				masked[i+1] = "[code]"
				break
			}
		}
	}
	return strings.Join(masked, "/")
}
