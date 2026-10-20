package shift

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tips"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// withRoles names staff 1 to 3 with roles: Asha (cashier), Priya
// (manager), Ravi (cashier).
func withRoles(t *testing.T, f *fixture) {
	t.Helper()
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		roles := map[string]int64{}
		for _, name := range []string{"cashier", "manager"} {
			r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: name, Permissions: "[]", CreatedAt: start})
			if err != nil {
				return err
			}
			roles[name] = r.ID
		}
		for _, s := range []struct{ name, role string }{{"Asha", "cashier"}, {"Priya", "manager"}, {"Ravi", "cashier"}} {
			if _, err := q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: "stf_" + s.name + "0001", VenueID: 1, RoleID: roles[s.role], Name: s.name, PinHash: "x", Active: true, CreatedAt: start, UpdatedAt: start,
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

func setting(t *testing.T, f *fixture, key, value string) {
	t.Helper()
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: key, Value: value, UpdatedAt: start})
	})
	if err != nil {
		t.Fatal(err)
	}
}

// tippedDay is a shift in which Ravi (3) takes a 50.00 cash tip and Priya
// (2) a 20.00 card tip, and a 10.00 tip Ravi took is voided with its
// payment.
func tippedDay(t *testing.T) (*fixture, Shift) {
	t.Helper()
	f := newFixture(t)
	withRoles(t, f)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 0)))
	w.sale(45000, cash(45000, 5000))

	priya := svcorder.Meta{VenueID: 1, ActorID: 2, DeviceID: 7}
	res := must(w.orders.Open(ctx, priya, svcorder.OpenOrder{Channel: order.ChannelCounter, Covers: 1}))
	must(w.orders.AddLine(ctx, priya, res.OrderID, svcorder.AddLine{ItemID: 1, Name: "Thali", Qty: 1, UnitPrice: inr(30000), Station: "kitchen"}))
	must(w.orders.TenderPayment(ctx, priya, res.OrderID, svcorder.TenderPayment{Method: order.PaymentCard, Amount: inr(30000), Tip: inr(2000)}))

	voided, _ := w.sale(10000)
	half := must(w.orders.TenderPayment(ctx, seller, voided, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(5000), Tip: inr(1000)}))
	must(w.orders.VoidPayment(ctx, seller, voided, svcorder.VoidPayment{PaymentID: half.Events[0].Seq, Reason: "wrong bill", AuthorisedBy: 2}))
	return f, open
}

func member(p TipPool, id int64) TipMember {
	for _, m := range p.Members {
		if m.StaffID == id {
			return m
		}
	}
	return TipMember{}
}

func TestATipPoolSplitsEvenlyByDefault(t *testing.T) {
	f, open := tippedDay(t)

	preview := must(f.svc.Tips(ctx, 1, open.ID))
	if preview.Pooled || preview.Rule != tips.Even || preview.Total != inr(7000) || preview.Problem != "" {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Members) != 2 || member(preview, 2).Share != inr(3500) || member(preview, 3).Share != inr(3500) {
		t.Errorf("preview members = %+v, want Priya and Ravi at 35.00 each", preview.Members)
	}
	if _, err := f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Priya0001"}}); !errors.Is(err, ErrShiftStillOpen) {
		t.Errorf("pooling an open shift: err = %v, want ErrShiftStillOpen", err)
	}

	closeExact(t, f, cashier, open.ID)
	manager := Meta{VenueID: 1, ActorID: 2}
	everyone := []PoolMember{{Staff: "stf_Asha0001"}, {Staff: "stf_Priya0001"}, {Staff: "stf_Ravi0001"}}
	peek := must(f.svc.PreviewTips(ctx, 1, open.ID, everyone))
	if peek.Pooled || member(peek, 1).Share != inr(2334) {
		t.Errorf("the preview for everyone = %+v", peek)
	}
	if again := must(f.svc.Tips(ctx, 1, open.ID)); again.Pooled {
		t.Error("a preview was recorded")
	}
	pool := must(f.svc.PoolTips(ctx, manager, open.ID, everyone))
	if !pool.Pooled || pool.PooledBy != 2 || pool.Total != inr(7000) {
		t.Fatalf("pool = %+v", pool)
	}
	// 70.00 three ways: 23.34, 23.33, 23.33.
	wants := []struct {
		id           int64
		name, role   string
		taken, share int64
	}{
		{1, "Asha", "cashier", 0, 2334},
		{2, "Priya", "manager", 2000, 2333},
		{3, "Ravi", "cashier", 5000, 2333},
	}
	sum := int64(0)
	for _, w := range wants {
		m := member(pool, w.id)
		if m.Name != w.name || m.Role != w.role || !m.InPool || m.Taken != inr(w.taken) || m.Share != inr(w.share) {
			t.Errorf("staff %d = %+v, want %+v", w.id, m, w)
		}
		sum += m.Share.Minor
	}
	if sum != pool.Total.Minor {
		t.Errorf("the shares come to %d, the pool is %d", sum, pool.Total.Minor)
	}

	if _, err := f.svc.PoolTips(ctx, manager, open.ID, everyone); err != nil {
		t.Errorf("the same pool retried: %v", err)
	}
	if _, err := f.svc.PoolTips(ctx, manager, open.ID, everyone[1:]); !errors.Is(err, ErrAlreadyPooled) {
		t.Errorf("a second, different pool: err = %v, want ErrAlreadyPooled", err)
	}
	if again := must(f.svc.Tips(ctx, 1, open.ID)); !again.Pooled || member(again, 1).Share != inr(2334) || member(again, 2).PublicID != "stf_Priya0001" {
		t.Errorf("reading the pool back = %+v", again)
	}

	// Tips never touch the sales: the report sells 450 + 300 + 100.
	z := must(f.svc.ZReportForShift(ctx, 1, open.ID))
	if z.Document.Report.Sales.Total != inr(75000) || z.Document.Report.Tips != inr(7000) {
		t.Errorf("sales %s, tips %s; want 750.00 sold and 70.00 in tips apart", z.Document.Report.Sales.Total, z.Document.Report.Tips)
	}
}

func TestATipPoolByHours(t *testing.T) {
	f, open := tippedDay(t)
	setting(t, f, SettingTipRule, "hours")

	preview := must(f.svc.Tips(ctx, 1, open.ID))
	if preview.Rule != tips.ByHours || preview.Problem == "" || len(preview.Members) != 2 || member(preview, 3).Taken != inr(5000) {
		t.Errorf("preview = %+v; want the takers listed and a note that hours are needed", preview)
	}
	closeExact(t, f, cashier, open.ID)
	if _, err := f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Priya0001", Minutes: 480}, {Staff: "stf_Ravi0001"}}); !errors.Is(err, tips.ErrInvalidPool) {
		t.Errorf("Ravi with no hours: err = %v, want ErrInvalidPool", err)
	}
	// Ravi pooled 8 hours, Priya 4: two thirds and a third.
	pool := must(f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Ravi0001", Minutes: 480}, {Staff: "stf_Priya0001", Minutes: 240}}))
	if member(pool, 3).Share != inr(4667) || member(pool, 2).Share != inr(2333) || member(pool, 3).Minutes != 480 {
		t.Errorf("pool = %+v, want 46.67 and 23.33", pool.Members)
	}
}

func TestATipPoolByRoleWeight(t *testing.T) {
	f, open := tippedDay(t)
	setting(t, f, SettingTipRule, "role_weight")
	setting(t, f, SettingTipRoleWeights, `{"manager": 2}`)
	closeExact(t, f, cashier, open.ID)
	// Priya weighs 2, Asha and Ravi 1: 35.00, 17.50, 17.50.
	pool := must(f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Asha0001"}, {Staff: "stf_Priya0001"}, {Staff: "stf_Ravi0001"}}))
	if member(pool, 2).Share != inr(3500) || member(pool, 2).Weight != 2 || member(pool, 1).Share != inr(1750) || member(pool, 3).Share != inr(1750) {
		t.Errorf("pool = %+v", pool.Members)
	}
}

// Someone who took a tip and is left out of the pool is still on the
// report, with what they took and no share.
func TestATakerLeftOutIsStillOnTheReport(t *testing.T) {
	f, open := tippedDay(t)
	closeExact(t, f, cashier, open.ID)
	pool := must(f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Ravi0001"}}))
	priya := member(pool, 2)
	if priya.InPool || priya.Taken != inr(2000) || priya.Share != inr(0) || member(pool, 3).Share != inr(7000) {
		t.Errorf("pool = %+v", pool.Members)
	}
}

func TestTipPoolRefusals(t *testing.T) {
	f, open := tippedDay(t)
	closeExact(t, f, cashier, open.ID)
	tests := []struct {
		name    string
		members []PoolMember
		want    error
	}{
		{"nobody", []PoolMember{}, tips.ErrInvalidPool},
		{"a stranger", []PoolMember{{Staff: "stf_NOBODY01"}}, ErrNotStaff},
		{"someone twice", []PoolMember{{Staff: "stf_Priya0001"}, {Staff: "stf_Priya0001"}}, tips.ErrInvalidPool},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.PoolTips(ctx, cashier, open.ID, tt.members); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	for _, s := range []struct{ key, value string }{
		{SettingTipRule, "coin_toss"},
		{SettingTipRoleWeights, "heavy"},
		{SettingTipRoleWeights, `{"manager": 0}`},
	} {
		setting(t, f, s.key, s.value)
		if _, err := f.svc.Tips(ctx, 1, open.ID); !errors.Is(err, ErrVenueConfig) {
			t.Errorf("%s = %q: err = %v, want ErrVenueConfig", s.key, s.value, err)
		}
		setting(t, f, SettingTipRule, "even")
		setting(t, f, SettingTipRoleWeights, "{}")
	}
}

func TestATipPoolCannotBeRewritten(t *testing.T) {
	f, open := tippedDay(t)
	closeExact(t, f, cashier, open.ID)
	must(f.svc.PoolTips(ctx, cashier, open.ID, []PoolMember{{Staff: "stf_Ravi0001"}}))
	for _, stmt := range []string{"UPDATE tip_pools SET total = 0", "DELETE FROM tip_pools"} {
		err := f.db.Tx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, stmt)
			return err
		})
		if err == nil {
			t.Errorf("the database let %q through", stmt)
		}
	}
}
