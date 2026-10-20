package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/device/printq"
	"github.com/mez-pos/mez/internal/domain/printjob"
	"github.com/mez-pos/mez/internal/service/printers"
)

// TestPrintersOverHTTP: the owner adds, routes, tests and retires
// printers; a cashier may not; refusals come back with their field.
func TestPrintersOverHTTP(t *testing.T) {
	w := newTill(t)
	ctx := context.Background()
	if err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateStation(ctx, gen.CreateStationParams{VenueID: 1, Code: "kitchen", Name: "Kitchen", CreatedAt: seededAt, UpdatedAt: seededAt})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	q, err := printq.New(printq.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := printers.New(w.db, q, nil, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithPrinters(svc), WithPrints(q))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string) (int, printersResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, owner)
		if rec.Code >= 400 {
			return rec.Code, printersResponse{}, errorOf(t, rec)
		}
		var out printersResponse
		if rec.Code != http.StatusAccepted {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
			}
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "/api/v1/printers", "")
	if status != http.StatusOK || len(out.Printers) != 0 || len(out.Stations) != 1 || out.Stations[0].PrinterID != nil {
		t.Fatalf("empty: %d %+v", status, out)
	}
	status, out, _ = call(http.MethodPost, "/api/v1/printers", `{"name":"Counter","transport":"tcp","address":"10.0.0.5","paper_mm":58}`)
	if status != http.StatusCreated || len(out.Printers) != 1 || out.Printers[0].Name != "Counter" || out.Printers[0].PaperMM != 58 || !out.Printers[0].Active {
		t.Fatalf("created: %d %+v", status, out)
	}
	id := out.Printers[0].ID

	_, out, _ = call(http.MethodPut, "/api/v1/printers/receipt", fmt.Sprintf(`{"printer_id":%d}`, id))
	if !out.Printers[0].IsReceipt {
		t.Error("not the receipt printer")
	}
	_, out, _ = call(http.MethodPut, "/api/v1/stations/kitchen/printer", fmt.Sprintf(`{"printer_id":%d}`, id))
	if out.Stations[0].PrinterID == nil || *out.Stations[0].PrinterID != id || len(out.Printers[0].Stations) != 1 || out.Printers[0].Stations[0] != "kitchen" {
		t.Errorf("station: %+v", out)
	}

	rec := send(w.handler, http.MethodPost, fmt.Sprintf("/api/v1/printers/%d/test", id), "", owner)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("test page: %d %s", rec.Code, rec.Body)
	}
	_, out, _ = call(http.MethodGet, "/api/v1/printers", "")
	if lj := out.Printers[0].LastJob; lj == nil || lj.State != printjob.Queued || lj.Label != "Test page, Counter" {
		t.Errorf("last job = %+v", lj)
	}

	_, out, _ = call(http.MethodPut, fmt.Sprintf("/api/v1/printers/%d", id), `{"name":"Front","transport":"windows","address":"EPSON TM-T82","paper_mm":80}`)
	if p := out.Printers[0]; p.Name != "Front" || p.Transport != printers.USB || p.Address != "EPSON TM-T82" {
		t.Errorf("updated: %+v", p)
	}
	_, out, _ = call(http.MethodPost, fmt.Sprintf("/api/v1/printers/%d/retire", id), "")
	if p := out.Printers[0]; p.Active || p.IsReceipt {
		t.Errorf("retired: %+v", p)
	}
	_, out, _ = call(http.MethodPost, fmt.Sprintf("/api/v1/printers/%d/restore", id), "")
	if !out.Printers[0].Active {
		t.Error("not brought back")
	}
	_, out, _ = call(http.MethodPut, "/api/v1/printers/receipt", `{"printer_id":null}`)
	if out.Printers[0].IsReceipt {
		t.Error("receipt printer not cleared")
	}

	refusals := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"no name", http.MethodPost, "/api/v1/printers", `{"name":"","transport":"tcp","address":"10.0.0.5","paper_mm":80}`, 422, CodeInvalidPrinter, "name"},
		{"a URL", http.MethodPost, "/api/v1/printers", `{"name":"X","transport":"tcp","address":"http://x","paper_mm":80}`, 422, CodeInvalidPrinter, "address"},
		{"a hostname", http.MethodPost, "/api/v1/printers", `{"name":"X","transport":"tcp","address":"printer.example.com","paper_mm":80}`, 422, CodeInvalidPrinter, "address"},
		{"a public address", http.MethodPost, "/api/v1/printers", `{"name":"X","transport":"tcp","address":"203.0.113.9:9100","paper_mm":80}`, 422, CodeInvalidPrinter, "address"},
		{"a printer shared from another computer", http.MethodPost, "/api/v1/printers", `{"name":"X","transport":"windows","address":"\\\\FRONTPC\\EPSON","paper_mm":80}`, 422, CodeInvalidPrinter, "address"},
		{"a name taken", http.MethodPost, "/api/v1/printers", `{"name":"Front","transport":"tcp","address":"10.0.0.6","paper_mm":80}`, 409, CodePrinterNameTaken, "name"},
		{"no such printer", http.MethodPut, "/api/v1/printers/999", `{"name":"X","transport":"tcp","address":"10.0.0.6","paper_mm":80}`, 404, CodePrinterNotFound, ""},
		{"no such station", http.MethodPut, "/api/v1/stations/pastry/printer", fmt.Sprintf(`{"printer_id":%d}`, id), 404, CodeStationNotFound, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(tt.method, tt.path, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	call(http.MethodPost, fmt.Sprintf("/api/v1/printers/%d/retire", id), "")
	if status, _, apiErr := call(http.MethodPost, fmt.Sprintf("/api/v1/printers/%d/test", id), ""); status != 409 || apiErr.Code != CodePrinterRetired {
		t.Errorf("a test page on a retired printer: %d %s", status, apiErr.Code)
	}

	// A cashier cannot change the printers.
	cashier := w.asStaff(w.signIn(t, "2468").Token)
	if rec := send(w.handler, http.MethodGet, "/api/v1/printers", "", cashier); rec.Code != http.StatusForbidden {
		t.Errorf("a cashier read the printer settings: %d", rec.Code)
	}
}

// TestSendErrorIsNotShown: a printer that does not answer is "Could not
// reach the printer" on the printers list and the queue. What the dial
// said, which names the address and how the network answered, stays in the
// server's log (ADR-259).
func TestSendErrorIsNotShown(t *testing.T) {
	w := newTill(t)
	ctx := context.Background()
	dialErr := errors.New("escpos: connecting to the printer at 10.0.0.7:9100: dial tcp 10.0.0.7:9100: connect: connection refused")
	refuse := printq.TransportFunc(func(context.Context, string, []byte) error { return dialErr })
	q, err := printq.New(printq.Options{DB: w.db, Poll: 10 * time.Millisecond, Transports: map[string]printq.Transport{printq.TCP: refuse}})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := printers.New(w.db, q, nil, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithPrinters(svc), WithPrints(q))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	rec := send(w.handler, http.MethodPost, "/api/v1/printers", `{"name":"Counter","transport":"tcp","address":"10.0.0.7","paper_mm":80}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w.handler, http.MethodPost, "/api/v1/printers/1/test", "", owner); rec.Code != http.StatusAccepted {
		t.Fatalf("test page: %d %s", rec.Code, rec.Body)
	}
	if err := q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	var jobs printJobsResponse
	for deadline := time.Now().Add(5 * time.Second); ; {
		rec := send(w.handler, http.MethodGet, "/api/v1/print-jobs", "", owner)
		if err := json.Unmarshal(rec.Body.Bytes(), &jobs); err != nil {
			t.Fatalf("print jobs: %v: %s", err, rec.Body)
		}
		if len(jobs.Jobs) == 1 && jobs.Jobs[0].State == printjob.Failed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the send was never tried: %+v", jobs)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := jobs.Jobs[0].LastError; got != printq.Unreachable {
		t.Errorf("print-jobs last_error = %q, want %q", got, printq.Unreachable)
	}

	rec = send(w.handler, http.MethodGet, "/api/v1/printers", "", owner)
	var list printersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if lj := list.Printers[0].LastJob; lj == nil || lj.Error != printq.Unreachable {
		t.Errorf("printers last_job = %+v, want error %q", lj, printq.Unreachable)
	}
	if body := rec.Body.String(); strings.Contains(body, "connection refused") || strings.Contains(body, "dial tcp") {
		t.Errorf("the dial error reached the client: %s", body)
	}
}
