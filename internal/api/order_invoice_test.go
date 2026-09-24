package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestInvoiceOverHTTP: the settling payment issues the invoice, the preview
// reads it back from its snapshot, and a reprint needs a manager's PIN and
// never takes a new number.
func TestInvoiceOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	path := "/api/v1/orders/" + id
	added := r.ok(t, http.MethodPost, path+"/lines",
		`{"item_id":1,"name":"Masala dosa","qty":1,"unit_price":{"minor":12000,"currency":"INR"},"station":"kitchen","tax_rate":500,"hsn":"996331"}`)
	paid := r.ok(t, http.MethodPost, path+"/payments", fmt.Sprintf(`{"method":"cash","amount":{"minor":%d,"currency":"INR"}}`,
		added.Order.Totals.BalanceDue.Minor))
	if len(paid.Order.Invoices) != 1 {
		t.Fatalf("no invoice with the settling payment: %+v", paid.Order.Invoices)
	}
	seq := paid.Order.Invoices[0].Seq

	rec := send(r.handler, http.MethodGet, fmt.Sprintf("%s/invoices/%d", path, seq), "", r.asStaff(r.staffToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("read: %d %s", rec.Code, rec.Body)
	}
	var inv issuedInvoiceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceNo != "INV/26-27/000001" || inv.Prices == nil || len(inv.Prices.Lines) != 1 ||
		inv.Prices.Total != paid.Order.Totals.Total || inv.Tax == nil || len(inv.Tenders) != 1 || inv.TimeZone != "Asia/Kolkata" {
		t.Errorf("invoice = %+v", inv)
	}

	reprint := fmt.Sprintf("%s/invoices/%d/reprint", path, seq)
	status, _, apiErr := r.call(t, http.MethodPost, reprint, `{"authorised_by":"stf_PRIYA001","pin":"0000","reason":"guest_copy"}`)
	if apiErr.Code != CodeInvalidPIN {
		t.Errorf("a wrong PIN: %d %s", status, apiErr.Code)
	}
	res := r.ok(t, http.MethodPost, reprint, `{`+r.override()+`,"reason":"guest_copy"}`)
	if len(res.Events) != 1 || res.Events[0].Type != "invoice.reprinted" || res.Order.Invoices[0].Reprints != 1 ||
		res.Order.Invoices[0].InvoiceNo != "INV/26-27/000001" {
		t.Errorf("reprint = %+v, invoices %+v", res.Events, res.Order.Invoices)
	}

	status, _, apiErr = r.call(t, http.MethodGet, fmt.Sprintf("%s/invoices/%d", path, seq-1), "")
	if status != http.StatusNotFound || apiErr.Code != CodeInvoiceNotFound {
		t.Errorf("a seq that is not an invoice: %d %s", status, apiErr.Code)
	}
	status, _, apiErr = r.call(t, http.MethodPost, fmt.Sprintf("%s/invoices/%d/reprint", path, seq-1), `{`+r.override()+`,"reason":"x"}`)
	if status != http.StatusNotFound || apiErr.Code != CodeInvoiceNotFound {
		t.Errorf("a reprint of nothing: %d %s", status, apiErr.Code)
	}
}
