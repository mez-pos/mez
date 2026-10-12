package backup

import (
	"fmt"
	"time"

	"github.com/benbjohnson/litestream"
)

// How often what changed is sent to the destination (ADR-315).
//
// Litestream's own pace is every second, and it was written for a server
// beside its bucket. A venue's bucket is billed by the request: a busy cafe
// sending every second makes about a million and a half writes a month,
// which costs more than the venue pays for the whole service. Thirty
// seconds is thirty times fewer, and what it risks is what was sold in the
// last half minute before the machine died. A clean stop sends everything
// first and risks nothing.
const (
	// DefaultSyncInterval is the pace when none is set.
	DefaultSyncInterval = 30 * time.Second
	// MinSyncInterval is the fastest that may be set. Faster is for a
	// bucket of one's own, and below this the requests outrun any use.
	MinSyncInterval = 5 * time.Second
	// MaxSyncInterval is the slowest. Past it, "backed up" would be said
	// of a venue that could lose a sitting.
	MaxSyncInterval = 10 * time.Minute
)

// CheckSyncInterval says whether d may be set as the pace.
func CheckSyncInterval(d time.Duration) error {
	if d < MinSyncInterval || d > MaxSyncInterval {
		return fmt.Errorf("must be from %s to %s", MinSyncInterval, MaxSyncInterval)
	}
	return nil
}

// pace is everything else that asks something of the destination on a clock,
// and how often. Each asking of a bucket is a request someone bills, a
// listing as much as a write, and two of these were most of the bill:
// Litestream's check for small files to delete lists the destination twice
// each time it runs, every fifteen seconds as it comes, sale or no sale;
// and its first merge, every thirty seconds, writes one file for each one
// sent once the sending itself is every thirty.
//
// In a variable so that a test can live through an hour in seconds.
var pace = struct {
	// sync is the pace when none is asked for.
	sync time.Duration
	// levels is how the small files sent are merged into bigger ones, so
	// that a restore reads a handful and not a day's worth. Litestream's
	// are thirty seconds, five minutes and an hour. The number of levels is
	// kept as it was: a destination written under the old pace has files
	// in each.
	levels litestream.CompactionLevels
	// l0Retention is how long a small file is kept after it has been
	// merged, and l0RetentionCheck how often they are looked for.
	l0Retention      time.Duration
	l0RetentionCheck time.Duration
	// probeBucket and probeFolder are how often the destination is looked
	// at while it answers, and probeGone while it does not: asking a bucket
	// that is not there costs nothing, and the answer is wanted sooner.
	probeBucket time.Duration
	probeFolder time.Duration
	probeGone   time.Duration
}{
	sync: DefaultSyncInterval,
	levels: litestream.CompactionLevels{
		{Level: 0},
		{Level: 1, Interval: 5 * time.Minute},
		{Level: 2, Interval: time.Hour},
		{Level: 3, Interval: 6 * time.Hour},
	},
	l0Retention:      litestream.DefaultL0Retention,
	l0RetentionCheck: time.Hour,
	probeBucket:      5 * time.Minute,
	probeFolder:      15 * time.Second,
	probeGone:        time.Minute,
}

// Multipart uploads to a bucket: only the nightly whole copy of a large
// database is big enough to be sent in parts, and each part is a request.
// Sixteen megabytes a part, two at a time, is a third of the requests of the
// SDK's five and at most thirty-two megabytes held in memory while it goes.
const (
	bucketPartSize    = 16 << 20
	bucketConcurrency = 2
)

// staleAfter is how long without reaching the destination before the backup
// is said to be failing. Litestream marks each look it takes, with something
// to send or not, so a healthy backup is never more than one interval old;
// three allow for a slow one.
func staleAfter(interval time.Duration) time.Duration {
	return max(time.Minute, 3*interval)
}
