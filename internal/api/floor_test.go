package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/table"
	"github.com/byjoellobo/nofiltr-pos/internal/service/floor"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// TestFloorOverHTTP: the owner lays out areas and tables, moves one, and
// retires and restores; refusals come back with their field; a cashier may
// not touch the floor plan.
func TestFloorOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := floor.New(floor.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithFloor(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string) (int, floorResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, owner)
		if rec.Code >= 400 {
			return rec.Code, floorResponse{}, errorOf(t, rec)
		}
		var out floorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "/api/v1/floor", "")
	if status != http.StatusOK || out.Grid != (gridResponse{Columns: floor.Columns, Rows: floor.Rows}) || len(out.Areas) != 0 || out.Tables == nil {
		t.Fatalf("empty: %d %+v", status, out)
	}
	status, out, _ = call(http.MethodPost, "/api/v1/floor/areas", `{"name":"Inside"}`)
	if status != http.StatusCreated || len(out.Areas) != 1 || out.Areas[0].Name != "Inside" || !out.Areas[0].Active {
		t.Fatalf("area: %d %+v", status, out)
	}
	area := out.Areas[0].ID
	status, out, _ = call(http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"T1","seats":4,"shape":"round","x":2,"y":1}`, area))
	if status != http.StatusCreated || len(out.Tables) != 1 {
		t.Fatalf("table: %d %+v", status, out)
	}
	if tb := out.Tables[0]; tb.Name != "T1" || tb.Seats != 4 || tb.Shape != floor.Round || tb.X != 2 || tb.Y != 1 || tb.AreaID != area || !tb.Active {
		t.Errorf("table = %+v", tb)
	}
	table := out.Tables[0].ID

	_, out, _ = call(http.MethodPut, fmt.Sprintf("/api/v1/floor/tables/%d", table), fmt.Sprintf(`{"area_id":%d,"name":"T1","seats":4,"shape":"round","x":5,"y":6}`, area))
	if tb := out.Tables[0]; tb.X != 5 || tb.Y != 6 {
		t.Errorf("moved: %+v", tb)
	}
	_, out, _ = call(http.MethodPut, fmt.Sprintf("/api/v1/floor/areas/%d", area), `{"name":"Inside bar"}`)
	if out.Areas[0].Name != "Inside bar" {
		t.Errorf("renamed: %+v", out.Areas)
	}
	call(http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"T2","seats":2,"shape":"square","x":0,"y":0}`, area))

	refusals := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"no name", http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"","seats":2,"shape":"square","x":1,"y":1}`, area), 422, CodeInvalidFloor, "name"},
		{"off the grid", http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"T3","seats":2,"shape":"square","x":12,"y":1}`, area), 422, CodeInvalidFloor, "x"},
		{"a cell taken", http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"T3","seats":2,"shape":"square","x":5,"y":6}`, area), 409, CodeCellTaken, "x"},
		{"a name taken", http.MethodPost, "/api/v1/floor/tables", fmt.Sprintf(`{"area_id":%d,"name":"T2","seats":2,"shape":"square","x":1,"y":1}`, area), 409, CodeFloorNameTaken, "name"},
		{"no such area for a table", http.MethodPost, "/api/v1/floor/tables", `{"area_id":999,"name":"T3","seats":2,"shape":"square","x":1,"y":1}`, 422, CodeAreaNotFound, "area_id"},
		{"an area name taken", http.MethodPost, "/api/v1/floor/areas", `{"name":"Inside bar"}`, 409, CodeFloorNameTaken, "name"},
		{"no such table", http.MethodPut, "/api/v1/floor/tables/999", fmt.Sprintf(`{"area_id":%d,"name":"T9","seats":2,"shape":"square","x":1,"y":1}`, area), 404, CodeTableNotFound, ""},
		{"no such area", http.MethodPost, "/api/v1/floor/areas/999/retire", "", 404, CodeAreaNotFound, ""},
		{"an area with tables", http.MethodPost, fmt.Sprintf("/api/v1/floor/areas/%d/retire", area), "", 409, CodeAreaInUse, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(tt.method, tt.path, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}

	// Retire both tables and the area; a table cannot come back into it.
	_, out, _ = call(http.MethodGet, "/api/v1/floor", "")
	if len(out.Tables) != 2 {
		t.Fatalf("tables = %+v", out.Tables)
	}
	for _, tb := range out.Tables {
		_, out, _ = call(http.MethodPost, fmt.Sprintf("/api/v1/floor/tables/%d/retire", tb.ID), "")
	}
	for _, tb := range out.Tables {
		if tb.Active {
			t.Errorf("not retired: %+v", tb)
		}
	}
	_, out, _ = call(http.MethodPost, fmt.Sprintf("/api/v1/floor/areas/%d/retire", area), "")
	if out.Areas[0].Active {
		t.Error("area not retired")
	}
	if status, _, apiErr := call(http.MethodPost, fmt.Sprintf("/api/v1/floor/tables/%d/restore", table), ""); status != 409 || apiErr.Code != CodeAreaRetired {
		t.Errorf("restored into a retired area: %d %s", status, apiErr.Code)
	}
	call(http.MethodPost, fmt.Sprintf("/api/v1/floor/areas/%d/restore", area), "")
	if _, out, _ = call(http.MethodPost, fmt.Sprintf("/api/v1/floor/tables/%d/restore", table), ""); !out.Areas[0].Active {
		t.Error("area not restored")
	}

	if rec := send(w.handler, http.MethodDelete, "/api/v1/floor", "", owner); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /floor: %d", rec.Code)
	}
	// A cashier cannot change, or read, the floor plan here.
	cashier := w.asStaff(w.signIn(t, "2468").Token)
	if rec := send(w.handler, http.MethodGet, "/api/v1/floor", "", cashier); rec.Code != http.StatusForbidden {
		t.Errorf("a cashier read the floor plan: %d", rec.Code)
	}
}

// TestLiveFloorOverHTTP: a cashier reads the floor in service and clears a
// table, but not an occupied one; the back office's plan stays the owner's.
func TestLiveFloorOverHTTP(t *testing.T) {
	w := newTill(t)
	ctx := context.Background()
	orders, err := svcorder.New(svcorder.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := floor.New(floor.Options{DB: w.db, Orders: orders})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithFloor(svc), WithOrders(orders))
	area, err := svc.CreateArea(ctx, 1, floor.AreaSettings{Name: "Inside"})
	if err != nil {
		t.Fatal(err)
	}
	t4, err := svc.CreateTable(ctx, 1, floor.TableSettings{AreaID: area, Name: "T4", Seats: 4, Shape: floor.Round})
	if err != nil {
		t.Fatal(err)
	}
	t5, err := svc.CreateTable(ctx, 1, floor.TableSettings{AreaID: area, Name: "T5", Seats: 2, Shape: floor.Square, X: 1})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := orders.Open(ctx, svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t4, Covers: 3})
	if err != nil {
		t.Fatal(err)
	}

	cashier := w.asStaff(w.signIn(t, "2468").Token)
	live := func(method, path string) (int, liveFloorResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, "", cashier)
		if rec.Code >= 400 {
			return rec.Code, liveFloorResponse{}, errorOf(t, rec)
		}
		var out liveFloorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := live(http.MethodGet, "/api/v1/floor/live")
	if status != http.StatusOK || len(out.Areas) != 1 || len(out.Tables) != 2 || out.Grid.Columns != floor.Columns {
		t.Fatalf("live: %d %+v", status, out)
	}
	seated, free := out.Tables[0], out.Tables[1]
	if seated.ID != t4 || seated.Status != table.StatusSeated || seated.Covers != 3 || seated.Since == nil ||
		seated.Balance == nil || seated.Balance.Minor != 0 || len(seated.Orders) != 1 || seated.Orders[0] != opened.PublicID {
		t.Errorf("T4 = %+v", seated)
	}
	if free.ID != t5 || free.Status != table.StatusFree || free.Since != nil || free.Balance != nil || free.Orders == nil {
		t.Errorf("T5 = %+v", free)
	}

	if status, out, _ := live(http.MethodPost, fmt.Sprintf("/api/v1/floor/tables/%d/clear", t5)); status != http.StatusOK || len(out.Tables) != 2 {
		t.Errorf("clearing a free table: %d", status)
	}
	if status, _, apiErr := live(http.MethodPost, fmt.Sprintf("/api/v1/floor/tables/%d/clear", t4)); status != http.StatusConflict || apiErr.Code != CodeTableOccupied {
		t.Errorf("clearing T4: %d %s", status, apiErr.Code)
	}
	if status, _, apiErr := live(http.MethodPost, "/api/v1/floor/tables/999/clear"); status != http.StatusNotFound || apiErr.Code != CodeTableNotFound {
		t.Errorf("clearing no table: %d %s", status, apiErr.Code)
	}
	if rec := send(w.handler, http.MethodGet, "/api/v1/floor", "", cashier); rec.Code != http.StatusForbidden {
		t.Errorf("a cashier read the plan: %d", rec.Code)
	}
}
