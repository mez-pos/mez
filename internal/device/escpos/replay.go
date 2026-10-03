package escpos

import (
	"fmt"
	"strings"
)

// Line is one line of a stream as paper: its text, how it sat on the paper
// and how it was printed. A barcode is a line of its own, its data the text.
type Line struct {
	Text    string
	Align   Align
	Width   int // character width multiplier, 1 to 8
	Height  int // character height multiplier, 1 to 8
	Bold    bool
	Barcode bool
}

// Page is a stream replayed: what a printer would put on the paper, and what
// it would do besides.
type Page struct {
	Lines []Line
	Cuts  []string // "partial" or "full"
	Kicks int      // times the cash drawer is opened
	// Codes are every barcode and QR code, as "barcode:DATA" and "qr:DATA".
	Codes []string
}

// Replay plays a stream the way a printer would, for the commands this
// package writes. A command it does not know is an error: a layout must not
// send anything the package does not know it sent. The same replay checks a
// layout in tests and draws a receipt preview on the settings screen, so what
// the owner sees is what the layout wrote.
func Replay(data []byte) (Page, error) {
	var p Page
	var cur strings.Builder
	width, height, bold, align := 1, 1, false, Left
	var qr string
	i := 0
	var bad error
	need := func(n int) []byte {
		if i+n > len(data) {
			bad = fmt.Errorf("escpos: the stream ends inside a command at %d", i)
			i = len(data)
			return make([]byte, n)
		}
		s := data[i : i+n]
		i += n
		return s
	}
	line := func(text string) Line {
		return Line{Text: text, Align: align, Width: width, Height: height, Bold: bold}
	}
	for i < len(data) && bad == nil {
		c := data[i]
		switch {
		case c == lf:
			p.Lines = append(p.Lines, line(cur.String()))
			cur.Reset()
			i++
		case c == esc:
			op := need(2)[1]
			switch op {
			case '@':
				width, height, bold, align = 1, 1, false, Left
			case 't', '-':
				need(1)
			case 'a':
				align = Align(need(1)[0])
			case 'E':
				bold = need(1)[0] == 1
			case 'd':
				for n := int(need(1)[0]); n > 0; n-- {
					p.Lines = append(p.Lines, line(""))
				}
			default:
				bad = fmt.Errorf("escpos: unknown ESC %q at %d", op, i)
			}
		case c == gs:
			op := need(2)[1]
			switch op {
			case '!':
				n := need(1)[0]
				width, height = int(n>>4)+1, int(n&0x0F)+1
			case 'h', 'w', 'H':
				need(1)
			case 'k':
				if m := need(1)[0]; m != 73 {
					bad = fmt.Errorf("escpos: barcode type %d at %d", m, i)
					break
				}
				n := int(need(1)[0])
				code := string(need(n)[2:])
				p.Codes = append(p.Codes, "barcode:"+code)
				// The printer ends a barcode with its own line feed.
				l := line(code)
				l.Barcode = true
				p.Lines = append(p.Lines, l)
				if i < len(data) && data[i] == lf {
					i++
				}
			case '(':
				if need(1)[0] != 'k' {
					bad = fmt.Errorf("escpos: GS ( not k at %d", i)
					break
				}
				lh := need(2)
				body := need(int(lh[0]) | int(lh[1])<<8)
				if len(body) < 2 {
					break
				}
				switch body[1] {
				case 80:
					qr = string(body[3:])
				case 81:
					p.Codes = append(p.Codes, "qr:"+qr)
				}
			case 'V':
				m := need(2)[0]
				p.Cuts = append(p.Cuts, map[byte]string{65: "full", 66: "partial"}[m])
			default:
				bad = fmt.Errorf("escpos: unknown GS %q at %d", op, i)
			}
		case c == dle:
			need(5)
			p.Kicks++
		case c < 0x20 || c > 0x7E:
			bad = fmt.Errorf("escpos: byte %#x in text at %d", c, i)
		default:
			cur.WriteByte(c)
			i++
		}
	}
	if bad != nil {
		return Page{}, bad
	}
	if cur.Len() > 0 {
		return Page{}, fmt.Errorf("escpos: text %q is not ended with a line feed", cur.String())
	}
	return p, nil
}
