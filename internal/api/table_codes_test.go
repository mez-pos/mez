package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// codesWorld is orderingWorld with the table codes mounted for the back
// office, at a venue that says where guests reach it.
func newCodesWorld(t *testing.T, publicURL string) *orderingWorld {
	t.Helper()
	o := newOrderingWorld(t)
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	o.rebuild(t, WithMenus(o.db), WithGuests(o.codes, "/order/"), WithGuestOrders(o.orders), WithTableCodes(o.codes, publicURL),
		WithStatic(map[string]fs.FS{"/order/": app}))
	return o
}

func decodeCodes(t *testing.T, rec *httptest.ResponseRecorder) tableCodesResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the codes may be cached, and they are credentials")
	}
	var got tableCodesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got
}

func codeOf(t *testing.T, list tableCodesResponse, tableID int64) tableCodeResponse {
	t.Helper()
	for _, c := range list.Tables {
		if c.TableID == tableID {
			return c
		}
	}
	t.Fatalf("table %d is not in %+v", tableID, list)
	return tableCodeResponse{}
}

// TestTheOwnerSeesRenewsAndTurnsOffCodes: every table with its code and the
// address its sticker says; a renew that stops the old code at once; and a
// table turned off, which no guest can order from until it has a code again.
func TestTheOwnerSeesRenewsAndTurnsOffCodes(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example/")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)

	list := decodeCodes(t, send(o.handler, http.MethodGet, "/api/v1/table-codes", "", owner))
	if list.Address != "https://cafe.example/order/t/" || len(list.Tables) == 0 {
		t.Fatalf("codes = %+v", list)
	}
	four := codeOf(t, list, 4)
	if four.Code != o.code || four.URL != "https://cafe.example/order/t/"+o.code || four.TableName == "" {
		t.Errorf("table 4 = %+v, want the code guests use, %s", four, o.code)
	}

	// Renewed: a new code in the answer, and the old one leads nowhere.
	renewed := codeOf(t, decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/tables/4/code/renew", "", owner)), 4)
	if renewed.Code == o.code || renewed.Code == "" || !strings.HasSuffix(renewed.URL, renewed.Code) {
		t.Fatalf("renewed = %+v", renewed)
	}
	if status, _, body := o.guest(http.MethodGet, "/order/api/t/"+o.code); status != http.StatusNotFound {
		t.Errorf("the old code still leads somewhere: %d %s", status, body)
	}
	if status, _, body := o.guest(http.MethodGet, "/order/api/t/"+renewed.Code); status != http.StatusOK {
		t.Errorf("the new code: %d %s", status, body)
	}

	// Turned off: no code, no address, and the code just made leads nowhere.
	off := codeOf(t, decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/tables/4/code/revoke", "", owner)), 4)
	if off.Code != "" || off.URL != "" {
		t.Errorf("turned off = %+v", off)
	}
	if status, _, _ := o.guest(http.MethodGet, "/order/api/t/"+renewed.Code); status != http.StatusNotFound {
		t.Errorf("a code turned off still leads somewhere: %d", status)
	}
	if rec := send(o.handler, http.MethodGet, "/api/v1/tables/4/code/qr", "", owner); rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeTableNotFound {
		t.Errorf("the QR of a table with no code: %d %s", rec.Code, rec.Body)
	}

	// Making the missing ones gives table 4 a code again and leaves the rest.
	five := codeOf(t, list, 5)
	made := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/table-codes", "", owner))
	if again := codeOf(t, made, 4); again.Code == "" || again.Code == renewed.Code || again.Code == o.code {
		t.Errorf("table 4 after making the missing codes = %+v", again)
	}
	if kept := codeOf(t, made, 5); kept.Code != five.Code {
		t.Errorf("table 5's code changed from %s to %s", five.Code, kept.Code)
	}
}

// TestATablesCodeIsDrawnAsItsPublicAddress: the picture is an SVG and nothing
// else, and it is the venue's own address whatever the request claims.
func TestATablesCodeIsDrawnAsItsPublicAddress(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
	forged := map[string]string{"X-Forwarded-Host": "evil.example", "Origin": "https://evil.example"}
	for k, v := range owner {
		forged[k] = v
	}
	rec := send(o.handler, http.MethodGet, "/api/v1/tables/4/code/qr", "", forged)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("the QR: %d %s", rec.Code, rec.Body)
	}
	var got tableCodeQRResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://cafe.example/order/t/"+o.code || got.TableID != 4 || got.TableName == "" {
		t.Errorf("the QR is of %+v, want the venue's own address", got)
	}
	if !strings.HasPrefix(got.SVG, "<svg") || strings.Contains(got.SVG, "<script") || strings.Contains(rec.Body.String(), "evil.example") {
		t.Errorf("the picture is not a plain SVG of the venue's address: %.80s", got.SVG)
	}
	// Another table's picture is another picture: the code is in it.
	other := send(o.handler, http.MethodGet, "/api/v1/tables/5/code/qr", "", owner)
	var five tableCodeQRResponse
	if err := json.Unmarshal(other.Body.Bytes(), &five); err != nil || other.Code != http.StatusOK || five.SVG == got.SVG || five.SVG == "" {
		t.Errorf("table 5's picture: %d, %v, the same as table 4's: %v", other.Code, err, five.SVG == got.SVG)
	}

	t.Run("with no public address there is nothing to print", func(t *testing.T) {
		o := newCodesWorld(t, "")
		owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
		list := decodeCodes(t, send(o.handler, http.MethodGet, "/api/v1/table-codes", "", owner))
		if four := codeOf(t, list, 4); list.Address != "" || four.URL != "" || four.Code == "" {
			t.Errorf("codes = %+v", list)
		}
		rec := send(o.handler, http.MethodGet, "/api/v1/tables/4/code/qr", "", owner)
		if rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNoPublicURL {
			t.Errorf("the QR: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestTableCodesAreForWhoeverRunsTheVenue: a guest's code is a credential.
// A cashier does not read them, nobody without a session does, and a table
// that is not on the floor has none.
func TestTableCodesAreForWhoeverRunsTheVenue(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
	cashier := o.asStaff(o.signIn(t, "2468").Token)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/table-codes"},
		{http.MethodPost, "/api/v1/table-codes"},
		{http.MethodPost, "/api/v1/tables/4/code/renew"},
		{http.MethodPost, "/api/v1/tables/4/code/revoke"},
		{http.MethodGet, "/api/v1/tables/4/code/qr"},
		{http.MethodGet, "/api/v1/table-codes/sheet"},
		{http.MethodPost, "/api/v1/pickup-code/renew"},
		{http.MethodPost, "/api/v1/pickup-code/revoke"},
		{http.MethodGet, "/api/v1/pickup-code/qr"},
	}
	for _, rt := range routes {
		if rec := send(o.handler, rt.method, rt.path, "", cashier); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as a cashier: %d %s", rt.method, rt.path, rec.Code, rec.Body)
		}
		if rec := send(o.handler, rt.method, rt.path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with no device: %d", rt.method, rt.path, rec.Code)
		}
	}
	// Nothing a cashier tried changed anything.
	if status, _, _ := o.guest(http.MethodGet, "/order/api/t/"+o.code); status != http.StatusOK {
		t.Errorf("table 4's code after a cashier tried to renew it: %d", status)
	}

	for _, path := range []string{"/api/v1/tables/999/code/renew", "/api/v1/tables/0/code/revoke", "/api/v1/tables/abc/code/renew"} {
		if rec := send(o.handler, http.MethodPost, path, "", owner); rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeTableNotFound {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := send(o.handler, http.MethodDelete, "/api/v1/table-codes", "", owner); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE on the codes: %d", rec.Code)
	}
}

// TestTheSheetIsEveryTableWithACode: one answer with each table's address
// drawn, the venue's own, and no table that has no code to stick on it.
func TestTheSheetIsEveryTableWithACode(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
	list := decodeCodes(t, send(o.handler, http.MethodGet, "/api/v1/table-codes", "", owner))
	decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/tables/5/code/revoke", "", owner))

	rec := send(o.handler, http.MethodGet, "/api/v1/table-codes/sheet", "", owner)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("the sheet: %d %s", rec.Code, rec.Body)
	}
	var sheet tableCodeSheetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sheet); err != nil {
		t.Fatal(err)
	}
	if len(sheet.Tables) != len(list.Tables)-1 {
		t.Fatalf("%d tables on the sheet, want %d: every one but the table with no code", len(sheet.Tables), len(list.Tables)-1)
	}
	seen := map[string]bool{}
	for _, tb := range sheet.Tables {
		want := codeOf(t, list, tb.TableID)
		if tb.TableID == 5 {
			t.Error("the table with no code is on the sheet")
		}
		if tb.URL != want.URL || tb.TableName != want.TableName || !strings.HasPrefix(tb.SVG, "<svg") || seen[tb.SVG] {
			t.Errorf("table %d on the sheet = %s, %q, a picture already seen: %v", tb.TableID, tb.URL, tb.TableName, seen[tb.SVG])
		}
		seen[tb.SVG] = true
	}

	t.Run("with no public address there is no sheet", func(t *testing.T) {
		o := newCodesWorld(t, "")
		owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
		rec := send(o.handler, http.MethodGet, "/api/v1/table-codes/sheet", "", owner)
		if rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNoPublicURL {
			t.Errorf("the sheet: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestTheVenuesPickupCode: none to begin with; made, it leads to the venue at
// an address of its own; renewed, the old one is gone; turned off, there is
// none. No guest route takes it for a table's, and the tables' codes are
// left as they were.
func TestTheVenuesPickupCode(t *testing.T) {
	o := newCodesWorld(t, "https://cafe.example")
	owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)

	before := decodeCodes(t, send(o.handler, http.MethodGet, "/api/v1/table-codes", "", owner))
	if before.Pickup.Code != "" || before.Pickup.URL != "" {
		t.Fatalf("a venue with no pickup code says %+v", before.Pickup)
	}
	if rec := send(o.handler, http.MethodGet, "/api/v1/pickup-code/qr", "", owner); rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeNoPickupCode {
		t.Errorf("the QR of no pickup code: %d %s", rec.Code, rec.Body)
	}

	made := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/pickup-code/renew", "", owner))
	first := made.Pickup
	if first.Code == "" || first.URL != "https://cafe.example/order/p/"+first.Code {
		t.Fatalf("the pickup code = %+v", first)
	}
	if four := codeOf(t, made, 4); four.Code != o.code {
		t.Errorf("table 4's code changed with the pickup code: %+v", four)
	}
	// It is no table's code: every guest route answers it as one that leads nowhere.
	if status, _, body := o.guest(http.MethodGet, "/order/api/t/"+first.Code); status != http.StatusNotFound || !strings.Contains(body, string(CodeTableCodeUnknown)) {
		t.Errorf("a pickup code as a table's: %d %s", status, body)
	}

	rec := send(o.handler, http.MethodGet, "/api/v1/pickup-code/qr", "", owner)
	var qr pickupCodeQRResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &qr); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("the QR: %d %s", rec.Code, rec.Body)
	}
	if qr.URL != first.URL || !strings.HasPrefix(qr.SVG, "<svg") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("the QR is of %q", qr.URL)
	}

	renewed := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/pickup-code/renew", "", owner)).Pickup
	if renewed.Code == first.Code || renewed.Code == "" {
		t.Errorf("renewed = %+v, was %+v", renewed, first)
	}
	off := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/pickup-code/revoke", "", owner)).Pickup
	if off.Code != "" || off.URL != "" {
		t.Errorf("turned off = %+v", off)
	}

	t.Run("with no public address it has a code and no address", func(t *testing.T) {
		o := newCodesWorld(t, "")
		owner := o.asStaff(o.signInAs(t, "stf_PRIYA001", "2468").Token)
		got := decodeCodes(t, send(o.handler, http.MethodPost, "/api/v1/pickup-code/renew", "", owner)).Pickup
		if got.Code == "" || got.URL != "" {
			t.Errorf("pickup = %+v", got)
		}
		if rec := send(o.handler, http.MethodGet, "/api/v1/pickup-code/qr", "", owner); rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNoPublicURL {
			t.Errorf("the QR: %d %s", rec.Code, rec.Body)
		}
	})
}
