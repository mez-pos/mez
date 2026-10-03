package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/byjoellobo/mez/internal/service/settings"
)

// TestVenueSettingsOverHTTP: the owner reads and renames the venue, a bad
// value comes back with its field, a cashier cannot touch it, and any paired
// device may read the venue's name.
func TestVenueSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string, header map[string]string) (int, settingsResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, header)
		if rec.Code >= 400 {
			return rec.Code, settingsResponse{}, errorOf(t, rec)
		}
		var out settingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "/api/v1/settings", "", owner)
	if status != http.StatusOK || out.Venue.Name == "" || out.Venue.Currency == "" || len(out.Settings) == 0 {
		t.Fatalf("read: %d %+v", status, out)
	}
	for _, e := range out.Settings {
		if e.Set {
			t.Errorf("%s is set on a new venue", e.Key)
		}
	}

	status, out, _ = call(http.MethodPut, "/api/v1/settings/venue",
		`{"name":" Blue Door ","legal_name":"Blue Door LLP","timezone":"Europe/London"}`, owner)
	if status != http.StatusOK || out.Venue.Name != "Blue Door" || out.Venue.LegalName != "Blue Door LLP" || out.Venue.Timezone != "Europe/London" {
		t.Fatalf("renamed: %d %+v", status, out.Venue)
	}

	refusals := []struct {
		name, body string
		status     int
		code       ErrorCode
		field      string
	}{
		{"no name", `{"name":"","timezone":"Asia/Kolkata"}`, 422, CodeInvalidSetting, "name"},
		{"not a timezone", `{"name":"A","timezone":"Mars/Olympus"}`, 422, CodeInvalidSetting, "timezone"},
		{"an unknown field", `{"name":"A","timezone":"Asia/Kolkata","currency":"USD"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(http.MethodPut, "/api/v1/settings/venue", tt.body, owner)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	if _, out, _ = call(http.MethodGet, "/api/v1/settings", "", owner); out.Venue.Name != "Blue Door" || out.Venue.Currency == "USD" {
		t.Errorf("a refused change was kept: %+v", out.Venue)
	}

	t.Run("a cashier cannot read or change them", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, req := range [][2]string{{http.MethodGet, "/api/v1/settings"}, {http.MethodPut, "/api/v1/settings/venue"}} {
			if rec := send(w.handler, req[0], req[1], `{"name":"X","timezone":"Asia/Kolkata"}`, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: %d", req[0], req[1], rec.Code)
			}
		}
	})

	t.Run("a device with nobody signed in reads the name and no more", func(t *testing.T) {
		rec := send(w.handler, http.MethodGet, "/api/v1/venue", "", w.asDevice())
		var name venueNameResponse
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &name) != nil || name.Name != "Blue Door" {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		if rec := send(w.handler, http.MethodGet, "/api/v1/settings", "", w.asDevice()); rec.Code != http.StatusUnauthorized {
			t.Errorf("settings with nobody signed in: %d", rec.Code)
		}
		if rec := send(w.handler, http.MethodGet, "/api/v1/venue", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("the venue name with no device: %d", rec.Code)
		}
		if rec := send(w.handler, http.MethodPost, "/api/v1/venue", "", w.asDevice()); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST /venue: %d", rec.Code)
		}
	})
}

// TestTaxSettingsOverHTTP: the owner reads the tax profile, changes it and
// the tax classes, and each refusal comes back with its code and field.
func TestTaxSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string) (int, taxSettingsResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, owner)
		if rec.Code >= 400 {
			return rec.Code, taxSettingsResponse{}, errorOf(t, rec)
		}
		var out taxSettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "/api/v1/settings/tax", "")
	if status != http.StatusOK || out.Profile == "" || out.Classes == nil || out.ItemsWithoutHSN == nil || out.Jurisdiction == nil {
		t.Fatalf("read: %d %+v", status, out)
	}

	status, out, _ = call(http.MethodPut, "/api/v1/settings/tax",
		`{"profile":"gst_in","registration_id":"29abcde1234f1z5","pricing":"inclusive"}`)
	if status != http.StatusOK || out.Profile != "gst_in" || out.RegistrationID != "29ABCDE1234F1Z5" || out.StateCode != "29" || !out.NeedsHSN {
		t.Fatalf("gst: %d %+v", status, out)
	}
	status, out, _ = call(http.MethodPut, "/api/v1/settings/tax",
		`{"profile":"sales_tax_us","jurisdiction":[{"name":"CA","rate":7250}]}`)
	if status != http.StatusOK || out.Pricing != "exclusive" || len(out.Jurisdiction) != 1 || out.Jurisdiction[0].Rate != 7250 || out.StateCode != "" {
		t.Fatalf("us: %d %+v", status, out)
	}

	status, out, _ = call(http.MethodPost, "/api/v1/settings/tax-classes", `{"name":"Zero-rated food","rate":0}`)
	if status != http.StatusCreated || len(out.Classes) == 0 {
		t.Fatalf("created: %d %+v", status, out)
	}
	class := out.Classes[len(out.Classes)-1]
	if class.Name != "Zero-rated food" || !class.Active {
		t.Fatalf("created: %+v", class)
	}
	path := fmt.Sprintf("/api/v1/settings/tax-classes/%d", class.ID)
	_, out, _ = call(http.MethodPut, path, `{"name":"Food","rate":5000}`)
	if c := out.Classes[len(out.Classes)-1]; c.Name != "Food" || c.Rate != 5000 {
		t.Fatalf("updated: %+v", c)
	}
	_, out, _ = call(http.MethodPost, path+"/retire", "")
	if out.Classes[len(out.Classes)-1].Active {
		t.Fatal("retired, and still active")
	}
	_, out, _ = call(http.MethodPost, path+"/restore", "")
	if !out.Classes[len(out.Classes)-1].Active {
		t.Fatal("brought back, and still retired")
	}

	refusals := []struct {
		name, method, path, body string
		status                   int
		code                     ErrorCode
		field                    string
	}{
		{"an unknown profile", http.MethodPut, "/api/v1/settings/tax", `{"profile":"gst"}`, 422, CodeInvalidSetting, "profile"},
		{"gst with no gstin", http.MethodPut, "/api/v1/settings/tax", `{"profile":"gst_in"}`, 422, CodeInvalidSetting, "registration_id"},
		{"a class with no name", http.MethodPost, "/api/v1/settings/tax-classes", `{"name":"","rate":0}`, 422, CodeInvalidSetting, "name"},
		{"a class named twice", http.MethodPost, "/api/v1/settings/tax-classes", `{"name":"food","rate":0}`, 409, CodeTaxClassNameTaken, "name"},
		{"no such class", http.MethodPut, "/api/v1/settings/tax-classes/999", `{"name":"X","rate":0}`, 404, CodeTaxClassNotFound, ""},
		{"retiring no such class", http.MethodPost, "/api/v1/settings/tax-classes/999/retire", "", 404, CodeTaxClassNotFound, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(tt.method, tt.path, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}

	t.Run("a ticket open holds back a change of profile", func(t *testing.T) {
		if _, err := w.db.Writer().ExecContext(context.Background(),
			`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
			 VALUES ('ord_OPENTAX1', 1, 'counter', 'open', 1, ?, ?)`, seededAt, seededAt); err != nil {
			t.Fatal(err)
		}
		status, _, apiErr := call(http.MethodPut, "/api/v1/settings/tax", `{"profile":"none"}`)
		if status != 409 || apiErr.Code != CodeTicketsOpen {
			t.Errorf("%d %s", status, apiErr.Code)
		}
	})

	t.Run("a cashier cannot read or change the tax", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, req := range [][2]string{
			{http.MethodGet, "/api/v1/settings/tax"}, {http.MethodPut, "/api/v1/settings/tax"},
			{http.MethodPost, "/api/v1/settings/tax-classes"}, {http.MethodPost, "/api/v1/settings/tax-classes/1/retire"},
		} {
			if rec := send(w.handler, req[0], req[1], `{"profile":"none"}`, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: %d", req[0], req[1], rec.Code)
			}
		}
	})
}

// TestNumberingSettingsOverHTTP: the owner reads how invoices are numbered,
// changes it, and each refusal comes back with its code and field.
func TestNumberingSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, body string) (int, numberingResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, "/api/v1/settings/numbering", body, owner)
		if rec.Code >= 400 {
			return rec.Code, numberingResponse{}, errorOf(t, rec)
		}
		var out numberingResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", method, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "")
	if status != http.StatusOK || out.InvoiceSeries != "INV" || out.CreditSeries != "CN" || out.Template == "" ||
		out.NextInvoiceNo == "" || out.NextCreditNo == "" || out.Currency == "" || out.MaxRoundingStep < 1 || out.FYLocked {
		t.Fatalf("read: %d %+v", status, out)
	}

	body := `{"invoice_series":"nf","credit_series":"CR","invoice_prefix":"","credit_prefix":"","template":"",` +
		`"financial_year_start_month":1,"cash_rounding_step":1}`
	status, out, _ = call(http.MethodPut, body)
	if status != http.StatusOK || out.InvoiceSeries != "NF" || out.InvoicePrefix != "NF" || out.CreditPrefix != "CR" ||
		out.FinancialYearStartMonth != 1 || out.CashRoundingStep != 1 {
		t.Fatalf("saved: %d %+v", status, out)
	}

	refusals := []struct {
		name, body string
		status     int
		code       ErrorCode
		field      string
	}{
		{"no year in the template", `{"template":"{prefix}-{seq}","financial_year_start_month":1,"cash_rounding_step":1}`, 422, CodeInvalidSetting, "template"},
		{"the same series for both", `{"invoice_series":"A","credit_series":"A","financial_year_start_month":1,"cash_rounding_step":1}`, 422, CodeInvalidSetting, "credit_series"},
		{"a month that is not", `{"financial_year_start_month":13,"cash_rounding_step":1}`, 422, CodeInvalidSetting, "financial_year_start_month"},
		{"an unknown field", `{"financial_year_start_month":1,"cash_rounding_step":1,"currency":"USD"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(http.MethodPut, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}

	t.Run("once a number is issued the year is fixed and a repeated number is refused", func(t *testing.T) {
		ctx := context.Background()
		if _, err := w.db.Writer().ExecContext(ctx,
			`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
			 VALUES ('ord_NUMBER01', 1, 'counter', 'closed', 1, ?, ?)`, seededAt, seededAt); err != nil {
			t.Fatal(err)
		}
		if _, err := w.db.Writer().ExecContext(ctx, `INSERT INTO invoice_series (venue_id, series, fy_start, last_seq) VALUES (1, 'NF', 2026, 1)`); err != nil {
			t.Fatal(err)
		}
		if _, err := w.db.Writer().ExecContext(ctx,
			`INSERT INTO invoices (venue_id, order_id, series, fy_start, seq, invoice_no, issued_at)
			 VALUES (1, 1, 'NF', 2026, 1, 'NF/26/000001', ?)`, seededAt); err != nil {
			t.Fatal(err)
		}
		status, out, _ := call(http.MethodGet, "")
		if !out.FYLocked {
			t.Fatalf("not locked: %d %+v", status, out)
		}
		status, _, apiErr := call(http.MethodPut, `{"invoice_series":"NF","financial_year_start_month":4,"cash_rounding_step":1}`)
		if status != 409 || apiErr.Code != CodeNumberingLocked || apiErr.Field != "financial_year_start_month" {
			t.Errorf("moving the year: %d %s %q", status, apiErr.Code, apiErr.Field)
		}
	})

	t.Run("a cashier cannot read or change it", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			if rec := send(w.handler, method, "/api/v1/settings/numbering", body, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s: %d", method, rec.Code)
			}
		}
	})
}

// TestRulesSettingsOverHTTP: the owner reads how shifts and tips are run,
// changes it, and each refusal comes back with its code and field.
func TestRulesSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, body string) (int, rulesResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, "/api/v1/settings/rules", body, owner)
		if rec.Code >= 400 {
			return rec.Code, rulesResponse{}, errorOf(t, rec)
		}
		var out rulesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", method, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "")
	if status != http.StatusOK || out.DayStart != "04:00" || out.VarianceThreshold != 0 || out.TipRule != "even" ||
		out.TicketLateMinutes != 20 || out.TipRoleWeights == nil || out.Currency != "INR" || out.OpenShiftDay != "" {
		t.Fatalf("read: %d %+v", status, out)
	}
	if !slices.Contains(out.Roles, "owner") || !slices.Contains(out.Roles, "cashier") {
		t.Fatalf("the roles a weight may name: %v", out.Roles)
	}

	body := `{"day_start":"05:30","variance_threshold":5000,"tip_rule":"role_weight",` +
		`"tip_role_weights":{"owner":3,"cashier":1},"ticket_late_minutes":12}`
	status, out, _ = call(http.MethodPut, body)
	if status != http.StatusOK || out.DayStart != "05:30" || out.VarianceThreshold != 5000 || out.TipRule != "role_weight" ||
		out.TipRoleWeights["owner"] != 3 || out.TipRoleWeights["cashier"] != 1 || out.TicketLateMinutes != 12 {
		t.Fatalf("saved: %d %+v", status, out)
	}

	refusals := []struct {
		name, body string
		status     int
		code       ErrorCode
		field      string
	}{
		{"a day start that is not a time", `{"day_start":"dawn","ticket_late_minutes":20}`, 422, CodeInvalidSetting, "day_start"},
		{"a negative threshold", `{"variance_threshold":-1,"ticket_late_minutes":20}`, 422, CodeInvalidSetting, "variance_threshold"},
		{"a tip rule that does not exist", `{"tip_rule":"coin_toss","ticket_late_minutes":20}`, 422, CodeInvalidSetting, "tip_rule"},
		{"a weight for a role that is not here", `{"tip_role_weights":{"barista":2},"ticket_late_minutes":20}`, 422, CodeInvalidSetting, "tip_role_weights"},
		{"role weights with no weights", `{"tip_rule":"role_weight","ticket_late_minutes":20}`, 422, CodeInvalidSetting, "tip_role_weights"},
		{"no minutes", `{}`, 422, CodeInvalidSetting, "ticket_late_minutes"},
		{"an unknown field", `{"ticket_late_minutes":20,"currency":"USD"}`, 400, CodeInvalidRequest, ""},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := call(http.MethodPut, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Errorf("%d %s %q, want %d %s %q", status, apiErr.Code, apiErr.Field, tt.status, tt.code, tt.field)
			}
		})
	}
	if _, out, _ = call(http.MethodGet, ""); out.DayStart != "05:30" || out.TipRule != "role_weight" {
		t.Errorf("a refused update changed the rules: %+v", out)
	}

	t.Run("a cashier cannot read or change them", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			if rec := send(w.handler, method, "/api/v1/settings/rules", body, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s: %d", method, rec.Code)
			}
		}
	})
	t.Run("another verb is not allowed", func(t *testing.T) {
		if rec := send(w.handler, http.MethodDelete, "/api/v1/settings/rules", "", owner); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("DELETE: %d", rec.Code)
		}
	})
}
