package shift

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// till sells through the real order service at venue 1.
type till struct {
	t      *testing.T
	orders *svcorder.Service
}

func newTillFor(t *testing.T, f *fixture) *till {
	t.Helper()
	orders, err := svcorder.New(svcorder.Options{DB: f.db, Clock: f.clock})
	if err != nil {
		t.Fatal(err)
	}
	return &till{t: t, orders: orders}
}

var seller = svcorder.Meta{VenueID: 1, ActorID: 3, DeviceID: 7}

// sale opens an order for price and takes tenders against it; the last
// tender that settles it issues the invoice. It returns the order and the
// payment IDs.
func (w *till) sale(price int64, tenders ...svcorder.TenderPayment) (int64, []int64) {
	w.t.Helper()
	res := must(w.orders.Open(ctx, seller, svcorder.OpenOrder{Channel: order.ChannelCounter, Covers: 1}))
	must(w.orders.AddLine(ctx, seller, res.OrderID, svcorder.AddLine{
		ItemID: 1, Name: "Thali", Qty: 1, UnitPrice: inr(price), Station: "kitchen",
	}))
	var payments []int64
	for _, tp := range tenders {
		r := must(w.orders.TenderPayment(ctx, seller, res.OrderID, tp))
		for _, e := range r.Events {
			if e.Type == order.TypePaymentTendered {
				payments = append(payments, e.Seq)
			}
		}
	}
	return res.OrderID, payments
}

func cash(amount, tip int64) svcorder.TenderPayment {
	return svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(amount), Tip: inr(tip)}
}

func card(amount int64) svcorder.TenderPayment {
	return svcorder.TenderPayment{Method: order.PaymentCard, Amount: inr(amount)}
}

func closing(minor int64) shift.Count {
	return must(shift.LumpSum(shift.CountClosing, inr(minor)))
}

// A day's cash, counted blind, ₹9.50 short, explained, closed.
func TestABlindCountReconcilesTheDrawer(t *testing.T) {
	f := newFixture(t)
	w := newTillFor(t, f)
	open := must(f.svc.Open(ctx, cashier, float(t, 200000)))

	w.sale(45000, cash(45000, 5000)) // 500 into the drawer, tip and all
	w.sale(30000, card(30000))       // nothing into the drawer
	must(f.svc.MoveCash(ctx, cashier, open.ID, movement(t, shift.CashOut, 12050, "ice")))
	orderID, paid := w.sale(40000, cash(20000, 0)) // half paid in cash, then rung off
	must(w.orders.VoidLine(ctx, seller, orderID, svcorder.VoidLine{LineID: 2, ReasonCode: "guest_left", AuthorisedBy: 2}))
	must(w.orders.Refund(ctx, seller, orderID, svcorder.Refund{Amount: inr(5000), AgainstPaymentID: paid[0], Reason: "cold"}))
	voidedOrder, voided := w.sale(10000, cash(10000, 0))
	must(w.orders.VoidPayment(ctx, seller, voidedOrder, svcorder.VoidPayment{PaymentID: voided[0], Reason: "wrong bill", AuthorisedBy: 2}))

	before := must(f.svc.ByID(ctx, 1, open.ID))
	wantSales := shift.CashSales{Taken: inr(80000), Voided: inr(10000), Refunded: inr(5000)}
	if before.Sales != wantSales {
		t.Errorf("sales = %+v, want %+v", before.Sales, wantSales)
	}
	// 2000 + 500 - 120.50 + 200 - 50 + 100 - 100 = 2529.50
	if before.Expected != inr(252950) || before.Counted() {
		t.Fatalf("before the count: expected %s, counted %v; want 2529.50, not counted", before.Expected, before.Counted())
	}

	if _, err := f.svc.Close(ctx, cashier, open.ID); !errors.Is(err, ErrNotCounted) {
		t.Errorf("close before the count: err = %v, want ErrNotCounted", err)
	}
	if _, err := f.svc.ExplainVariance(ctx, cashier, open.ID, "a coin rolled away"); !errors.Is(err, ErrNotCounted) {
		t.Errorf("a reason before the count: err = %v, want ErrNotCounted", err)
	}

	counted := must(shift.NewCount(shift.CountClosing, "INR", []shift.Denomination{{Face: 50000, Count: 5}, {Face: 2000, Count: 1}}))
	got := must(f.svc.Count(ctx, cashier, open.ID, counted))
	rec := got.Closing.Reconciliation
	if rec.Expected != inr(252950) || rec.Counted != inr(252000) || rec.Variance != inr(-950) || !rec.Short() || !rec.NeedsReason() {
		t.Errorf("reconciliation = %+v; want 2529.50 expected, 2520.00 counted, 9.50 short, a reason needed", rec)
	}
	if got.Closing.CountedBy != 3 || len(got.Closing.Count.Denominations) != 2 {
		t.Errorf("closing = %+v", got.Closing)
	}

	// Counted: the drawer is closed to everything.
	if _, err := w.orders.Open(ctx, seller, svcorder.OpenOrder{Channel: order.ChannelCounter, Covers: 1}); !errors.Is(err, shift.ErrNoOpenShift) {
		t.Errorf("a sale after the count: err = %v, want ErrNoOpenShift", err)
	}
	if _, err := f.svc.MoveCash(ctx, cashier, open.ID, movement(t, shift.CashIn, 100, "found it")); !errors.Is(err, ErrDrawerCounted) {
		t.Errorf("cash in after the count: err = %v, want ErrDrawerCounted", err)
	}
	if _, err := f.svc.Count(ctx, cashier, open.ID, closing(252950)); !errors.Is(err, ErrAlreadyCounted) {
		t.Errorf("a recount to match: err = %v, want ErrAlreadyCounted", err)
	}
	if _, err := f.svc.Count(ctx, cashier, open.ID, counted); err != nil {
		t.Errorf("the same count retried: %v", err)
	}
	if cur := must(f.svc.Current(ctx, 1)); cur.ID != open.ID {
		t.Errorf("a counted shift is still the current one: got %d", cur.ID)
	}

	if _, err := f.svc.Close(ctx, cashier, open.ID); !errors.Is(err, ErrVarianceUnexplained) {
		t.Errorf("close unexplained: err = %v, want ErrVarianceUnexplained", err)
	}
	explained := must(f.svc.ExplainVariance(ctx, cashier, open.ID, "  change given twice to table 4 "))
	if explained.Reason == nil || explained.Reason.Reason != "change given twice to table 4" || explained.Reason.ExplainedBy != 3 {
		t.Errorf("reason = %+v", explained.Reason)
	}
	if _, err := f.svc.ExplainVariance(ctx, cashier, open.ID, "a better story"); !errors.Is(err, ErrAlreadyExplained) {
		t.Errorf("a second reason: err = %v, want ErrAlreadyExplained", err)
	}
	closed := must(f.svc.Close(ctx, cashier, open.ID))
	if closed.Status != shift.StatusClosed || closed.Expected != inr(252950) {
		t.Errorf("closed = %+v", closed)
	}
}

func TestTheThresholdDecidesWhetherAReasonIsNeeded(t *testing.T) {
	tests := []struct {
		name    string
		setting string // "" leaves it unset
		counted int64
		reason  bool
		wantErr error
	}{
		{"to the paisa, strict", "", 100000, false, nil},
		{"a paisa out, strict", "", 99999, true, nil},
		{"within ₹10", "1000", 99050, false, nil},
		{"at ₹10 exactly", "1000", 101000, false, nil},
		{"past ₹10", "1000", 101001, true, nil},
		{"a threshold that is not a number", "ten rupees", 100000, false, ErrVenueConfig},
		{"a negative threshold", "-5", 100000, false, ErrVenueConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setting != "" {
				err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
					return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingVarianceThreshold, Value: tt.setting, UpdatedAt: start})
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
			got, err := f.svc.Count(ctx, cashier, open.ID, closing(tt.counted))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if again := must(f.svc.ByID(ctx, 1, open.ID)); again.Counted() {
					t.Error("a refused count was recorded")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Closing.Reconciliation.NeedsReason() != tt.reason {
				t.Errorf("needs a reason = %v, want %v (%+v)", !tt.reason, tt.reason, got.Closing.Reconciliation)
			}
			_, err = f.svc.Close(ctx, cashier, open.ID)
			if tt.reason != errors.Is(err, ErrVarianceUnexplained) {
				t.Errorf("close: err = %v, want unexplained %v", err, tt.reason)
			}
		})
	}
}

// The threshold is recorded with the count: loosening it after cannot
// excuse a shortfall already counted.
func TestTheThresholdIsTheOneInForceWhenCounted(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	must(f.svc.Count(ctx, cashier, open.ID, closing(99000)))
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingVarianceThreshold, Value: "100000", UpdatedAt: start})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Close(ctx, cashier, open.ID); !errors.Is(err, ErrVarianceUnexplained) {
		t.Errorf("close after loosening: err = %v, want ErrVarianceUnexplained", err)
	}
}

func TestCountRefusals(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	tests := []struct {
		name    string
		shiftID int64
		count   shift.Count
		want    error
	}{
		{"an opening count", open.ID, must(shift.LumpSum(shift.CountOpening, inr(0))), ErrInvalidCommand},
		{"a negative drawer", open.ID, shift.Count{Kind: shift.CountClosing, Total: inr(-1)}, ErrInvalidCommand},
		{"notes that do not add up", open.ID, shift.Count{
			Kind: shift.CountClosing, Denominations: []shift.Denomination{{Face: 50000, Count: 1}}, Total: inr(1),
		}, ErrInvalidCommand},
		{"no such shift", 99, closing(0), ErrShiftNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.Count(ctx, cashier, tt.shiftID, tt.count); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	if _, err := f.svc.ExplainVariance(ctx, cashier, open.ID, "   "); !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("a blank reason: err = %v, want ErrInvalidCommand", err)
	}
}

func TestTheCountCannotBeRewritten(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	must(f.svc.Count(ctx, cashier, open.ID, closing(99000)))
	must(f.svc.ExplainVariance(ctx, cashier, open.ID, "short"))

	tests := []struct {
		name string
		sql  string
	}{
		{"edit the count", "UPDATE drawer_counts SET total = 100000 WHERE kind = 'closing'"},
		{"delete the count", "DELETE FROM drawer_counts WHERE kind = 'closing'"},
		{"edit the reason", "UPDATE variance_reasons SET reason = 'nothing'"},
		{"delete the reason", "DELETE FROM variance_reasons"},
		{"a variance that does not follow", `INSERT INTO drawer_counts (shift_id, kind, denominations, total, counted_by, counted_at, expected, variance, threshold)
			VALUES (2, 'closing', '[]', 100, 3, '2026-09-27 18:00:00+00:00', 100, 50, 0)`},
		{"an opening count with an expected figure", `INSERT INTO drawer_counts (shift_id, kind, denominations, total, counted_by, counted_at, expected, variance, threshold)
			VALUES (2, 'opening', '[]', 100, 3, '2026-09-27 18:00:00+00:00', 100, 0, 0)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.db.Tx(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, tt.sql)
				return err
			})
			if err == nil {
				t.Error("the database let it through")
			}
		})
	}
}
