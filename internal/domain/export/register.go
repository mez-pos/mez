package export

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// The register's column keys this package can fill, from every profile's
// RegisterColumns. A dimension is text a row is grouped by; an amount is
// summed over the rows that share every dimension. A profile naming a key
// not here fails the export, and a test walks every profile's columns, so
// a new regime's register is filled on purpose or not at all.
var (
	dimensionKeys = []string{"invoice_no", "invoice_date", "place_of_supply", "hsn", "rate", "jurisdiction", "invoice_value"}
	amountKeys    = []string{"taxable_value", "net", "taxable", "exempt", "cgst", "sgst", "igst", "vat", "tax", "gross"}
)

// componentKey is the column a tax component's amount goes in. A union
// territory's UTGST shares the state's column, as the label says.
var componentKey = map[string]string{"CGST": "cgst", "SGST": "sgst", "UTGST": "sgst", "IGST": "igst", "VAT": "vat"}

// fineRow is one line of one document (or one authority's part of it)
// before rows are grouped.
type fineRow struct {
	dims    map[string]string
	amounts map[string]int64
}

// register fills the profile's register columns. With no profile, an
// untaxed venue's register is its invoices and their values.
func register(in Input, docs []document) (Table, error) {
	cols := registerColumns(in.Taxes.Profile)
	for _, c := range cols {
		if !slices.Contains(dimensionKeys, c.Key) && !slices.Contains(amountKeys, c.Key) {
			return Table{}, fmt.Errorf("%w: the register has a %q column this build cannot fill", ErrExport, c.Key)
		}
	}
	keys := make([]string, len(cols))
	t := Table{Header: make([]string, len(cols))}
	for i, c := range cols {
		keys[i], t.Header[i] = c.Key, c.Label
	}
	perAuthority := slices.Contains(keys, "jurisdiction")
	hasExempt := slices.Contains(keys, "exempt")

	var rows []fineRow
	for _, d := range docs {
		for _, lt := range d.Detail.Lines {
			rows = append(rows, fineRows(in, d, lt, perAuthority, hasExempt)...)
		}
	}
	t.Rows = group(rows, keys, in.Currency)
	return t, nil
}

func registerColumns(p tax.TaxProfile) []tax.Column {
	if p == nil {
		return []tax.Column{
			{Key: "invoice_no", Label: "Invoice No."},
			{Key: "invoice_date", Label: "Date"},
			{Key: "gross", Label: "Value"},
		}
	}
	return p.RegisterColumns()
}

// fineRows is a line's register rows: one, or one per authority when the
// register is kept by authority.
func fineRows(in Input, d document, lt tax.LineTax, perAuthority, hasExempt bool) []fineRow {
	dims := map[string]string{
		"invoice_no":      d.Number,
		"invoice_date":    d.At.Format("2006-01-02"),
		"place_of_supply": in.Taxes.Venue.StateCode,
		"hsn":             lt.HSN,
		"rate":            rateText(lt.Rate),
		"invoice_value":   Decimal(d.Detail.Total),
	}
	exempt := len(lt.Components) == 0
	if perAuthority {
		if exempt {
			return []fineRow{{dims: with(dims, "jurisdiction", "", "rate", rateText(0)), amounts: map[string]int64{"exempt": lt.Taxable.Minor}}}
		}
		out := make([]fineRow, len(lt.Components))
		for i, c := range lt.Components {
			out[i] = fineRow{
				dims:    with(dims, "jurisdiction", c.Name, "rate", rateText(c.Rate)),
				amounts: map[string]int64{"taxable": lt.Taxable.Minor, "tax": c.Amount.Minor, "gross": lt.Total.Minor},
			}
		}
		return out
	}
	a := map[string]int64{"tax": lt.Tax.Minor, "gross": lt.Total.Minor}
	value := lt.Taxable.Minor
	if exempt && hasExempt {
		a["exempt"] = value
	} else {
		a["taxable_value"], a["net"], a["taxable"] = value, value, value
	}
	for _, c := range lt.Components {
		if k, ok := componentKey[strings.ToUpper(c.Name)]; ok {
			a[k] += c.Amount.Minor
		}
	}
	return []fineRow{{dims: dims, amounts: a}}
}

// with is dims with two keys replaced.
func with(dims map[string]string, k1, v1, k2, v2 string) map[string]string {
	out := make(map[string]string, len(dims))
	maps.Copy(out, dims)
	out[k1], out[k2] = v1, v2
	return out
}

// group sums the rows that share every dimension column, in the order
// each group first appears.
func group(rows []fineRow, keys []string, currency string) [][]string {
	type sum struct {
		dims    []string
		amounts map[string]int64
	}
	var order []string
	groups := map[string]*sum{}
	for _, r := range rows {
		var id []string
		for _, k := range keys {
			if slices.Contains(dimensionKeys, k) {
				id = append(id, r.dims[k])
			}
		}
		key := strings.Join(id, "\x00")
		g, ok := groups[key]
		if !ok {
			g = &sum{dims: id, amounts: map[string]int64{}}
			groups[key] = g
			order = append(order, key)
		}
		for k, v := range r.amounts {
			g.amounts[k] += v
		}
	}
	out := make([][]string, 0, len(order))
	for _, key := range order {
		g := groups[key]
		row := make([]string, len(keys))
		d := 0
		for i, k := range keys {
			if slices.Contains(dimensionKeys, k) {
				row[i] = g.dims[d]
				d++
				continue
			}
			row[i] = Decimal(money.Amount{Minor: g.amounts[k], Currency: currency})
		}
		out = append(out, row)
	}
	return out
}
