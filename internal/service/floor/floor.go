// Package floor is the back office's floor plan (phase 08, Task 1,
// ADR-085): the venue's areas, and each table's name, seats, shape and the
// grid cell it stands in. Layout only; a table's state comes from its
// orders (Task 2).
package floor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// The refusals. The API maps them to codes; those that are about a field
// also carry an *Invalid.
var (
	ErrTableNotFound = errors.New("no such table at this venue")
	ErrAreaNotFound  = errors.New("no such area at this venue")
	ErrAreaRetired   = errors.New("the area is retired")
	ErrAreaInUse     = errors.New("the area still has tables in service")
	ErrDuplicate     = errors.New("that name is taken")
	ErrCellTaken     = errors.New("another table stands in that cell")
)

// Invalid is a setting that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// The grid every area is laid out on: Columns cells across, Rows down. A
// tablet shows it whole in landscape, and twelve by eight holds a busy
// cafe's floor with room between the tables.
const (
	Columns = 12
	Rows    = 8
)

// The bounds on what the back office types.
const (
	maxAreaName  = 30
	maxTableName = 20
	maxSeats     = 40
)

// Shape is how a table is drawn. It changes nothing but the drawing: every
// table takes one cell.
type Shape string

// The shapes.
const (
	Square Shape = "square"
	Round  Shape = "round"
	Long   Shape = "long"
)

// Shapes is every shape, for the generated vocabulary.
func Shapes() []Shape { return []Shape{Square, Round, Long} }

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Orders is what the live floor needs of the orders: every one not yet
// closed, projected. *order.Service is the implementation.
type Orders interface {
	OpenOrders(ctx context.Context, venueID int64) (svcorder.OpenTickets, error)
}

// Options configure a Service. DB is required; Orders is needed only for
// the live floor.
type Options struct {
	DB     *db.DB
	Orders Orders
	Clock  Clock
	// OnChange is told a venue's floor version moved, after the change is
	// committed: the hub's Notify, so terminals read the floor again.
	OnChange func(venueID int64)
}

// Service is the floor plan, and the floor as it stands in service.
type Service struct {
	db       *db.DB
	orders   Orders
	clock    Clock
	onChange func(venueID int64)
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil {
		return nil, errors.New("floor: a database is required")
	}
	if o.Clock == nil {
		o.Clock = systemClock{}
	}
	if o.OnChange == nil {
		o.OnChange = func(int64) {}
	}
	return &Service{db: o.DB, orders: o.Orders, clock: o.Clock, onChange: o.OnChange}, nil
}

// write runs a change to a venue's floor in one transaction, moves the
// venue's floor version on in the same one, and says so once committed.
func (s *Service) write(ctx context.Context, venueID int64, fn func(tx *gen.Queries) error) error {
	err := s.db.WriteTx(ctx, func(tx *gen.Queries) error {
		if err := fn(tx); err != nil {
			return err
		}
		return tx.BumpFloorVersion(ctx, venueID)
	})
	if err == nil {
		s.onChange(venueID)
	}
	return err
}

// AreaSettings is an area as the back office edits it.
type AreaSettings struct {
	Name string `json:"name"`
}

func (a AreaSettings) clean() (AreaSettings, error) {
	a.Name = strings.TrimSpace(a.Name)
	switch {
	case a.Name == "":
		return a, &Invalid{"name", "Give the area a name, like Inside or Terrace."}
	case len(a.Name) > maxAreaName:
		return a, &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", maxAreaName)}
	}
	return a, nil
}

// TableSettings is a table as the back office edits it.
type TableSettings struct {
	AreaID int64  `json:"area_id"`
	Name   string `json:"name"`
	Seats  int64  `json:"seats"`
	Shape  Shape  `json:"shape"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
}

func (t TableSettings) clean() (TableSettings, error) {
	t.Name = strings.TrimSpace(t.Name)
	switch {
	case t.Name == "":
		return t, &Invalid{"name", "Give the table a name, like 4 or T4."}
	case len(t.Name) > maxTableName:
		return t, &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", maxTableName)}
	case t.Seats < 1 || t.Seats > maxSeats:
		return t, &Invalid{"seats", fmt.Sprintf("Seats is a number from 1 to %d.", maxSeats)}
	case t.Shape != Square && t.Shape != Round && t.Shape != Long:
		return t, &Invalid{"shape", "Choose square, round or long."}
	case t.X < 0 || t.X >= Columns:
		return t, &Invalid{"x", fmt.Sprintf("Column is a number from 1 to %d.", Columns)}
	case t.Y < 0 || t.Y >= Rows:
		return t, &Invalid{"y", fmt.Sprintf("Row is a number from 1 to %d.", Rows)}
	}
	return t, nil
}

// Plan is the whole floor: every area and every table, retired ones too.
type Plan struct {
	Areas  []gen.AreasForVenueRow
	Tables []gen.TablesForVenueRow
}

// Plan reads the venue's floor plan.
func (s *Service) Plan(ctx context.Context, venueID int64) (Plan, error) {
	q := s.db.Queries()
	areas, err := q.AreasForVenue(ctx, venueID)
	if err != nil {
		return Plan{}, fmt.Errorf("floor: listing areas: %w", err)
	}
	tables, err := q.TablesForVenue(ctx, venueID)
	if err != nil {
		return Plan{}, fmt.Errorf("floor: listing tables: %w", err)
	}
	return Plan{Areas: areas, Tables: tables}, nil
}

// CreateArea adds an area.
func (s *Service) CreateArea(ctx context.Context, venueID int64, in AreaSettings) (int64, error) {
	in, err := in.clean()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now().UTC()
	var id int64
	err = s.write(ctx, venueID, func(tx *gen.Queries) error {
		var err error
		id, err = tx.CreateArea(ctx, gen.CreateAreaParams{VenueID: venueID, Name: in.Name, CreatedAt: now, UpdatedAt: now})
		return err
	})
	return id, duplicate(err, "An area")
}

// RenameArea renames an area.
func (s *Service) RenameArea(ctx context.Context, venueID, id int64, in AreaSettings) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	err = s.write(ctx, venueID, func(tx *gen.Queries) error {
		n, err := tx.RenameArea(ctx, gen.RenameAreaParams{Name: in.Name, Now: s.clock.Now().UTC(), ID: id, VenueID: venueID})
		return one(n, err, ErrAreaNotFound)
	})
	return duplicate(err, "An area")
}

// SetAreaActive retires an area or brings it back. An area is retired only
// once its tables are, so no table in service stands somewhere hidden.
func (s *Service) SetAreaActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.write(ctx, venueID, func(tx *gen.Queries) error {
		a, err := area(ctx, tx, venueID, id)
		if err != nil {
			return err
		}
		if !active {
			n, err := tx.CountActiveTablesInArea(ctx, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return fmt.Errorf("%w: %s has %d", ErrAreaInUse, a.Name, n)
			}
		}
		n, err := tx.SetAreaActive(ctx, gen.SetAreaActiveParams{Active: active, Now: s.clock.Now().UTC(), ID: id, VenueID: venueID})
		return one(n, err, ErrAreaNotFound)
	})
}

// CreateTable places a new table.
func (s *Service) CreateTable(ctx context.Context, venueID int64, in TableSettings) (int64, error) {
	in, err := in.clean()
	if err != nil {
		return 0, err
	}
	now := s.clock.Now().UTC()
	var id int64
	err = s.write(ctx, venueID, func(tx *gen.Queries) error {
		if err := placeable(ctx, tx, venueID, 0, in); err != nil {
			return err
		}
		var err error
		id, err = tx.CreateTable(ctx, gen.CreateTableParams{
			VenueID: venueID, AreaID: in.AreaID, Name: in.Name, Seats: in.Seats, Shape: string(in.Shape),
			X: in.X, Y: in.Y, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, duplicate(err, "A table")
}

// UpdateTable changes a table's settings, its place among them: a drag on
// the editor and the numeric fallback both land here. A retired table can
// be edited and moved, and takes no cell until it is brought back.
func (s *Service) UpdateTable(ctx context.Context, venueID, id int64, in TableSettings) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	err = s.write(ctx, venueID, func(tx *gen.Queries) error {
		t, err := tableByID(ctx, tx, venueID, id)
		if err != nil {
			return err
		}
		if t.Active {
			if err := placeable(ctx, tx, venueID, id, in); err != nil {
				return err
			}
		} else if _, err := area(ctx, tx, venueID, in.AreaID); err != nil {
			if errors.Is(err, ErrAreaNotFound) {
				return fmt.Errorf("%w: %w", err, &Invalid{"area_id", "Choose an area."})
			}
			return err
		}
		n, err := tx.UpdateTable(ctx, gen.UpdateTableParams{
			AreaID: in.AreaID, Name: in.Name, Seats: in.Seats, Shape: string(in.Shape), X: in.X, Y: in.Y,
			Now: s.clock.Now().UTC(), ID: id, VenueID: venueID,
		})
		return one(n, err, ErrTableNotFound)
	})
	return duplicate(err, "A table")
}

// SetTableActive retires a table or brings it back. Brought back, it needs
// its cell free and its area in service.
func (s *Service) SetTableActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.write(ctx, venueID, func(tx *gen.Queries) error {
		t, err := tableByID(ctx, tx, venueID, id)
		if err != nil {
			return err
		}
		if active && !t.Active {
			at := TableSettings{AreaID: t.AreaID, X: t.X, Y: t.Y}
			if err := placeable(ctx, tx, venueID, id, at); err != nil {
				return err
			}
		}
		n, err := tx.SetTableActive(ctx, gen.SetTableActiveParams{Active: active, Now: s.clock.Now().UTC(), ID: id, VenueID: venueID})
		return one(n, err, ErrTableNotFound)
	})
}

// placeable refuses a place in another venue's area, a retired area, or a
// cell another table in service already stands in. except is the table
// being moved, so it does not collide with itself.
func placeable(ctx context.Context, tx *gen.Queries, venueID, except int64, in TableSettings) error {
	a, err := area(ctx, tx, venueID, in.AreaID)
	if errors.Is(err, ErrAreaNotFound) {
		return fmt.Errorf("%w: %w", err, &Invalid{"area_id", "Choose an area."})
	}
	if err != nil {
		return err
	}
	if !a.Active {
		return fmt.Errorf("%w: %w", ErrAreaRetired, &Invalid{"area_id", a.Name + " is retired. Bring it back first."})
	}
	other, err := tx.TableInCell(ctx, gen.TableInCellParams{AreaID: in.AreaID, X: in.X, Y: in.Y, ExceptID: except})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("floor: reading the cell: %w", err)
	}
	return fmt.Errorf("%w: %w", ErrCellTaken,
		&Invalid{"x", fmt.Sprintf("Table %s is already at column %d, row %d.", other.Name, in.X+1, in.Y+1)})
}

func area(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.AreaByIDRow, error) {
	a, err := q.AreaByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && a.VenueID != venueID) {
		return gen.AreaByIDRow{}, fmt.Errorf("%w: %d", ErrAreaNotFound, id)
	}
	if err != nil {
		return gen.AreaByIDRow{}, fmt.Errorf("floor: reading area %d: %w", id, err)
	}
	return a, nil
}

func tableByID(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.TableByIDRow, error) {
	t, err := q.TableByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && t.VenueID != venueID) {
		return gen.TableByIDRow{}, fmt.Errorf("%w: %d", ErrTableNotFound, id)
	}
	if err != nil {
		return gen.TableByIDRow{}, fmt.Errorf("floor: reading table %d: %w", id, err)
	}
	return t, nil
}

// one refuses a write that matched no row.
func one(n int64, err error, notFound error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return notFound
	}
	return nil
}

// duplicate turns a unique name into a refusal under the name field. The
// cell's own unique index is checked first by placeable, so a unique
// violation that reaches here is the name.
func duplicate(err error, what string) error {
	if db.IsUniqueViolation(err) {
		return fmt.Errorf("%w: %w", ErrDuplicate, &Invalid{"name", what + " with that name already exists."})
	}
	return err
}
