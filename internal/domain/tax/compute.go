package tax

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// componentsFunc is a profile's one decision per line: which components tax
// it, at what rates. An exempt or zero-rated line has none.
type componentsFunc func(Line) ([]Component, error)

// work is one line in flight.
type work struct {
	line  Line
	comps []Component
	base  money.Amount // the value tax is charged on, before inclusive absorption
	tax   []int64      // minor units per component
}

// compute is the engine every profile shares. The rules, in order:
//
//  1. Each line's taxable base: its Amount if prices exclude tax, or the
//     Amount with tax backed out (Amount × 100% / (100% + rate), rounded
//     half to even) if they include it.
//  2. Line-level rounding: each component on each line is base × rate,
//     rounded half to even.
//  3. Order-level reconciliation: for each group of lines sharing the same
//     components, each component's total must equal the group's summed base
//     × rate, rounded once. Any difference is settled one minor unit at a
//     time on the lines whose rounding was closest to the other way (largest
//     remainder), earliest line first on a tie.
//  4. Inclusive pricing: the line's taxable value absorbs whatever the
//     rounding left, so taxable + tax is the menu price to the paisa.
//
// Components with the same rate on the same lines, CGST and SGST, see the
// same inputs and so always come out equal, line by line.
func compute(profile string, lines []Line, ctx VenueContext, specs componentsFunc) (TaxResult, error) {
	if _, err := money.New(0, ctx.Currency); err != nil {
		return TaxResult{}, fmt.Errorf("%w: currency: %v", ErrInvalidContext, err)
	}
	if ctx.Pricing != Inclusive && ctx.Pricing != Exclusive {
		return TaxResult{}, fmt.Errorf("%w: pricing %q", ErrInvalidContext, ctx.Pricing)
	}
	ws := make([]work, len(lines))
	for i, l := range lines {
		w, err := prepare(l, ctx, specs)
		if err != nil {
			return TaxResult{}, fmt.Errorf("line %d: %w", l.ID, err)
		}
		ws[i] = w
	}
	for _, group := range groups(ws) {
		for j := range ws[group[0]].comps {
			if err := reconcile(ws, group, j); err != nil {
				return TaxResult{}, err
			}
		}
	}
	return assemble(profile, ctx, ws)
}

// prepare checks a line and works out its components and taxable base.
func prepare(l Line, ctx VenueContext, specs componentsFunc) (work, error) {
	if l.Amount.Currency != ctx.Currency {
		return work{}, fmt.Errorf("%w: amount in %q, venue in %q", ErrInvalidLine, l.Amount.Currency, ctx.Currency)
	}
	if l.Amount.Minor < 0 {
		return work{}, fmt.Errorf("%w: amount %d is negative", ErrInvalidLine, l.Amount.Minor)
	}
	var comps []Component
	if !l.Exempt {
		var err error
		if comps, err = specs(l); err != nil {
			return work{}, err
		}
	}
	total := Rate(0)
	for _, c := range comps {
		if err := c.Rate.Validate(); err != nil {
			return work{}, fmt.Errorf("%s: %w", c.Name, err)
		}
		total += c.Rate
	}
	if err := total.Validate(); err != nil {
		return work{}, err
	}
	base := l.Amount
	if ctx.Pricing == Inclusive && total > 0 {
		var err error
		if base, err = l.Amount.Scale(RateScale, RateScale+int64(total)); err != nil {
			return work{}, err
		}
	}
	return work{line: l, comps: comps, base: base, tax: make([]int64, len(comps))}, nil
}

// groups collects lines taxed by the same components at the same rates, in
// order of first appearance, so results are deterministic.
func groups(ws []work) [][]int {
	var keys []string
	byKey := map[string][]int{}
	for i, w := range ws {
		if len(w.comps) == 0 {
			continue
		}
		parts := make([]string, len(w.comps))
		for j, c := range w.comps {
			parts[j] = fmt.Sprintf("%s@%d", c.Name, c.Rate)
		}
		k := strings.Join(parts, "|")
		if _, seen := byKey[k]; !seen {
			keys = append(keys, k)
		}
		byKey[k] = append(byKey[k], i)
	}
	out := make([][]int, len(keys))
	for i, k := range keys {
		out[i] = byKey[k]
	}
	return out
}

// reconcile sets component j on every line of a group: rounded per line,
// then settled to the group total. See compute for the rule.
func reconcile(ws []work, group []int, j int) error {
	rate := int64(ws[group[0]].comps[j].Rate)
	if rate == 0 {
		return nil
	}
	bases := make([]money.Amount, len(group))
	for k, i := range group {
		bases[k] = ws[i].base
	}
	sum, err := money.Sum(ws[group[0]].base.Currency, bases...)
	if err != nil {
		return err
	}
	target, err := sum.Scale(rate, RateScale)
	if err != nil {
		return err
	}

	type candidate struct {
		i        int
		residual int64 // exact tax × RateScale minus the rounded tax × RateScale
	}
	cands := make([]candidate, len(group))
	var rounded int64
	for k, i := range group {
		t, err := ws[i].base.Scale(rate, RateScale)
		if err != nil {
			return err
		}
		// base × rate cannot overflow: Scale above just computed it.
		cands[k] = candidate{i: i, residual: ws[i].base.Minor*rate - t.Minor*RateScale}
		ws[i].tax[j] = t.Minor
		rounded += t.Minor
	}

	diff := target.Minor - rounded
	step := int64(1)
	if diff < 0 {
		step, diff = -1, -diff
	}
	// Up: the lines rounded furthest down first. Down: the lines rounded
	// furthest up first. A stable sort keeps line order on a tie.
	slices.SortStableFunc(cands, func(a, b candidate) int {
		if step > 0 {
			return cmp.Compare(b.residual, a.residual)
		}
		return cmp.Compare(a.residual, b.residual)
	})
	if diff > int64(len(cands)) {
		return fmt.Errorf("tax: reconciling %d units over %d lines", diff, len(cands))
	}
	for k := range diff {
		ws[cands[k].i].tax[j] += step
	}
	return nil
}
