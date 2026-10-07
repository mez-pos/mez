package wire

import "testing"

func TestMaskCodes(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{"host mode, a table", "/order/api/t/ABC123/menu", "/order/api/t/[code]/menu"},
		{"host mode, a counter", "/order/api/c/XYZ/orders", "/order/api/c/[code]/orders"},
		{"host mode, a pickup", "/order/api/p/PK99", "/order/api/p/[code]"},
		{"path mode, a table", "/v/cafe/t/ABC123/menu", "/v/cafe/t/[code]/menu"},
		{"path mode, a counter", "/v/cafe/c/XYZ", "/v/cafe/c/[code]"},
		{"path mode, a pickup with a slash after", "/v/cafe/p/PK99/", "/v/cafe/p/[code]/"},
		{"the marker at the end has nothing to mask", "/order/api/t", "/order/api/t"},
		{"an empty segment after the marker is not a code", "/order/api/t//menu", "/order/api/t//menu"},
		{"a code that is itself a marker is still masked once", "/v/cafe/t/c/menu", "/v/cafe/t/[code]/menu"},
		{"no code", "/v/cafe/", "/v/cafe/"},
		{"a venue called t is a venue, not a marker, when nothing follows", "/v/t", "/v/t"},
		{"a longer segment is not a marker", "/order/api/tables/5", "/order/api/tables/5"},
		{"the health probe", "/healthz", "/healthz"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskCodes(tt.path); got != tt.want {
				t.Errorf("MaskCodes(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
