package tax

import "testing"

func TestNeedsHSN(t *testing.T) {
	tests := []struct {
		name    string
		profile TaxProfile
		want    bool
	}{
		{"GST shows it on every line", GST{}, true},
		{"VAT does not", VAT{}, false},
		{"US sales tax does not", USSalesTax{}, false},
		{"no tax at all does not", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsHSN(tt.profile); got != tt.want {
				t.Errorf("NeedsHSN = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidHSN(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"9963", true},
		{"996331", true},
		{"21069099", true},
		{"", false},
		{"996", false},
		{"99633", false},
		{"9963310", false},
		{"996331001", false},
		{"99A3", false},
		{" 9963", false},
		{"-963", false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := ValidHSN(tt.code); got != tt.want {
				t.Errorf("ValidHSN(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
