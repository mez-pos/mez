package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/service/settings"
)

// TestReceiptPreviewOverHTTP: the screen's preview answers with a sample
// receipt laid out line by line at the paper's width, saving nothing.
func TestReceiptPreviewOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	const path = "/api/v1/settings/receipt/preview"

	rec := send(w.handler, http.MethodPost, path, `{"header":["Unit 4, MG Road"],"footer":["Thanks"],"hide_barcode":true,"paper_mm":58}`, owner)
	var out receiptPreviewResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Columns != 32 || len(out.Lines) < 10 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body)
	}
	var text strings.Builder
	for _, l := range out.Lines {
		text.WriteString(l.Text + "\n")
		if l.Barcode {
			t.Errorf("a barcode is in the preview though it is off: %+v", l)
		}
	}
	if !strings.Contains(text.String(), "Unit 4, MG Road") || !strings.Contains(text.String(), "Thanks") || !strings.Contains(text.String(), "Cafe") {
		t.Errorf("preview text:\n%s", text.String())
	}

	rec = send(w.handler, http.MethodGet, "/api/v1/settings/receipt", "", owner)
	var saved receiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || len(saved.Header) != 0 || saved.HideBarcode {
		t.Fatalf("a preview saved something: %d %s", rec.Code, rec.Body)
	}

	refusals := []struct {
		name, body string
		status     int
		code       ErrorCode
		field      string
	}{
		{"too many lines", `{"header":["1","2","3","4","5"],"paper_mm":80}`, 422, CodeInvalidSetting, "header"},
		{"text the printer cannot print", `{"footer":["谢"],"paper_mm":80}`, 422, CodeInvalidSetting, "footer"},
		{"paper that is no width", `{"paper_mm":70}`, 422, CodeInvalidSetting, "paper_mm"},
		{"an unknown field", `{"paper_mm":80,"logo":"x"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(w.handler, http.MethodPost, path, tt.body, owner)
			if apiErr := errorOf(t, rec); rec.Code != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", rec.Code, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	if rec = send(w.handler, http.MethodGet, path, "", owner); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET on the preview: %d", rec.Code)
	}
	cashier := w.asStaff(w.signIn(t, "2468").Token)
	if rec = send(w.handler, http.MethodPost, path, `{"paper_mm":80}`, cashier); rec.Code != http.StatusForbidden {
		t.Errorf("a cashier: %d", rec.Code)
	}
}
