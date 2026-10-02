package db

import (
	"context"
	"time"
)

// Health statuses. The strings are the ones /api/v1/health publishes, and
// cmd/mez maps them onto the API constants explicitly rather than letting
// two packages agree by accident.
const (
	StatusOK      = "ok"
	StatusFailing = "failing"
	StatusUnknown = "unknown"
)

// probeTimeout bounds one health check. A probe that has not answered in two
// seconds has told us what we needed to know.
const probeTimeout = 2 * time.Second

// Health is the last thing the prober found out.
type Health struct {
	// Status is StatusOK, StatusFailing or StatusUnknown.
	Status string

	// Error is why it is failing, empty when it is not. It is a string rather
	// than an error because it is read to be rendered, not to be matched on.
	Error string

	// CheckedAt is when the answer was taken. An answer that stops moving is
	// itself a symptom.
	CheckedAt time.Time
}

// Health returns the cached status without touching the database.
//
// The HTTP handler must be able to answer while the database is hung. A health
// endpoint that does IO is one more thing that can block, and whatever is
// asking is usually asking because something already has.
func (d *DB) Health() Health {
	d.healthMu.RLock()
	defer d.healthMu.RUnlock()
	return d.health
}

// proberLoop checks the database on a timer until ctx is cancelled.
func (d *DB) proberLoop(ctx context.Context, interval time.Duration) {
	defer close(d.proberDone)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.probe(ctx)
		}
	}
}

// probe asks the database one cheap question and records the answer.
//
// It reads the schema rather than running SELECT 1, because SELECT 1 is
// answered out of memory and would keep reporting ok with the file deleted
// underneath it. Reading the schema touches a page.
func (d *DB) probe(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	var tables int
	err := d.read.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema").Scan(&tables)

	previous := d.Health().Status
	if err != nil {
		d.setHealth(StatusFailing, err)
		if previous != StatusFailing {
			d.log.Error("database health check failed", "error", err)
		}
		return
	}

	d.setHealth(StatusOK, nil)
	if previous == StatusFailing {
		d.log.Info("database health recovered")
	}
}

// setHealth records an answer. It takes the error rather than a string so that
// the caller cannot forget to clear the previous one.
func (d *DB) setHealth(status string, err error) {
	health := Health{Status: status, CheckedAt: time.Now().UTC()}
	if err != nil {
		health.Error = err.Error()
	}

	d.healthMu.Lock()
	defer d.healthMu.Unlock()
	d.health = health
}
