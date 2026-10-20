package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/mez-pos/mez/internal/device/printq"
	"github.com/mez-pos/mez/internal/domain/printjob"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// TestPrintQueueOverHTTP: a paid bill's receipt, with no receipt printer set
// up, is on the queue, dead, with the reason; a retry with still no printer
// says so; once one is set, the retry is taken; cancel needs order.edit.
func TestPrintQueueOverHTTP(t *testing.T) {
	r := newRegister(t)
	q, err := printq.New(printq.Options{DB: r.db})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := svcorder.New(svcorder.Options{DB: r.db, Prints: q})
	if err != nil {
		t.Fatal(err)
	}
	r.rebuild(t, WithOrders(orders), WithPrints(q))

	id := r.openOrder(t)
	path := "/api/v1/orders/" + id
	added := r.ok(t, http.MethodPost, path+"/lines",
		`{"item_id":1,"name":"Filter coffee","qty":1,"unit_price":{"minor":4000,"currency":"INR"},"station":"bar","tax_rate":5000,"hsn":"996331"}`)
	r.ok(t, http.MethodPost, path+"/payments", fmt.Sprintf(`{"method":"cash","amount":{"minor":%d,"currency":"INR"}}`, added.Order.Totals.BalanceDue.Minor))

	list := func() []printJobResponse {
		t.Helper()
		rec := send(r.handler, http.MethodGet, "/api/v1/print-jobs", "", r.asStaff(r.staffToken))
		if rec.Code != http.StatusOK {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		var out printJobsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Jobs
	}
	jobs := list()
	if len(jobs) != 1 || jobs[0].Kind != printjob.Receipt || jobs[0].State != printjob.Dead ||
		jobs[0].LastError != printq.NoPrinter || jobs[0].Label != "Receipt INV/26-27/000001" || jobs[0].MaxAttempts != printjob.MaxAttempts {
		t.Fatalf("jobs = %+v", jobs)
	}
	job := fmt.Sprintf("/api/v1/print-jobs/%d", jobs[0].ID)

	status, _, apiErr := r.call(t, http.MethodPost, job+"/retry", "")
	if status != http.StatusConflict || apiErr.Code != CodeNoPrinter {
		t.Errorf("a retry with no printer: %d %s", status, apiErr.Code)
	}
	ctx := context.Background()
	if _, err := r.db.Writer().ExecContext(ctx, `INSERT INTO printers (venue_id, name, transport, address, is_receipt, created_at, updated_at)
		VALUES (1, 'Counter', 'tcp', '127.0.0.1:1', 1, '2026-09-18 04:30:00+00:00', '2026-09-18 04:30:00+00:00')`); err != nil {
		t.Fatal(err)
	}
	if rec := send(r.handler, http.MethodPost, job+"/retry", "", r.asStaff(r.staffToken)); rec.Code != http.StatusNoContent {
		t.Fatalf("retry: %d %s", rec.Code, rec.Body)
	}
	if jobs := list(); jobs[0].State != printjob.Queued || jobs[0].Printer != "Counter" {
		t.Errorf("after the retry: %+v", jobs[0])
	}
	if rec := send(r.handler, http.MethodPost, job+"/cancel", "", r.asStaff(r.staffToken)); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body)
	}
	if jobs := list(); jobs[0].State != printjob.Dead || jobs[0].LastError != "cancelled by Asha" {
		t.Errorf("after the cancel: %+v", jobs[0])
	}

	status, _, apiErr = r.call(t, http.MethodPost, job+"/cancel", "")
	if status != http.StatusConflict || apiErr.Code != CodePrintJobState {
		t.Errorf("a second cancel: %d %s", status, apiErr.Code)
	}
	status, _, apiErr = r.call(t, http.MethodPost, "/api/v1/print-jobs/999/retry", "")
	if status != http.StatusNotFound || apiErr.Code != CodePrintJobNotFound {
		t.Errorf("no such job: %d %s", status, apiErr.Code)
	}
	if rec := send(r.handler, http.MethodGet, "/api/v1/print-jobs", "", r.asDevice()); rec.Code != http.StatusUnauthorized {
		t.Errorf("the queue without a signed-in person: %d", rec.Code)
	}
}
