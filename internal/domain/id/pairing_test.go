package id

import (
	"bytes"
	"strings"
	"testing"
)

func TestPairingCode(t *testing.T) {
	code, err := NewPairingCode()
	if err != nil {
		t.Fatalf("NewPairingCode: %v", err)
	}
	if len(code) != PairingCodeLength || strings.Trim(code, Alphabet) != "" {
		t.Errorf("code %q is not %d symbols of the alphabet", code, PairingCodeLength)
	}
	fixed, err := NewGenerator(bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 31})).PairingCode()
	if err != nil || fixed != "0123456Z" {
		t.Errorf("PairingCode over a fixed source = %q, %v", fixed, err)
	}
	if _, err := NewGenerator(bytes.NewReader([]byte{1})).PairingCode(); err == nil {
		t.Error("a short entropy source gave a code")
	}
}
