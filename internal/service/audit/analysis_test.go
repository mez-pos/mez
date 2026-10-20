package audit

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
)

func tally(count, minor int64) Tally { return Tally{Count: count, Amount: inr(minor)} }

// The day of audit_test.go, worked out by hand. Asha did all eight:
//
//	void     Chai   30.00  wrong_item     Priya
//	void     Thali 120.00  out_of_stock   Priya
//	comp     Vada   50.00  regular        Priya
//	discount Coffee 60.00  happy hour     Priya
//	discount bill   10.00  loyalty        Asha
//	payment void    10.00  card declined  Priya
//	refund          50.00  cold food      Priya
//	reopen                 wrong tender   Priya
func TestTheDayAddedUp(t *testing.T) {
	f := newFixture(t)
	f.aDay(t)
	got, err := f.s.Analysis(context.Background(), 1, Filter{From: "2026-10-01", To: "2026-10-01", Limit: 3, Before: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "2026-10-01" || got.To != "2026-10-01" || got.Currency != "INR" {
		t.Errorf("range %s to %s in %s", got.From, got.To, got.Currency)
	}
	if got.Total != tally(8, 33000) {
		t.Errorf("total %+v, want 8 worth 330.00: a page size and cursor are not read", got.Total)
	}
	wantKinds := []KindTally{
		{Void, tally(2, 15000)}, {Comp, tally(1, 5000)}, {Discount, tally(2, 7000)},
		{Refund, tally(1, 5000)}, {PaymentVoid, tally(1, 1000)}, {Reopen, tally(1, 0)}, {TipApproved, tally(0, 0)},
	}
	if !slices.Equal(got.ByKind, wantKinds) {
		t.Errorf("by kind %+v, want %+v", got.ByKind, wantKinds)
	}

	if len(got.ByReason) != 8 {
		t.Fatalf("%d reasons, want 8: %+v", len(got.ByReason), got.ByReason)
	}
	if first := got.ByReason[0]; first != (ReasonTally{Void, "out_of_stock", tally(1, 12000)}) {
		t.Errorf("the largest reason is %+v, want the 120.00 void", first)
	}
	if last := got.ByReason[7]; last != (ReasonTally{Reopen, "wrong tender", tally(1, 0)}) {
		t.Errorf("the smallest reason is %+v, want the reopen", last)
	}

	if len(got.By) != 1 || got.By[0].Person == nil || got.By[0].Person.Name != "Asha" || got.By[0].Tally != tally(8, 33000) {
		t.Fatalf("by %+v, want Asha with all eight", got.By)
	}
	if !slices.Equal(got.By[0].Kinds, wantKinds) {
		t.Errorf("Asha by kind %+v", got.By[0].Kinds)
	}
	if len(got.AuthorisedBy) != 2 {
		t.Fatalf("authorised by %+v, want Priya then Asha", got.AuthorisedBy)
	}
	priya, asha := got.AuthorisedBy[0], got.AuthorisedBy[1]
	if priya.Person.Name != "Priya" || priya.Tally != tally(7, 32000) || priya.Person.ID != "stf_PRIYA001" {
		t.Errorf("Priya authorised %+v, want 7 worth 320.00", priya)
	}
	if asha.Person.Name != "Asha" || asha.Tally != tally(1, 1000) || asha.Kinds[2] != (KindTally{Discount, tally(1, 1000)}) {
		t.Errorf("Asha authorised %+v, want her own 10.00 discount", asha)
	}
}

func TestTheAnalysisIsFiltered(t *testing.T) {
	f := newFixture(t)
	f.aDay(t)
	ctx := context.Background()
	tests := []struct {
		name string
		f    Filter
		want Tally
	}{
		{"only voids", Filter{From: "2026-10-01", To: "2026-10-01", Kind: Void}, tally(2, 15000)},
		{"what Priya did or authorised", Filter{From: "2026-10-01", To: "2026-10-01", StaffID: "stf_PRIYA001"}, tally(7, 32000)},
		{"the day before", day("2026-09-30", "2026-09-30"), tally(0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.s.Analysis(ctx, 1, tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if got.Total != tt.want {
				t.Errorf("total %+v, want %+v", got.Total, tt.want)
			}
			if len(got.ByKind) != len(Kinds()) || got.ByReason == nil || got.By == nil || got.AuthorisedBy == nil {
				t.Errorf("a quiet answer still has every list: %+v", got)
			}
		})
	}
	refusals := []struct {
		name string
		f    Filter
		is   error
	}{
		{"backwards", day("2026-10-02", "2026-10-01"), ErrBadFilter},
		{"someone not here", Filter{From: "2026-10-01", To: "2026-10-01", StaffID: "stf_NOBODY01"}, ErrStaffNotFound},
		{"a kind we do not have", Filter{From: "2026-10-01", To: "2026-10-01", Kind: "theft"}, ErrBadFilter},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.s.Analysis(ctx, 1, tt.f); !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want %v", err, tt.is)
			}
		})
	}
}

func TestTheAnalysisAsTables(t *testing.T) {
	f := newFixture(t)
	f.aDay(t)
	ctx := context.Background()
	all := day("2026-10-01", "2026-10-01")
	people := []string{"Count", "Amount (INR)", "Voids", "Comps", "Discounts", "Refunds", "Payment voids", "Reopens", "Tips approved"}
	tests := []struct {
		sheet  AnalysisSheet
		header []string
		rows   [][]string
	}{
		{SheetReason, []string{"Kind", "Reason", "Count", "Amount (INR)"}, [][]string{
			{"Void", "out_of_stock", "1", "120.00"},
			{"Discount", "happy hour", "1", "60.00"},
			{"Reopen", "wrong tender", "1", "0.00"},
		}},
		{SheetBy, append([]string{"By"}, people...), [][]string{
			{"Asha", "8", "330.00", "2", "1", "2", "1", "1", "1", "0"},
		}},
		{SheetAuthorised, append([]string{"Authorised by"}, people...), [][]string{
			{"Priya", "7", "320.00", "2", "1", "1", "1", "1", "1", "0"},
			{"Asha", "1", "10.00", "0", "0", "1", "0", "0", "0", "0"},
		}},
	}
	for _, tt := range tests {
		t.Run(string(tt.sheet), func(t *testing.T) {
			got, err := f.s.AnalysisTable(ctx, 1, all, tt.sheet)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Header, tt.header) {
				t.Errorf("header %q, want %q", got.Header, tt.header)
			}
			for _, want := range tt.rows {
				if !slices.ContainsFunc(got.Rows, func(row []string) bool { return slices.Equal(row, want) }) {
					t.Errorf("no row %q in %q", want, got.Rows)
				}
			}
			if _, err := got.CSV(); err != nil {
				t.Errorf("as CSV: %v", err)
			}
		})
	}
	for _, sheet := range []AnalysisSheet{"", "kind", "By"} {
		if _, err := f.s.AnalysisTable(ctx, 1, all, sheet); !errors.Is(err, ErrNoSheet) {
			t.Errorf("%q: err %v, want ErrNoSheet", sheet, err)
		}
		if _, err := (Analysis{}).Table(sheet); !errors.Is(err, ErrNoSheet) {
			t.Errorf("%q: Table err %v, want ErrNoSheet", sheet, err)
		}
	}
}

// analyse on its own: the grouping and ordering rules, with no database.
func TestAnalyse(t *testing.T) {
	asha, ravi := &Person{ID: "stf_A", Name: "Asha"}, &Person{ID: "stf_R", Name: "Ravi"}
	gone := &Person{Name: "Someone no longer on record"}
	amt := func(minor int64) *money.Amount { a := inr(minor); return &a }
	entries := []Entry{
		{ID: 1, Kind: Void, By: asha, AuthorisedBy: ravi, Reason: "wrong_item", Amount: amt(3000)},
		{ID: 2, Kind: Void, By: asha, AuthorisedBy: ravi, Reason: "wrong_item", Amount: amt(4000)},
		{ID: 3, Kind: Refund, By: ravi, Reason: "cold food", Amount: amt(7000)},       // nobody recorded as authorising
		{ID: 4, Kind: Discount, AuthorisedBy: gone, Reason: "=1+1", Amount: amt(500)}, // a guest's, by someone since removed
		{ID: 5, Kind: Reopen, By: ravi, AuthorisedBy: ravi},                           // no reason, no amount
	}
	got, err := analyse(entries, "INR")
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != tally(5, 14500) {
		t.Errorf("total %+v", got.Total)
	}
	// Equal amounts: the more frequent first.
	wantReasons := []ReasonTally{
		{Void, "wrong_item", tally(2, 7000)}, {Refund, "cold food", tally(1, 7000)},
		{Discount, "=1+1", tally(1, 500)}, {Reopen, "", tally(1, 0)},
	}
	if !slices.Equal(got.ByReason, wantReasons) {
		t.Errorf("by reason %+v, want %+v", got.ByReason, wantReasons)
	}
	names := func(ps []PersonTally) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = "nobody"
			if p.Person != nil {
				out[i] = p.Person.Name
			}
		}
		return out
	}
	// Asha and Ravi are both worth 70.00; Ravi has two, Asha two: by name.
	if by := names(got.By); !slices.Equal(by, []string{"Asha", "Ravi", "nobody"}) {
		t.Errorf("by %q", by)
	}
	if let := names(got.AuthorisedBy); !slices.Equal(let, []string{"Ravi", "nobody", "Someone no longer on record"}) {
		t.Errorf("authorised by %q", let)
	}

	table, err := got.Table(SheetReason)
	if err != nil {
		t.Fatal(err)
	}
	if row := table.Rows[2]; row[1] != "'=1+1" {
		t.Errorf("row %q: a reason typed like a formula must stay text", row)
	}
	who, _ := got.Table(SheetBy)
	let, _ := got.Table(SheetAuthorised)
	if who.Rows[2][0] != "A guest" || let.Rows[1][0] != "Not recorded" {
		t.Errorf("nobody is %q as who did it and %q as who authorised it", who.Rows[2][0], let.Rows[1][0])
	}

	quiet, err := analyse(nil, "INR")
	if err != nil || quiet.Total != tally(0, 0) || len(quiet.ByKind) != len(Kinds()) || quiet.ByReason == nil || quiet.By == nil {
		t.Errorf("nothing added up: %+v, err %v", quiet, err)
	}
	usd := money.Amount{Minor: 100, Currency: "USD"}
	if _, err := analyse([]Entry{{ID: 9, Kind: Refund, Amount: &usd}}, "INR"); err == nil {
		t.Error("an amount in another currency must be refused, not summed")
	}
}
