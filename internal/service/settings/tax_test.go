package settings

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
	"github.com/mez-pos/mez/internal/service/menus"
)

const gstin = "29ABCDE1234F1Z5"

func TestCleanTax(t *testing.T) {
	ca := []tax.Component{{Name: "CA", Rate: 6000}, {Name: "LA", Rate: 2250}}
	tests := []struct {
		name  string
		in    TaxInput
		field string // the refused field; "" when fine
	}{
		{"none", TaxInput{Profile: "none", RegistrationID: "ignored"}, ""},
		{"gst", TaxInput{Profile: "gst_in", RegistrationID: gstin}, ""},
		{"gst, lower case and spaces", TaxInput{Profile: "gst_in", RegistrationID: " 29abcde 1234f1z5 "}, ""},
		{"gst, exclusive", TaxInput{Profile: "gst_in", RegistrationID: gstin, Pricing: "exclusive"}, ""},
		{"gst, no gstin", TaxInput{Profile: "gst_in"}, "registration_id"},
		{"gst, a short gstin", TaxInput{Profile: "gst_in", RegistrationID: "29ABC"}, "registration_id"},
		{"gst, the wrong state", TaxInput{Profile: "gst_in", RegistrationID: gstin, StateCode: "27"}, "state_code"},
		{"gst, not a state", TaxInput{Profile: "gst_in", RegistrationID: gstin, StateCode: "99"}, "state_code"},
		{"gst, pricing is a guess", TaxInput{Profile: "gst_in", RegistrationID: gstin, Pricing: "gross"}, "pricing"},
		// The composition scheme keeps a GSTIN and nothing else (ADR-401).
		{"composition", TaxInput{Profile: "gst_in_composition", RegistrationID: " 29abcde 1234f1z5 "}, ""},
		{"composition, a pricing it ignores", TaxInput{Profile: "gst_in_composition", RegistrationID: gstin, Pricing: "gross"}, ""},
		{"composition, no gstin", TaxInput{Profile: "gst_in_composition"}, "registration_id"},
		{"composition, not a gstin", TaxInput{Profile: "gst_in_composition", RegistrationID: "GB123456789"}, "registration_id"},
		{"vat", TaxInput{Profile: "vat_uk", RegistrationID: "GB123456789"}, ""},
		{"vat, no number", TaxInput{Profile: "vat_uk"}, "registration_id"},
		{"us", TaxInput{Profile: "sales_tax_us", Jurisdiction: ca}, ""},
		{"us, exclusive said out loud", TaxInput{Profile: "sales_tax_us", Pricing: "exclusive", Jurisdiction: ca}, ""},
		{"us, inclusive", TaxInput{Profile: "sales_tax_us", Pricing: "inclusive", Jurisdiction: ca}, "pricing"},
		{"us, no rates", TaxInput{Profile: "sales_tax_us"}, "jurisdiction"},
		{"us, a rate with no name", TaxInput{Profile: "sales_tax_us", Jurisdiction: []tax.Component{{Name: " ", Rate: 1}}}, "jurisdiction"},
		{"us, a rate over 100%", TaxInput{Profile: "sales_tax_us", Jurisdiction: []tax.Component{{Name: "CA", Rate: 100001}}}, "jurisdiction"},
		{"an unknown profile", TaxInput{Profile: "gst"}, "profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := cleanTax(tt.in, "INR")
			var inv *Invalid
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
		})
	}
}

func TestUpdateTaxSwitchesProfileAndClearsWhatItLeavesBehind(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	read := func() TaxSetup {
		t.Helper()
		got, err := s.Tax(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	// GST: the state code comes from the GSTIN when left out.
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: "29abcde1234f1z5", Pricing: "exclusive"}); err != nil {
		t.Fatal(err)
	}
	got := read()
	if got.Profile != "gst_in" || got.RegistrationID != gstin || got.StateCode != "29" || got.Pricing != "exclusive" || !got.NeedsHSN {
		t.Fatalf("gst: %+v", got)
	}

	// The composition scheme: the GSTIN stays; the state code and the
	// pricing go, since nothing is split or added (ADR-401).
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in_composition", RegistrationID: gstin, Pricing: "exclusive", StateCode: "29"}); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Profile != "gst_in_composition" || got.RegistrationID != gstin || got.StateCode != "" || got.Pricing != "" || got.NeedsHSN {
		t.Fatalf("composition: %+v", got)
	}
	if _, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: 1, Key: "tax.state_code"}); err == nil {
		t.Error("the GST state code was kept under the composition scheme")
	}

	// US sales tax: the GST leftovers go, the price is always exclusive.
	us := []tax.Component{{Name: "CA", Rate: 6000}, {Name: "LA", Rate: 2250}}
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "sales_tax_us", Jurisdiction: us}); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Profile != "sales_tax_us" || got.StateCode != "" || got.Pricing != "exclusive" || got.NeedsHSN || len(got.Jurisdiction) != 2 || got.Jurisdiction[1].Rate != 2250 {
		t.Fatalf("us: %+v", got)
	}
	if _, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: 1, Key: "tax.state_code"}); err == nil {
		t.Error("the GST state code was kept under US sales tax")
	}

	// VAT: inclusive by default, the jurisdiction goes.
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "vat_uk", RegistrationID: "GB123456789"}); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Profile != "vat_uk" || got.Pricing != "inclusive" || len(got.Jurisdiction) != 0 || got.RegistrationID != "GB123456789" {
		t.Fatalf("vat: %+v", got)
	}

	// None: nothing is left but the profile.
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "none", RegistrationID: "GB123456789", Pricing: "exclusive"}); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Profile != "none" || got.RegistrationID != "" || got.Pricing != "" {
		t.Fatalf("none: %+v", got)
	}
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range o.Entries {
		if e.Set {
			t.Errorf("%s is still set under no tax", e.Key)
		}
	}
}

func TestUpdateTaxRefusedKeepsEverything(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: gstin}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "vat_uk"}); err == nil {
		t.Fatal("a VAT profile with no number was saved")
	}
	got, err := s.Tax(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "gst_in" || got.RegistrationID != gstin {
		t.Fatalf("a refused change was kept: %+v", got)
	}
	if err := s.UpdateTax(ctx, 99, TaxInput{Profile: "none"}); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

// openTicket puts an order of the given status on the venue.
func openTicket(t *testing.T, d *db.DB, status string) {
	t.Helper()
	_, err := d.Writer().ExecContext(context.Background(),
		`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
		 VALUES (?, 1, 'counter', ?, 1, ?, ?)`, "ord_"+status, status, now, now)
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangingProfileNeedsNoOpenTickets(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	openTicket(t, d, "open")

	err := s.UpdateTax(ctx, 1, TaxInput{Profile: "vat_uk", RegistrationID: "GB123456789"})
	if !errors.Is(err, ErrOpenOrders) {
		t.Fatalf("a profile change with a ticket open: %v", err)
	}
	if got, _ := s.Tax(ctx, 1); got.Profile != "gst_in" {
		t.Fatalf("the refused change was kept: %+v", got)
	}

	// Correcting the GSTIN under the same profile changes no total, and is
	// not held back.
	before, _ := s.Tax(ctx, 1)
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: "29ZZZZZ9999Z1Z9", Pricing: before.Pricing}); err != nil {
		t.Fatalf("a correction with a ticket open: %v", err)
	}
	// Whether prices include tax may change with tickets open: each order
	// carries the taxes it opened under (ADR-374), so nothing open is
	// re-priced.
	other := "exclusive"
	if before.Pricing == "exclusive" {
		other = "inclusive"
	}
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: "29ZZZZZ9999Z1Z9", Pricing: other}); err != nil {
		t.Fatalf("a pricing change with a ticket open: %v", err)
	}
	if got, _ := s.Tax(ctx, 1); got.Pricing != other {
		t.Fatalf("the pricing change was not kept: %+v", got)
	}

	// Closed tickets do not hold it back.
	if _, err := d.Writer().ExecContext(ctx, `UPDATE orders SET status = 'closed', closed_at = ?`, now); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "vat_uk", RegistrationID: "GB123456789"}); err != nil {
		t.Fatalf("after the tickets closed: %v", err)
	}
}

// menuFixture adds a category and a station, and an item for each code given.
func menuFixture(t *testing.T, d *db.DB, classID int64, hsns ...string) []int64 {
	t.Helper()
	ctx := context.Background()
	var ids []int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		cat, err := q.CreateCategory(ctx, gen.CreateCategoryParams{VenueID: 1, Name: "Food", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		st, err := q.CreateStation(ctx, gen.CreateStationParams{VenueID: 1, Code: "kitchen", Name: "Kitchen", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		co, err := q.CreateCourse(ctx, gen.CreateCourseParams{VenueID: 1, Number: 1, Name: "Mains", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		for i, hsn := range hsns {
			id, err := q.CreateItem(ctx, gen.CreateItemParams{
				VenueID: 1, CategoryID: cat, Name: string(rune('A' + i)), Price: 10000, Hsn: hsn,
				TaxClassID: classID, StationID: st, CourseID: co, SortOrder: int64(i), CreatedAt: now, UpdatedAt: now,
			})
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestItemsWithoutHSNAreListedWhileTheProfileNeedsThem(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	class, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "Food", Rate: 5000})
	if err != nil {
		t.Fatal(err)
	}
	menuFixture(t, d, class, "996331", "", "")

	got, err := s.Tax(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NeedsHSN || len(got.ItemsWithoutHSN) != 2 || got.ItemsWithoutHSN[0].Name != "B" || got.ItemsWithoutHSN[1].Name != "C" {
		t.Fatalf("under GST: %+v", got.ItemsWithoutHSN)
	}

	// Moving to a profile that prints no HSN reports none, and refuses nothing.
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "vat_uk", RegistrationID: "GB123456789"}); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Tax(ctx, 1); got.NeedsHSN || len(got.ItemsWithoutHSN) != 0 {
		t.Fatalf("under VAT: %+v", got)
	}

	// Moving back to one that does reports them rather than refusing.
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: gstin}); err != nil {
		t.Fatalf("back to GST with items missing a code: %v", err)
	}
	if got, _ = s.Tax(ctx, 1); len(got.ItemsWithoutHSN) != 2 {
		t.Fatalf("back under GST: %+v", got.ItemsWithoutHSN)
	}
}

func TestTaxClasses(t *testing.T) {
	var told []int64
	s, d := newService(t)
	s.onChange = func(v int64) { told = append(told, v) }
	ctx := context.Background()
	version := func() int64 {
		t.Helper()
		v, err := d.MenuVersion(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	classes := func() []TaxClass {
		t.Helper()
		got, err := s.Tax(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		return got.Classes
	}

	before := version()
	food, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: " Food ", Rate: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if version() != before+1 || !slices.Equal(told, []int64{1}) {
		t.Fatalf("the menu version %d -> %d, tills told %v", before, version(), told)
	}
	zero, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "Bottled water", Exempt: true})
	if err != nil {
		t.Fatal(err)
	}
	got := classes()
	if len(got) != 2 || got[0] != (TaxClass{ID: food, Name: "Food", Rate: 5000, Active: true}) || !got[1].Exempt {
		t.Fatalf("created: %+v", got)
	}

	// A rate change, and a rename.
	if err := s.UpdateTaxClass(ctx, 1, food, TaxClassInput{Name: "Food and drink", Rate: 18000}); err != nil {
		t.Fatal(err)
	}
	if c := classes()[0]; c.Name != "Food and drink" || c.Rate != 18000 {
		t.Fatalf("updated: %+v", c)
	}

	refusals := []struct {
		name  string
		do    func() error
		field string
		is    error
	}{
		{"no name", func() error { _, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: " "}); return err }, "name", nil},
		{"over 100%", func() error { _, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "X", Rate: 100001}); return err }, "rate", nil},
		{"negative", func() error { _, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "X", Rate: -1}); return err }, "rate", nil},
		{"exempt with a rate", func() error {
			_, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "X", Rate: 5000, Exempt: true})
			return err
		}, "rate", nil},
		{"a name taken, in any case", func() error { _, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "FOOD AND DRINK"}); return err }, "name", ErrTaxClassName},
		{"a rename onto another's name", func() error { return s.UpdateTaxClass(ctx, 1, zero, TaxClassInput{Name: "food and drink"}) }, "name", ErrTaxClassName},
		{"no such class", func() error { return s.UpdateTaxClass(ctx, 1, 999, TaxClassInput{Name: "X"}) }, "", ErrTaxClassNotFound},
		{"retiring no such class", func() error { return s.SetTaxClassActive(ctx, 1, 999, false) }, "", ErrTaxClassNotFound},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			v := version()
			err := tt.do()
			var inv *Invalid
			if err == nil || (tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field)) || (tt.is != nil && !errors.Is(err, tt.is)) {
				t.Fatalf("got %v", err)
			}
			if version() != v {
				t.Error("a refused edit bumped the menu version")
			}
		})
	}
	// Saving a class under its own name is not a clash.
	if err := s.UpdateTaxClass(ctx, 1, food, TaxClassInput{Name: "food and drink", Rate: 12000}); err != nil {
		t.Fatalf("an edit under its own name: %v", err)
	}

	// A class on the menu cannot be retired; once the items go, it can.
	items := menuFixture(t, d, food, "996331", "996331")
	var inUse *TaxClassInUse
	if err := s.SetTaxClassActive(ctx, 1, food, false); !errors.As(err, &inUse) || !slices.Equal(inUse.Items, []string{"A", "B"}) {
		t.Fatalf("retiring a class in use: %v", err)
	}
	for _, id := range items {
		if _, err := d.Writer().ExecContext(ctx, `UPDATE items SET active = 0 WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetTaxClassActive(ctx, 1, food, false); err != nil {
		t.Fatal(err)
	}
	if classes()[0].Active {
		t.Error("retired, and still active")
	}
	if err := s.SetTaxClassActive(ctx, 1, food, true); err != nil {
		t.Fatal(err)
	}
	if !classes()[0].Active {
		t.Error("brought back, and still retired")
	}
}

func TestAnItemCannotChargeARetiredClass(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if err := s.UpdateTax(ctx, 1, TaxInput{Profile: "gst_in", RegistrationID: gstin}); err != nil {
		t.Fatal(err)
	}
	food, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "Food", Rate: 5000})
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.CreateTaxClass(ctx, 1, TaxClassInput{Name: "Old rate", Rate: 12000})
	if err != nil {
		t.Fatal(err)
	}
	items := menuFixture(t, d, old, "996331")
	if _, err := d.Writer().ExecContext(ctx, `UPDATE items SET active = 0 WHERE id = ?`, items[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTaxClassActive(ctx, 1, old, false); err != nil {
		t.Fatal(err)
	}

	menu, err := menus.New(menus.Options{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	var inv *menus.Invalid
	if err := menu.SetItemActive(ctx, 1, items[0], true); !errors.As(err, &inv) || inv.Field != "tax_class_id" {
		t.Fatalf("bringing back an item on a retired class: %v", err)
	}
	ed, err := menu.Editor(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	in := menus.ItemInput{
		CategoryID: ed.Categories[0].ID, Name: "Tea", Price: money.Amount{Minor: 5000, Currency: "INR"}, HSN: "996331",
		TaxClassID: old, StationID: ed.Stations[0].ID, CourseID: ed.Courses[0].ID,
	}
	if _, err := menu.CreateItem(ctx, 1, in); !errors.As(err, &inv) || inv.Field != "tax_class_id" {
		t.Fatalf("a new item on a retired class: %v", err)
	}
	if err := s.SetTaxClassActive(ctx, 1, old, true); err != nil {
		t.Fatal(err)
	}
	if err := menu.SetItemActive(ctx, 1, items[0], true); err != nil {
		t.Fatalf("after the class came back: %v", err)
	}
	in.TaxClassID = food
	if _, err := menu.CreateItem(ctx, 1, in); err != nil {
		t.Fatalf("a new item on an active class: %v", err)
	}
}
