package menus

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

var now = time.Date(2026, 9, 30, 6, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

// The fixture's venues and the ids each was given. Venue 1 charges no tax;
// venue 2 is on GST. Each has a kitchen, a retired bar, one course and one
// tax class.
const (
	plain = int64(1)
	gst   = int64(2)
)

type refs struct{ kitchen, bar, course, taxClass int64 }

type world struct {
	s       *Service
	d       *db.DB
	refs    map[int64]refs
	nudged  []int64
	elseCat int64 // a category of venue 2, to name from venue 1
	// photoDir is where the fixture's photo store writes.
	photoDir string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	w := &world{d: d, refs: map[int64]refs{}}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for _, v := range []struct {
			public, profile, reg string
		}{{"ven_MENUS001", "none", ""}, {"ven_MENUS002", "gst_in", "29ABCDE1234F1Z5"}} {
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: v.public, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4,
				TaxProfile: v.profile, TaxRegistrationID: v.reg, CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		if err := q.SetSetting(ctx, gen.SetSettingParams{VenueID: gst, Key: svcorder.SettingTaxStateCode, Value: "29", UpdatedAt: now}); err != nil {
			return err
		}
		for _, venue := range []int64{plain, gst} {
			var r refs
			var err error
			if r.kitchen, err = q.CreateStation(ctx, gen.CreateStationParams{VenueID: venue, Code: "kitchen", Name: "Kitchen", CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			if r.bar, err = q.CreateStation(ctx, gen.CreateStationParams{VenueID: venue, Code: "bar", Name: "Bar", SortOrder: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			if r.course, err = q.CreateCourse(ctx, gen.CreateCourseParams{VenueID: venue, Number: 1, Name: "Mains", CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			if r.taxClass, err = q.CreateTaxClass(ctx, gen.CreateTaxClassParams{VenueID: venue, Name: "Food 5%", Rate: 5000, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			w.refs[venue] = r
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Retired by hand, not through the service: the station tests that retire
	// one must not depend on the code they are testing for their setup.
	if _, err := d.Writer().ExecContext(ctx, `UPDATE stations SET active = 0 WHERE code = 'bar'`); err != nil {
		t.Fatal(err)
	}
	w.photoDir = filepath.Join(t.TempDir(), "photos")
	photos, err := NewPhotoStore(w.photoDir)
	if err != nil {
		t.Fatal(err)
	}
	w.s, err = New(Options{DB: d, Clock: clock{}, OnChange: func(v int64) { w.nudged = append(w.nudged, v) }, Photos: photos})
	if err != nil {
		t.Fatal(err)
	}
	w.elseCat = w.category(t, gst, "Elsewhere")
	w.nudged = nil
	return w
}

func (w *world) category(t *testing.T, venue int64, name string) int64 {
	t.Helper()
	id, err := w.s.CreateCategory(context.Background(), venue, CategoryInput{Name: name})
	if err != nil {
		t.Fatalf("creating category %q: %v", name, err)
	}
	return id
}

func (w *world) item(venue, category int64, name string, price int64, hsn string) ItemInput {
	r := w.refs[venue]
	return ItemInput{
		CategoryID: category, Name: name, Price: money.Amount{Minor: price, Currency: "INR"}, HSN: hsn,
		TaxClassID: r.taxClass, StationID: r.kitchen, CourseID: r.course,
	}
}

func (w *world) create(t *testing.T, venue int64, in ItemInput) int64 {
	t.Helper()
	id, err := w.s.CreateItem(context.Background(), venue, in)
	if err != nil {
		t.Fatalf("creating item %q: %v", in.Name, err)
	}
	return id
}

func (w *world) editor(t *testing.T, venue int64) Editor {
	t.Helper()
	e, err := w.s.Editor(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (w *world) version(t *testing.T, venue int64) int64 {
	t.Helper()
	v, err := w.d.MenuVersion(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func fieldOf(err error) string {
	var invalid *Invalid
	if errors.As(err, &invalid) {
		return invalid.Field
	}
	return ""
}

func TestAnEditBumpsTheVersionAndNudgesTheStream(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)
	cat := w.category(t, plain, "  Coffee ")
	w.create(t, plain, w.item(plain, cat, "Flat white", 18000, ""))

	if got := w.version(t, plain); got != before+2 {
		t.Errorf("version = %d, want %d: two edits, two bumps", got, before+2)
	}
	if !slices.Equal(w.nudged, []int64{plain, plain}) {
		t.Errorf("nudged %v, want venue %d twice", w.nudged, plain)
	}
	e := w.editor(t, plain)
	if e.Version != w.version(t, plain) || e.Currency != "INR" || e.NeedsHSN {
		t.Errorf("editor = version %d, %s, needs HSN %v", e.Version, e.Currency, e.NeedsHSN)
	}
	if len(e.Categories) != 1 || e.Categories[0].Name != "Coffee" || !e.Categories[0].Active {
		t.Errorf("categories = %+v; the name is trimmed", e.Categories)
	}
	if len(e.Items) != 1 || e.Items[0].Price.Minor != 18000 || e.Items[0].ModifierGroups == nil {
		t.Errorf("items = %+v", e.Items)
	}
	if len(e.Stations) != 2 || e.Stations[1].Active {
		t.Errorf("stations = %+v; the retired bar is listed, as retired", e.Stations)
	}
}

func TestARefusedEditWritesNothingAndTellsNobody(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)
	if _, err := w.s.CreateCategory(context.Background(), plain, CategoryInput{Name: "  "}); fieldOf(err) != "name" {
		t.Fatalf("err = %v, want a name refusal", err)
	}
	if _, err := w.s.CreateItem(context.Background(), plain, w.item(plain, 999, "Ghost", 100, "")); fieldOf(err) != "category_id" {
		t.Fatalf("err = %v, want a category refusal", err)
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: a refusal must change nothing", got, before, w.nudged)
	}
}

func TestItemsAreChecked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	retired := w.category(t, plain, "Breakfast")
	if err := w.s.SetCategoryActive(ctx, plain, retired, false); err != nil {
		t.Fatal(err)
	}
	r := w.refs[plain]

	tests := []struct {
		name  string
		edit  func(*ItemInput)
		field string
	}{
		{"no name", func(in *ItemInput) { in.Name = " " }, "name"},
		{"a name too long", func(in *ItemInput) { in.Name = strings.Repeat("a", MaxNameLength+1) }, "name"},
		{"a price below zero", func(in *ItemInput) { in.Price.Minor = -1 }, "price"},
		{"a price in another currency", func(in *ItemInput) { in.Price.Currency = "USD" }, "price"},
		{"a price with no currency", func(in *ItemInput) { in.Price.Currency = "" }, "price"},
		{"a malformed HSN where the profile has none", func(in *ItemInput) { in.HSN = "99633" }, "hsn"},
		{"no category", func(in *ItemInput) { in.CategoryID = 0 }, "category_id"},
		{"another venue's category", func(in *ItemInput) { in.CategoryID = w.elseCat }, "category_id"},
		{"a retired category", func(in *ItemInput) { in.CategoryID = retired }, "category_id"},
		{"no tax class", func(in *ItemInput) { in.TaxClassID = 0 }, "tax_class_id"},
		{"another venue's tax class", func(in *ItemInput) { in.TaxClassID = w.refs[gst].taxClass }, "tax_class_id"},
		{"a retired station", func(in *ItemInput) { in.StationID = r.bar }, "station_id"},
		{"another venue's station", func(in *ItemInput) { in.StationID = w.refs[gst].kitchen }, "station_id"},
		{"no course", func(in *ItemInput) { in.CourseID = 0 }, "course_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := w.item(plain, cat, "Dosa", 12000, "")
			tt.edit(&in)
			_, err := w.s.CreateItem(ctx, plain, in)
			if got := fieldOf(err); got != tt.field {
				t.Errorf("refused field = %q (%v), want %q", got, err, tt.field)
			}
		})
	}

	t.Run("a well-formed HSN is kept where the profile has none", func(t *testing.T) {
		id, err := w.s.CreateItem(ctx, plain, w.item(plain, cat, "Idli", 8000, "996331"))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got := findItem(w.editor(t, plain), id).HSN; got != "996331" {
			t.Errorf("HSN = %q", got)
		}
	})
	t.Run("a free item is fine", func(t *testing.T) {
		if _, err := w.s.CreateItem(ctx, plain, w.item(plain, cat, "Water", 0, "")); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestGSTNeedsAnHSNOnEveryItem(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, gst, "Food")
	if !w.editor(t, gst).NeedsHSN {
		t.Error("a GST venue's editor must say every item needs an HSN")
	}
	tests := []struct {
		hsn   string
		field string
	}{
		{"", "hsn"},
		{"99633", "hsn"},
		{"99A331", "hsn"},
		{"996331", ""},
		{" 9963 ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.hsn, func(t *testing.T) {
			_, err := w.s.CreateItem(ctx, gst, w.item(gst, cat, "Thali", 25000, tt.hsn))
			if got := fieldOf(err); got != tt.field || (tt.field == "" && err != nil) {
				t.Errorf("err = %v, want field %q", err, tt.field)
			}
		})
	}
	e := w.editor(t, gst)
	if last := e.Items[len(e.Items)-1]; last.HSN != "9963" {
		t.Errorf("stored HSN = %q, want it trimmed", last.HSN)
	}
}

func TestUpdatingAnItem(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	coffee := w.category(t, plain, "Coffee")
	tea := w.category(t, plain, "Tea")
	w.create(t, plain, w.item(plain, tea, "Masala chai", 6000, ""))
	latte := w.create(t, plain, w.item(plain, coffee, "Latte", 18000, ""))
	mocha := w.create(t, plain, w.item(plain, coffee, "Mocha", 20000, ""))

	in := w.item(plain, coffee, "Caffe latte", 19000, "")
	if err := w.s.UpdateItem(ctx, plain, latte, in); err != nil {
		t.Fatal(err)
	}
	e := w.editor(t, plain)
	got := itemsIn(e, coffee)
	if !slices.Equal(got, []string{"Caffe latte", "Mocha"}) {
		t.Errorf("coffee = %v; an edit keeps the item's place", got)
	}
	if p := findItem(e, latte).Price.Minor; p != 19000 {
		t.Errorf("price = %d, want 19000", p)
	}

	// Moved to tea, it goes to the end of tea.
	in.CategoryID = tea
	if err := w.s.UpdateItem(ctx, plain, latte, in); err != nil {
		t.Fatal(err)
	}
	if got := itemsIn(w.editor(t, plain), tea); !slices.Equal(got, []string{"Masala chai", "Caffe latte"}) {
		t.Errorf("tea = %v", got)
	}

	t.Run("an item of another venue is not found", func(t *testing.T) {
		other := w.create(t, gst, w.item(gst, w.elseCat, "Thali", 25000, "996331"))
		err := w.s.UpdateItem(ctx, plain, other, w.item(plain, coffee, "Stolen", 1, ""))
		if !errors.Is(err, ErrItemNotFound) {
			t.Errorf("err = %v, want not found", err)
		}
	})
	t.Run("an update is checked like a create", func(t *testing.T) {
		bad := w.item(plain, coffee, "Mocha", -5, "")
		if err := w.s.UpdateItem(ctx, plain, mocha, bad); fieldOf(err) != "price" {
			t.Errorf("err = %v, want a price refusal", err)
		}
	})
}

// The till reads the menu through db.DB.Menu: retiring is what takes an
// item off it, and a retired category takes its items with it.
func TestRetiringTakesItOffTheTill(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	food := w.category(t, plain, "Food")
	drinks := w.category(t, plain, "Drinks")
	dosa := w.create(t, plain, w.item(plain, food, "Dosa", 12000, ""))
	w.create(t, plain, w.item(plain, food, "Idli", 8000, ""))
	w.create(t, plain, w.item(plain, drinks, "Lassi", 9000, ""))

	onTill := func() []string {
		m, err := w.d.Menu(ctx, plain)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, it := range m.Items {
			names = append(names, it.Name)
		}
		return names
	}

	steps := []struct {
		name string
		do   func() error
		want []string
	}{
		{"retire an item", func() error { return w.s.SetItemActive(ctx, plain, dosa, false) }, []string{"Idli", "Lassi"}},
		{"retire its category", func() error { return w.s.SetCategoryActive(ctx, plain, food, false) }, []string{"Lassi"}},
		{"bring the item back, category still retired", func() error { return w.s.SetItemActive(ctx, plain, dosa, true) }, []string{"Lassi"}},
		{"bring the category back", func() error { return w.s.SetCategoryActive(ctx, plain, food, true) }, []string{"Dosa", "Idli", "Lassi"}},
	}
	for _, st := range steps {
		if err := st.do(); err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got := onTill(); !slices.Equal(got, st.want) {
			t.Errorf("after %s the till sells %v, want %v", st.name, got, st.want)
		}
	}
	if len(w.editor(t, plain).Items) != 3 {
		t.Error("the editor lists every item, retired or not")
	}

	if err := w.s.SetItemActive(ctx, plain, 999, false); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("retiring a missing item: %v", err)
	}
	if err := w.s.SetCategoryActive(ctx, plain, w.elseCat, false); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("retiring another venue's category: %v", err)
	}
	if err := w.s.RenameCategory(ctx, plain, w.elseCat, CategoryInput{Name: "Mine"}); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("renaming another venue's category: %v", err)
	}
}

func TestOrdering(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	a := w.category(t, plain, "A")
	b := w.category(t, plain, "B")
	c := w.category(t, plain, "C")
	x := w.create(t, plain, w.item(plain, a, "X", 100, ""))
	y := w.create(t, plain, w.item(plain, a, "Y", 100, ""))
	z := w.create(t, plain, w.item(plain, a, "Z", 100, ""))
	if err := w.s.SetCategoryActive(ctx, plain, b, false); err != nil {
		t.Fatal(err)
	}

	if err := w.s.OrderCategories(ctx, plain, []int64{c, a, b}); err != nil {
		t.Fatal(err)
	}
	if got := categoryNames(w.editor(t, plain)); !slices.Equal(got, []string{"C", "A", "B"}) {
		t.Errorf("categories = %v", got)
	}
	if err := w.s.OrderItems(ctx, plain, a, []int64{z, x, y}); err != nil {
		t.Fatal(err)
	}
	if got := itemsIn(w.editor(t, plain), a); !slices.Equal(got, []string{"Z", "X", "Y"}) {
		t.Errorf("items = %v", got)
	}

	stale := []struct {
		name string
		ids  []int64
	}{
		{"one missing", []int64{c, a}},
		{"one twice", []int64{c, a, a}},
		{"one twice, another missing", []int64{c, c, a}},
		{"another venue's", []int64{c, a, w.elseCat}},
		{"none", nil},
	}
	for _, tt := range stale {
		t.Run("categories: "+tt.name, func(t *testing.T) {
			before := w.version(t, plain)
			if err := w.s.OrderCategories(ctx, plain, tt.ids); !errors.Is(err, ErrStaleOrder) {
				t.Errorf("err = %v, want stale", err)
			}
			if w.version(t, plain) != before {
				t.Error("a stale order must change nothing")
			}
		})
	}
	t.Run("items: one missing", func(t *testing.T) {
		if err := w.s.OrderItems(ctx, plain, a, []int64{x, y}); !errors.Is(err, ErrStaleOrder) {
			t.Errorf("err = %v, want stale", err)
		}
	})
	t.Run("items: another category's", func(t *testing.T) {
		if err := w.s.OrderItems(ctx, plain, c, []int64{x}); !errors.Is(err, ErrStaleOrder) {
			t.Errorf("err = %v, want stale", err)
		}
	})
	t.Run("items: a category that is not the venue's", func(t *testing.T) {
		if err := w.s.OrderItems(ctx, plain, w.elseCat, nil); !errors.Is(err, ErrCategoryNotFound) {
			t.Errorf("err = %v, want category not found", err)
		}
	})
	t.Run("an empty category orders as empty", func(t *testing.T) {
		if err := w.s.OrderItems(ctx, plain, c, []int64{}); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestNewNeedsADatabase(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("New with no database must fail")
	}
}

func itemsIn(e Editor, category int64) []string {
	var out []string
	for _, it := range e.Items {
		if it.CategoryID == category {
			out = append(out, it.Name)
		}
	}
	return out
}

func findItem(e Editor, id int64) Item {
	for _, it := range e.Items {
		if it.ID == id {
			return it
		}
	}
	return Item{}
}

func categoryNames(e Editor) []string {
	out := make([]string, len(e.Categories))
	for i, c := range e.Categories {
		out[i] = c.Name
	}
	return out
}

// Sold out lasts until the next trading day starts where the venue is:
// the fixture's clock is 12:00 in Kolkata, so 04:00 IST tomorrow, which
// is 22:30 UTC today.
func TestSoldOutForToday(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	want := time.Date(2026, 9, 30, 22, 30, 0, 0, time.UTC)

	before := w.version(t, plain)
	until, err := w.s.SetItemSoldOut(ctx, plain, dosa, true)
	if err != nil {
		t.Fatal(err)
	}
	if until == nil || !until.Equal(want) {
		t.Fatalf("until = %v, want %s", until, want)
	}
	if w.version(t, plain) != before+1 || len(w.nudged) == 0 {
		t.Error("marking sold out is a menu edit: it bumps the version and nudges the stream")
	}

	m, err := w.d.Menu(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 1 || m.Items[0].SoldOutUntil == nil || !m.Items[0].SoldOutUntil.Equal(want) {
		t.Errorf("the till's dosa = %+v; it stays on the menu, marked", m.Items)
	}
	if got := findItem(w.editor(t, plain), dosa).SoldOutUntil; got == nil || !got.Equal(want) {
		t.Errorf("the editor's dosa sold out until %v", got)
	}

	back, err := w.s.SetItemSoldOut(ctx, plain, dosa, false)
	if err != nil || back != nil {
		t.Fatalf("on sale again = %v, %v", back, err)
	}
	if findItem(w.editor(t, plain), dosa).SoldOutUntil != nil {
		t.Error("back on sale, it has no sold-out time")
	}

	if _, err := w.s.SetItemSoldOut(ctx, plain, 999, true); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("a missing item: %v", err)
	}
	other := w.create(t, gst, w.item(gst, w.elseCat, "Thali", 25000, "996331"))
	if _, err := w.s.SetItemSoldOut(ctx, plain, other, true); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("another venue's item: %v", err)
	}
}

// Sold out lasts until the venue's own trading day starts: at 05:30 IST the
// fixture's 12:00 sells out until 05:30 tomorrow, which is 00:00 UTC.
func TestSoldOutLastsUntilTheVenuesDayStart(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	set := func(value string) {
		t.Helper()
		err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
			return q.SetSetting(ctx, gen.SetSettingParams{VenueID: plain, Key: svcshift.SettingDayStart, Value: value, UpdatedAt: time.Date(2026, 9, 30, 6, 30, 0, 0, time.UTC)})
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	set("05:30")
	until, err := w.s.SetItemSoldOut(ctx, plain, dosa, true)
	if want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); err != nil || until == nil || !until.Equal(want) {
		t.Fatalf("until = %v, %v; want %s", until, err, want)
	}

	set("dawn")
	if _, err := w.s.SetItemSoldOut(ctx, plain, dosa, true); !errors.Is(err, svcshift.ErrVenueConfig) {
		t.Errorf("a day start that is not a time: err = %v, want ErrVenueConfig", err)
	}
	if _, err := w.s.SetItemSoldOut(ctx, plain, dosa, false); err != nil {
		t.Errorf("putting it back on sale needs no day start: %v", err)
	}
}
