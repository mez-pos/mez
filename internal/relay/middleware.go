package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// headerRequestID is the correlation id, the same header the venue server
// reads, so one guest request is one thread through both logs.
const headerRequestID = "X-Request-Id"

const maxRequestIDLen = 64

type ctxKey int

const (
	requestIDKey ctxKey = iota
	infoKey
)

// requestInfo is what a handler learns about a request that the access log,
// one layer out, writes down afterwards. Only the request's own goroutine
// touches it.
type requestInfo struct {
	venueID string
	// cut says why a response that had already started was ended early.
	cut string
}

// noteVenue records which venue a request was for.
func noteVenue(r *http.Request, venueID string) {
	if info, _ := r.Context().Value(infoKey).(*requestInfo); info != nil {
		info.venueID = venueID
	}
}

// noteCut records that a response was cut short, and why.
func noteCut(r *http.Request, why string) {
	if info, _ := r.Context().Value(infoKey).(*requestInfo); info != nil {
		info.cut = why
	}
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// requestID gives every request an id and echoes it. An inbound one is kept
// only if it is plain printable ASCII: it ends up in a log a person reads.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if !usableRequestID(id) {
			id = s.newRequestID()
		}
		w.Header().Set(headerRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

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

func (s *Server) newRequestID() string {
	// crypto/rand.Read never returns an error (Go 1.24+): it stops the
	// program instead, which for a source of randomness is the right call.
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// accessLog writes one structured line per request, after it is over. For a
// venue's own connection that is when its socket closes.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := s.now()
		rec := &recorder{ResponseWriter: w}
		info := &requestInfo{}
		r = r.WithContext(context.WithValue(r.Context(), infoKey, info))

		// Deferred, so a response cut with http.ErrAbortHandler is still a line.
		defer func() {
			fields := []any{
				"request_id", requestIDFrom(r.Context()),
				"method", r.Method,
				// The path and never the query: a query is where a guest's
				// details would be if anything put them in a URL. And the
				// path with its code masked: a table code is the guest's
				// credential, and the venue's own log hides it (ADR-205),
				// so the relay's must too.
				"path", wire.MaskCodes(r.URL.Path),
				"status", rec.statusCode(),
				"bytes", rec.written,
				"duration_ms", s.now().Sub(started).Milliseconds(),
				"remote", s.clientIP(r),
			}
			if info.venueID != "" {
				fields = append(fields, "venue_id", info.venueID)
			}
			if info.cut != "" {
				fields = append(fields, "cut", info.cut)
			}
			level := slog.LevelInfo
			if r.URL.Path == "/healthz" {
				// A probe every few seconds is not news.
				level = slog.LevelDebug
			}
			s.log.Log(r.Context(), level, "relay: request", fields...)
		}()

		next.ServeHTTP(rec, r)
	})
}

// recorder notes the status and size of a response as it passes.
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

func (r *recorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.written += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController and the WebSocket upgrade reach the
// real writer for flushing, deadlines and hijacking.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// statusCode is what the client received. A handler that wrote nothing sent
// a 200.
func (r *recorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// clientIP is the address a request came from.
//
// Behind the operator's own proxy that is the last entry of X-Forwarded-For,
// the one that proxy appended; anything before it is whatever the client
// chose to send. With no proxy the header is ignored altogether.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if fwd := r.Header.Values("X-Forwarded-For"); len(fwd) > 0 {
			parts := strings.Split(fwd[len(fwd)-1], ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// scheme is how the guest reached the relay, as far as the relay can know.
func (s *Server) scheme(r *http.Request) string {
	if s.trustProxy {
		if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
			return p
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
