package qr

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// goldens are codes checked, module for module, against python-qrcode when
// this package was written (TestAgainstAReferenceEncoder, ADR-217). Each is
// kept as the first eight bytes of the SHA-256 of its matrix, so a change
// to the encoder that moves one module fails here without that library.
//
// They are picked for where the format changes: the smallest code, one
// filled to the byte, the first with a version number (7), the first with a
// sixteen bit count (10), blocks of two lengths (14), the version whose
// alignment patterns are spaced unlike the rest (32), and the largest.
var goldens = []struct {
	version, mask int
	data          string
	sum           string
}{
	{1, 0, "mez", "3ae198cc2f7578f8"},
	{1, 7, "14 bytes fill!", "425f64d248c35868"},
	{2, 1, "https://mez.example/t", "9aa444d7408fc756"},
	{4, 2, "upi://pay?am=540.00&cu=INR&pa=cafe%40okbank", "d341214eddf3d8ce"},
	{5, 3, "upi://pay?am=540.00&cu=INR&pa=cafe%40okbank&pn=Filter%20Coffee&tn=Table%20T5", "4853577586e047a3"},
	{7, 4, "version seven is the first to carry its own number", "97c820838533e99a"},
	{10, 5, "version ten is the first whose byte count is sixteen bits wide", "04e9076792a63e18"},
	{14, 6, "version fourteen has blocks of two lengths", "4b1199f1374d3917"},
	{32, 0, "version thirty-two is the one whose alignment step is the odd one out", "a9e17012fb5cac14"},
	{40, 1, "the largest", "9798f07a5d757201"},
}

func TestGoldens(t *testing.T) {
	for _, g := range goldens {
		sum := sha256.Sum256([]byte(matrix(encode([]byte(g.data), g.version, g.mask))))
		if got := hex.EncodeToString(sum[:8]); got != g.sum {
			t.Errorf("version %d, mask %d, %q: the code changed (%s, was %s)", g.version, g.mask, g.data, got, g.sum)
		}
	}
}

// mezV1M3 is "mez" at version 1 under mask 3, whole, so one code in this
// package can be read by eye.
const mezV1M3 = `
111111101000001111111
100000101010001000001
101110100111101011101
101110101100101011101
101110100001001011101
100000100111101000001
111111101010101111111
000000001001100000000
101101110001101001011
110000001011111001101
101011110111000000011
111111010011001111000
111010110010100101011
000000001011001001100
111111101001100100100
100000101010000110111
101110100010111111001
101110101111001111110
101110101110101100000
100000100010010100101
111111101010010010000`

func TestACodeInFull(t *testing.T) {
	want := strings.ReplaceAll(strings.TrimSpace(mezV1M3), "\n", "/")
	if got := matrix(encode([]byte("mez"), 1, 3)); got != want {
		t.Errorf("got\n%s\nwant\n%s", strings.ReplaceAll(got, "/", "\n"), strings.TrimSpace(mezV1M3))
	}
}
