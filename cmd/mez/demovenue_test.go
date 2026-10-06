package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/service/floor"
)

// TestDemoVenueHasEveryRole: a screen built for a role is tried by signing in
// as that role, so a role added to auth.DefaultRoles must arrive with a
// person in the demo. Task 1a adds the waiter, and this fails until the demo
// has one.
func TestDemoVenueHasEveryRole(t *testing.T) {
	have := map[string]bool{}
	names := map[string]bool{}
	for _, p := range demoStaff {
		if have[p.Role] {
			t.Errorf("two demo people are %s", p.Role)
		}
		have[p.Role] = true
		if names[p.Name] {
			t.Errorf("two demo people are called %s", p.Name)
		}
		names[p.Name] = true
		if p.PIN != "" {
			t.Errorf("%s has a fixed PIN %q: demo PINs are drawn at setup, never written in the code", p.Name, p.PIN)
		}
	}
	for _, r := range auth.DefaultRoles() {
		if r.Name == "owner" {
			continue // setup's own flags name the owner
		}
		if !have[r.Name] {
			t.Errorf("the demo venue has no %s", r.Name)
		}
	}
}

// TestDemoFloorIsAValidPlan: the floor service refuses a table off the grid,
// on another's cell or with a shape it does not draw. The demo is written
// straight to the tables, so this is what keeps it a floor the back office
// could have made.
func TestDemoFloorIsAValidPlan(t *testing.T) {
	if got := countTables(demoFloor); got != 12 || len(demoFloor) != 2 {
		t.Fatalf("%d tables in %d areas, want 12 in 2", got, len(demoFloor))
	}
	shapes := floor.Shapes()
	names := map[string]bool{}
	for _, a := range demoFloor {
		cells := map[[2]int64]string{}
		for _, tb := range a.Tables {
			if names[tb.Name] {
				t.Errorf("two tables are called %s", tb.Name)
			}
			names[tb.Name] = true
			if tb.X < 0 || tb.X >= floor.Columns || tb.Y < 0 || tb.Y >= floor.Rows {
				t.Errorf("%s at %d,%d is off the %dx%d grid", tb.Name, tb.X, tb.Y, floor.Columns, floor.Rows)
			}
			if tb.Seats < 1 || tb.Seats > 40 {
				t.Errorf("%s seats %d", tb.Name, tb.Seats)
			}
			if !slices.Contains(shapes, floor.Shape(tb.Shape)) {
				t.Errorf("%s is %q, not a shape", tb.Name, tb.Shape)
			}
			cell := [2]int64{tb.X, tb.Y}
			if other, taken := cells[cell]; taken {
				t.Errorf("%s and %s share a cell in %s", other, tb.Name, a.Name)
			}
			cells[cell] = tb.Name
		}
	}
}

// TestSetupDemoVenue: -demo-menu leaves the floor and the people a screen is
// tried with, each of whom signs in with the PIN the summary printed.
func TestSetupDemoVenue(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t, "-demo-menu")
	out := runSetupIn(t, args)
	d := opened(t, dir)

	if got := field(t, out, "Demo floor"); got != "12" {
		t.Errorf("summary says %s tables, want 12", got)
	}
	if !strings.Contains(out, "PINs drawn now") {
		t.Errorf("the summary does not say the demo PINs were drawn:\n%s", out)
	}
	for _, fixed := range []string{"2222", "3333", "4444", "5555"} {
		if strings.Contains(out, "PIN "+fixed) {
			t.Errorf("the summary shows the old fixed PIN %s:\n%s", fixed, out)
		}
	}

	svc, err := floor.New(floor.Options{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.Plan(ctx, 1)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(p.Areas) != 2 || len(p.Tables) != 12 {
		t.Fatalf("%d areas, %d tables, want 2 and 12", len(p.Areas), len(p.Tables))
	}
	// Read before the loop below: saving a table moves the version on too.
	if v, err := d.Queries().FloorVersion(ctx, 1); err != nil || v != 1 {
		t.Errorf("floor version = %d, %v: setup writes the floor once, and terminals compare the version to know the plan moved", v, err)
	}
	// The floor's own validation accepts every table as it stands.
	for _, tb := range p.Tables {
		in := floor.TableSettings{AreaID: tb.AreaID, Name: tb.Name, Seats: tb.Seats, Shape: floor.Shape(tb.Shape), X: tb.X, Y: tb.Y}
		if err := svc.UpdateTable(ctx, 1, tb.ID, in); err != nil {
			t.Errorf("table %s does not pass the floor service: %v", tb.Name, err)
		}
	}

	// Each role has its person, and each person's printed PIN works.
	people, err := d.Queries().StaffForVenue(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: d})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := devices.Redeem(ctx, field(t, out, "Pairing code"))
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, row := range people {
		roles[row.Name] = row.RoleName
		if row.Name == "Priya" {
			continue
		}
		pin := printedPIN(t, out, row.Name)
		if _, err := staff.SignIn(ctx, paired.Device, row.PublicID, pin); err != nil {
			t.Errorf("%s cannot sign in with the printed PIN %s: %v", row.Name, pin, err)
		}
	}
	for _, p := range demoStaff {
		if roles[p.Name] != p.Role {
			t.Errorf("%s is %q, want %q", p.Name, roles[p.Name], p.Role)
		}
	}
	if len(people) != len(demoStaff)+1 {
		t.Errorf("%d people, want the owner and %d more", len(people), len(demoStaff))
	}
}

// TestSetupWithoutDemoHasNoFloorOrStaff: a real venue starts with its owner
// alone and an empty floor.
func TestSetupWithoutDemoHasNoFloorOrStaff(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t)
	out := runSetupIn(t, args)
	// Not "Demo" alone: the summary names the database, in a temp directory
	// that carries this test's name.
	if strings.Contains(out, "Demo menu") || strings.Contains(out, "Demo floor") || strings.Contains(out, "Demo staff") {
		t.Errorf("a plain setup mentions a demo:\n%s", out)
	}
	d := opened(t, dir)
	people, err := d.Queries().StaffForVenue(ctx, 1)
	if err != nil || len(people) != 1 {
		t.Errorf("%d people (%v), want the owner alone", len(people), err)
	}
	if tables, err := d.Queries().TablesForVenue(ctx, 1); err != nil || len(tables) != 0 {
		t.Errorf("%d tables (%v), want none", len(tables), err)
	}
}

// printedPIN reads a demo person's PIN out of the summary:
// "    Arjun     manager  PIN 2222".
func printedPIN(t *testing.T, out, name string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 4 && f[0] == name && f[2] == "PIN" {
			return f[3]
		}
	}
	t.Fatalf("no PIN for %s in the summary:\n%s", name, out)
	return ""
}

// TestDemoVenueRefusesARoleTheVenueLacks: the demo names roles, and a venue
// without one is an error, not a person with no role.
func TestDemoVenueRefusesARoleTheVenueLacks(t *testing.T) {
	// The role check comes before any write, so no queries are needed.
	if _, err := writeDemoVenue(context.Background(), nil, 1, map[string]int64{}, time.Now()); err == nil ||
		!strings.Contains(err.Error(), "role") {
		t.Errorf("err = %v, want one naming the missing role", err)
	}
}
