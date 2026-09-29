package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
)

// ZPageSize is the most reports a page of the list holds.
const ZPageSize = 50

// CorrectZReport issues a correction of a closed shift's Z-report: the
// figures worked out again from the orders, under the next number, naming
// the report it corrects and why. The report it corrects stays as it was.
// A retry of the same correction by the same person, whose figures have
// not moved since, gets that correction back rather than another.
func (s *Service) CorrectZReport(ctx context.Context, meta Meta, shiftID int64, reason string) (ZReport, error) {
	if err := meta.validate(); err != nil {
		return ZReport{}, err
	}
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return ZReport{}, fmt.Errorf("%w: a correction needs a reason", ErrInvalidCommand)
	case utf8.RuneCountInString(reason) > MaxReasonLength:
		return ZReport{}, fmt.Errorf("%w: the reason is over %d characters", ErrInvalidCommand, MaxReasonLength)
	}
	now := s.clock.Now().UTC()
	var number int64
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		if shift.Status(row.Status) != shift.StatusClosed {
			return fmt.Errorf("%w: %s", ErrShiftStillOpen, row.PublicID)
		}
		latest, err := latestFor(ctx, q, row.ID)
		if err != nil {
			return err
		}
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		sh, err := load(ctx, q, row, v.currency)
		if err != nil {
			return err
		}
		doc, err := document(ctx, q, meta.VenueID, sh, v.currency, meta.ActorID, now)
		if err != nil {
			return err
		}
		d := latest.Document
		if d.Corrects != 0 && d.Reason == reason && d.GeneratedBy == meta.ActorID && reflect.DeepEqual(d.Report, doc.Report) {
			number = d.Number
			return nil
		}
		number, err = issue(ctx, q, meta.VenueID, row.ID, &latest, reason, doc)
		return err
	})
	if err != nil {
		return ZReport{}, fmt.Errorf("correcting the Z-report: %w", err)
	}
	return s.ZReport(ctx, meta.VenueID, number)
}

// ZReport is one of the venue's reports by its number.
func (s *Service) ZReport(ctx context.Context, venueID, number int64) (ZReport, error) {
	row, err := s.db.Queries().ZReportByNumber(ctx, gen.ZReportByNumberParams{VenueID: venueID, Number: number})
	if errors.Is(err, sql.ErrNoRows) {
		return ZReport{}, fmt.Errorf("%w: %d", ErrZReportNotFound, number)
	}
	if err != nil {
		return ZReport{}, fmt.Errorf("reading Z-report %d: %w", number, err)
	}
	return decodeZ(row.ID, row.ShiftID, row.Body)
}

// ZReportForShift is a shift's report as it now stands: its latest
// correction, or the original.
func (s *Service) ZReportForShift(ctx context.Context, venueID, shiftID int64) (ZReport, error) {
	q := s.db.Queries()
	if _, err := shiftAt(ctx, q, venueID, shiftID); err != nil {
		return ZReport{}, err
	}
	return latestFor(ctx, q, shiftID)
}

// ZReports is a page of the venue's reports, newest first, with numbers
// below before (0 for the first page).
func (s *Service) ZReports(ctx context.Context, venueID, before int64) ([]ZSummary, error) {
	v, err := venueOf(ctx, s.db.Queries(), venueID)
	if err != nil {
		return nil, err
	}
	if before <= 0 {
		before = 1 << 62
	}
	rows, err := s.db.Queries().ZReportsBefore(ctx, gen.ZReportsBeforeParams{VenueID: venueID, Before: before, PageSize: ZPageSize})
	if err != nil {
		return nil, fmt.Errorf("listing the Z-reports: %w", err)
	}
	out := make([]ZSummary, len(rows))
	for i, r := range rows {
		out[i] = ZSummary{
			Number: r.Number, ShiftID: r.ShiftID, ShiftPublicID: r.ShiftPublicID, TradingDay: r.TradingDay,
			Correction: r.CorrectsID.Valid, Reason: r.Reason, GeneratedAt: r.GeneratedAt.UTC(),
			SalesTotal: money.Amount{Minor: r.SalesTotalMinor, Currency: v.currency},
		}
	}
	return out, nil
}

func latestFor(ctx context.Context, q *gen.Queries, shiftID int64) (ZReport, error) {
	row, err := q.LatestZReportForShift(ctx, shiftID)
	if errors.Is(err, sql.ErrNoRows) {
		return ZReport{}, fmt.Errorf("%w: shift %d has none", ErrZReportNotFound, shiftID)
	}
	if err != nil {
		return ZReport{}, fmt.Errorf("reading shift %d's Z-report: %w", shiftID, err)
	}
	return decodeZ(row.ID, row.ShiftID, row.Body)
}

func decodeZ(id, shiftID int64, body string) (ZReport, error) {
	var doc ZDocument
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return ZReport{}, fmt.Errorf("decoding Z-report %d: %w", id, err)
	}
	return ZReport{ID: id, ShiftID: shiftID, Document: doc}, nil
}
