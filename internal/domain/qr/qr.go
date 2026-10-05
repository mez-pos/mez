// Package qr draws QR codes (ISO/IEC 18004), enough of them for a payment
// link: bytes in, at error correction level M, in the smallest version that
// holds them (phase 11, Task 4e3, ADR-217).
//
// It is here, and not a dependency, because it is small, it never changes,
// and a guest's phone should not have to carry it: the server draws the code
// and the phone shows a picture.
//
// A code is a square of modules, dark or light. Encoding goes: the bytes
// with a header saying how many, padded to the version's capacity; split
// into blocks, each with Reed-Solomon error correction; the blocks
// interleaved and laid in a zigzag from the bottom right, around the
// patterns a scanner finds the code by; then one of eight masks, chosen to
// avoid shapes that confuse scanners, and the mask's number written beside
// the corners.
package qr

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTooLong means the data does not fit the largest code, version 40.
var ErrTooLong = errors.New("qr: the data is too long for a QR code")

// MaxBytes is the most bytes a code at level M holds: version 40.
const MaxBytes = 2331

// Level M, about 15% of a code recoverable: for each version, the error
// correction codewords in each block, and how many blocks. From the
// standard's table 9; index 0 is unused.
var (
	eccPerBlock = [41]int{0, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28}
	blockCount  = [41]int{0, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49}
)

// formatM is level M's two bits in the format information.
const formatM = 0

// Code is a QR code: Size modules a side.
type Code struct {
	Version int
	Size    int
	Mask    int
	dark    []bool // row by row
}

// Dark reports whether the module at column x, row y is dark. Outside the
// code is light.
func (c Code) Dark(x, y int) bool {
	if x < 0 || y < 0 || x >= c.Size || y >= c.Size {
		return false
	}
	return c.dark[y*c.Size+x]
}

// Encode draws data as a QR code at level M, in the smallest version that
// holds it, under the mask that scores best.
func Encode(data []byte) (Code, error) {
	version, ok := versionFor(len(data))
	if !ok {
		return Code{}, fmt.Errorf("%w: %d bytes, at most %d", ErrTooLong, len(data), MaxBytes)
	}
	return encode(data, version, -1), nil
}

// encode draws data at a version that holds it. mask is 0 to 7, or -1 for
// the one that scores best.
func encode(data []byte, version, mask int) Code {
	g := newGrid(version)
	g.drawFunctionPatterns()
	g.drawCodewords(interleave(codewords(data, version), version))
	if mask < 0 {
		best := -1
		for m := range 8 {
			g.applyMask(m)
			g.drawFormat(m)
			if p := g.penalty(); best < 0 || p < best {
				best, mask = p, m
			}
			g.applyMask(m) // a mask is its own undoing
		}
	}
	g.applyMask(mask)
	g.drawFormat(mask)
	return Code{Version: version, Size: g.size, Mask: mask, dark: g.dark}
}

// versionFor is the smallest version whose data capacity holds n bytes
// with their header.
func versionFor(n int) (int, bool) {
	for v := 1; v <= 40; v++ {
		if 4+countBits(v)+8*n <= 8*dataCodewords(v) {
			return v, true
		}
	}
	return 0, false
}

// countBits is how wide the byte count is in the header.
func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// rawModules is how many modules of a version carry data and error
// correction, with every function pattern taken out.
func rawModules(version int) int {
	n := (16*version+128)*version + 64
	if version >= 2 {
		align := version/7 + 2
		n -= (25*align-10)*align - 55
		if version >= 7 {
			n -= 36
		}
	}
	return n
}

func dataCodewords(version int) int {
	return rawModules(version)/8 - eccPerBlock[version]*blockCount[version]
}

// codewords is the data as the version's data codewords: byte mode, the
// count, the bytes, a terminator, and padding to capacity.
func codewords(data []byte, version int) []byte {
	var b bits
	b.put(0b0100, 4) // byte mode
	b.put(len(data), countBits(version))
	for _, c := range data {
		b.put(int(c), 8)
	}
	capacity := 8 * dataCodewords(version)
	b.put(0, min(4, capacity-b.n))
	b.put(0, (8-b.n%8)%8)
	for pad := 0xEC; b.n < capacity; pad ^= 0xEC ^ 0x11 {
		b.put(pad, 8)
	}
	return b.buf
}

// bits is a bit string, most significant bit first.
type bits struct {
	buf []byte
	n   int
}

func (b *bits) put(value, width int) {
	for i := width - 1; i >= 0; i-- {
		if b.n%8 == 0 {
			b.buf = append(b.buf, 0)
		}
		if value>>i&1 == 1 {
			b.buf[b.n/8] |= 0x80 >> (b.n % 8)
		}
		b.n++
	}
}

// interleave splits the data codewords into the version's blocks, gives
// each its error correction, and deals them out a byte from each in turn,
// so a smudge spoils a little of every block and not all of one.
func interleave(data []byte, version int) []byte {
	blocks, ecc := blockCount[version], eccPerBlock[version]
	raw := rawModules(version) / 8
	short := blocks - raw%blocks // how many blocks are a byte shorter
	shortData := raw/blocks - ecc
	divisor := rsDivisor(ecc)

	dataOf := make([][]byte, blocks)
	eccOf := make([][]byte, blocks)
	for i, at := 0, 0; i < blocks; i++ {
		n := shortData
		if i >= short {
			n++
		}
		dataOf[i] = data[at : at+n]
		eccOf[i] = rsRemainder(dataOf[i], divisor)
		at += n
	}
	out := make([]byte, 0, raw)
	for i := 0; i <= shortData; i++ {
		for j := range blocks {
			if i < len(dataOf[j]) {
				out = append(out, dataOf[j][i])
			}
		}
	}
	for i := range ecc {
		for j := range blocks {
			out = append(out, eccOf[j][i])
		}
	}
	return out
}

// Reed-Solomon over GF(2^8) with the QR polynomial x^8+x^4+x^3+x^2+1.

func gfMul(x, y byte) byte {
	var z int
	for i := 7; i >= 0; i-- {
		z = z<<1 ^ (z>>7)*0x11D
		z ^= int(y>>i&1) * int(x)
	}
	return byte(z)
}

// rsDivisor is the generator polynomial of the given degree, its leading
// coefficient left out: the product of (x - 2^i) for i below degree.
func rsDivisor(degree int) []byte {
	out := make([]byte, degree)
	out[degree-1] = 1
	root := byte(1)
	for range degree {
		for j := range out {
			out[j] = gfMul(out[j], root)
			if j+1 < degree {
				out[j] ^= out[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return out
}

// rsRemainder is the error correction for data: what is left of it
// divided by the generator.
func rsRemainder(data, divisor []byte) []byte {
	out := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ out[0]
		copy(out, out[1:])
		out[len(out)-1] = 0
		for i, d := range divisor {
			out[i] ^= gfMul(d, factor)
		}
	}
	return out
}

// grid is a code being drawn: every module, and which of them belong to a
// function pattern and so take no data and no mask.
type grid struct {
	version  int
	size     int
	dark     []bool
	function []bool
}

func newGrid(version int) *grid {
	size := version*4 + 17
	return &grid{version: version, size: size, dark: make([]bool, size*size), function: make([]bool, size*size)}
}

func (g *grid) at(x, y int) bool { return g.dark[y*g.size+x] }

// setFunction draws a module of a function pattern.
func (g *grid) setFunction(x, y int, dark bool) {
	g.dark[y*g.size+x] = dark
	g.function[y*g.size+x] = true
}

func (g *grid) drawFunctionPatterns() {
	// The timing patterns: alternating modules along row 6 and column 6.
	for i := range g.size {
		g.setFunction(6, i, i%2 == 0)
		g.setFunction(i, 6, i%2 == 0)
	}
	// The three finders, with the light band around each.
	g.drawFinder(3, 3)
	g.drawFinder(g.size-4, 3)
	g.drawFinder(3, g.size-4)
	// The alignment patterns, except where a finder is.
	pos := alignmentPositions(g.version)
	last := len(pos) - 1
	for i, cx := range pos {
		for j, cy := range pos {
			if (i == 0 && j == 0) || (i == 0 && j == last) || (i == last && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					g.setFunction(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	// Room for the format information, written once the mask is known, and
	// the version's own number from version 7.
	g.drawFormat(0)
	g.drawVersion()
}

func (g *grid) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= g.size || y >= g.size {
				continue
			}
			d := max(abs(dx), abs(dy))
			g.setFunction(x, y, d != 2 && d != 4)
		}
	}
}

// alignmentPositions is where the alignment patterns' centres are, along
// either axis.
func alignmentPositions(version int) []int {
	if version == 1 {
		return nil
	}
	n := version/7 + 2
	step := (version*8 + n*3 + 5) / (n*4 - 4) * 2
	out := make([]int, n)
	out[0] = 6
	for i, pos := n-1, version*4+17-7; i >= 1; i, pos = i-1, pos-step {
		out[i] = pos
	}
	return out
}

// drawFormat writes the level and the mask, with their own error
// correction, beside the top left finder and again split between the other
// two.
func (g *grid) drawFormat(mask int) {
	data := formatM<<3 | mask
	rem := data
	for range 10 {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	word := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return word>>i&1 == 1 }

	for i := 0; i <= 5; i++ {
		g.setFunction(8, i, bit(i))
	}
	g.setFunction(8, 7, bit(6))
	g.setFunction(8, 8, bit(7))
	g.setFunction(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		g.setFunction(14-i, 8, bit(i))
	}
	for i := range 8 {
		g.setFunction(g.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		g.setFunction(8, g.size-15+i, bit(i))
	}
	g.setFunction(8, g.size-8, true) // always dark
}

// drawVersion writes the version, with its error correction, in two blocks
// by the far finders. Versions below 7 have none.
func (g *grid) drawVersion() {
	if g.version < 7 {
		return
	}
	rem := g.version
	for range 12 {
		rem = rem<<1 ^ (rem>>11)*0x1F25
	}
	word := g.version<<12 | rem
	for i := range 18 {
		dark := word>>i&1 == 1
		a, b := g.size-11+i%3, i/3
		g.setFunction(a, b, dark)
		g.setFunction(b, a, dark)
	}
}

// drawCodewords lays the bits in the zigzag: two columns at a time from
// the right, up then down, skipping the function patterns and the timing
// column.
func (g *grid) drawCodewords(data []byte) {
	i := 0
	for right := g.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := range g.size {
			for j := range 2 {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = g.size - 1 - vert
				}
				if g.function[y*g.size+x] || i >= len(data)*8 {
					continue
				}
				g.dark[y*g.size+x] = data[i/8]>>(7-i%8)&1 == 1
				i++
			}
		}
	}
}

// applyMask flips the data modules a mask picks. Applying it twice puts
// them back.
func (g *grid) applyMask(mask int) {
	for y := range g.size {
		for x := range g.size {
			if g.function[y*g.size+x] {
				continue
			}
			var flip bool
			switch mask {
			case 0:
				flip = (x+y)%2 == 0
			case 1:
				flip = y%2 == 0
			case 2:
				flip = x%3 == 0
			case 3:
				flip = (x+y)%3 == 0
			case 4:
				flip = (x/3+y/2)%2 == 0
			case 5:
				flip = x*y%2+x*y%3 == 0
			case 6:
				flip = (x*y%2+x*y%3)%2 == 0
			case 7:
				flip = ((x+y)%2+x*y%3)%2 == 0
			}
			if flip {
				g.dark[y*g.size+x] = !g.dark[y*g.size+x]
			}
		}
	}
}

// penalty scores the code as the standard does: lower is easier to scan.
// Runs of one colour, blocks of one colour, shapes like a finder, and a
// balance far from half dark each cost.
func (g *grid) penalty() int {
	total := 0
	line := make([]bool, g.size)
	for i := range g.size {
		for j := range g.size {
			line[j] = g.at(j, i)
		}
		total += linePenalty(line)
		for j := range g.size {
			line[j] = g.at(i, j)
		}
		total += linePenalty(line)
	}
	for y := 0; y < g.size-1; y++ {
		for x := 0; x < g.size-1; x++ {
			if c := g.at(x, y); c == g.at(x+1, y) && c == g.at(x, y+1) && c == g.at(x+1, y+1) {
				total += 3
			}
		}
	}
	dark := 0
	for _, d := range g.dark {
		if d {
			dark++
		}
	}
	// Five points for each whole 5% the dark share is from half.
	all := g.size * g.size
	total += (abs(dark*20-all*10)+all-1)/all*10 - 10
	if total < 0 {
		return 0
	}
	return total
}

// finderLike is the 1:1:3:1:1 a scanner looks for, with four light modules
// on one side, both ways round.
var finderLike = [2][11]bool{
	{true, false, true, true, true, false, true, false, false, false, false},
	{false, false, false, false, true, false, true, true, true, false, true},
}

// linePenalty scores one row or column: three for a run of five of one
// colour and one for each module past that, and forty for each shape like
// a finder.
func linePenalty(line []bool) int {
	total := 0
	run := 1
	for i := 1; i <= len(line); i++ {
		if i < len(line) && line[i] == line[i-1] {
			run++
			continue
		}
		if run >= 5 {
			total += run - 2
		}
		run = 1
	}
	for i := 0; i+11 <= len(line); i++ {
		for _, pattern := range finderLike {
			match := true
			for k, want := range pattern {
				if line[i+k] != want {
					match = false
					break
				}
			}
			if match {
				total += 40
			}
		}
	}
	return total
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// SVG draws the code as an SVG picture with a light band of quiet modules
// around it, which a scanner needs: four is the standard's. Dark modules
// are one path, a rectangle for each run along a row. It carries no size:
// the page gives it one.
func (c Code) SVG(quiet int) string {
	side := c.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, side, side)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, side, side)
	for y := range c.Size {
		for x := 0; x < c.Size; x++ {
			if !c.Dark(x, y) {
				continue
			}
			run := 1
			for c.Dark(x+run, y) {
				run++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x+quiet, y+quiet, run, run)
			x += run
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
