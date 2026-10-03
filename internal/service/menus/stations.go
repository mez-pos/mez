package menus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// ErrStationNotFound means the station is not this venue's.
var ErrStationNotFound = errors.New("no such station at this venue")

// maxCodeLength is the schema's limit on a station's code.
const maxCodeLength = 32

// StationInput is a station as the editor sends it. The code is what a line
// snapshots and what the kitchen stream routes on, so it is chosen once, when
// the station is made, and never changes. Empty takes one from the name.
type StationInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// RetireInput says where what a station or course carries goes when it is
// retired: the id of another one in service. It may be left out when
// nothing needs to go anywhere.
type RetireInput struct {
	MoveTo int64 `json:"move_to"`
}

// StationRetired is what retiring a station moved.
type StationRetired struct {
	ItemsMoved int64 `json:"items_moved"`
	// PrinterMoved is true when the station's printer now serves the station
	// its items went to. The printer is left alone, and the station it came
	// from stops printing, when the other already had one.
	PrinterMoved bool `json:"printer_moved"`
}

// CreateStation adds a station at the end of the list.
func (s *Service) CreateStation(ctx context.Context, venueID int64, in StationInput) (int64, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return 0, err
	}
	code, err := stationCode(in.Code, name)
	if err != nil {
		return 0, err
	}
	now := s.now()
	var id int64
	err = s.edit(ctx, venueID, func(q *gen.Queries) error {
		next, err := q.NextStationSort(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: next station place: %w", err)
		}
		id, err = q.CreateStation(ctx, gen.CreateStationParams{
			VenueID: venueID, Code: code, Name: name, SortOrder: next, CreatedAt: now, UpdatedAt: now,
		})
		if db.IsUniqueViolation(err) {
			return &Invalid{"code", fmt.Sprintf("A station already uses the code %q. Give this one a code of its own.", code)}
		}
		return err
	})
	return id, err
}

// RenameStation changes a station's name. Its code stays.
func (s *Service) RenameStation(ctx context.Context, venueID, id int64, in StationInput) error {
	name, err := cleanName(in.Name)
	if err != nil {
		return err
	}
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.RenameStation(ctx, gen.RenameStationParams{Name: name, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrStationNotFound)
	})
}

// OrderStations puts the stations in the order given. ids must name every
// station the venue has, retired ones included, once each.
func (s *Service) OrderStations(ctx context.Context, venueID int64, ids []int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		have, err := q.StationIDs(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: reading stations: %w", err)
		}
		if !samePermutation(have, ids) {
			return ErrStaleOrder
		}
		now := s.now()
		for i, id := range ids {
			n, err := q.SetStationSort(ctx, gen.SetStationSortParams{SortOrder: int64(i), Now: now, VenueID: venueID, ID: id})
			if err := oneRow(n, err, ErrStationNotFound); err != nil {
				return err
			}
		}
		return nil
	})
}

// RetireStation takes a station out of service. Its items and its printer
// must go somewhere: in.MoveTo names another station in service, which takes
// every item (retired ones too, so none is brought back onto a station that is
// gone) and the printer, if it has none of its own. Tickets already sent to
// the station stay on its board until they are done.
//
// A venue keeps at least one station in service. Retiring one already
// retired changes nothing.
func (s *Service) RetireStation(ctx context.Context, venueID, id int64, in RetireInput) (StationRetired, error) {
	var out StationRetired
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		st, err := q.StationRouting(ctx, gen.StationRoutingParams{VenueID: venueID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrStationNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("menus: reading station %d: %w", id, err)
		}
		if !st.Active {
			return nil
		}
		live, err := q.ActiveStationCount(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: counting stations: %w", err)
		}
		if live < 2 {
			return &Invalid{"id", "This is the only station in service. Add another before retiring it."}
		}
		items, err := q.ItemCountForStation(ctx, gen.ItemCountForStationParams{VenueID: venueID, StationID: id})
		if err != nil {
			return fmt.Errorf("menus: counting station %d's items: %w", id, err)
		}
		if items > 0 || st.PrinterID.Valid {
			target, err := q.StationRouting(ctx, gen.StationRoutingParams{VenueID: venueID, ID: in.MoveTo})
			switch {
			case in.MoveTo == 0:
				return &Invalid{"move_to", "Choose the station its items and printer go to."}
			case in.MoveTo == id:
				return &Invalid{"move_to", "Choose a different station."}
			case errors.Is(err, sql.ErrNoRows):
				return &Invalid{"move_to", "That is not a station here."}
			case err != nil:
				return fmt.Errorf("menus: reading station %d: %w", in.MoveTo, err)
			case !target.Active:
				return &Invalid{"move_to", "That station is retired. Choose one in service."}
			}
			now := s.now()
			if out.ItemsMoved, err = q.MoveStationItems(ctx, gen.MoveStationItemsParams{
				ToStation: in.MoveTo, Now: now, VenueID: venueID, FromStation: id,
			}); err != nil {
				return fmt.Errorf("menus: moving station %d's items: %w", id, err)
			}
			if st.PrinterID.Valid {
				if !target.PrinterID.Valid {
					if _, err := q.SetStationPrinterByID(ctx, gen.SetStationPrinterByIDParams{
						PrinterID: st.PrinterID, Now: now, VenueID: venueID, ID: in.MoveTo,
					}); err != nil {
						return fmt.Errorf("menus: giving station %d the printer: %w", in.MoveTo, err)
					}
					out.PrinterMoved = true
				}
				if _, err := q.SetStationPrinterByID(ctx, gen.SetStationPrinterByIDParams{Now: now, VenueID: venueID, ID: id}); err != nil {
					return fmt.Errorf("menus: freeing station %d's printer: %w", id, err)
				}
			}
		}
		n, err := q.SetStationActive(ctx, gen.SetStationActiveParams{Active: false, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrStationNotFound)
	})
	return out, err
}

// RestoreStation brings a station back into service. What was moved off it
// stays where it went.
func (s *Service) RestoreStation(ctx context.Context, venueID, id int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetStationActive(ctx, gen.SetStationActiveParams{Active: true, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrStationNotFound)
	})
}

// stationCode is the code a station is made with: the one given, or one
// made from its name. It is what the schema accepts: a lowercase letter, then
// lowercase letters, digits, hyphens and underscores, up to 32.
func stationCode(given, name string) (string, error) {
	code := strings.TrimSpace(given)
	if code == "" {
		code = slug(name)
		if code == "" {
			return "", &Invalid{"code", "Give it a code of lowercase letters and digits, like bar."}
		}
		return code, nil
	}
	if !validCode(code) {
		return "", &Invalid{"code", "Use lowercase letters, digits, - and _, starting with a letter, up to 32 characters."}
	}
	return code, nil
}

// slug is a name as a code: lowercase letters and digits, runs of anything
// else made one hyphen. Empty when nothing usable starts with a letter.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9' && b.Len() > 0:
			if dash {
				b.WriteByte('-')
				dash = false
			}
			b.WriteRune(r)
		case b.Len() > 0:
			dash = true
		}
	}
	out := b.String()
	if len(out) > maxCodeLength {
		out = strings.TrimRight(out[:maxCodeLength], "-")
	}
	return out
}

func validCode(code string) bool {
	if len(code) < 1 || len(code) > maxCodeLength || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	return strings.Trim(code, "abcdefghijklmnopqrstuvwxyz0123456789_-") == ""
}
