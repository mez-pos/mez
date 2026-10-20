package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/device/escpos"
)

func previewText(p ReceiptPreview) string {
	var b strings.Builder
	for _, l := range p.Lines {
		b.WriteString(l.Text + "\n")
	}
	return b.String()
}

func TestPreviewReceiptUnderTheVenueAndTheDraft(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	in := ReceiptInput{
		Header: []string{"Unit 4, MG Road, Bengaluru"}, Footer: []string{"Thank you. Come back soon!"},
		HideModifiers: true,
	}
	for _, mm := range []struct{ paper, columns int }{{80, 48}, {58, 32}} {
		p, err := s.PreviewReceipt(ctx, 1, in, mm.paper)
		if err != nil {
			t.Fatal(err)
		}
		text := previewText(p)
		if p.Columns != mm.columns {
			t.Errorf("%dmm: %d columns, want %d", mm.paper, p.Columns, mm.columns)
		}
		// The venue's own details (the fixture is a GST cafe), the real next
		// number, the draft's lines and switches.
		for _, want := range []string{"Cafe Pvt Ltd", "GSTIN 29ABCDE1234F1Z5", "TAX INVOICE", "INV/26-27/000001", "Unit 4, MG Road", "Thank you. Come back soon!", "CGST 2.5%"} {
			if !strings.Contains(text, want) {
				t.Errorf("%dmm: no %q in\n%s", mm.paper, want, text)
			}
		}
		if strings.Contains(text, "Oat milk") {
			t.Errorf("%dmm: a hidden part is shown:\n%s", mm.paper, text)
		}
		for _, l := range p.Lines {
			if len(l.Text)*l.Width > mm.columns {
				t.Errorf("%dmm: %q is wider than the paper", mm.paper, l.Text)
			}
		}
	}
}

func TestPreviewReceiptDescribesEachLine(t *testing.T) {
	s, _ := newService(t)
	p, err := s.PreviewReceipt(context.Background(), 1, ReceiptInput{}, 80)
	if err != nil {
		t.Fatal(err)
	}
	var title, total, barcode *PreviewLine
	for i := range p.Lines {
		l := &p.Lines[i]
		switch {
		case l.Text == "TAX INVOICE":
			title = l
		case strings.HasPrefix(l.Text, "TOTAL"):
			total = l
		case l.Barcode:
			barcode = l
		}
	}
	if title == nil || title.Align != "center" || !title.Bold {
		t.Errorf("title = %+v: centred and bold", title)
	}
	if total == nil || total.Height != 2 || !total.Bold {
		t.Errorf("total = %+v: double height and bold", total)
	}
	if barcode == nil || barcode.Text != "INV/26-27/000001" || barcode.Align != "center" {
		t.Errorf("barcode = %+v: the invoice number, centred", barcode)
	}
	for _, l := range p.Lines {
		if l.Width < 1 || l.Height < 1 || l.Align == "" {
			t.Errorf("a line with no size or alignment: %+v", l)
		}
	}
	// With the barcode off there is none.
	p, err = s.PreviewReceipt(context.Background(), 1, ReceiptInput{HideBarcode: true}, 80)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range p.Lines {
		if l.Barcode {
			t.Error("a barcode is shown though it is off")
		}
	}
}

func TestPreviewReceiptRefusesWhatASaveWould(t *testing.T) {
	long := strings.Repeat("a", escpos.MaxTemplateLineChars+1)
	tests := []struct {
		name  string
		in    ReceiptInput
		paper int
		field string
	}{
		{"five header lines", ReceiptInput{Header: []string{"1", "2", "3", "4", "5"}}, 80, "header"},
		{"a footer line too long", ReceiptInput{Footer: []string{long}}, 58, "footer"},
		{"text the printer cannot print", ReceiptInput{Header: []string{"स्व"}}, 80, "header"},
		{"paper that is not a width", ReceiptInput{}, 70, "paper_mm"},
		{"no paper", ReceiptInput{}, 0, "paper_mm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			_, err := s.PreviewReceipt(context.Background(), 1, tt.in, tt.paper)
			var inv *Invalid
			if !errors.As(err, &inv) || inv.Field != tt.field {
				t.Fatalf("err = %v, want a refusal of %q", err, tt.field)
			}
		})
	}
	s, _ := newService(t)
	if _, err := s.PreviewReceipt(context.Background(), 99, ReceiptInput{}, 80); !errors.Is(err, ErrNoVenue) {
		t.Errorf("an unknown venue: %v", err)
	}
}

func TestPreviewReceiptSavesNothing(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.PreviewReceipt(ctx, 1, ReceiptInput{Header: []string{"Unit 4"}, Footer: []string{"Thanks"}, HideBarcode: true}, 80); err != nil {
		t.Fatal(err)
	}
	o, _ := s.Read(ctx, 1)
	for _, e := range o.Entries {
		if e.Set {
			t.Errorf("%s was stored by a preview: %q", e.Key, e.Value)
		}
	}
}

// A venue whose tax profile is none previews with no tax lines and no HSN.
func TestPreviewReceiptUnderNoTax(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if _, err := d.Writer().ExecContext(ctx, `UPDATE venues SET tax_profile = 'none', tax_registration_id = '' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewReceipt(ctx, 1, ReceiptInput{}, 80)
	if err != nil {
		t.Fatal(err)
	}
	text := previewText(p)
	if !strings.Contains(text, "Subtotal") || strings.Contains(text, "CGST") || strings.Contains(text, "HSN") || strings.Contains(text, "GSTIN") {
		t.Errorf("under no tax:\n%s", text)
	}
}
