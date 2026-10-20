package settings

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mez-pos/mez/internal/device/escpos"
	"github.com/mez-pos/mez/internal/domain/invoice"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/domain/tips"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/printing"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
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
	{Name: svcorder.SettingInvoiceSeries, Default: svcorder.DefaultSeries, Check: seriesCode},
	{Name: svcorder.SettingInvoicePrefix, Check: prefix},
	{Name: svcorder.SettingInvoiceTemplate, Default: svcorder.DefaultTemplate, Check: template},
	{Name: svcorder.SettingCreditSeries, Default: svcorder.DefaultCreditSeries, Check: seriesCode},
	{Name: svcorder.SettingCreditPrefix, Check: prefix},
	{Name: svcorder.SettingTicketLateMinutes, Default: strconv.Itoa(svcorder.DefaultLateMinutes), Check: wholeNumber(1, 600, "minutes")},
	{Name: svcshift.SettingDayStart, Default: shift.FormatDayStart(shift.DefaultDayStart), Check: timeOfDay},
	{Name: svcshift.SettingVarianceThreshold, Default: strconv.FormatInt(svcshift.DefaultVarianceThreshold, 10), Check: wholeNumber(0, 1_000_000_00, "minor units")},
	{Name: svcshift.SettingTipRule, Default: string(tips.Even), Check: tipRule},
	{Name: svcshift.SettingTipRoleWeights, Check: roleWeights},
	{Name: svcorder.SettingTipMaxPercent, Default: strconv.Itoa(svcorder.DefaultTipMaxPercent), Check: tipCeiling},
	{Name: printing.SettingReceiptHeader, Check: receiptLines},
	{Name: printing.SettingReceiptFooter, Check: receiptLines},
	{Name: printing.SettingReceiptHideBarcode, Default: "false", Check: oneOf("true", "false")},
	{Name: printing.SettingReceiptHideOrderRef, Default: "false", Check: oneOf("true", "false")},
	{Name: printing.SettingReceiptHideModifiers, Default: "false", Check: oneOf("true", "false")},
	{Name: svcorder.SettingUPIAddress, Check: upiAddress},
	{Name: svcorder.SettingUPIPayee, Check: upiPayee},
	{Name: svcorder.SettingPayMode, Default: string(svcorder.PayAtTable), Check: oneOf(string(svcorder.PayAtTable), string(svcorder.PayFirst))},
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

// timeOfDay is a 24 hour time like 04:00: when the trading day begins.
func timeOfDay(v string) string {
	if _, err := shift.ParseDayStart(v); err != nil {
		return "Enter a time of day like 04:00, on the 24 hour clock."
	}
	return ""
}

// receiptLines is a header or footer as stored: up to a few non-blank lines
// of text the printer can print. Printing is ASCII only (ADR-079), so a
// character it would turn into "?" is refused here, where it can be fixed,
// rather than printed on every receipt.
func receiptLines(v string) string {
	lines := strings.Split(v, "\n")
	if len(lines) > escpos.MaxTemplateLines {
		return fmt.Sprintf("Use at most %d lines.", escpos.MaxTemplateLines)
	}
	for i, l := range lines {
		switch r, bad := escpos.Unprintable(l); {
		case strings.TrimSpace(l) == "":
			return fmt.Sprintf("Line %d is empty.", i+1)
		case utf8.RuneCountInString(l) > escpos.MaxTemplateLineChars:
			return fmt.Sprintf("Keep line %d to %d characters.", i+1, escpos.MaxTemplateLineChars)
		case bad:
			return fmt.Sprintf("Line %d has %q, which the printer cannot print. Use letters, digits and common punctuation.", i+1, r)
		}
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

// tipCeiling is the largest tip without a manager, a whole percent of the
// payment (ADR-261).
func tipCeiling(v string) string {
	if wholeNumber(0, svcorder.MaxTipMaxPercent, "percent")(v) != "" {
		return fmt.Sprintf("Enter a whole percent from 0 to %d. 0 asks for a manager on every tip.", svcorder.MaxTipMaxPercent)
	}
	return ""
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
