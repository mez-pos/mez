package qr

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestSmallestVersion: the standard's byte capacities at level M, at the
// edge of each.
func TestSmallestVersion(t *testing.T) {
	tests := []struct{ bytes, version int }{
		{0, 1}, {1, 1}, {14, 1}, {15, 2}, {26, 2}, {27, 3}, {42, 3}, {62, 4}, {63, 5}, {84, 5}, {85, 6},
		{122, 7}, {152, 8}, {180, 9}, {181, 10}, {213, 10}, {214, 11}, {666, 20}, {1370, 30}, {2331, 40},
	}
	for _, tt := range tests {
		c, err := Encode(bytes.Repeat([]byte{'a'}, tt.bytes))
		if err != nil {
			t.Fatalf("%d bytes: %v", tt.bytes, err)
		}
		if c.Version != tt.version || c.Size != tt.version*4+17 {
			t.Errorf("%d bytes: version %d, size %d, want version %d", tt.bytes, c.Version, c.Size, tt.version)
		}
	}
}

func TestTooLong(t *testing.T) {
	if _, err := Encode(make([]byte, MaxBytes+1)); !errors.Is(err, ErrTooLong) {
		t.Errorf("err = %v, want ErrTooLong", err)
	}
	if _, err := Encode(make([]byte, MaxBytes)); err != nil {
		t.Errorf("the most a code holds: %v", err)
	}
}

// TestEveryCodeHasItsFinders: whatever the data, version and mask, the
// three corners a scanner looks for are there, and the fourth is not one.
func TestEveryCodeHasItsFinders(t *testing.T) {
	finder := []string{"1111111", "1000001", "1011101", "1011101", "1011101", "1000001", "1111111"}
	at := func(c Code, x0, y0 int) bool {
		for dy, row := range finder {
			for dx, want := range row {
				if c.Dark(x0+dx, y0+dy) != (want == '1') {
					return false
				}
			}
		}
		return true
	}
	for _, n := range []int{0, 5, 80, 300, 2331} {
		c, err := Encode(bytes.Repeat([]byte{0xA5}, n))
		if err != nil {
			t.Fatal(err)
		}
		far := c.Size - 7
		if !at(c, 0, 0) || !at(c, far, 0) || !at(c, 0, far) {
			t.Errorf("%d bytes: a finder is missing", n)
		}
		if at(c, far, far) {
			t.Errorf("%d bytes: a fourth finder", n)
		}
		// The module that is always dark.
		if !c.Dark(8, c.Size-8) {
			t.Errorf("%d bytes: the dark module is light", n)
		}
	}
}

// TestTheMaskChosenScoresBest: Encode picks the mask with the lowest
// penalty, and says which.
func TestTheMaskChosenScoresBest(t *testing.T) {
	data := []byte("upi://pay?am=540.00&cu=INR&pa=cafe%40okbank&pn=Filter%20Coffee&tn=Table%20T5")
	c, err := Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	score := func(mask int) int {
		forced := encode(data, c.Version, mask)
		g := &grid{version: forced.Version, size: forced.Size, dark: forced.dark}
		return g.penalty()
	}
	best := score(c.Mask)
	for mask := range 8 {
		if s := score(mask); s < best {
			t.Errorf("mask %d scores %d, the chosen mask %d scores %d", mask, s, c.Mask, best)
		}
	}
	if matrix(c) != matrix(encode(data, c.Version, c.Mask)) {
		t.Error("the code is not the one its own mask draws")
	}
}

func TestLinePenalty(t *testing.T) {
	line := func(s string) []bool {
		out := make([]bool, len(s))
		for i, r := range s {
			out[i] = r == '1'
		}
		return out
	}
	tests := []struct {
		name string
		line string
		want int
	}{
		{"alternating", "101010101010", 0},
		{"a run of four", "111101010101", 0},
		{"a run of five", "111110101010", 3},
		{"a run of seven", "111111101010", 5},
		{"two runs", "1111100000", 6},
		{"a finder with light after it", "10111010000", 40},
		{"a finder with light before it", "00001011101", 40},
		{"a finder with five light after it", "101110100000", 40 + 3},
	}
	for _, tt := range tests {
		if got := linePenalty(line(tt.line)); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestReedSolomon(t *testing.T) {
	// The standard's own worked example, version 1 at level M: sixteen data
	// codewords and their ten error correction codewords.
	data := []byte{0x10, 0x20, 0x0C, 0x56, 0x61, 0x80, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11}
	want := []byte{0xA5, 0x24, 0xD4, 0xC1, 0xED, 0x36, 0xC7, 0x87, 0x2C, 0x55}
	if got := rsRemainder(data, rsDivisor(10)); !bytes.Equal(got, want) {
		t.Errorf("got  % X\nwant % X", got, want)
	}
}

func TestDarkOutsideIsLight(t *testing.T) {
	c, _ := Encode([]byte("mez"))
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {c.Size, 0}, {0, c.Size}} {
		if c.Dark(p[0], p[1]) {
			t.Errorf("(%d, %d) is outside the code and dark", p[0], p[1])
		}
	}
}

// TestSVG: the picture is the code, with its quiet band, and nothing a
// browser would run.
func TestSVG(t *testing.T) {
	c := encode([]byte("mez"), 1, 3)
	svg := c.SVG(4)
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 29 29"`) || !strings.HasSuffix(svg, `</svg>`) {
		t.Fatalf("svg = %.120s", svg)
	}
	if !strings.Contains(svg, `<rect width="29" height="29" fill="#fff"/>`) {
		t.Error("no light ground: a scanner needs the quiet band drawn, not left to the page")
	}
	for _, bad := range []string{"<script", "href", "onload", "<image", "<foreignObject"} {
		if strings.Contains(svg, bad) {
			t.Errorf("the picture carries %q", bad)
		}
	}

	// Paint the path's rectangles and compare with the code.
	painted := make(map[[2]int]bool)
	runs := regexp.MustCompile(`M(\d+) (\d+)h(\d+)v1h-(\d+)z`).FindAllStringSubmatch(svg, -1)
	for _, m := range runs {
		x, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[2])
		n, _ := strconv.Atoi(m[3])
		if m[3] != m[4] {
			t.Fatalf("a run that does not close: %s", m[0])
		}
		for i := range n {
			if painted[[2]int{x + i, y}] {
				t.Errorf("(%d, %d) is painted twice", x+i, y)
			}
			painted[[2]int{x + i, y}] = true
		}
	}
	dark := 0
	for y := range c.Size {
		for x := range c.Size {
			if c.Dark(x, y) {
				dark++
				if !painted[[2]int{x + 4, y + 4}] {
					t.Errorf("module (%d, %d) is dark and not painted", x, y)
				}
			}
		}
	}
	if len(painted) != dark {
		t.Errorf("%d modules painted, %d dark", len(painted), dark)
	}
}
