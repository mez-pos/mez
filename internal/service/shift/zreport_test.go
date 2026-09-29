package shift

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
)

// withStaff names staff 1 to 3 at venue 1: Asha, Priya (the manager, 2)
// and Ravi (the cashier, 3).
func withStaff(t *testing.T, f *fixture) {
	t.Helper()
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		role, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: "staff", Permissions: "[]", CreatedAt: start})
		if err != nil {
			return err
		}
		for _, n := range []string{"Asha", "Priya", "Ravi"} {
			if _, err := q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: "stf_" + n + "0001", VenueID: 1, RoleID: role.ID, Name: n, PinHash: "x", Active: true, CreatedAt: start, UpdatedAt: start,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// closeExact counts the drawer at exactly what is expected and closes.
func closeExact(t *testing.T, f *fixture, meta Meta, shiftID int64) Shift {
	t.Helper()
	sh := must(f.svc.ByID(ctx, meta.VenueID, shiftID))
	must(f.svc.Count(ctx, meta, shiftID, must(shift.LumpSum(shift.CountClosing, sh.Expected))))
	return must(f.svc.Close(ctx, meta, shiftID))
}

func TestClosingIssuesTheZReport(t *testing.T) {
	f := newFixture(t)
	withStaff(t, f)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 200000)))
	w.sale(45000, cash(45000, 5000))
	w.sale(30000, card(30000))
	must(f.svc.MoveCash(ctx, cashier, open.ID, movement(t, shift.CashOut, 12050, "ice")))

	f.clock.set(start.Add(5 * time.Hour))
	closed := closeExact(t, f, Meta{VenueID: 1, ActorID: 2, DeviceID: 7}, open.ID)
	if closed.ZReport != 1 {
		t.Fatalf("the closed shift's report = %d, want Z 1", closed.ZReport)
	}

	z := must(f.svc.ZReport(ctx, 1, 1))
	doc := z.Document
	if z.ShiftID != open.ID || doc.Number != 1 || doc.Corrects != 0 || doc.GeneratedBy != 2 || !doc.GeneratedAt.Equal(start.Add(5*time.Hour)) {
		t.Errorf("report = %+v", z)
	}
	if doc.Shift.PublicID != open.PublicID || doc.Shift.TradingDay != "2026-09-27" || doc.Shift.ClosedBy != 2 {
		t.Errorf("shift = %+v", doc.Shift)
	}
	r := doc.Report
	if r.Sales.Bills != 2 || r.Sales.Total != inr(75000) || r.Tips != inr(5000) || r.Balance.Difference != inr(0) {
		t.Errorf("figures = %+v, tips %s, balance %+v", r.Sales, r.Tips, r.Balance)
	}
	// 2000 float - 120.50 out + 500 cash (450 and a 50 tip) = 2379.50.
	d := doc.Drawer
	if d.Float != inr(200000) || d.CashOut != inr(12050) || d.CashIn != inr(0) || d.Sales.Taken != inr(50000) ||
		d.Expected != inr(237950) || d.Counted != inr(237950) || d.Variance != inr(0) || d.CountedBy != 2 {
		t.Errorf("drawer = %+v", d)
	}
	if len(r.ByStaff) != 1 || r.ByStaff[0].Name != "Ravi" || doc.Names[2] != "Priya" || doc.Names[3] != "Ravi" {
		t.Errorf("names: by staff %+v, names %v", r.ByStaff, doc.Names)
	}
	if got := must(f.svc.ZReportForShift(ctx, 1, open.ID)); got.Document.Number != 1 {
		t.Errorf("the shift's report = %d, want 1", got.Document.Number)
	}
}

func TestZNumbersRunPerVenueAndNeverReset(t *testing.T) {
	f := newFixture(t)
	for want := int64(1); want <= 3; want++ {
		open := must(f.svc.Open(ctx, cashier, float(t, 0)))
		if got := closeExact(t, f, cashier, open.ID); got.ZReport != want {
			t.Errorf("venue 1's shift %d closed as Z %d, want %d", open.ID, got.ZReport, want)
		}
	}
	other := Meta{VenueID: 2, ActorID: 3}
	open := must(f.svc.Open(ctx, other, float(t, 0)))
	if got := closeExact(t, f, other, open.ID); got.ZReport != 1 {
		t.Errorf("venue 2's first close = Z %d, want 1", got.ZReport)
	}
	if _, err := f.svc.ZReport(ctx, 2, 3); !errors.Is(err, ErrZReportNotFound) {
		t.Errorf("venue 1's Z 3 from venue 2: err = %v, want ErrZReportNotFound", err)
	}
}

func TestACorrectionIsANewReportNamingTheOld(t *testing.T) {
	f := newFixture(t)
	first := must(f.svc.Open(ctx, cashier, float(t, 0)))
	closeExact(t, f, cashier, first.ID)
	second := must(f.svc.Open(ctx, cashier, float(t, 0)))

	if _, err := f.svc.CorrectZReport(ctx, cashier, second.ID, "wrong"); !errors.Is(err, ErrShiftStillOpen) {
		t.Errorf("correcting an open shift: err = %v, want ErrShiftStillOpen", err)
	}
	closeExact(t, f, cashier, second.ID)
	if _, err := f.svc.CorrectZReport(ctx, cashier, first.ID, "  "); !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("a correction with no reason: err = %v, want ErrInvalidCommand", err)
	}

	manager := Meta{VenueID: 1, ActorID: 2}
	fixed := must(f.svc.CorrectZReport(ctx, manager, first.ID, "the cakes were in the wrong category"))
	if fixed.Document.Number != 3 || fixed.Document.Corrects != 1 || fixed.Document.Reason != "the cakes were in the wrong category" || fixed.ShiftID != first.ID {
		t.Errorf("correction = %+v", fixed.Document)
	}
	again := must(f.svc.CorrectZReport(ctx, manager, first.ID, "the cakes were in the wrong category"))
	if again.Document.Number != 3 {
		t.Errorf("a retried correction issued Z %d, want Z 3 again", again.Document.Number)
	}
	if orig := must(f.svc.ZReport(ctx, 1, 1)); orig.Document.Corrects != 0 || orig.Document.Reason != "" {
		t.Errorf("the original changed: %+v", orig.Document)
	}
	if now := must(f.svc.ZReportForShift(ctx, 1, first.ID)); now.Document.Number != 3 {
		t.Errorf("the first shift's report now = %d, want the correction", now.Document.Number)
	}
	if sh := must(f.svc.ByID(ctx, 1, first.ID)); sh.ZReport != 3 {
		t.Errorf("the shift names Z %d, want 3", sh.ZReport)
	}

	list := must(f.svc.ZReports(ctx, 1, 0))
	if len(list) != 3 || list[0].Number != 3 || !list[0].Correction || list[1].Number != 2 || list[2].Number != 1 || list[2].Correction {
		t.Errorf("list = %+v, want 3 (a correction), 2, 1", list)
	}
	if page := must(f.svc.ZReports(ctx, 1, 3)); len(page) != 2 || page[0].Number != 2 {
		t.Errorf("the page before 3 = %+v, want 2 and 1", page)
	}
}

// A shift whose report cannot be built does not close: no closed shift is
// ever without its report.
func TestACloseWhoseReportFailsDoesNotHappen(t *testing.T) {
	f := newFixture(t)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	orderID, _ := w.sale(10000, card(10000))
	// A stream the projection refuses, written past every check, stands in
	// for a corrupted file. The drawer's cash does not read it; the report
	// projects it and fails.
	err := f.db.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO order_events (order_id, seq, type, payload, actor_id, at, shift_id)
			VALUES (?, 99, ?, '{"payment_id":42,"reason":"x","authorised_by":2}', 3, '2026-09-27 19:00:00+00:00', ?)`,
			orderID, string(order.TypePaymentVoided), open.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	c := must(f.svc.ByID(ctx, 1, open.ID))
	must(f.svc.Count(ctx, cashier, open.ID, must(shift.LumpSum(shift.CountClosing, c.Expected))))
	if _, err := f.svc.Close(ctx, cashier, open.ID); err == nil {
		t.Fatal("a close with a report that cannot be built went through")
	}
	if sh := must(f.svc.ByID(ctx, 1, open.ID)); sh.Status != shift.StatusOpen || sh.ZReport != 0 {
		t.Errorf("after the failed close: status %s, Z %d; want open, none", sh.Status, sh.ZReport)
	}
}

func TestAZReportCannotBeRewritten(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	closeExact(t, f, cashier, open.ID)
	for _, stmt := range []string{
		"UPDATE z_reports SET body = '{}'",
		"DELETE FROM z_reports",
		"INSERT INTO z_reports (venue_id, number, shift_id, reason, body, generated_by, generated_at) VALUES (1, 9, 1, '', '{}', 3, '2026-09-27 18:00:00+00:00')",
		"INSERT INTO z_reports (venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at) VALUES (1, 9, 1, 1, '', '{}', 3, '2026-09-27 18:00:00+00:00')",
		"INSERT INTO z_reports (venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at) VALUES (1, 1, 1, 1, 'dup', '{}', 3, '2026-09-27 18:00:00+00:00')",
	} {
		err := f.db.Tx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, stmt)
			return err
		})
		if err == nil {
			t.Errorf("the database let %q through", stmt)
		}
	}
}
