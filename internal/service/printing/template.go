package printing

import (
	"context"
	"fmt"
	"strings"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/device/escpos"
)

// The venue settings behind a receipt's template (phase 10, Task 6f,
// ADR-135). The header and the footer are lines joined by a newline; the
// three switches that hide a part are "true" or "false". Unset is no lines and everything
// shown, which is the receipt as it was before there was a template.
const (
	SettingReceiptHeader        = "receipt.header"
	SettingReceiptFooter        = "receipt.footer"
	SettingReceiptHideBarcode   = "receipt.hide_barcode"
	SettingReceiptHideOrderRef  = "receipt.hide_order_ref"
	SettingReceiptHideModifiers = "receipt.hide_modifiers"
)

// SplitLines is a stored header or footer as its lines: trimmed, with the
// blank ones dropped, and no more than escpos.MaxTemplateLines. A value the
// settings screen saved is already that; this is for one written another way,
// which must not stop a receipt printing.
func SplitLines(v string) []string {
	var out []string
	for _, l := range strings.Split(v, "\n") {
		if l = strings.TrimSpace(l); l != "" && len(out) < escpos.MaxTemplateLines {
			out = append(out, l)
		}
	}
	return out
}

// JoinLines is lines as stored: one per line, nothing for none.
func JoinLines(lines []string) string { return strings.Join(lines, "\n") }

// TemplateOf is the template a venue's settings describe. It reads nothing
// that can fail: a switch that is not "true" is off, and text that will print
// as "?" prints as it will. A receipt's extras are never worth a refused sale.
func TemplateOf(settings map[string]string) escpos.Template {
	hidden := func(key string) bool { return strings.TrimSpace(settings[key]) == "true" }
	return escpos.Template{
		Header:        SplitLines(settings[SettingReceiptHeader]),
		Footer:        SplitLines(settings[SettingReceiptFooter]),
		HideBarcode:   hidden(SettingReceiptHideBarcode),
		HideOrderRef:  hidden(SettingReceiptHideOrderRef),
		HideModifiers: hidden(SettingReceiptHideModifiers),
	}
}

// templateFor reads the venue's template in the transaction a job is queued
// in: a receipt prints with the template as it stands when it is paid, and a
// duplicate with the template as it stands when it is asked for.
func templateFor(ctx context.Context, tx *gen.Queries, venueID int64) (escpos.Template, error) {
	rows, err := tx.SettingsForVenue(ctx, venueID)
	if err != nil {
		return escpos.Template{}, fmt.Errorf("printing: reading the receipt template of venue %d: %w", venueID, err)
	}
	settings := make(map[string]string, len(rows))
	for _, r := range rows {
		settings[r.Key] = r.Value
	}
	return TemplateOf(settings), nil
}
