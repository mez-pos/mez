package tax

import (
	"fmt"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// assemble turns the worked lines into a TaxResult: each line's taxable
// value, tax and total, the rate-wise summary, and the order totals.
func assemble(profile string, ctx VenueContext, ws []work) (TaxResult, error) {
	cur := ctx.Currency
	res := TaxResult{Profile: profile, Pricing: ctx.Pricing, Lines: make([]LineTax, 0, len(ws))}
	var summary []ComponentTotal
	find := func(c Component) *ComponentTotal {
		for i := range summary {
			if summary[i].Name == c.Name && summary[i].Rate == c.Rate {
				return &summary[i]
			}
		}
		summary = append(summary, ComponentTotal{Name: c.Name, Rate: c.Rate, Taxable: zero(cur), Amount: zero(cur)})
		return &summary[len(summary)-1]
	}

	var gross, taxable, taxTotal, total []money.Amount
	for _, w := range ws {
		lt, err := lineTax(w, ctx)
		if err != nil {
			return TaxResult{}, fmt.Errorf("line %d: %w", w.line.ID, err)
		}
		for _, ca := range lt.Components {
			ct := find(Component{Name: ca.Name, Rate: ca.Rate})
			if ct.Taxable, err = ct.Taxable.Add(lt.Taxable); err != nil {
				return TaxResult{}, err
			}
			if ct.Amount, err = ct.Amount.Add(ca.Amount); err != nil {
				return TaxResult{}, err
			}
		}
		res.Lines = append(res.Lines, lt)
		gross = append(gross, w.line.Amount)
		taxable = append(taxable, lt.Taxable)
		taxTotal = append(taxTotal, lt.Tax)
		total = append(total, lt.Total)
	}
	res.Components = summary

	var err error
	for _, t := range []struct {
		dst *money.Amount
		src []money.Amount
	}{{&res.Gross, gross}, {&res.Taxable, taxable}, {&res.Tax, taxTotal}, {&res.Total, total}} {
		if *t.dst, err = money.Sum(cur, t.src...); err != nil {
			return TaxResult{}, err
		}
	}
	return res, nil
}

// lineTax is one line's figures. Under inclusive pricing the taxable value
// is whatever the price leaves after tax, so the line totals exactly.
func lineTax(w work, ctx VenueContext) (LineTax, error) {
	cur := ctx.Currency
	lt := LineTax{LineID: w.line.ID, HSN: w.line.HSN, Components: make([]ComponentAmount, len(w.comps))}
	var tax int64
	for j, c := range w.comps {
		lt.Rate += c.Rate
		lt.Components[j] = ComponentAmount{Name: c.Name, Rate: c.Rate, Amount: money.Amount{Minor: w.tax[j], Currency: cur}}
		tax += w.tax[j]
	}
	lt.Tax = money.Amount{Minor: tax, Currency: cur}

	var err error
	if ctx.Pricing == Inclusive {
		if lt.Taxable, err = w.line.Amount.Sub(lt.Tax); err != nil {
			return LineTax{}, err
		}
		if lt.Taxable.Minor < 0 {
			return LineTax{}, fmt.Errorf("%w: tax %d exceeds the price %d", ErrInvalidLine, tax, w.line.Amount.Minor)
		}
	} else {
		lt.Taxable = w.line.Amount
	}
	if lt.Total, err = lt.Taxable.Add(lt.Tax); err != nil {
		return LineTax{}, err
	}
	return lt, nil
}

func zero(currency string) money.Amount { return money.Amount{Minor: 0, Currency: currency} }
