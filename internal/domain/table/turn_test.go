package table

import (
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

var (
	t0 = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(10 * time.Minute)
	t2 = t0.Add(20 * time.Minute)
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// open is an order at the table: its status, covers, opening, balance and
// the status of each of its lines.
func open(status order.Status, covers int64, at time.Time, balance int64, lines ...order.LineStatus) order.State {
	s := order.State{Status: status, Covers: covers, OpenedAt: at, Totals: order.Totals{BalanceDue: inr(balance)}}
	for i, ls := range lines {
		s.Lines = append(s.Lines, order.Line{ID: int64(i + 1), Status: ls})
	}
	return s
}

func TestDerive(t *testing.T) {
	active, voided := order.LineStatusActive, order.LineStatusVoided
	tests := []struct {
		name    string
		turn    Turn
		status  Status
		covers  int64
		since   time.Time
		balance int64 // -1: no balance
	}{
		{"never used", Turn{}, StatusFree, 0, time.Time{}, -1},
		{"cleared after its last order", Turn{LastClosedAt: t0, ClearedAt: t1}, StatusFree, 0, time.Time{}, -1},
		{"its last order closed, not cleared since", Turn{LastClosedAt: t1, ClearedAt: t0}, StatusDirty, 0, t1, -1},
		{"closed and never cleared", Turn{LastClosedAt: t0}, StatusDirty, 0, t0, -1},
		{"cleared the moment it closed", Turn{LastClosedAt: t0, ClearedAt: t0}, StatusFree, 0, time.Time{}, -1},
		{"seated, nothing ordered", Turn{Open: []order.State{open(order.StatusOpen, 4, t0, 0)}}, StatusSeated, 4, t0, 0},
		{"seated, only a voided line", Turn{Open: []order.State{open(order.StatusOpen, 2, t0, 0, voided)}}, StatusSeated, 2, t0, 0},
		{"ordered", Turn{Open: []order.State{open(order.StatusOpen, 2, t0, 45000, active, voided)}}, StatusOrdered, 2, t0, 45000},
		{"held, with lines", Turn{Open: []order.State{open(order.StatusHeld, 2, t0, 20000, active)}}, StatusOrdered, 2, t0, 20000},
		{"reopened, with lines", Turn{Open: []order.State{open(order.StatusReopened, 2, t0, 0, active)}}, StatusOrdered, 2, t0, 0},
		{"settling", Turn{Open: []order.State{open(order.StatusSettling, 3, t0, 10000, active)}}, StatusSettling, 3, t0, 10000},
		{
			"a new round after the bill: ordered wins",
			Turn{Open: []order.State{open(order.StatusSettling, 2, t0, 10000, active), open(order.StatusOpen, 1, t1, 30000, active)}},
			StatusOrdered, 3, t0, 40000,
		},
		{
			"settling beside an empty ticket",
			Turn{Open: []order.State{open(order.StatusOpen, 0, t0, 0), open(order.StatusSettling, 2, t1, 5000, active)}},
			StatusSettling, 2, t0, 5000,
		},
		{
			"an order open beats a dirty past",
			Turn{Open: []order.State{open(order.StatusOpen, 2, t2, 0)}, LastClosedAt: t1},
			StatusSeated, 2, t2, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Derive(tt.turn)
			if err != nil {
				t.Fatal(err)
			}
			if v.Status != tt.status || v.Covers != tt.covers || !v.Since.Equal(tt.since) {
				t.Errorf("got %s, %d covers, since %v; want %s, %d, %v", v.Status, v.Covers, v.Since, tt.status, tt.covers, tt.since)
			}
			if (tt.balance < 0) == v.BalanceValid || (v.BalanceValid && v.Balance != inr(tt.balance)) {
				t.Errorf("balance = %v (valid %v), want %d", v.Balance, v.BalanceValid, tt.balance)
			}
		})
	}
}

func TestDeriveRefuses(t *testing.T) {
	tests := []struct {
		name string
		open []order.State
	}{
		{"a closed order among the open", []order.State{open(order.StatusClosed, 2, t0, 0)}},
		{"balances in two currencies", []order.State{
			open(order.StatusOpen, 2, t0, 100),
			{Status: order.StatusOpen, OpenedAt: t1, Totals: order.Totals{BalanceDue: money.Amount{Minor: 1, Currency: "GBP"}}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Derive(Turn{Open: tt.open}); err == nil {
				t.Error("derived")
			}
		})
	}
}

// Every status Derive gives is one the table machine knows, and every
// move a real turn makes between them is one it allows.
func TestDerivedTurnFollowsTheMachine(t *testing.T) {
	seated := Turn{Open: []order.State{open(order.StatusOpen, 2, t0, 0)}}
	ordered := Turn{Open: []order.State{open(order.StatusOpen, 2, t0, 100, order.LineStatusActive)}}
	settling := Turn{Open: []order.State{open(order.StatusSettling, 2, t0, 100, order.LineStatusActive)}}
	dirty := Turn{LastClosedAt: t1, ClearedAt: t0}
	free := Turn{LastClosedAt: t1, ClearedAt: t2}
	turn := []Turn{{}, seated, ordered, settling, ordered, settling, dirty, free}
	prev := StatusFree
	for i, tn := range turn[1:] {
		v, err := Derive(tn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Transition(prev, v.Status); err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		prev = v.Status
	}
}

func TestClear(t *testing.T) {
	tests := []struct {
		name                    string
		unclosed                int64
		lastClosedAt, clearedAt time.Time
		wantErr                 error
	}{
		{"a dirty table", 0, t1, t0, nil},
		{"a dirty table never cleared", 0, t0, time.Time{}, nil},
		{"a free table again: no harm", 0, t0, t1, nil},
		{"a table never used", 0, time.Time{}, time.Time{}, nil},
		{"guests still at it", 1, t1, t0, ErrOccupied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Clear(tt.unclosed, tt.lastClosedAt, tt.clearedAt); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLeavable(t *testing.T) {
	paid := open(order.StatusOpen, 2, t0, 0)
	paid.Totals.Paid = inr(100)
	comped := open(order.StatusOpen, 2, t0, 0, order.LineStatusComped)
	tests := []struct {
		name string
		o    order.State
		want bool
	}{
		{"nothing on it", open(order.StatusOpen, 2, t0, 0), true},
		{"only a voided line", open(order.StatusOpen, 2, t0, 0, order.LineStatusVoided), true},
		{"a comped line, nothing owed", comped, true},
		{"a line on it", open(order.StatusOpen, 2, t0, 100, order.LineStatusActive), false},
		{"being paid", open(order.StatusSettling, 2, t0, 0), false},
		{"money taken", paid, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Leavable(tt.o); got != tt.want {
				t.Errorf("Leavable = %v, want %v", got, tt.want)
			}
		})
	}
}
