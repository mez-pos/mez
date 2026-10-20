package menus

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
)

// station makes an active station of the venue through the service.
func (w *world) station(t *testing.T, venue int64, code, name string) int64 {
	t.Helper()
	id, err := w.s.CreateStation(context.Background(), venue, StationInput{Code: code, Name: name})
	if err != nil {
		t.Fatalf("creating station %q: %v", name, err)
	}
	return id
}

// printerFor gives a station a printer, as the printers screen does.
func (w *world) printerFor(t *testing.T, venue, station int64, name string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		id, err = q.CreatePrinter(ctx, gen.CreatePrinterParams{
			VenueID: venue, Name: name, Transport: "tcp", Address: "10.0.0.9", PaperMm: 80, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		n, err := q.SetStationPrinterByID(ctx, gen.SetStationPrinterByIDParams{PrinterID: sql.NullInt64{Int64: id, Valid: true}, Now: now, VenueID: venue, ID: station})
		return oneRow(n, err, ErrStationNotFound)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// printerOf is the printer a station sends to, or 0.
func (w *world) printerOf(t *testing.T, venue, station int64) int64 {
	t.Helper()
	st, err := w.d.Queries().StationRouting(context.Background(), gen.StationRoutingParams{VenueID: venue, ID: station})
	if err != nil {
		t.Fatal(err)
	}
	return st.PrinterID.Int64
}

func stationByID(e Editor, id int64) (Station, bool) {
	i := slices.IndexFunc(e.Stations, func(s Station) bool { return s.ID == id })
	if i < 0 {
		return Station{}, false
	}
	return e.Stations[i], true
}

func TestCodesFromNames(t *testing.T) {
	tests := []struct{ name, want string }{
		{"Bar", "bar"},
		{"Juice Bar", "juice-bar"},
		{"  Pastry & Cakes ", "pastry-cakes"},
		{"Bar 2", "bar-2"},
		{"9 Lives", "lives"},
		{"--x--", "x"},
		{"Café Bar", "caf-bar"},
		{"123", ""},
		{"!!!", ""},
		{strings.Repeat("a", 31) + " b c", strings.Repeat("a", 31)},
		{strings.Repeat("ab ", 20), "ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slug(tt.name)
			if got != tt.want {
				t.Errorf("slug(%q) = %q, want %q", tt.name, got, tt.want)
			}
			if got != "" && !validCode(got) {
				t.Errorf("slug(%q) = %q, which the schema would refuse", tt.name, got)
			}
		})
	}
}

func TestCreateAStation(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	before := w.version(t, plain)

	juice, err := w.s.CreateStation(ctx, plain, StationInput{Name: "  Juice Bar "})
	if err != nil {
		t.Fatal(err)
	}
	pastry := w.station(t, plain, "cakes", "Pastry")
	if w.version(t, plain) != before+2 || !slices.Equal(w.nudged, []int64{plain, plain}) {
		t.Errorf("version %d (was %d), nudged %v: two edits, two bumps", w.version(t, plain), before, w.nudged)
	}

	e := w.editor(t, plain)
	got, _ := stationByID(e, juice)
	if got != (Station{ID: juice, Code: "juice-bar", Name: "Juice Bar", Active: true}) {
		t.Errorf("juice = %+v: the code comes from the name, the name is trimmed", got)
	}
	if got, _ := stationByID(e, pastry); got.Code != "cakes" {
		t.Errorf("pastry = %+v: a code given is kept", got)
	}
	if n := len(e.Stations); n != 4 || e.Stations[2].ID != juice || e.Stations[3].ID != pastry {
		t.Errorf("stations = %+v: new ones go at the end", e.Stations)
	}
	if m, err := w.d.Menu(ctx, plain); err != nil || len(m.Stations) != 3 {
		t.Errorf("the till's stations: %+v, %v; kitchen and the two new ones", m.Stations, err)
	}

	// Another venue may use the same code.
	if _, err := w.s.CreateStation(ctx, gst, StationInput{Name: "Juice Bar"}); err != nil {
		t.Errorf("the same code at another venue: %v", err)
	}
}

func TestCreateAStationRefusals(t *testing.T) {
	tests := []struct {
		name  string
		in    StationInput
		field string
	}{
		{"no name", StationInput{Code: "x", Name: " "}, "name"},
		{"a long name", StationInput{Name: strings.Repeat("a", MaxNameLength+1)}, "name"},
		{"a name that makes no code", StationInput{Name: "123"}, "code"},
		{"a code in capitals", StationInput{Code: "Bar2", Name: "Bar"}, "code"},
		{"a code with a space", StationInput{Code: "juice bar", Name: "Juice"}, "code"},
		{"a code starting with a digit", StationInput{Code: "2bar", Name: "Bar"}, "code"},
		{"a code over 32", StationInput{Code: strings.Repeat("a", 33), Name: "Bar"}, "code"},
		{"a code in use", StationInput{Code: "kitchen", Name: "Second kitchen"}, "code"},
		{"a code a retired station holds", StationInput{Name: "Bar"}, "code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			before := w.version(t, plain)
			_, err := w.s.CreateStation(context.Background(), plain, tt.in)
			if fieldOf(err) != tt.field {
				t.Fatalf("err = %v, want a refusal of %q", err, tt.field)
			}
			if w.version(t, plain) != before || len(w.nudged) != 0 || len(w.editor(t, plain).Stations) != 2 {
				t.Error("a refusal must change nothing and tell nobody")
			}
		})
	}
}

func TestRenameAStation(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	if err := w.s.RenameStation(ctx, plain, r.kitchen, StationInput{Code: "ignored", Name: " Hot kitchen "}); err != nil {
		t.Fatal(err)
	}
	if got, _ := stationByID(w.editor(t, plain), r.kitchen); got.Name != "Hot kitchen" || got.Code != "kitchen" {
		t.Errorf("kitchen = %+v: the name changes, the code never does", got)
	}
	if err := w.s.RenameStation(ctx, plain, r.kitchen, StationInput{Name: ""}); fieldOf(err) != "name" {
		t.Errorf("an empty name: %v", err)
	}
	if err := w.s.RenameStation(ctx, plain, 999, StationInput{Name: "X"}); !errors.Is(err, ErrStationNotFound) {
		t.Errorf("a missing station: %v", err)
	}
	if err := w.s.RenameStation(ctx, plain, w.refs[gst].kitchen, StationInput{Name: "X"}); !errors.Is(err, ErrStationNotFound) {
		t.Errorf("another venue's station: %v", err)
	}
}

func TestOrderStations(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	juice := w.station(t, plain, "juice", "Juice")

	if err := w.s.OrderStations(ctx, plain, []int64{juice, r.bar, r.kitchen}); err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, s := range w.editor(t, plain).Stations {
		got = append(got, s.ID)
	}
	if !slices.Equal(got, []int64{juice, r.bar, r.kitchen}) {
		t.Errorf("order = %v, want juice, bar (retired), kitchen", got)
	}
	for _, stale := range [][]int64{
		{juice, r.kitchen},                   // a station missing
		{juice, r.bar, r.kitchen, r.kitchen}, // one twice
		{juice, r.bar, w.refs[gst].kitchen},  // another venue's
		nil,
	} {
		if err := w.s.OrderStations(ctx, plain, stale); !errors.Is(err, ErrStaleOrder) {
			t.Errorf("OrderStations(%v) = %v, want ErrStaleOrder", stale, err)
		}
	}
}

func TestRetireAStationNeedsToSayWhereThingsGo(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	juice := w.station(t, plain, "juice", "Juice")
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	w.nudged = nil
	before := w.version(t, plain)

	tests := []struct {
		name string
		in   RetireInput
	}{
		{"naming nowhere", RetireInput{}},
		{"itself", RetireInput{MoveTo: r.kitchen}},
		{"a retired station", RetireInput{MoveTo: r.bar}},
		{"a station that is not there", RetireInput{MoveTo: 999}},
		{"another venue's station", RetireInput{MoveTo: w.refs[gst].kitchen}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.s.RetireStation(ctx, plain, r.kitchen, tt.in)
			if fieldOf(err) != "move_to" {
				t.Fatalf("err = %v, want a refusal of move_to", err)
			}
		})
	}
	e := w.editor(t, plain)
	if st, _ := stationByID(e, r.kitchen); !st.Active || e.Items[0].StationID != r.kitchen || e.Version != before || len(w.nudged) != 0 {
		t.Errorf("a refused retire changed something: %+v, item on %d, version %d (was %d), nudged %v", st, e.Items[0].StationID, e.Version, before, w.nudged)
	}

	moved, err := w.s.RetireStation(ctx, plain, r.kitchen, RetireInput{MoveTo: juice})
	if err != nil {
		t.Fatal(err)
	}
	if moved != (StationRetired{ItemsMoved: 1}) {
		t.Errorf("moved = %+v, want one item and no printer", moved)
	}
	e = w.editor(t, plain)
	if st, _ := stationByID(e, r.kitchen); st.Active {
		t.Error("the kitchen is still in service")
	}
	if i := slices.IndexFunc(e.Items, func(it Item) bool { return it.ID == dosa }); e.Items[i].StationID != juice {
		t.Errorf("dosa is on station %d, want juice %d", e.Items[i].StationID, juice)
	}
	if e.Version != before+1 || len(w.nudged) != 1 {
		t.Errorf("version %d (was %d), nudged %v: one edit, one bump", e.Version, before, w.nudged)
	}
	m, err := w.d.Menu(ctx, plain)
	if err != nil || len(m.Items) != 1 || m.Items[0].Station != "juice" {
		t.Errorf("the till's menu: %+v, %v; dosa goes to juice", m.Items, err)
	}
}

func TestRetiringAStationMovesRetiredItemsToo(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	juice := w.station(t, plain, "juice", "Juice")
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	if err := w.s.SetItemActive(ctx, plain, dosa, false); err != nil {
		t.Fatal(err)
	}
	moved, err := w.s.RetireStation(ctx, plain, r.kitchen, RetireInput{MoveTo: juice})
	if err != nil || moved.ItemsMoved != 1 {
		t.Fatalf("moved %+v, %v: a retired item counts", moved, err)
	}
	// Brought back, it is on a station that is in service.
	if err := w.s.SetItemActive(ctx, plain, dosa, true); err != nil {
		t.Fatalf("bringing dosa back: %v", err)
	}
}

func TestRetireAStationWithAPrinter(t *testing.T) {
	tests := []struct {
		name        string
		targetOwn   bool // the station it moves to has a printer already
		wantMoved   bool
		wantTargetP func(own, from int64) int64
	}{
		{"the printer follows when the other has none", false, true, func(_, from int64) int64 { return from }},
		{"the other keeps its own printer", true, false, func(own, _ int64) int64 { return own }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			ctx := context.Background()
			r := w.refs[plain]
			juice := w.station(t, plain, "juice", "Juice")
			from := w.printerFor(t, plain, r.kitchen, "Pass")
			var own int64
			if tt.targetOwn {
				own = w.printerFor(t, plain, juice, "Juice printer")
			}

			// No items, but a printer: it still has to say where it goes.
			if _, err := w.s.RetireStation(ctx, plain, r.kitchen, RetireInput{}); fieldOf(err) != "move_to" {
				t.Fatalf("a printer and no destination: %v", err)
			}
			moved, err := w.s.RetireStation(ctx, plain, r.kitchen, RetireInput{MoveTo: juice})
			if err != nil {
				t.Fatal(err)
			}
			if moved.PrinterMoved != tt.wantMoved || moved.ItemsMoved != 0 {
				t.Errorf("moved = %+v", moved)
			}
			if got := w.printerOf(t, plain, juice); got != tt.wantTargetP(own, from) {
				t.Errorf("juice prints on %d, want %d", got, tt.wantTargetP(own, from))
			}
			if got := w.printerOf(t, plain, r.kitchen); got != 0 {
				t.Errorf("the retired kitchen still names printer %d", got)
			}
		})
	}
}

func TestRetireAStationThatCarriesNothing(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	juice := w.station(t, plain, "juice", "Juice")
	moved, err := w.s.RetireStation(ctx, plain, juice, RetireInput{})
	if err != nil || moved != (StationRetired{}) {
		t.Fatalf("an empty station: %+v, %v; it needs no destination", moved, err)
	}
	// Again: nothing to do.
	if moved, err = w.s.RetireStation(ctx, plain, juice, RetireInput{}); err != nil || moved != (StationRetired{}) {
		t.Errorf("retiring it again: %+v, %v", moved, err)
	}
	if _, err := w.s.RetireStation(ctx, plain, 999, RetireInput{}); !errors.Is(err, ErrStationNotFound) {
		t.Errorf("a missing station: %v", err)
	}
	if _, err := w.s.RetireStation(ctx, plain, w.refs[gst].kitchen, RetireInput{}); !errors.Is(err, ErrStationNotFound) {
		t.Errorf("another venue's station: %v", err)
	}
}

func TestTheLastStationInServiceStays(t *testing.T) {
	w := newWorld(t) // the bar is retired: the kitchen is the only one
	_, err := w.s.RetireStation(context.Background(), plain, w.refs[plain].kitchen, RetireInput{})
	if fieldOf(err) != "id" {
		t.Fatalf("err = %v, want the only station refused", err)
	}
	if st, _ := stationByID(w.editor(t, plain), w.refs[plain].kitchen); !st.Active {
		t.Error("the only station was retired")
	}
}

func TestRestoreAStation(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	before := w.version(t, plain)
	if err := w.s.RestoreStation(ctx, plain, r.bar); err != nil {
		t.Fatal(err)
	}
	if st, _ := stationByID(w.editor(t, plain), r.bar); !st.Active || w.version(t, plain) != before+1 {
		t.Errorf("bar = %+v, version %d (was %d)", st, w.version(t, plain), before)
	}
	m, err := w.d.Menu(ctx, plain)
	if err != nil || len(m.Stations) != 2 {
		t.Errorf("the till's stations: %+v, %v; the bar is back", m.Stations, err)
	}
	if err := w.s.RestoreStation(ctx, plain, 999); !errors.Is(err, ErrStationNotFound) {
		t.Errorf("a missing station: %v", err)
	}
}

// An item cannot name a station that is retired, when it is made or when it
// comes back.
func TestItemsAvoidRetiredStations(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	r := w.refs[plain]
	cat := w.category(t, plain, "Food")
	in := w.item(plain, cat, "Dosa", 12000, "")
	in.StationID = r.bar
	if _, err := w.s.CreateItem(ctx, plain, in); fieldOf(err) != "station_id" {
		t.Fatalf("a new item on a retired station: %v", err)
	}
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	if err := w.s.SetItemActive(ctx, plain, dosa, false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE items SET station_id = ? WHERE id = ?`, r.bar, dosa); err != nil {
		t.Fatal(err)
	}
	if err := w.s.SetItemActive(ctx, plain, dosa, true); fieldOf(err) != "station_id" {
		t.Errorf("bringing back an item on a retired station: %v", err)
	}
}
