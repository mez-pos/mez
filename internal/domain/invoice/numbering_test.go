package invoice

import (
	"errors"
	"testing"
)

func TestCheckNumbering(t *testing.T) {
	series := func(code, prefix, template string) Series {
		t.Helper()
		s, err := NewSeries(code, prefix, IndiaFY, template, 16)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	const good = "{prefix}/{fy:short}/{seq:06d}"
	tests := []struct {
		name            string
		invoices, creds Series
		field           string // "" when fine
	}{
		{"the default", series("INV", "INV", good), series("CN", "CN", good), ""},
		{"prefixes unlike the codes", series("INV", "NF", good), series("CN", "CR", good), ""},
		{"the long year", series("INV", "INV", "{prefix}/{fy}/{seq}"), series("CN", "CN", "{prefix}/{fy}/{seq}"), ""},
		{"no year", series("INV", "INV", "{prefix}-{seq:06d}"), series("CN", "CN", "{prefix}-{seq:06d}"), "template"},
		{"no prefix", series("INV", "INV", "{fy:short}/{seq:06d}"), series("CN", "CN", "{fy:short}/{seq:06d}"), "template"},
		{"one prefix for both", series("INV", "X", good), series("CN", "X", good), "credit_prefix"},
		{"one code for both", series("INV", "INV", good), series("INV", "CN", good), "credit_series"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNumbering(tt.invoices, tt.creds)
			var ne *NumberingError
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &ne) || ne.Field != tt.field || !errors.Is(err, ErrNumbering)):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
		})
	}
}

func TestTemplateUses(t *testing.T) {
	tests := []struct {
		template   string
		prefix, fy bool
	}{
		{"{prefix}/{fy}/{seq}", true, true},
		{"{prefix}/{fy:short}/{seq:06d}", true, true},
		{"{prefix}-{seq}", true, false},
		{"{fy}-{seq}", false, true},
		{"{seq}", false, false},
	}
	for _, tt := range tests {
		tpl, err := ParseTemplate(tt.template)
		if err != nil {
			t.Fatal(err)
		}
		if tpl.UsesPrefix() != tt.prefix || tpl.UsesFY() != tt.fy {
			t.Errorf("%s: prefix %v, fy %v", tt.template, tpl.UsesPrefix(), tpl.UsesFY())
		}
	}
}
