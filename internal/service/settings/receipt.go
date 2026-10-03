package settings

import (
	"context"
	"errors"
	"strings"

	"github.com/byjoellobo/mez/internal/device/escpos"
	"github.com/byjoellobo/mez/internal/service/printing"
)

// ReceiptInput is what a venue adds to its receipts and credit notes beyond
// what its tax profile requires (phase 10, Task 6f, ADR-135). Printing is
// ASCII only (ADR-079): a character the printer would turn into "?" is
// refused when saved.
type ReceiptInput struct {
	// Header is printed under the venue's name and registration, Footer last:
	// up to MaxLines lines each, MaxLineChars long (longer wraps on narrow
	// paper). Blank lines are dropped.
	Header []string `json:"header"`
	Footer []string `json:"footer"`
	// The three parts of a receipt a venue may leave off, each hidden when
	// true: the invoice number's barcode, the order's id under the date (the
	// table number stays), and the choices under each line. Everything else on
	// a receipt is the tax profile's, and stays.
	HideBarcode   bool `json:"hide_barcode"`
	HideOrderRef  bool `json:"hide_order_ref"`
	HideModifiers bool `json:"hide_modifiers"`
}

// Receipt is the template as it stands, and the limits a form should hold to.
type Receipt struct {
	ReceiptInput
	MaxLines     int
	MaxLineChars int
}

// Receipt reads the venue's receipt template. An unset key reads as its
// default: no lines, every part shown.
func (s *Service) Receipt(ctx context.Context, venueID int64) (Receipt, error) {
	_, stored, err := rawVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return Receipt{}, err
	}
	tpl := printing.TemplateOf(stored)
	return Receipt{
		ReceiptInput: ReceiptInput{
			Header: nonNilLines(tpl.Header), Footer: nonNilLines(tpl.Footer),
			HideBarcode: tpl.HideBarcode, HideOrderRef: tpl.HideOrderRef, HideModifiers: tpl.HideModifiers,
		},
		MaxLines: escpos.MaxTemplateLines, MaxLineChars: escpos.MaxTemplateLineChars,
	}, nil
}

// UpdateReceipt saves the receipt template, or nothing. Each line is trimmed
// and blank ones dropped before it is counted; each is checked, through Set,
// by the same rule as every other key. A switch left at its default is not
// stored.
func (s *Service) UpdateReceipt(ctx context.Context, venueID int64, in ReceiptInput) error {
	if _, _, err := rawVenue(ctx, s.db.Queries(), venueID); err != nil {
		return err
	}
	header, footer := cleanLines(in.Header), cleanLines(in.Footer)
	flag := func(hide bool) string {
		if hide {
			return "true"
		}
		return "" // the default: not stored, so the key reads as unset
	}
	err := s.Set(ctx, venueID, map[string]string{
		printing.SettingReceiptHeader:        printing.JoinLines(header),
		printing.SettingReceiptFooter:        printing.JoinLines(footer),
		printing.SettingReceiptHideBarcode:   flag(in.HideBarcode),
		printing.SettingReceiptHideOrderRef:  flag(in.HideOrderRef),
		printing.SettingReceiptHideModifiers: flag(in.HideModifiers),
	})
	return fieldNamed(err)
}

// cleanLines trims each line and drops the blank ones. A line with a newline
// in it is the lines on either side of it, as a pasted block of text is, so
// what is stored and read back is always a list of single lines.
func cleanLines(lines []string) []string {
	var out []string
	for _, block := range lines {
		for _, l := range strings.Split(block, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
	}
	return out
}

func nonNilLines(lines []string) []string {
	if lines == nil {
		return []string{}
	}
	return lines
}

// fieldNamed renames a refused key to the field a form calls it.
func fieldNamed(err error) error {
	var inv *Invalid
	if !errors.As(err, &inv) {
		return err
	}
	switch inv.Field {
	case printing.SettingReceiptHeader:
		return &Invalid{"header", inv.Message}
	case printing.SettingReceiptFooter:
		return &Invalid{"footer", inv.Message}
	}
	return err
}
