package settings

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/device/escpos"
)

func stockReceipt() ReceiptInput {
	return ReceiptInput{Header: []string{}, Footer: []string{}}
}

func receiptWith(f func(*ReceiptInput)) ReceiptInput {
	in := stockReceipt()
	f(&in)
	return in
}

func TestReceiptOfANewVenueIsUnchanged(t *testing.T) {
	s, _ := newService(t)
	got, err := s.Receipt(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := Receipt{ReceiptInput: stockReceipt(), MaxLines: escpos.MaxTemplateLines, MaxLineChars: escpos.MaxTemplateLineChars}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := s.Receipt(context.Background(), 99); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

func TestUpdateReceiptSavesAndReadsBack(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	in := ReceiptInput{
		Header:        []string{"Unit 4, MG Road, Bengaluru", "Tel 080 1234 5678"},
		Footer:        []string{"Thank you. Come back soon!", "Wi-Fi: filtercoffee / brewbrew"},
		HideBarcode:   true,
		HideModifiers: true,
	}
	if err := s.UpdateReceipt(ctx, 1, in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Receipt(ctx, 1)
	if err != nil || !reflect.DeepEqual(got.ReceiptInput, in) {
		t.Fatalf("got %+v, %v\nwant %+v", got.ReceiptInput, err, in)
	}

	// Stored as the layouts read them: one line each, a switch only when hiding.
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]string{}
	for _, e := range o.Entries {
		if e.Set {
			stored[e.Key] = e.Value
		}
	}
	want := map[string]string{
		"receipt.header":         "Unit 4, MG Road, Bengaluru\nTel 080 1234 5678",
		"receipt.footer":         "Thank you. Come back soon!\nWi-Fi: filtercoffee / brewbrew",
		"receipt.hide_barcode":   "true",
		"receipt.hide_modifiers": "true",
	}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored %v, want %v", stored, want)
	}
}

func TestUpdateReceiptTrimsAndDropsBlankLines(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	err := s.UpdateReceipt(ctx, 1, ReceiptInput{
		Header: []string{"", "  Unit 4  ", "   ", "Tel 080 1234 5678", ""},
		Footer: []string{"  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Receipt(ctx, 1)
	if !reflect.DeepEqual(got.Header, []string{"Unit 4", "Tel 080 1234 5678"}) || len(got.Footer) != 0 {
		t.Errorf("header %q, footer %q", got.Header, got.Footer)
	}
}

func TestUpdateReceiptBackToDefaultsClearsTheKeys(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdateReceipt(ctx, 1, ReceiptInput{Header: []string{"Unit 4"}, Footer: []string{"Thanks"}, HideBarcode: true, HideOrderRef: true, HideModifiers: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateReceipt(ctx, 1, ReceiptInput{}); err != nil {
		t.Fatal(err)
	}
	o, _ := s.Read(ctx, 1)
	for _, e := range o.Entries {
		if strings.HasPrefix(e.Key, "receipt.") && e.Set {
			t.Errorf("%s is still set to %q", e.Key, e.Value)
		}
	}
}

func TestUpdateReceiptRefusals(t *testing.T) {
	long := strings.Repeat("a", escpos.MaxTemplateLineChars+1)
	tests := []struct {
		name  string
		in    ReceiptInput
		field string // "" is accepted
	}{
		{"five header lines", receiptWith(func(in *ReceiptInput) { in.Header = []string{"1", "2", "3", "4", "5"} }), "header"},
		{"four header lines", receiptWith(func(in *ReceiptInput) { in.Header = []string{"1", "2", "3", "4"} }), ""},
		{"five footer lines", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"1", "2", "3", "4", "5"} }), "footer"},
		{"blank lines are not counted", receiptWith(func(in *ReceiptInput) { in.Header = []string{"1", "", "2", "", "3", "", "4", ""} }), ""},
		{"a header line too long", receiptWith(func(in *ReceiptInput) { in.Header = []string{long} }), "header"},
		{"a footer line of exactly the limit", receiptWith(func(in *ReceiptInput) { in.Footer = []string{long[1:]} }), ""},
		{"Devanagari in the header", receiptWith(func(in *ReceiptInput) { in.Header = []string{"स्वागत"} }), "header"},
		{"an emoji in the footer", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"Thanks \U0001F600"} }), "footer"},
		{"a pasted block counts as its lines", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"1\n2", "3\r\n4", "5"} }), "footer"},
		{"a pasted block within the limit", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"1\n2", "3\r\n4"} }), ""},
		{"a control character", receiptWith(func(in *ReceiptInput) { in.Header = []string{"bell\a"} }), "header"},
		{"accents are spelled, so they pass", receiptWith(func(in *ReceiptInput) { in.Header = []string{"Café crème"} }), ""},
		{"the rupee sign is spelled, so it passes", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"Min. order ₹100"} }), ""},
		{"a question mark typed", receiptWith(func(in *ReceiptInput) { in.Footer = []string{"Open daily?"} }), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			ctx := context.Background()
			err := s.UpdateReceipt(ctx, 1, tt.in)
			var inv *Invalid
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
			if tt.field == "" {
				return
			}
			got, err := s.Receipt(ctx, 1)
			if err != nil || !reflect.DeepEqual(got.ReceiptInput, stockReceipt()) {
				t.Fatalf("after the refusal: %+v, %v", got.ReceiptInput, err)
			}
		})
	}
}

func TestUpdateReceiptIsAllOrNothing(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	// A good header, a bad footer: neither is kept.
	err := s.UpdateReceipt(ctx, 1, ReceiptInput{Header: []string{"Unit 4"}, Footer: []string{"谢谢"}, HideBarcode: true})
	if err == nil {
		t.Fatal("want a refusal")
	}
	o, _ := s.Read(ctx, 1)
	for _, e := range o.Entries {
		if e.Set {
			t.Errorf("%s was saved by a refused update: %q", e.Key, e.Value)
		}
	}
	if err := s.UpdateReceipt(ctx, 99, ReceiptInput{}); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

// The three switches are checked as keys too, so the generic write path
// refuses what the form never sends.
func TestReceiptKeysAreChecked(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	for key, bad := range map[string]string{
		"receipt.hide_barcode": "yes", "receipt.hide_order_ref": "1", "receipt.header": "a\n\nb", "receipt.footer": strings.Repeat("a", 49),
	} {
		var inv *Invalid
		if err := s.Set(ctx, 1, map[string]string{key: bad}); !errors.As(err, &inv) || inv.Field != key {
			t.Errorf("%s = %q: err = %v, want a refusal", key, bad, err)
		}
	}
}

// A newline inside a line makes two, so what is read back is single lines.
func TestAPastedBlockIsStoredAsLines(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if err := s.UpdateReceipt(ctx, 1, ReceiptInput{Footer: []string{"Thank you.\r\nCome back soon!"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Receipt(ctx, 1)
	if !reflect.DeepEqual(got.Footer, []string{"Thank you.", "Come back soon!"}) {
		t.Errorf("footer = %q", got.Footer)
	}
}
