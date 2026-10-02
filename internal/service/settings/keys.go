package settings

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/invoice"
	"github.com/byjoellobo/mez/internal/domain/tips"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// key is one setting the venue can hold: its name in the settings table, what
// it is for, the value it takes when unset, and the check that refuses a value
// the code reading it would later call a venue config error.
//
// The check is about the value alone. Rules that span keys (a GST profile
// needing a state code, a series that must differ from the credit series)
// belong to the task that edits those keys together.
type key struct {
	Name    string
	Default string
	Check   func(value string) string // "" when fine, else what to tell the owner
}

// keys is every key anything reads from the settings table. Order is the
// order the API lists them in.
var keys = []key{
	{Name: svcorder.SettingTaxPricing, Check: oneOf("inclusive", "exclusive")},
	{Name: svcorder.SettingTaxStateCode, Check: stateCode},
	{Name: svcorder.SettingTaxJurisdiction, Check: jurisdiction},
	{Name: svcorder.SettingInvoiceSeries, Default: "INV", Check: seriesCode},
	{Name: svcorder.SettingInvoicePrefix, Check: prefix},
	{Name: svcorder.SettingInvoiceTemplate, Default: invoice.DefaultTemplate, Check: template},
	{Name: svcorder.SettingCreditSeries, Default: "CN", Check: seriesCode},
	{Name: svcorder.SettingCreditPrefix, Check: prefix},
	{Name: svcorder.SettingTicketLateMinutes, Default: "20", Check: wholeNumber(1, 600, "minutes")},
	{Name: svcshift.SettingVarianceThreshold, Default: "0", Check: wholeNumber(0, 1_000_000_00, "minor units")},
	{Name: svcshift.SettingTipRule, Default: string(tips.Even), Check: tipRule},
	{Name: svcshift.SettingTipRoleWeights, Check: roleWeights},
}

// find is the key named name.
func find(name string) (key, bool) {
	i := slices.IndexFunc(keys, func(k key) bool { return k.Name == name })
	if i < 0 {
		return key{}, false
	}
	return keys[i], true
}

func oneOf(allowed ...string) func(string) string {
	return func(v string) string {
		if slices.Contains(allowed, v) {
			return ""
		}
		return "Choose one of: " + strings.Join(allowed, ", ") + "."
	}
}

// wholeNumber accepts an integer from lo to hi, written plainly.
func wholeNumber(lo, hi int64, unit string) func(string) string {
	return func(v string) string {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || strconv.FormatInt(n, 10) != v || n < lo || n > hi {
			return fmt.Sprintf("Enter a whole number of %s from %d to %d.", unit, lo, hi)
		}
		return ""
	}
}

func stateCode(v string) string {
	if len(v) == 2 && v[0] >= '0' && v[0] <= '9' && v[1] >= '0' && v[1] <= '9' {
		return ""
	}
	return "Enter the two digit GST state code, like 29."
}

func jurisdiction(v string) string {
	var rates []struct {
		Name string `json:"name"`
		Rate int64  `json:"rate"`
	}
	if err := json.Unmarshal([]byte(v), &rates); err != nil || len(rates) == 0 {
		return `Enter the rates as a JSON list, like [{"name":"CA","rate":7250}].`
	}
	for _, r := range rates {
		if strings.TrimSpace(r.Name) == "" || r.Rate < 0 || r.Rate > 100_000 {
			return "Each rate needs a name and a rate from 0 to 100 percent."
		}
	}
	return ""
}

// seriesCode is a short run of capitals and digits: it names a counter and
// is part of every number issued from it.
func seriesCode(v string) string {
	if len(v) < 1 || len(v) > 8 || strings.Trim(v, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		return "Use 1 to 8 capital letters or digits, like INV."
	}
	return ""
}

func prefix(v string) string {
	if len(v) > 8 || strings.ContainsAny(v, "{}") {
		return "Keep the prefix to 8 characters, without braces."
	}
	return ""
}

func template(v string) string {
	if _, err := invoice.ParseTemplate(v); err != nil {
		return "That is not a number format: " + err.Error() + "."
	}
	return ""
}

func tipRule(v string) string {
	for _, r := range tips.Rules() {
		if string(r) == v {
			return ""
		}
	}
	return "Choose even, hours or role weight."
}

func roleWeights(v string) string {
	var w map[string]int64
	if err := json.Unmarshal([]byte(v), &w); err != nil {
		return `Enter the weights as a JSON object, like {"manager":2,"cashier":1}.`
	}
	for role, n := range w {
		if n < 1 || n > 100 {
			return fmt.Sprintf("Give %q a weight from 1 to 100.", role)
		}
	}
	return ""
}
