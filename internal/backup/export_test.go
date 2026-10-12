package backup

import (
	"testing"
	"time"

	"github.com/benbjohnson/litestream"
)

// A test has no half minute to wait for a sale to be sent. Every engine a
// test of this package starts sends each second, as Litestream itself would,
// unless it asks for another pace.
func init() { pace.sync = time.Second }

// LiveAnHour makes everything this package does on a clock run scale times
// faster for the length of a test, and returns how long a venue's d then
// takes. With it a test can count what an hour of trading asks of a
// bucket in a few seconds.
func LiveAnHour(t *testing.T, scale int) func(d time.Duration) time.Duration {
	t.Helper()
	fast := func(d time.Duration) time.Duration { return d / time.Duration(scale) }
	was := pace
	levels := make(litestream.CompactionLevels, len(was.levels))
	for i, lvl := range was.levels {
		levels[i] = &litestream.CompactionLevel{Level: lvl.Level, Interval: fast(lvl.Interval)}
	}
	pace.levels = levels
	pace.l0Retention, pace.l0RetentionCheck = fast(was.l0Retention), fast(was.l0RetentionCheck)
	pace.probeBucket, pace.probeFolder, pace.probeGone = fast(was.probeBucket), fast(was.probeFolder), fast(was.probeGone)
	t.Cleanup(func() { pace = was })
	return fast
}

// HurryLitestream makes Litestream merge and delete its small files
// within a fraction of a second for the length of a test, where it would
// take minutes. What a venue's data directory looks like after a morning
// is then what a test's looks like after a moment: the small files that
// could rebuild a backup from nothing are gone.
func HurryLitestream(t *testing.T) {
	t.Helper()
	was := pace
	pace.levels = litestream.CompactionLevels{
		{Level: 0},
		{Level: 1, Interval: 100 * time.Millisecond},
		{Level: 2, Interval: 400 * time.Millisecond},
	}
	pace.l0Retention = 100 * time.Millisecond
	pace.l0RetentionCheck = 50 * time.Millisecond
	t.Cleanup(func() { pace = was })
}

// SwapsLive says, for a test, whether m may stop a running backup under
// the open database, whatever system the test runs on.
func (m *Manager) SwapsLive(live bool) { m.swapsLive = live }
