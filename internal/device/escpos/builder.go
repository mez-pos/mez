// Package escpos builds ESC/POS byte streams for thermal receipt printers:
// the commands (text, alignment, sizes, bold, barcode, QR, cut, drawer kick),
// the layouts a venue prints (customer receipt, kitchen ticket, Z-report),
// and a test page. Sending bytes to a printer is Send, over TCP 9100; the
// durable queue that retries is the print queue's (phase 07, Task 5).
//
// The commands are the Epson TM-series set, which almost every 80mm and 58mm
// thermal printer sold to cafes implements. Where printers differ, this
// package picks the variant with the widest support and says so.
package escpos

import (
	"bytes"
	"errors"
	"fmt"
)

// The control bytes the commands are made of.
const (
	esc = 0x1B
	gs  = 0x1D
	dle = 0x10
	dc4 = 0x14
	lf  = 0x0A
)

// ErrInvalid means a command was given something the printer cannot do: a
// size past 8, a barcode it cannot encode, a QR code too long to print.
var ErrInvalid = errors.New("escpos: invalid command")

// Align is where a line sits on the paper.
type Align byte

// The three alignments, as ESC a numbers them.
const (
	Left   Align = 0
	Center Align = 1
	Right  Align = 2
)

// QRLevel is how much of a QR code can be damaged and still read.
type QRLevel byte

// The four QR error-correction levels, as GS ( k function 169 numbers them.
const (
	QRLow      QRLevel = 48 // 7%
	QRMedium   QRLevel = 49 // 15%
	QRQuartile QRLevel = 50 // 25%
	QRHigh     QRLevel = 51 // 30%
)

// DrawerPin is which of the two drawer-kick connector pins to pulse. Most
// drawers are wired to pin 2; a second drawer, if any, to pin 5.
type DrawerPin byte

// The two pins, as DLE DC4 numbers them.
const (
	DrawerPin2 DrawerPin = 0
	DrawerPin5 DrawerPin = 1
)

// Builder accumulates commands. The first invalid command is kept and every
// later one ignored, so a layout can chain calls and check once in Bytes:
// a half-built receipt is never handed to a printer.
type Builder struct {
	buf bytes.Buffer
	err error
}

// New starts a byte stream with ESC @, which resets the printer to its
// defaults, and ESC t 0, code page PC437: Text sends ASCII only, and PC437 is
// the one page every printer has.
func New() *Builder {
	b := &Builder{}
	b.raw(esc, '@', esc, 't', 0)
	return b
}

// Bytes is the stream built so far, or the first invalid command.
func (b *Builder) Bytes() ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	return bytes.Clone(b.buf.Bytes()), nil
}

func (b *Builder) raw(p ...byte) *Builder {
	if b.err == nil {
		b.buf.Write(p)
	}
	return b
}

func (b *Builder) fail(format string, args ...any) *Builder {
	if b.err == nil {
		b.err = fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
	}
	return b
}

// Text writes s as the printer's ASCII, with no line feed. See Printable for
// what happens to anything else.
func (b *Builder) Text(s string) *Builder { return b.raw([]byte(Printable(s))...) }

// Line writes s and a line feed.
func (b *Builder) Line(s string) *Builder { return b.Text(s).raw(lf) }

// Feed advances the paper n lines (ESC d), 0 to 255.
func (b *Builder) Feed(n int) *Builder {
	if n < 0 || n > 255 {
		return b.fail("feed %d lines", n)
	}
	return b.raw(esc, 'd', byte(n))
}

// Align sets where the following lines sit (ESC a). It applies from the
// start of a line, so set it before the text, not in the middle.
func (b *Builder) Align(a Align) *Builder {
	if a > Right {
		return b.fail("align %d", a)
	}
	return b.raw(esc, 'a', byte(a))
}

// Bold turns emphasis on or off (ESC E).
func (b *Builder) Bold(on bool) *Builder { return b.raw(esc, 'E', flag(on)) }

// Underline turns a one-dot underline on or off (ESC -).
func (b *Builder) Underline(on bool) *Builder { return b.raw(esc, '-', flag(on)) }

// Size sets the character width and height multipliers, 1 to 8 each (GS !).
// Double width halves the columns a line holds.
func (b *Builder) Size(width, height int) *Builder {
	if width < 1 || width > 8 || height < 1 || height > 8 {
		return b.fail("size %dx%d", width, height)
	}
	return b.raw(gs, '!', byte((width-1)<<4|(height-1)))
}

// Normal resets size, bold and underline, and aligns left.
func (b *Builder) Normal() *Builder {
	return b.Size(1, 1).Bold(false).Underline(false).Align(Left)
}

// Barcode prints data as Code 128, code set B, with the digits printed
// under it (GS k 73). height is in dots, 1 to 255; 80 is about 10mm.
// Code set B takes printable ASCII, which covers an invoice number.
func (b *Builder) Barcode(data string, height int) *Builder {
	if height < 1 || height > 255 {
		return b.fail("barcode height %d", height)
	}
	// Two bytes of the length go to the {B code-set prefix.
	if data == "" || len(data) > 253 {
		return b.fail("barcode of %d bytes", len(data))
	}
	for i := 0; i < len(data); i++ {
		if c := data[i]; c < 0x20 || c > 0x7E || c == '{' {
			return b.fail("barcode byte %q is not code set B", c)
		}
	}
	b.raw(gs, 'h', byte(height)) // height
	b.raw(gs, 'w', 2)            // module width: 2 dots reads well on 80mm and 58mm
	b.raw(gs, 'H', 2)            // human-readable digits below
	b.raw(gs, 'k', 73, byte(len(data)+2), '{', 'B')
	return b.raw([]byte(data)...).raw(lf)
}

// maxQR is what the printer's QR store accepts in one go; model 2 at level
// M holds about 2300 bytes, which is more than a receipt ever needs.
const maxQR = 2000

// QR prints data as a model 2 QR code (GS ( k), module size 1 to 16 dots.
// 6 prints about 25mm for a short URL, comfortable for a phone camera.
func (b *Builder) QR(data string, module int, level QRLevel) *Builder {
	if module < 1 || module > 16 {
		return b.fail("QR module %d", module)
	}
	if level < QRLow || level > QRHigh {
		return b.fail("QR level %d", level)
	}
	if data == "" || len(data) > maxQR {
		return b.fail("QR of %d bytes", len(data))
	}
	store := len(data) + 3
	b.raw(gs, '(', 'k', 4, 0, 49, 65, 50, 0)                          // function 165: model 2
	b.raw(gs, '(', 'k', 3, 0, 49, 67, byte(module))                   // function 167: module size
	b.raw(gs, '(', 'k', 3, 0, 49, 69, byte(level))                    // function 169: error correction
	b.raw(gs, '(', 'k', byte(store&0xFF), byte(store>>8), 49, 80, 48) // function 180: store
	b.raw([]byte(data)...)
	return b.raw(gs, '(', 'k', 3, 0, 49, 81, 48) // function 181: print
}

// Cut feeds the paper past the cutter and cuts (GS V 66 n). A partial cut
// leaves a hinge so the slip hangs from the printer for the guest to tear;
// a full cut drops it.
func (b *Builder) Cut(partial bool) *Builder {
	mode := byte(65) // feed and full cut
	if partial {
		mode = 66
	}
	return b.raw(gs, 'V', mode, 3)
}

// KickDrawer pulses the drawer connector (DLE DC4 1 m t), for t × 100ms,
// 1 to 8. It is a real-time command: the printer acts on it as it arrives,
// even mid-receipt or out of paper, which is what a till wants from its
// drawer. 2 (200ms) is the common default in drawer manuals: long enough to
// release the latch, short enough not to heat the solenoid.
func (b *Builder) KickDrawer(pin DrawerPin, pulse100ms int) *Builder {
	if pin > DrawerPin5 {
		return b.fail("drawer pin %d", pin)
	}
	if pulse100ms < 1 || pulse100ms > 8 {
		return b.fail("drawer pulse %d", pulse100ms)
	}
	return b.raw(dle, dc4, 1, byte(pin), byte(pulse100ms))
}

func flag(on bool) byte {
	if on {
		return 1
	}
	return 0
}
