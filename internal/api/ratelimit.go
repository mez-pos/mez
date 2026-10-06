package api

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// pairLimit is what one address may spend on pairing: enough for a person
// mistyping a code, far too little to work through forty bits of them.
var pairLimit = limit{requests: 10, per: time.Minute}

// limit is a fixed window: at most requests in every per.
type limit struct {
	requests int
	per      time.Duration
}

// key is the limiter key for one address under this limit. The limit is
// named by its shape, which is enough to keep two limits apart.
func (lim limit) key(addr string) string {
	return "rl:" + strconv.Itoa(lim.requests) + "/" + lim.per.String() + ":" + addr
}

// maxLimiterKeys bounds the memory one limiter can hold. A venue has a
// handful of addresses; anything near this is someone cycling source
// addresses, and the windows that have turned over are dropped.
const maxLimiterKeys = 4096

// limiter counts requests per key in fixed windows. It is in memory on
// purpose: it protects this process, it resets when the process does, and a
// till must not wait on the database to find out whether it may proceed.
type limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	now     func() time.Time
}

// window is one key's fixed window. It remembers how long it is, so that
// when the map is full a prune can tell a window that has turned over from
// one that is merely older than some other limit's.
type window struct {
	started time.Time
	per     time.Duration
	count   int
}

func newLimiter(now func() time.Time) *limiter {
	return &limiter{windows: map[string]*window{}, now: now}
}

// allow reports whether key may proceed under lim, and if not, how long
// until its window turns over.
func (l *limiter) allow(key string, lim limit) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	w := l.windows[key]
	if w == nil || now.Sub(w.started) >= lim.per {
		if len(l.windows) >= maxLimiterKeys {
			l.prune(now)
		}
		l.windows[key] = &window{started: now, per: lim.per, count: 1}
		return true, 0
	}
	if w.count < lim.requests {
		w.count++
		return true, 0
	}
	return false, w.started.Add(lim.per).Sub(now)
}

// within reports whether key is still under lim, without counting anything:
// for a limit on something only some requests do, asked before it is known
// whether this one does it.
func (l *limiter) within(key string, lim limit) bool {
	return l.wait(key, lim) == 0
}

// wait is how long until key may proceed under lim, and zero when it may
// now. It counts nothing.
func (l *limiter) wait(key string, lim limit) time.Duration {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.windows[key]
	if w == nil || now.Sub(w.started) >= lim.per || w.count < lim.requests {
		return 0
	}
	return w.started.Add(lim.per).Sub(now)
}

// prune drops windows that have turned over, each judged by its own
// length. It runs under the lock, and only when the map is full. It is a
// best effort and not a bound: a map full of live windows stays full, and
// the new key is added anyway, because refusing a till over memory the
// process has plenty of would be the worse failure.
func (l *limiter) prune(now time.Time) {
	for key, w := range l.windows {
		if now.Sub(w.started) >= w.per {
			delete(l.windows, key)
		}
	}
}

// rateLimit wraps a handler with a limit per remote address. The key is the
// limit's and the address's together, so two limits on one address count
// separately: a guest reading the menu spends nothing of the pairing limit.
func (s *Server) rateLimit(lim limit, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The address as this machine sees it. There is no proxy in front of
		// a till, so no forwarded header is trusted: one would let a caller
		// pick its own bucket.
		ok, retryAfter := s.limiter.allow(lim.key(remoteHost(r)), lim)
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
			writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, "too many requests; wait and try again")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// retryAfterSeconds rounds a wait up to whole seconds, never below one: a
// Retry-After of 0 invites an immediate retry that is refused again.
func retryAfterSeconds(d time.Duration) int {
	return max(int(math.Ceil(d.Seconds())), 1)
}
