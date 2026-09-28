package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
	svcshift "github.com/byjoellobo/nofiltr-pos/internal/service/shift"
)

// seededClock stops at seededAt: 10:00 on 18 September in Kolkata.
type seededClock struct{}

func (seededClock) Now() time.Time { return seededAt }

// shiftTill is a till with shifts and orders wired in, and no shift open.
type shiftTill struct {
	*till
	cashier, owner string // staff session tokens
}

func newShiftTill(t *testing.T) *shiftTill {
	t.Helper()
	w := newTill(t)
	orders, err := svcorder.New(svcorder.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	shifts, err := svcshift.New(svcshift.Options{DB: w.db, Clock: seededClock{}})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithOrders(orders), WithShifts(shifts))
	return &shiftTill{
		till:    w,
		cashier: w.signIn(t, "2468").Token,
		owner:   w.signInAs(t, "stf_PRIYA001", "2468").Token,
	}
}

// do sends a request as the staff member signed in with token, with an
// idempotency key when key is not empty.
func (w *shiftTill) do(token, method, path, body, key string) (int, []byte) {
	header := w.asStaff(token)
	if key != "" {
		header[HeaderIdempotencyKey] = key
	}
	rec := send(w.handler, method, path, body, header)
	return rec.Code, rec.Body.Bytes()
}

func (w *shiftTill) shift(t *testing.T, token, method, path, body string, want int) shiftResponse {
	t.Helper()
	status, raw := w.do(token, method, path, body, "")
	if status != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, status, want, raw)
	}
	var out shiftResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s %s: %v: %s", method, path, err, raw)
	}
	return out
}

func (w *shiftTill) refused(t *testing.T, token, method, path, body string, status int, code ErrorCode) {
	t.Helper()
	got, raw := w.do(token, method, path, body, "")
	if got != status {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, got, status, raw)
	}
	var e errorBody
	if err := json.Unmarshal(raw, &e); err != nil || e.Error.Code != code {
		t.Fatalf("%s %s: code %q, want %q: %s", method, path, e.Error.Code, code, raw)
	}
}

func (w *shiftTill) current(t *testing.T) *shiftResponse {
	t.Helper()
	status, raw := w.do(w.cashier, http.MethodGet, "/api/v1/shift", "", "")
	if status != http.StatusOK {
		t.Fatalf("GET shift: %d %s", status, raw)
	}
	// Blind: nothing that gives the drawer's figure away before the count.
	if !strings.Contains(string(raw), `"counted":true`) {
		for _, tell := range []string{"drawer", "expected", "sales", "reconciliation", "variance"} {
			if strings.Contains(string(raw), tell) {
				t.Errorf("the shift says %q before the blind count: %s", tell, raw)
			}
		}
	}
	var out currentShiftResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.Shift
}

const openOrderBody = `{"channel":"counter","covers":1}`

// TestAShiftOverHTTP: nothing sells until the owner counts a float in;
// the cashier pays out with the owner's override; the owner closes, and
// the till is shut again.
func TestAShiftOverHTTP(t *testing.T) {
	w := newShiftTill(t)

	if got := w.current(t); got != nil {
		t.Fatalf("a new venue has a shift open: %+v", got)
	}
	w.refused(t, w.cashier, http.MethodPost, "/api/v1/orders", openOrderBody, http.StatusConflict, CodeNoOpenShift)

	float := `{"total":{"minor":110000,"currency":"INR"},"denominations":[{"face":1000,"count":10},{"face":50000,"count":2}]}`
	w.refused(t, w.cashier, http.MethodPost, "/api/v1/shift/open", float, http.StatusForbidden, CodeForbidden)
	w.refused(t, w.owner, http.MethodPost, "/api/v1/shift/open",
		`{"total":{"minor":100000,"currency":"INR"},"denominations":[{"face":50000,"count":1}]}`,
		http.StatusUnprocessableEntity, CodeInvalidCount)

	opened := w.shift(t, w.owner, http.MethodPost, "/api/v1/shift/open", float, http.StatusCreated)
	if opened.Status != shift.StatusOpen || opened.Float.Minor != 110000 || opened.TradingDay != "2026-09-18" {
		t.Errorf("opened = %+v", opened)
	}
	w.refused(t, w.owner, http.MethodPost, "/api/v1/shift/open", float, http.StatusConflict, CodeShiftOpen)
	if got := w.current(t); got == nil || got.ID != opened.ID {
		t.Fatalf("current = %+v, want shift %d", got, opened.ID)
	}

	if status, raw := w.do(w.cashier, http.MethodPost, "/api/v1/orders", openOrderBody, ""); status != http.StatusCreated {
		t.Fatalf("an order in an open shift: %d %s", status, raw)
	}

	cash := fmt.Sprintf("/api/v1/shift/%d/cash", opened.ID)
	payout := func(by string) string {
		return `{"kind":"cash_out","amount":{"minor":12050,"currency":"INR"},"reason":"ice from the shop",` +
			`"override":{"authorised_by":"` + by + `","pin":"2468"}}`
	}
	w.refused(t, w.cashier, http.MethodPost, cash, payout("stf_ASHA0001"), http.StatusForbidden, CodeForbidden)
	w.refused(t, w.cashier, http.MethodPost, cash,
		`{"kind":"cash_out","amount":{"minor":100,"currency":"INR"},"reason":" ","override":{"authorised_by":"stf_PRIYA001","pin":"2468"}}`,
		http.StatusUnprocessableEntity, CodeInvalidCash)

	for range 2 { // a retry on flaky wifi records it once
		if status, raw := w.do(w.cashier, http.MethodPost, cash, payout("stf_PRIYA001"), "payout-1"); status != http.StatusOK {
			t.Fatalf("payout: %d %s", status, raw)
		}
	}
	moved := w.current(t)
	if len(moved.Events) != 2 {
		t.Fatalf("events = %+v, want opened and one payout", moved.Events)
	}
	out := moved.Events[1]
	if out.Type != svcshift.EventCashOut || out.Amount.Minor != 12050 || out.Reason != "ice from the shop" || out.AuthorisedBy != 2 || out.ActorID != 1 {
		t.Errorf("payout = %+v", out)
	}

	closePath := fmt.Sprintf("/api/v1/shift/%d/close", opened.ID)
	countPath := fmt.Sprintf("/api/v1/shift/%d/count", opened.ID)
	reasonPath := fmt.Sprintf("/api/v1/shift/%d/reason", opened.ID)
	w.refused(t, w.cashier, http.MethodPost, closePath, "", http.StatusForbidden, CodeForbidden)
	w.refused(t, w.owner, http.MethodPost, closePath, "", http.StatusConflict, CodeNotCounted)
	w.refused(t, w.cashier, http.MethodPost, reasonPath, `{"reason":"early"}`, http.StatusConflict, CodeNotCounted)

	// The blind count: ₹970 counted against ₹1,100 float less ₹120.50 out,
	// revealed only in the answer.
	blind := `{"total":{"minor":97000,"currency":"INR"},"denominations":[{"face":50000,"count":1},{"face":1000,"count":47}]}`
	counted := w.shift(t, w.cashier, http.MethodPost, countPath, blind, http.StatusOK)
	rec := counted.Reconciliation
	if !counted.Counted || rec == nil || rec.Expected.Minor != 97950 || rec.Counted.Minor != 97000 ||
		rec.Variance.Minor != -950 || !rec.Short || rec.Over || !rec.ReasonRequired || rec.CountedBy != 1 {
		t.Fatalf("the reveal = %+v", rec)
	}
	w.refused(t, w.cashier, http.MethodPost, countPath, `{"total":{"minor":97950,"currency":"INR"}}`, http.StatusConflict, CodeAlreadyCounted)
	w.refused(t, w.cashier, http.MethodPost, cash, payout("stf_PRIYA001"), http.StatusConflict, CodeDrawerCounted)
	w.refused(t, w.cashier, http.MethodPost, "/api/v1/orders", openOrderBody, http.StatusConflict, CodeNoOpenShift)
	if got := w.current(t); got == nil || got.Reconciliation == nil || got.Reconciliation.Variance.Minor != -950 {
		t.Errorf("after the count the shift shows its reconciliation: %+v", got)
	}

	w.refused(t, w.owner, http.MethodPost, closePath, "", http.StatusConflict, CodeVarianceUnexplained)
	w.refused(t, w.cashier, http.MethodPost, reasonPath, `{"reason":"  "}`, http.StatusBadRequest, CodeInvalidRequest)
	explained := w.shift(t, w.cashier, http.MethodPost, reasonPath, `{"reason":"change given twice"}`, http.StatusOK)
	if explained.Reason == nil || explained.Reason.Reason != "change given twice" || explained.Reason.ExplainedBy != 1 {
		t.Errorf("reason = %+v", explained.Reason)
	}
	w.refused(t, w.cashier, http.MethodPost, reasonPath, `{"reason":"another"}`, http.StatusConflict, CodeAlreadyExplained)

	closed := w.shift(t, w.owner, http.MethodPost, closePath, "", http.StatusOK)
	if closed.Status != shift.StatusClosed || closed.ClosedAt == nil || closed.ClosedBy != 2 {
		t.Errorf("closed = %+v", closed)
	}
	w.refused(t, w.owner, http.MethodPost, closePath, "", http.StatusConflict, CodeShiftClosed)
	w.refused(t, w.cashier, http.MethodPost, cash, payout("stf_PRIYA001"), http.StatusConflict, CodeShiftClosed)
	w.refused(t, w.owner, http.MethodPost, "/api/v1/shift/99/close", "", http.StatusNotFound, CodeShiftUnknown)

	if got := w.current(t); got != nil {
		t.Errorf("after close: current = %+v", got)
	}
	w.refused(t, w.cashier, http.MethodPost, "/api/v1/orders", openOrderBody, http.StatusConflict, CodeNoOpenShift)
}

// TestAFloatTypedInWhole: a float with no notes and coins, and one of
// nothing, are both a float.
func TestAFloatTypedInWhole(t *testing.T) {
	w := newShiftTill(t)
	opened := w.shift(t, w.owner, http.MethodPost, "/api/v1/shift/open", `{"total":{"minor":0,"currency":"INR"}}`, http.StatusCreated)
	if opened.Float.Minor != 0 || opened.Float.Currency != "INR" {
		t.Errorf("float = %+v", opened.Float)
	}
}

// TestShiftRoutesAreGuarded: no device, no staff session, no answer.
func TestShiftRoutesAreGuarded(t *testing.T) {
	w := newShiftTill(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/shift"},
		{http.MethodPost, "/api/v1/shift/open"},
		{http.MethodPost, "/api/v1/shift/1/cash"},
		{http.MethodPost, "/api/v1/shift/1/close"},
	} {
		if rec := send(w.handler, route.method, route.path, "{}", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with no device: %d, want 401", route.method, route.path, rec.Code)
		}
		if rec := send(w.handler, route.method, route.path, "{}", w.asDevice()); rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
			t.Errorf("%s %s with no staff session: %d", route.method, route.path, rec.Code)
		}
	}
}
