package relay

import (
	"sync"
	"time"
)

// Limit is what one venue's guests may ask of it through the relay: Burst
// requests at once, refilled at PerMinute.
//
// It is per venue and not per guest. The relay cannot tell a table of six
// from one phone opening six tabs, and the thing being protected is the
// venue's own machine, which is also running the counter. The venue limits
// each table itself (Phase 11, Task 6), where it knows what a table is.
type Limit struct {
	PerMinute int
	Burst     int
}

// DefaultLimit lets a full room open the menu together, photographs and all,
// and settles at twenty requests a second, which is far more ordering than a
// cafe produces and far less than a script does.
var DefaultLimit = Limit{PerMinute: 1200, Burst: 300}

// bucket is a token bucket. One lives on each connected venue, so there is no
// table of them to grow and nothing to prune.
type bucket struct {
	mu     sync.Mutex
	lim    Limit
	tokens float64
	last   time.Time
}

func newBucket(lim Limit, now time.Time) *bucket {
	return &bucket{lim: lim, tokens: float64(lim.Burst), last: now}
}

// take spends one token. When there is none it says how long until there is.
func (b *bucket) take(now time.Time) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	perSecond := float64(b.lim.PerMinute) / 60
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = min(b.tokens+elapsed.Seconds()*perSecond, float64(b.lim.Burst))
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / perSecond * float64(time.Second))
}
