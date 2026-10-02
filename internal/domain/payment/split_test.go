package payment

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func amounts(s Split) []int64 {
	out := make([]int64, len(s.Shares))
	for i, sh := range s.Shares {
		out[i] = sh.Amount.Minor
	}
	return out
}

func TestEvenly(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		n     int
		want  []int64
	}{
		{"a thousand three ways", 100000, 3, []int64{33334, 33333, 33333}},
		{"two paise over three", 2, 3, []int64{1, 1, 0}},
		{"one payer", 4200, 1, []int64{4200}},
		{"nothing to pay", 0, 4, []int64{0, 0, 0, 0}},
		{"eight ways", 100001, 8, []int64{12501, 12500, 12500, 12500, 12500, 12500, 12500, 12500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Evenly(inr(tt.total), tt.n)
			if err != nil {
				t.Fatal(err)
			}
			if got := amounts(s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shares %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByAmount(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		fixed []int64
		want  []int64
	}{
		{"one pays 500, another the rest", 128000, []int64{50000}, []int64{50000, 78000}},
		{"amounts that settle exactly", 100000, []int64{60000, 40000}, []int64{60000, 40000}},
		{"three fixed and the rest", 100000, []int64{10000, 20000, 30000}, []int64{10000, 20000, 30000, 40000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := make([]money.Amount, len(tt.fixed))
			for i, f := range tt.fixed {
				fixed[i] = inr(f)
			}
			s, err := ByAmount(inr(tt.total), fixed)
			if err != nil {
				t.Fatal(err)
			}
			if got := amounts(s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shares %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByItem(t *testing.T) {
	// Three friends. Asha had the pasta, Ben the burger, and all three
	// shared a pizza of 1000.01 that does not divide by three.
	items := []Item{
		{LineID: 2, Amount: inr(42000), Payers: []int{0}},
		{LineID: 3, Amount: inr(38000), Payers: []int{1}},
		{LineID: 4, Amount: inr(100001), Payers: []int{0, 1, 2}},
		{LineID: 5, Amount: inr(0), Payers: []int{2}}, // a comped drink, zero due
	}
	s, err := ByItem(inr(180001), 3, items)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := amounts(s), []int64{42000 + 33334, 38000 + 33334, 33333}; !reflect.DeepEqual(got, want) {
		t.Errorf("shares %v, want %v", got, want)
	}
	if got := s.Shares[2].LineIDs; !reflect.DeepEqual(got, []int64{4, 5}) {
		t.Errorf("payer 2 covers lines %v, want [4 5]", got)
	}
}

func TestByGuest(t *testing.T) {
	// A table of four: seats 1 and 2 had mains, seat 4 a dessert, and the
	// table shared a starter platter. Seat 3 only had the platter.
	items := []GuestItem{
		{LineID: 2, Amount: inr(45000), Guest: 1},
		{LineID: 3, Amount: inr(52000), Guest: 2},
		{LineID: 4, Amount: inr(24000), Guest: 4},
		{LineID: 5, Amount: inr(60002), Guest: SharedByTable}, // 150.01, 150.01, 150.00, 150.00
	}
	s, err := ByGuest(inr(181002), 4, items)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := amounts(s), []int64{60001, 67001, 15000, 39000}; !reflect.DeepEqual(got, want) {
		t.Errorf("shares %v, want %v", got, want)
	}
}

func TestTipsStayOutOfTheBill(t *testing.T) {
	s, err := Evenly(inr(100000), 3)
	if err != nil {
		t.Fatal(err)
	}
	tipped, err := s.WithTips([]money.Amount{inr(5000), inr(0), inr(10000)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(amounts(tipped), amounts(s)) {
		t.Errorf("tips changed the amounts: %v, was %v", amounts(tipped), amounts(s))
	}
	if tips, err := tipped.Tips(); err != nil || tips != inr(15000) {
		t.Errorf("tips = %+v, %v; want 15000", tips, err)
	}
	if err := tipped.Check(); err != nil {
		t.Errorf("a tipped split must still settle the bill exactly: %v", err)
	}
	if s.Shares[0].Tip != inr(0) {
		t.Error("WithTips changed the split it was called on")
	}
}

func TestStrategiesRefuse(t *testing.T) {
	usd := money.Amount{Minor: 100, Currency: "USD"}
	tests := []struct {
		name    string
		run     func() (Split, error)
		wantErr error
	}{
		{"evenly zero ways", func() (Split, error) { return Evenly(inr(100), 0) }, ErrInvalidSplit},
		{"evenly a negative bill", func() (Split, error) { return Evenly(inr(-100), 2) }, ErrInvalidSplit},
		{"evenly with no currency", func() (Split, error) { return Evenly(money.Amount{Minor: 1}, 2) }, money.ErrInvalidCurrency},
		{"by amount with no amounts", func() (Split, error) { return ByAmount(inr(100), nil) }, ErrInvalidSplit},
		{"by amount with a zero amount", func() (Split, error) { return ByAmount(inr(100), []money.Amount{inr(0)}) }, ErrInvalidSplit},
		{"by amount over the bill", func() (Split, error) { return ByAmount(inr(100), []money.Amount{inr(60), inr(41)}) }, ErrOverAllocated},
		{"by amount in dollars", func() (Split, error) { return ByAmount(inr(100), []money.Amount{usd}) }, money.ErrCurrencyMismatch},
		{"by item with no payers", func() (Split, error) { return ByItem(inr(100), 0, nil) }, ErrInvalidSplit},
		{"an item nobody pays for", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(100)}})
		}, ErrUnassigned},
		{"an item for payer 3 of 2", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(100), Payers: []int{2}}})
		}, ErrUnassigned},
		{"an item paid twice by one payer", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(100), Payers: []int{1, 1}}})
		}, ErrUnassigned},
		{"a negative item", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(-1), Payers: []int{0}}})
		}, ErrInvalidSplit},
		{"items short of the bill", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(99), Payers: []int{0}}})
		}, ErrMismatch},
		{"a line counted twice", func() (Split, error) {
			return ByItem(inr(100), 2, []Item{{LineID: 1, Amount: inr(100), Payers: []int{0}}, {LineID: 1, Amount: inr(100), Payers: []int{1}}})
		}, ErrMismatch},
		{"an item in dollars", func() (Split, error) {
			return ByItem(inr(100), 1, []Item{{LineID: 1, Amount: usd, Payers: []int{0}}})
		}, money.ErrCurrencyMismatch},
		{"by guest with no guests", func() (Split, error) { return ByGuest(inr(100), 0, nil) }, ErrInvalidSplit},
		{"a guest at seat 5 of 4", func() (Split, error) {
			return ByGuest(inr(100), 4, []GuestItem{{LineID: 1, Amount: inr(100), Guest: 5}})
		}, ErrUnassigned},
		{"a negative seat", func() (Split, error) {
			return ByGuest(inr(100), 4, []GuestItem{{LineID: 1, Amount: inr(100), Guest: -1}})
		}, ErrUnassigned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.run(); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckCatchesABrokenSplit(t *testing.T) {
	good, err := Evenly(inr(1000), 2)
	if err != nil {
		t.Fatal(err)
	}
	broken := func(f func(*Split)) Split {
		s := Split{Total: good.Total, Shares: append([]Share(nil), good.Shares...)}
		f(&s)
		return s
	}
	tests := []struct {
		name string
		s    Split
	}{
		{"a paisa lost", broken(func(s *Split) { s.Shares[1].Amount = inr(499) })},
		{"a paisa invented", broken(func(s *Split) { s.Shares[1].Amount = inr(501) })},
		{"a negative share", broken(func(s *Split) { s.Shares[0].Amount = inr(-1); s.Shares[1].Amount = inr(1001) })},
		{"a negative tip", broken(func(s *Split) { s.Shares[0].Tip = inr(-1) })},
		{"a tip in dollars", broken(func(s *Split) { s.Shares[0].Tip = money.Amount{Minor: 0, Currency: "USD"} })},
		{"shares out of order", broken(func(s *Split) { s.Shares[0].Payer = 1 })},
		{"an amount in dollars", broken(func(s *Split) { s.Shares[0].Amount = money.Amount{Minor: 500, Currency: "USD"} })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.Check(); err == nil {
				t.Error("Check passed a broken split")
			}
		})
	}
	if _, err := good.WithTips([]money.Amount{inr(1)}); !errors.Is(err, ErrInvalidSplit) {
		t.Errorf("one tip for two payers: err = %v", err)
	}
	if _, err := good.WithTips([]money.Amount{inr(1), inr(-1)}); !errors.Is(err, ErrInvalidSplit) {
		t.Errorf("a negative tip: err = %v", err)
	}
}

// TestEveryStrategySumsExactly is the property from phase 03: for random
// totals and two to eight payers, every strategy's shares sum to the bill
// exactly, no share is negative, and an even split never differs between
// payers by more than a paisa.
func TestEveryStrategySumsExactly(t *testing.T) {
	for seed := range uint64(2000) {
		rng := rand.New(rand.NewPCG(seed, 11))
		total := inr(rng.Int64N(10_000_000))
		n := 2 + rng.IntN(7)

		even, err := Evenly(total, n)
		mustSettle(t, seed, "evenly", even, err)
		lo, hi := even.Shares[0].Amount.Minor, even.Shares[0].Amount.Minor
		for _, sh := range even.Shares {
			lo, hi = min(lo, sh.Amount.Minor), max(hi, sh.Amount.Minor)
		}
		if hi-lo > 1 {
			t.Fatalf("seed %d: an even split ranges %d to %d", seed, lo, hi)
		}

		byAmount, err := ByAmount(total, randomAmounts(rng, total, n))
		if total.Minor > 0 {
			mustSettle(t, seed, "by amount", byAmount, err)
		}

		items := randomItems(rng, total, n)
		byItem, err := ByItem(total, n, items)
		mustSettle(t, seed, "by item", byItem, err)

		guests := make([]GuestItem, len(items))
		for i, it := range items {
			guests[i] = GuestItem{LineID: it.LineID, Amount: it.Amount, Guest: rng.IntN(n + 1)}
		}
		byGuest, err := ByGuest(total, n, guests)
		mustSettle(t, seed, "by guest", byGuest, err)
	}
}

func mustSettle(t *testing.T, seed uint64, strategy string, s Split, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("seed %d %s: %v", seed, strategy, err)
	}
	if err := s.Check(); err != nil {
		t.Fatalf("seed %d %s: %v", seed, strategy, err)
	}
}

// randomAmounts picks up to n-1 positive fixed amounts that fit in total.
func randomAmounts(rng *rand.Rand, total money.Amount, n int) []money.Amount {
	var out []money.Amount
	left := total.Minor
	for range 1 + rng.IntN(n-1) {
		if left == 0 {
			break
		}
		a := 1 + rng.Int64N(left)
		out = append(out, inr(a))
		left -= a
	}
	return out
}

// randomItems cuts total into one to twelve lines and gives each a random,
// non-empty set of payers.
func randomItems(rng *rand.Rand, total money.Amount, n int) []Item {
	count := 1 + rng.IntN(12)
	items := make([]Item, count)
	left := total.Minor
	for i := range items {
		amount := left
		if i < count-1 {
			amount = rng.Int64N(left + 1)
		}
		left -= amount
		payers := rng.Perm(n)[:1+rng.IntN(n)]
		items[i] = Item{LineID: int64(i + 2), Amount: inr(amount), Payers: payers}
	}
	return items
}
