package tips

import (
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func shares(t *testing.T, got []Share) []int64 {
	t.Helper()
	out := make([]int64, len(got))
	sum := int64(0)
	for i, s := range got {
		out[i] = s.Share.Minor
		sum += s.Share.Minor
	}
	return append(out, sum)
}

func TestSplit(t *testing.T) {
	asha := Member{StaffID: 1, Minutes: 480, Weight: 1}
	priya := Member{StaffID: 2, Minutes: 240, Weight: 2}
	ravi := Member{StaffID: 3, Minutes: 240, Weight: 1}
	tests := []struct {
		name string
		pool int64
		rule Rule
		want []int64 // each share, then their sum
	}{
		// 1000.00 three ways: 333.34, 333.33, 333.33, the paisa to the first.
		{"evenly", 100000, Even, []int64{33334, 33333, 33333, 100000}},
		// 8 hours, 4 and 4: a half and two quarters.
		{"by hours", 100000, ByHours, []int64{50000, 25000, 25000, 100000}},
		// Weights 1, 2 and 1.
		{"by role weight", 100000, ByRoleWeight, []int64{25000, 50000, 25000, 100000}},
		// 0.07 by hours 2:1:1 is 0.035, 0.0175, 0.0175: 3, 2, 2.
		{"a few paise by hours", 7, ByHours, []int64{3, 2, 2, 7}},
		{"nothing to split", 0, Even, []int64{0, 0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(inr(tt.pool), tt.rule, []Member{asha, priya, ravi})
			if err != nil {
				t.Fatal(err)
			}
			if s := shares(t, got); !equal(s, tt.want) {
				t.Errorf("shares = %v, want %v", s, tt.want)
			}
			if got[1].StaffID != 2 || got[1].Minutes != 240 {
				t.Errorf("a share lost its member: %+v", got[1])
			}
		})
	}
}

func TestSplitRefusals(t *testing.T) {
	ok := Member{StaffID: 1, Minutes: 60, Weight: 1}
	tests := []struct {
		name    string
		pool    int64
		rule    Rule
		members []Member
	}{
		{"nobody", 100, Even, nil},
		{"a negative pool", -1, Even, []Member{ok}},
		{"no rule", 100, "coin_toss", []Member{ok}},
		{"someone twice", 100, Even, []Member{ok, ok}},
		{"no staff member", 100, Even, []Member{{}}},
		{"no hours by hours", 100, ByHours, []Member{ok, {StaffID: 2}}},
		{"a role that weighs nothing", 100, ByRoleWeight, []Member{ok, {StaffID: 2, Minutes: 60}}},
		{"negative minutes", 100, Even, []Member{{StaffID: 1, Minutes: -5}}},
		{"two days in one shift", 100, ByHours, []Member{{StaffID: 1, Minutes: MaxMinutes + 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Split(inr(tt.pool), tt.rule, tt.members); !errors.Is(err, ErrInvalidPool) {
				t.Errorf("err = %v, want ErrInvalidPool", err)
			}
		})
	}
}

var at = time.Date(2026, 9, 28, 20, 0, 0, 0, time.UTC)

func ev(shiftID, seq, actor int64, p order.Payload) shift.StampedEvent {
	return shift.StampedEvent{Event: order.NewEvent(seq, at, actor, 1, p), ShiftID: shiftID}
}

func TestTaken(t *testing.T) {
	orders := [][]shift.StampedEvent{
		{
			ev(1, 2, 3, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(45000), Tip: inr(5000), TenderedBy: 3}),
			ev(1, 3, 2, order.PaymentTendered{Method: order.PaymentCard, Amount: inr(10000), Tip: inr(1500), TenderedBy: 2}),
			ev(1, 4, 2, order.PaymentVoided{PaymentID: 3, Reason: "charged twice", AuthorisedBy: 2}),
			ev(1, 5, 2, order.PaymentTendered{Method: order.PaymentCard, Amount: inr(10000), Tip: inr(1000), TenderedBy: 2}),
		},
		{
			// Tipped yesterday, voided today: today's pool gives it back.
			ev(0, 2, 3, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(100), Tip: inr(700), TenderedBy: 3}),
			ev(1, 3, 2, order.PaymentVoided{PaymentID: 2, Reason: "wrong bill", AuthorisedBy: 2}),
			// A tender with no tip at all, as old ones have.
			ev(1, 4, 5, order.PaymentTendered{Method: order.PaymentUPI, Amount: inr(100), TenderedBy: 5}),
		},
		{ev(2, 2, 3, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(100), Tip: inr(9900), TenderedBy: 3})},
	}
	got, err := Taken("INR", 1, orders)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]money.Amount{3: inr(4300), 2: inr(1000)}
	if len(got) != len(want) || got[3] != want[3] || got[2] != want[2] {
		t.Errorf("taken = %v, want %v", got, want)
	}
	if _, err := Taken("inr", 1, nil); err == nil {
		t.Error("a malformed currency: want an error")
	}
}

func equal(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
