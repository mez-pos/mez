package tax

import "testing"

func TestProfileByCode(t *testing.T) {
	tests := []struct {
		code  string
		known bool
		// charges: the code has a regime that computes tax.
		charges bool
		gst     bool
	}{
		{CodeNone, true, false, false},
		{CodeGST, true, true, true},
		// The composition scheme charges nothing, and its bill is GST's (ADR-401).
		{CodeGSTComposition, true, false, true},
		{CodeVAT, true, true, false},
		{CodeSales, true, true, false},
		{"gst", false, false, false},
		{"", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			p, ok := ProfileByCode(tt.code)
			if ok != tt.known || (p != nil) != tt.charges {
				t.Errorf("ProfileByCode(%q) = %v, %v; want known %v, charging %v", tt.code, p, ok, tt.known, tt.charges)
			}
			if got := UnderGST(tt.code); got != tt.gst {
				t.Errorf("UnderGST(%q) = %v, want %v", tt.code, got, tt.gst)
			}
		})
	}
}

func TestValidGSTIN(t *testing.T) {
	for s, want := range map[string]bool{
		"29AABCU9603R1ZX":  true,
		"29aabcu9603r1zx":  false,
		"29AABCU9603R1Z":   false,
		"AB29CU9603R1ZXQ":  false,
		"29AABCU9603R1ZX ": false,
		"":                 false,
	} {
		if got := ValidGSTIN(s); got != want {
			t.Errorf("ValidGSTIN(%q) = %v, want %v", s, got, want)
		}
	}
}

// The rule's own words, which a bill of supply must print (CGST Rule 5(1)(g)).
func TestCompositionDeclaration(t *testing.T) {
	if CompositionDeclaration != "Composition taxable person, not eligible to collect tax on supplies" {
		t.Errorf("the declaration is %q", CompositionDeclaration)
	}
}
