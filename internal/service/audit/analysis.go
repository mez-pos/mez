package audit

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// Tally is how many corrections, and what they were worth. A reopen is
// worth nothing: it counts and adds no amount.
type Tally struct {
	Count  int64        `json:"count"`
	Amount money.Amount `json:"amount"`
}

// KindTally is one kind's corrections.
type KindTally struct {
	Kind Kind `json:"kind"`
	Tally
}

// ReasonTally is one kind's corrections for one reason: a reason code
// ("wrong_item") or the words typed; empty when none was given.
type ReasonTally struct {
	Kind   Kind   `json:"kind"`
	Reason string `json:"reason"`
	Tally
}

// PersonTally is one person's corrections: all of them, and by kind.
type PersonTally struct {
	// Person is absent for nobody on record: a guest, as who did it; not
	// recorded (a discount or refund before ADR-117), as who authorised it.
	Person *Person `json:"person,omitempty"`
	Tally
	// Kinds holds every kind, none included, in Kinds' order.
	Kinds []KindTally `json:"kinds"`
}

// Analysis is a filter's corrections added up (phase 10, Task 5e1,
// ADR-122): the same entries the trail lists, grouped.
type Analysis struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Currency string `json:"currency"`
	Total    Tally  `json:"total"`
	// ByKind holds every kind, none included, in Kinds' order.
	ByKind []KindTally `json:"by_kind"`
	// The rest are largest amount first.
	ByReason     []ReasonTally `json:"by_reason"`
	By           []PersonTally `json:"by"`
	AuthorisedBy []PersonTally `json:"authorised_by"`
}

// AnalysisSheet is one grouping of the analysis as a table, for a CSV.
type AnalysisSheet string

// The sheets.
const (
	SheetReason     AnalysisSheet = "reason"
	SheetBy         AnalysisSheet = "by"
	SheetAuthorised AnalysisSheet = "authorised_by"
)

// AnalysisSheets is every sheet, for the generated vocabulary.
func AnalysisSheets() []AnalysisSheet {
	return []AnalysisSheet{SheetReason, SheetBy, SheetAuthorised}
}

// ErrNoSheet refuses a sheet the analysis does not have.
var ErrNoSheet = errors.New("no such grouping of the corrections")

// Analysis adds up everything the filter finds. Before and Limit are not
// read: it is the whole filter, as the trail's CSV is.
func (s *Service) Analysis(ctx context.Context, venueID int64, f Filter) (Analysis, error) {
	q := s.db.Queries()
	f.Before, f.Limit = 0, MaxPage
	ask, err := asked(ctx, q, venueID, f)
	if err != nil {
		return Analysis{}, err
	}
	v, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return Analysis{}, fmt.Errorf("audit: reading venue %d: %w", venueID, err)
	}
	entries, err := s.everything(ctx, q, ask)
	if err != nil {
		return Analysis{}, err
	}
	a, err := analyse(entries, v.Currency)
	if err != nil {
		return Analysis{}, err
	}
	a.From, a.To = f.From, f.To
	return a, nil
}

// AnalysisTable is one grouping of the analysis as a table, for a CSV
// download: the same figures Analysis answers with.
func (s *Service) AnalysisTable(ctx context.Context, venueID int64, f Filter, sheet AnalysisSheet) (export.Table, error) {
	// Before the trail is read: a wrong name should not cost a year of it.
	if !slices.Contains(AnalysisSheets(), sheet) {
		return export.Table{}, fmt.Errorf("%w: %q", ErrNoSheet, string(sheet))
	}
	a, err := s.Analysis(ctx, venueID, f)
	if err != nil {
		return export.Table{}, err
	}
	return a.Table(sheet)
}

// everything reads every page of a filter, newest first, projecting each
// order once.
func (s *Service) everything(ctx context.Context, q *gen.Queries, ask asking) ([]Entry, error) {
	var all []Entry
	states := map[int64]order.State{}
	for {
		entries, next, err := s.entries(ctx, q, ask, states)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
		if next == 0 {
			return all, nil
		}
		ask.params.BeforeID = next
	}
}

// analyse groups entries. An amount in another currency than the venue's
// is refused: a sum across currencies would be a number that means nothing.
func analyse(entries []Entry, currency string) (Analysis, error) {
	zero := money.Amount{Currency: currency}
	a := Analysis{Currency: currency, Total: Tally{Amount: zero}, ByReason: []ReasonTally{}}
	kinds := newKinds(zero)
	reasons := map[[2]string]*ReasonTally{}
	did, let := newPeople(zero), newPeople(zero)

	for _, e := range entries {
		worth := int64(0)
		if e.Amount != nil {
			if e.Amount.Currency != currency {
				return Analysis{}, fmt.Errorf("audit: entry %d is in %s, the venue in %s", e.ID, e.Amount.Currency, currency)
			}
			worth = e.Amount.Minor
		}
		add := func(t *Tally) {
			t.Count++
			t.Amount.Minor += worth
		}
		add(&a.Total)
		if i := slices.Index(Kinds(), e.Kind); i >= 0 {
			add(&kinds[i].Tally)
		}
		key := [2]string{string(e.Kind), e.Reason}
		r, ok := reasons[key]
		if !ok {
			r = &ReasonTally{Kind: e.Kind, Reason: e.Reason, Tally: Tally{Amount: zero}}
			reasons[key] = r
		}
		add(&r.Tally)
		did.add(e.By, e.Kind, worth)
		let.add(e.AuthorisedBy, e.Kind, worth)
	}

	a.ByKind = kinds
	for _, r := range reasons {
		a.ByReason = append(a.ByReason, *r)
	}
	slices.SortFunc(a.ByReason, func(x, y ReasonTally) int {
		return cmp.Or(
			cmp.Compare(y.Amount.Minor, x.Amount.Minor), cmp.Compare(y.Count, x.Count),
			cmp.Compare(slices.Index(Kinds(), x.Kind), slices.Index(Kinds(), y.Kind)), cmp.Compare(x.Reason, y.Reason),
		)
	})
	a.By, a.AuthorisedBy = did.sorted(), let.sorted()
	return a, nil
}

// newKinds is every kind with nothing against it.
func newKinds(zero money.Amount) []KindTally {
	out := make([]KindTally, len(Kinds()))
	for i, k := range Kinds() {
		out[i] = KindTally{Kind: k, Tally: Tally{Amount: zero}}
	}
	return out
}

// peopleTally groups by person. Someone is known by their public id; those
// with none (nobody, or someone no longer on record) by their name.
type peopleTally struct {
	zero  money.Amount
	byKey map[string]*PersonTally
}

func newPeople(zero money.Amount) peopleTally {
	return peopleTally{zero: zero, byKey: map[string]*PersonTally{}}
}

func (p peopleTally) add(who *Person, kind Kind, worth int64) {
	key := "nobody"
	if who != nil {
		key = "id:" + who.ID + "\x00" + who.Name
	}
	t, ok := p.byKey[key]
	if !ok {
		t = &PersonTally{Tally: Tally{Amount: p.zero}, Kinds: newKinds(p.zero)}
		if who != nil {
			person := *who
			t.Person = &person
		}
		p.byKey[key] = t
	}
	t.Count++
	t.Amount.Minor += worth
	if i := slices.Index(Kinds(), kind); i >= 0 {
		t.Kinds[i].Count++
		t.Kinds[i].Amount.Minor += worth
	}
}

// sorted is the people, largest amount first, then most corrections, then
// by name, nobody last.
func (p peopleTally) sorted() []PersonTally {
	out := make([]PersonTally, 0, len(p.byKey))
	for _, t := range p.byKey {
		out = append(out, *t)
	}
	nobody := func(t PersonTally) int {
		if t.Person == nil {
			return 1
		}
		return 0
	}
	name := func(t PersonTally) string {
		if t.Person == nil {
			return ""
		}
		return t.Person.Name + " " + t.Person.ID
	}
	slices.SortFunc(out, func(x, y PersonTally) int {
		return cmp.Or(
			cmp.Compare(y.Amount.Minor, x.Amount.Minor), cmp.Compare(y.Count, x.Count),
			cmp.Compare(nobody(x), nobody(y)), cmp.Compare(name(x), name(y)),
		)
	})
	return out
}

// Table is one grouping as a table: money in major units with the currency
// in the header (ADR-119), typed text guarded against running as a formula.
func (a Analysis) Table(sheet AnalysisSheet) (export.Table, error) {
	amount := "Amount (" + a.Currency + ")"
	count := func(n int64) string { return strconv.FormatInt(n, 10) }
	switch sheet {
	case SheetReason:
		t := export.Table{Header: []string{"Kind", "Reason", "Count", amount}, Rows: [][]string{}}
		for _, r := range a.ByReason {
			t.Rows = append(t.Rows, []string{kindWords[r.Kind], export.Text(r.Reason), count(r.Count), export.Decimal(r.Amount)})
		}
		return t, nil
	case SheetBy:
		return peopleTable("By", "A guest", a.By, amount), nil
	case SheetAuthorised:
		return peopleTable("Authorised by", "Not recorded", a.AuthorisedBy, amount), nil
	}
	return export.Table{}, fmt.Errorf("%w: %q", ErrNoSheet, string(sheet))
}

// peopleTable is a row per person: all their corrections, then how many of
// each kind.
func peopleTable(heading, nobody string, people []PersonTally, amount string) export.Table {
	t := export.Table{Header: []string{heading, "Count", amount}, Rows: [][]string{}}
	for _, k := range Kinds() {
		t.Header = append(t.Header, kindWords[k]+"s")
	}
	for _, p := range people {
		name := nobody
		if p.Person != nil {
			name = export.Text(p.Person.Name)
		}
		row := []string{name, strconv.FormatInt(p.Count, 10), export.Decimal(p.Amount)}
		for _, k := range p.Kinds {
			row = append(row, strconv.FormatInt(k.Count, 10))
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}
