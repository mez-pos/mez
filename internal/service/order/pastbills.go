package order

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mez-pos/mez/internal/db/gen"
)

// maxPastBills is how many closed bills the till lists before a search:
// the last few hours of a busy counter, which is where refunds come from.
const maxPastBills = 30

// PastBills lists closed bills for the till (Task 6b): the latest, or, with
// a query, those whose invoice or credit note number it is, or whose
// invoice's sequence number it is when it is all digits.
func (s *Service) PastBills(ctx context.Context, venueID int64, query string) ([]Result, error) {
	v, err := loadVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return nil, err
	}
	q := s.db.Queries()
	type row struct {
		id       int64
		publicID string
	}
	var rows []row
	query = strings.TrimSpace(query)
	if query == "" {
		found, err := q.ClosedOrdersForVenue(ctx, gen.ClosedOrdersForVenueParams{VenueID: venueID, Limit: maxPastBills})
		if err != nil {
			return nil, fmt.Errorf("listing the past bills of venue %d: %w", venueID, err)
		}
		for _, f := range found {
			rows = append(rows, row{f.ID, f.PublicID})
		}
	} else {
		seq, _ := strconv.ParseInt(query, 10, 64) // 0 when not a number: no invoice has seq 0
		found, err := q.OrdersByDocument(ctx, gen.OrdersByDocumentParams{VenueID: venueID, Number: query, Seq: seq})
		if err != nil {
			return nil, fmt.Errorf("searching the bills of venue %d: %w", venueID, err)
		}
		for _, f := range found {
			rows = append(rows, row{f.ID, f.PublicID})
		}
	}
	out := make([]Result, 0, len(rows))
	for _, r := range rows {
		state, err := s.db.LoadOrder(ctx, r.id, v.Taxes)
		if err != nil {
			return nil, err
		}
		out = append(out, Result{OrderID: r.id, PublicID: r.publicID, State: state})
	}
	return out, nil
}
