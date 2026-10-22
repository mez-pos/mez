// Package health composes what the venue says of its own health (ADR-361;
// Phase 15, Task 4): each printer's last success and last failure, the
// backup's state, the shift now open and the opening hours as set. The
// tunnel sends it to a relay that asked for it, on the socket the venue
// already holds; mez Cloud's clock alerts read it there.
//
// Everything here is read from the venue's own records. Nothing is asked
// of the printers, the backup's destination or anything outside the
// machine: the summary says what the venue last saw, which is what the
// owner's alert should say too.
package health

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/printjob"
	"github.com/mez-pos/mez/internal/relay/wire"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// Backups is what the summary needs of the venue's backup. *backup.Manager
// is the implementation.
type Backups interface {
	Status() backup.Status
}

// Options configures a Service.
type Options struct {
	// DB is required.
	DB *db.DB
	// Backups is the venue's backup. Nil is a server built without one,
	// which reports as "off".
	Backups Backups
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Service makes the summary.
type Service struct {
	db      *db.DB
	backups Backups
	now     func() time.Time
}

// ErrNoVenue is a machine with no venue yet, or more than one: there is no
// one venue whose health this is.
var ErrNoVenue = errors.New("health: this machine does not have exactly one venue")

// New checks the options.
func New(o Options) (*Service, error) {
	if o.DB == nil {
		return nil, errors.New("health: Options.DB is required")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{db: o.DB, backups: o.Backups, now: o.Now}, nil
}

// Summary is the venue's health now, read from its records. The venue is
// the one this machine has: with none or several there is no summary.
func (s *Service) Summary(ctx context.Context) (wire.Health, error) {
	q := s.db.Queries()
	venues, err := q.ListVenues(ctx)
	if err != nil {
		return wire.Health{}, fmt.Errorf("health: listing venues: %w", err)
	}
	if len(venues) != 1 {
		return wire.Health{}, fmt.Errorf("%w: it has %d", ErrNoVenue, len(venues))
	}
	v := venues[0]

	h := wire.Health{At: s.now().UTC(), Timezone: v.Timezone, Backup: backupHealth(s.backups)}

	printers, err := q.PrintersForVenue(ctx, v.ID)
	if err != nil {
		return wire.Health{}, fmt.Errorf("health: listing printers: %w", err)
	}
	outcomes, err := q.PrintOutcomesForVenue(ctx, v.ID)
	if err != nil {
		return wire.Health{}, fmt.Errorf("health: reading print outcomes: %w", err)
	}
	h.Printers = printerHealth(printers, outcomes)

	switch open, err := q.OpenShiftForVenue(ctx, v.ID); {
	case err == nil:
		h.Shift = &wire.ShiftHealth{ID: open.PublicID, TradingDay: open.TradingDay, OpenedAt: open.OpenedAt.UTC()}
	case !errors.Is(err, sql.ErrNoRows):
		return wire.Health{}, fmt.Errorf("health: reading the open shift: %w", err)
	}

	week, err := svcshift.OpeningHours(ctx, q, v.ID)
	if err != nil {
		return wire.Health{}, fmt.Errorf("health: %w", err)
	}
	if len(week) > 0 {
		h.Hours = make(map[string]wire.DayHours, len(week))
		for key, d := range week {
			h.Hours[key] = wire.DayHours{Open: d.Open, Close: d.Close}
		}
	}
	return h, nil
}

// printerHealth is each printer with its outcomes, in the printers' order.
func printerHealth(printers []gen.PrintersForVenueRow, outcomes []gen.PrintOutcomesForVenueRow) []wire.PrinterHealth {
	byPrinter := make(map[int64]wire.PrinterHealth, len(printers))
	for _, o := range outcomes {
		if !o.PrinterID.Valid {
			continue
		}
		p := byPrinter[o.PrinterID.Int64]
		switch printjob.State(o.State) {
		case printjob.Done:
			if o.DoneAt.Valid {
				at := o.DoneAt.Time.UTC()
				p.LastSuccess = &at
			}
		case printjob.Failed, printjob.Dead:
			at := o.UpdatedAt.UTC()
			p.LastFailure = &at
			p.Error = o.LastError
		}
		byPrinter[o.PrinterID.Int64] = p
	}
	out := make([]wire.PrinterHealth, 0, len(printers))
	for _, pr := range printers {
		p := byPrinter[pr.ID]
		p.Name, p.Active = pr.Name, pr.Active
		if p.LastFailure != nil && p.LastSuccess != nil && p.LastSuccess.After(*p.LastFailure) {
			// It printed since: the failure is over, and its words with it.
			p.Error = ""
		}
		out = append(out, p)
	}
	return out
}

// backupHealth is the backup as the venue sees it.
func backupHealth(b Backups) wire.BackupHealth {
	if b == nil {
		return wire.BackupHealth{State: string(backup.StateOff)}
	}
	st := b.Status()
	out := wire.BackupHealth{State: string(st.State), Error: st.Error}
	if !st.LastSyncedAt.IsZero() {
		at := st.LastSyncedAt.UTC()
		out.LastSyncedAt = &at
	}
	return out
}
