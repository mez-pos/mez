package backup

import (
	"testing"
	"time"

	"github.com/benbjohnson/litestream"
)

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
