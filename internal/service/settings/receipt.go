package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/device/escpos"
	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
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

// PreviewLine is one line of the sample receipt as it sits on the paper.
type PreviewLine struct {
	Text string `json:"text"`
	// Align is left, center or right. Width and Height are the character size
	// multipliers; Bold is bold. A Barcode line is the number under its bars.
	Align   string `json:"align"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bold    bool   `json:"bold"`
	Barcode bool   `json:"barcode"`
}

// ReceiptPreview is a sample receipt laid out for a paper width: each line as
// the printer would put it, so the screen shows the wrapping and which parts
// print without a printer.
type ReceiptPreview struct {
	Columns int           `json:"columns"`
	Lines   []PreviewLine `json:"lines"`
}

// PreviewReceipt lays out a made-up paid bill, under the venue's own name,
// registration and tax profile and the template in in, for paperMM (80 or 58).
// Nothing is saved. It runs the checks a save runs on the template, so a
// draft a save would refuse is refused here with the same field and words, and
// it lays the bill out with the same code that prints a real one.
func (s *Service) PreviewReceipt(ctx context.Context, venueID int64, in ReceiptInput, paperMM int) (ReceiptPreview, error) {
	paper, ok := map[int]escpos.Paper{80: escpos.Paper80, 58: escpos.Paper58}[paperMM]
	if !ok {
		return ReceiptPreview{}, &Invalid{"paper_mm", "Choose 80mm or 58mm paper."}
	}
	q := s.db.Queries()
	v, stored, err := rawVenue(ctx, q, venueID)
	if err != nil {
		return ReceiptPreview{}, err
	}
	header, footer := cleanLines(in.Header), cleanLines(in.Footer)
	for _, c := range []struct {
		field, key string
		lines      []string
	}{
		{"header", printing.SettingReceiptHeader, header}, {"footer", printing.SettingReceiptFooter, footer},
	} {
		if len(c.lines) == 0 {
			continue
		}
		k, _ := find(c.key)
		if msg := k.Check(printing.JoinLines(c.lines)); msg != "" {
			return ReceiptPreview{}, &Invalid{c.field, msg}
		}
	}

	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return ReceiptPreview{}, fmt.Errorf("venue %d's timezone %q: %w", venueID, v.Timezone, err)
	}
	seller := order.InvoiceSeller{Name: v.Name, LegalName: v.LegalName, TaxProfile: v.TaxProfile, Registration: v.TaxRegistrationID}
	if seller.LegalName == "" {
		seller.LegalName = v.Name
	}
	if v.TaxProfile == ProfileGST {
		seller.StateCode = stored[svcorder.SettingTaxStateCode]
	}
	// The real next number shows its real format; a venue whose numbering
	// cannot issue one yet still gets a preview.
	invoiceNo := "INV/0001"
	if n, err := s.Numbering(ctx, venueID); err == nil {
		invoiceNo = n.NextInvoiceNo
	}

	tpl := escpos.Template{
		Header: header, Footer: footer,
		HideBarcode: in.HideBarcode, HideOrderRef: in.HideOrderRef, HideModifiers: in.HideModifiers,
	}
	data, err := escpos.SampleReceipt(seller, v.Currency, invoiceNo, s.clock.Now().In(loc), tpl).Layout(paper)
	if err != nil {
		return ReceiptPreview{}, fmt.Errorf("laying out the sample receipt: %w", err)
	}
	page, err := escpos.Replay(data)
	if err != nil {
		return ReceiptPreview{}, fmt.Errorf("replaying the sample receipt: %w", err)
	}
	out := ReceiptPreview{Columns: paper.Columns, Lines: make([]PreviewLine, len(page.Lines))}
	for i, l := range page.Lines {
		out.Lines[i] = PreviewLine{
			Text: l.Text, Align: map[escpos.Align]string{escpos.Left: "left", escpos.Center: "center", escpos.Right: "right"}[l.Align],
			Width: l.Width, Height: l.Height, Bold: l.Bold, Barcode: l.Barcode,
		}
	}
	return out, nil
}
