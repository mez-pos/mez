package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
)

// at is a fixed instant. The clock is injected everywhere else in this project
// and a test that uses the real one is a test that cannot assert what it wrote.
var at = time.Date(2026, 9, 18, 4, 5, 6, 0, time.UTC)

func newVenueParams() gen.CreateVenueParams {
	return gen.CreateVenueParams{
		PublicID:                "ven_7K3M9QX2",
		Name:                    "Filter Coffee House",
		LegalName:               "Filter Coffee House Pvt Ltd",
		Currency:                "INR",
		Timezone:                "Asia/Kolkata",
		FinancialYearStartMonth: 4,
		TaxProfile:              "gst_in",
		TaxRegistrationID:       "29ABCDE1234F1Z5",
		CashRoundingStepMinor:   50,
		CreatedAt:               at,
		UpdatedAt:               at,
	}
}

// TestVenueRoundTrip is the integration test the phase asks for: a temp file
// database, migrated up, a venue written and read back. No mocks. The thing
// being tested is the whole path, and a mock of SQLite would only prove that
// the mock agrees with the test.
func TestVenueRoundTrip(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	want := newVenueParams()

	var created gen.Venue
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		created, err = q.CreateVenue(ctx, want)
		return err
	})
	if err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}

	if created.ID == 0 {
		t.Error("the new venue has no id")
	}

	// Read it back through the read pool, which is a different connection and
	// a different handle from the one that wrote it.
	got, err := d.Queries().VenueByPublicID(ctx, want.PublicID)
	if err != nil {
		t.Fatalf("VenueByPublicID: %v", err)
	}

	if got != created {
		t.Errorf("read back %+v, want %+v", got, created)
	}

	checks := []struct {
		field string
		got   any
		want  any
	}{
		{field: "name", got: got.Name, want: want.Name},
		{field: "currency", got: got.Currency, want: want.Currency},
		{field: "timezone", got: got.Timezone, want: want.Timezone},
		{field: "tax_profile", got: got.TaxProfile, want: want.TaxProfile},
		{field: "financial_year_start_month", got: got.FinancialYearStartMonth, want: want.FinancialYearStartMonth},
		{field: "cash_rounding_step_minor", got: got.CashRoundingStepMinor, want: want.CashRoundingStepMinor},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}

// TestTimestampsSurviveTheRoundTrip is why the timestamp columns are declared
// TIMESTAMP rather than TEXT. The declared type is what makes the driver hand
// back a time.Time; a TEXT column would not scan into one at all, and every
// caller would be parsing strings. ADR-043.
func TestTimestampsSurviveTheRoundTrip(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateVenue(ctx, newVenueParams())
		return err
	}); err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}

	got, err := d.Queries().VenueByPublicID(ctx, "ven_7K3M9QX2")
	if err != nil {
		t.Fatalf("VenueByPublicID: %v", err)
	}

	if !got.CreatedAt.Equal(at) {
		t.Errorf("created_at = %s, want %s", got.CreatedAt, at)
	}

	// The instant is right and the offset is zero, but the zone is a fixed
	// zone with no name rather than time.UTC itself: the driver parses
	// "+00:00" and Go builds a nameless zone for it. That is why anything
	// rendering one of these to JSON calls .UTC() first, or it writes
	// "+00:00" where the rest of the API writes "Z".
	if _, offset := got.CreatedAt.Zone(); offset != 0 {
		t.Errorf("created_at came back at offset %d, want 0", offset)
	}
	if rendered := got.CreatedAt.UTC().Format(time.RFC3339); rendered != "2026-09-18T04:05:06Z" {
		t.Errorf("rendered as %q after .UTC(), want the Z form", rendered)
	}

	// And what actually landed in the column is text SQLite's own date
	// functions understand, not a number of seconds.
	//
	// The CAST is load-bearing. Selecting the column itself gives back a
	// time.Time, which database/sql then formats into a string on the way into
	// this variable, so the value read would be Go's rendering rather than the
	// stored bytes. Casting strips the declared type and with it the
	// conversion.
	var stored string
	if err := d.Reader().QueryRowContext(ctx,
		"SELECT CAST(created_at AS TEXT) FROM venues WHERE public_id = ?", "ven_7K3M9QX2").Scan(&stored); err != nil {
		t.Fatalf("reading the raw column: %v", err)
	}
	if stored != "2026-09-18 04:05:06+00:00" {
		t.Errorf("stored as %q, want SQLite's own UTC form", stored)
	}
}

func TestVenueUpdateAndList(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	var venue gen.Venue
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		venue, err = q.CreateVenue(ctx, newVenueParams())
		return err
	}); err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}

	later := at.Add(time.Hour)
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID:                      venue.ID,
			Name:                    "Filter Coffee House, Indiranagar",
			LegalName:               venue.LegalName,
			Timezone:                venue.Timezone,
			FinancialYearStartMonth: venue.FinancialYearStartMonth,
			TaxProfile:              venue.TaxProfile,
			TaxRegistrationID:       venue.TaxRegistrationID,
			CashRoundingStepMinor:   1,
			UpdatedAt:               later,
		})
		return err
	}); err != nil {
		t.Fatalf("UpdateVenue: %v", err)
	}

	got, err := d.Queries().VenueByID(ctx, venue.ID)
	if err != nil {
		t.Fatalf("VenueByID: %v", err)
	}
	if got.Name != "Filter Coffee House, Indiranagar" {
		t.Errorf("name = %q, want the updated one", got.Name)
	}
	if got.CashRoundingStepMinor != 1 {
		t.Errorf("cash_rounding_step_minor = %d, want 1", got.CashRoundingStepMinor)
	}
	if !got.UpdatedAt.Equal(later) {
		t.Errorf("updated_at = %s, want %s", got.UpdatedAt, later)
	}
	if !got.CreatedAt.Equal(at) {
		t.Errorf("created_at = %s, want it unchanged at %s", got.CreatedAt, at)
	}

	venues, err := d.Queries().ListVenues(ctx)
	if err != nil {
		t.Fatalf("ListVenues: %v", err)
	}
	if len(venues) != 1 {
		t.Fatalf("ListVenues returned %d venues, want 1", len(venues))
	}
}

// TestListReturnsAnEmptySliceNotNil is the emit_empty_slices setting, as a
// test. The difference reaches a terminal as `[]` rather than `null`, and a
// client that has to handle both is a client with a bug in it.
func TestListReturnsAnEmptySliceNotNil(t *testing.T) {
	d := migratedDB(t)

	venues, err := d.Queries().ListVenues(context.Background())
	if err != nil {
		t.Fatalf("ListVenues: %v", err)
	}
	if venues == nil {
		t.Error("ListVenues returned nil for an empty table, which encodes as null")
	}
	if len(venues) != 0 {
		t.Errorf("ListVenues returned %d venues on an empty database", len(venues))
	}
}

func TestSettingsUpsert(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	var venue gen.Venue
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		venue, err = q.CreateVenue(ctx, newVenueParams())
		return err
	}); err != nil {
		t.Fatalf("CreateVenue: %v", err)
	}

	set := func(key, value string, when time.Time) {
		t.Helper()
		if err := d.WriteTx(ctx, func(q *gen.Queries) error {
			return q.SetSetting(ctx, gen.SetSettingParams{
				VenueID:   venue.ID,
				Key:       key,
				Value:     value,
				UpdatedAt: when,
			})
		}); err != nil {
			t.Fatalf("SetSetting(%q): %v", key, err)
		}
	}

	set("receipt.footer", "Thank you", at)
	set("tip.rule", "pooled", at)

	// The same key again must update rather than fail or duplicate.
	later := at.Add(time.Hour)
	set("receipt.footer", "See you soon", later)

	got, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: venue.ID, Key: "receipt.footer"})
	if err != nil {
		t.Fatalf("Setting: %v", err)
	}
	if got.Value != "See you soon" {
		t.Errorf("value = %q, want the updated one", got.Value)
	}
	if !got.UpdatedAt.Equal(later) {
		t.Errorf("updated_at = %s, want %s", got.UpdatedAt, later)
	}

	all, err := d.Queries().SettingsForVenue(ctx, venue.ID)
	if err != nil {
		t.Fatalf("SettingsForVenue: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("%d settings, want 2: the upsert inserted instead of updating", len(all))
	}

	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.DeleteSetting(ctx, gen.DeleteSettingParams{VenueID: venue.ID, Key: "tip.rule"})
	}); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}

	if _, err := d.Queries().Setting(ctx, gen.SettingParams{VenueID: venue.ID, Key: "tip.rule"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("reading a deleted setting: error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestStaffAndRoles(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	var venue gen.Venue
	var role gen.Role
	var member gen.Staff

	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		var err error
		if venue, err = q.CreateVenue(ctx, newVenueParams()); err != nil {
			return err
		}
		if role, err = q.CreateRole(ctx, gen.CreateRoleParams{
			VenueID:     venue.ID,
			Name:        "cashier",
			Permissions: `["order.create","payment.take"]`,
			CreatedAt:   at,
		}); err != nil {
			return err
		}
		member, err = q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID:  "stf_A1B2C3D4",
			VenueID:   venue.ID,
			RoleID:    role.ID,
			Name:      "Asha",
			PinHash:   "argon2id$fake",
			Active:    true,
			CreatedAt: at,
			UpdatedAt: at,
		})
		return err
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// Active is a bool in Go because the column is declared BOOLEAN. As an
	// INTEGER it would be an int64 here, and every caller would compare it
	// against 1.
	if !member.Active {
		t.Error("the new staff member is not active")
	}

	got, err := d.Queries().StaffByPublicID(ctx, "stf_A1B2C3D4")
	if err != nil {
		t.Fatalf("StaffByPublicID: %v", err)
	}
	if got != member {
		t.Errorf("read back %+v, want %+v", got, member)
	}

	active, err := d.Queries().ActiveStaffForVenue(ctx, venue.ID)
	if err != nil {
		t.Fatalf("ActiveStaffForVenue: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("%d active staff, want 1", len(active))
	}

	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.DeactivateStaff(ctx, gen.DeactivateStaffParams{ID: member.ID, UpdatedAt: at.Add(time.Hour)})
	}); err != nil {
		t.Fatalf("DeactivateStaff: %v", err)
	}

	active, err = d.Queries().ActiveStaffForVenue(ctx, venue.ID)
	if err != nil {
		t.Fatalf("ActiveStaffForVenue: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("%d active staff after deactivating the only one, want 0", len(active))
	}

	// Deactivated, not deleted: the record is still there, because their name
	// is on every order they took.
	if _, err := d.Queries().StaffByPublicID(ctx, "stf_A1B2C3D4"); err != nil {
		t.Errorf("a deactivated staff member could not be read back: %v", err)
	}
}

// TestWriteTxRollsBack checks that the generated queries inherit the
// transaction rather than quietly running on their own connection. If they did
// not, the first statement would already be committed by the time the second
// failed.
func TestWriteTxRollsBack(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()

	sentinel := errors.New("the second step failed")

	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.CreateVenue(ctx, newVenueParams()); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WriteTx error = %v, want it to wrap %v", err, sentinel)
	}

	venues, err := d.Queries().ListVenues(ctx)
	if err != nil {
		t.Fatalf("ListVenues: %v", err)
	}
	if len(venues) != 0 {
		t.Errorf("%d venues after a rolled back transaction, want 0", len(venues))
	}
}

// TestTheReadPoolRefusesGeneratedWrites pins the guard from ADR-037 against
// the generated code specifically: a write reaching the read pool fails in
// SQLite, not in review.
func TestTheReadPoolRefusesGeneratedWrites(t *testing.T) {
	d := migratedDB(t)

	if _, err := d.Queries().CreateVenue(context.Background(), newVenueParams()); err == nil {
		t.Error("the read pool accepted a generated write")
	}
}
