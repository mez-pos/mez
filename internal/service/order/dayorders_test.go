package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
)

// movedClock is a clock the test sets.
type movedClock struct{ at time.Time }

func (c *movedClock) Now() time.Time { return c.at }

func TestDayStatusOf(t *testing.T) {
	line := func(s order.LineStatus) order.Line { return order.Line{Status: s} }
	tests := []struct {
		name string
		st   order.State
		want DayStatus
	}{
		{"open", order.State{Status: order.StatusOpen}, DayOpen},
		{"held", order.State{Status: order.StatusHeld}, DayOpen},
		{"reopened by a voided payment", order.State{Status: order.StatusReopened}, DayOpen},
		{"closed and kept", order.State{Status: order.StatusClosed, Lines: []order.Line{line(order.LineStatusActive)}}, DayPaid},
		{"closed, one line voided", order.State{Status: order.StatusClosed, Lines: []order.Line{line(order.LineStatusVoided), line(order.LineStatusActive)}}, DayPaid},
		{"closed, given away", order.State{Status: order.StatusClosed, Lines: []order.Line{line(order.LineStatusComped)}}, DayPaid},
		{"closed, every line voided", order.State{Status: order.StatusClosed, Lines: []order.Line{line(order.LineStatusVoided)}}, DayVoided},
		{"closed with nothing on it", order.State{Status: order.StatusClosed}, DayVoided},
		{
			"closed, part credited",
			order.State{Status: order.StatusClosed, Lines: []order.Line{line(order.LineStatusActive)}, Totals: order.Totals{Credited: inr(100)}},
			DayRefunded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DayStatusOf(tt.st); got != tt.want {
				t.Errorf("DayStatusOf = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestDayOrders: the day's list is everything opened or closed since the
// trading day began and whatever is still open from before, newest first,
// with what it all collected.
func TestDayOrders(t *testing.T) {
	_, d, rec := newService(t)
	ctx := context.Background()
	// 18 September, 10:00 in Kolkata; the day began at 04:00.
	clock := &movedClock{at: now.Add(-24 * time.Hour)}
	s, err := New(Options{DB: d, Clock: clock, Publisher: rec})
	if err != nil {
		t.Fatal(err)
	}

	// Yesterday: a bill paid and closed, and a ticket left open.
	splitBill(t, s)
	leftOpen := open(t, s).OrderID

	// Last night, 01:30: still yesterday's trading day.
	clock.at = now.Add(-9 * time.Hour)
	splitBill(t, s)

	clock.at = now
	paid, _ := splitBill(t, s)
	clock.at = now.Add(time.Minute)
	credited, closed := splitBill(t, s)
	must(t)(s.Credit(ctx, cashier(""), credited, IssueCredit{
		Credit: order.CreditRequest{Lines: []order.CreditItem{{LineID: 2, Qty: 1}}}, Reason: "cold", AuthorisedBy: 7,
		Refunds: []RefundPart{{PaymentID: closed.Tenders[0].ID, Amount: inr(12600)}},
	}))
	clock.at = now.Add(2 * time.Minute)
	openNow := open(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), openNow, coffee(1)))

	day, err := s.DayOrders(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if day.Day != "2026-09-18" {
		t.Errorf("day = %s", day.Day)
	}
	want := []struct {
		id     int64
		status DayStatus
	}{{openNow, DayOpen}, {credited, DayRefunded}, {paid, DayPaid}, {leftOpen, DayOpen}}
	if len(day.Orders) != len(want) {
		t.Fatalf("orders = %d, want %d", len(day.Orders), len(want))
	}
	for i, w := range want {
		got := day.Orders[i]
		if got.OrderID != w.id || DayStatusOf(got.State) != w.status {
			t.Errorf("order %d = %d %s, want %d %s", i, got.OrderID, DayStatusOf(got.State), w.id, w.status)
		}
	}
	// Two bills of 37,800, less the 12,600 given back.
	if day.Collected != inr(37800+37800-12600) {
		t.Errorf("collected = %d", day.Collected.Minor)
	}

	// Past 04:00 the next morning it is a new day: only what is still open.
	clock.at = now.Add(20 * time.Hour)
	next, err := s.DayOrders(ctx, 1)
	if err != nil || next.Day != "2026-09-19" || len(next.Orders) != 2 || !next.Collected.IsZero() {
		t.Errorf("the next day = %s, %d orders, %d collected, %v", next.Day, len(next.Orders), next.Collected.Minor, err)
	}

	if other, err := s.DayOrders(ctx, 2); err == nil && len(other.Orders) != 0 {
		t.Errorf("another venue's orders: %+v", other.Orders)
	}
}

// TestDayOrdersFollowsTheDayStart: a venue whose day starts at 11:00 still
// lists the night before at 10:00, and a setting that is no time is refused.
func TestDayOrdersFollowsTheDayStart(t *testing.T) {
	_, d, rec := newService(t)
	ctx := context.Background()
	clock := &movedClock{at: now.Add(-9 * time.Hour)} // 01:00 in Kolkata
	s, err := New(Options{DB: d, Clock: clock, Publisher: rec})
	if err != nil {
		t.Fatal(err)
	}
	lastNight, _ := splitBill(t, s)
	clock.at = now // 10:00

	set := func(value string) {
		t.Helper()
		err := d.WriteTx(ctx, func(q *gen.Queries) error {
			return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingDayStart, Value: value, UpdatedAt: now})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if day, err := s.DayOrders(ctx, 1); err != nil || len(day.Orders) != 0 {
		t.Fatalf("with the default start: %d orders, %v", len(day.Orders), err)
	}
	set("11:00")
	day, err := s.DayOrders(ctx, 1)
	if err != nil || day.Day != "2026-09-17" || len(day.Orders) != 1 || day.Orders[0].OrderID != lastNight {
		t.Errorf("with an 11:00 start: %s, %d orders, %v", day.Day, len(day.Orders), err)
	}
	set("teatime")
	if _, err := s.DayOrders(ctx, 1); !errors.Is(err, ErrVenueConfig) {
		t.Errorf("a day start that is no time: %v", err)
	}
}
