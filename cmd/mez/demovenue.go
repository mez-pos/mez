package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
)

// demoPerson is one member of the demo venue's staff. The owner is not here:
// setup's own flags name them.
type demoPerson struct {
	Name string
	Role string
	PIN  string
}

// demoStaff is one person for every role but the owner, so a screen built for
// a role can be signed into as that role. The PINs are fixed and printed:
// they are public, which is why only -demo-menu makes them, and why the
// summary says so. A role added to auth.DefaultRoles needs a person here
// (TestDemoVenueHasEveryRole fails until it has one).
var demoStaff = []demoPerson{
	{Name: "Arjun", Role: "manager", PIN: "2222"},
	{Name: "Meera", Role: "cashier", PIN: "3333"},
	{Name: "Kabir", Role: "waiter", PIN: "5555"},
	{Name: "Ravi", Role: "kitchen", PIN: "4444"},
}

// demoTable is a table of the demo floor, placed on the area's grid.
type demoTable struct {
	Name  string
	Seats int64
	Shape string
	X, Y  int64
}

// demoArea is an area of the demo floor and its tables.
type demoArea struct {
	Name   string
	Tables []demoTable
}

// demoFloor is twelve tables in two areas: a room of eight, mixed in size and
// shape so the floor has something to look at, and a terrace of four. Every
// table stands in its own cell of floor.Columns by floor.Rows
// (TestDemoFloorIsAValidPlan keeps it so).
var demoFloor = []demoArea{
	{Name: "Inside", Tables: []demoTable{
		{"T1", 2, "round", 1, 1}, {"T2", 2, "round", 3, 1}, {"T3", 4, "square", 5, 1}, {"T4", 4, "square", 7, 1},
		{"T5", 4, "square", 1, 4}, {"T6", 6, "long", 4, 4}, {"T7", 6, "long", 8, 4}, {"T8", 8, "long", 4, 6},
	}},
	{Name: "Terrace", Tables: []demoTable{
		{"T9", 2, "round", 2, 2}, {"T10", 2, "round", 5, 2}, {"T11", 4, "square", 2, 5}, {"T12", 4, "square", 5, 5},
	}},
}

// countTables is how many tables a floor holds, for the summary.
func countTables(floor []demoArea) int {
	n := 0
	for _, a := range floor {
		n += len(a.Tables)
	}
	return n
}

// writeDemoVenue adds the demo floor and staff to a venue, inside the
// transaction that creates it, so a venue never exists half demonstrated.
func writeDemoVenue(ctx context.Context, q *gen.Queries, venueID int64, roles map[string]int64, now time.Time) ([]demoPerson, error) {
	for _, p := range demoStaff {
		roleID, ok := roles[p.Role]
		if !ok {
			return nil, fmt.Errorf("the demo staff names a role %q the venue does not have", p.Role)
		}
		hash, err := auth.HashPIN(p.PIN)
		if err != nil {
			return nil, fmt.Errorf("demo PIN for %s: %w", p.Name, err)
		}
		public, err := id.NewPublic("stf")
		if err != nil {
			return nil, err
		}
		if _, err := q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: public, VenueID: venueID, RoleID: roleID, Name: p.Name, PinHash: hash,
			Active: true, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		}); err != nil {
			return nil, fmt.Errorf("demo staff %s: %w", p.Name, err)
		}
	}

	for _, a := range demoFloor {
		areaID, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: venueID, Name: a.Name, CreatedAt: now.UTC(), UpdatedAt: now.UTC()})
		if err != nil {
			return nil, fmt.Errorf("demo area %s: %w", a.Name, err)
		}
		for _, t := range a.Tables {
			if _, err := q.CreateTable(ctx, gen.CreateTableParams{
				VenueID: venueID, AreaID: areaID, Name: t.Name, Seats: t.Seats, Shape: t.Shape, X: t.X, Y: t.Y,
				CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
			}); err != nil {
				return nil, fmt.Errorf("demo table %s: %w", t.Name, err)
			}
		}
	}
	// The floor version is what terminals compare to know the plan moved.
	if err := q.BumpFloorVersion(ctx, venueID); err != nil {
		return nil, fmt.Errorf("the demo floor's version: %w", err)
	}
	return slices.Clone(demoStaff), nil
}
