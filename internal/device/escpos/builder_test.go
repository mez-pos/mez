package escpos

import (
	"bytes"
	"errors"
	"testing"
)

// preamble is what New writes: ESC @ and code page PC437.
var preamble = []byte{0x1B, '@', 0x1B, 't', 0}

func TestCommands(t *testing.T) {
	tests := []struct {
		name  string
		build func(*Builder) *Builder
		want  []byte
	}{
		{"text is ASCII", func(b *Builder) *Builder { return b.Text("Café ₹5") }, []byte("Cafe Rs.5")},
		{"line ends in LF", func(b *Builder) *Builder { return b.Line("hi") }, []byte("hi\n")},
		{"feed", func(b *Builder) *Builder { return b.Feed(3) }, []byte{0x1B, 'd', 3}},
		{"align centre", func(b *Builder) *Builder { return b.Align(Center) }, []byte{0x1B, 'a', 1}},
		{"align right", func(b *Builder) *Builder { return b.Align(Right) }, []byte{0x1B, 'a', 2}},
		{"bold on and off", func(b *Builder) *Builder { return b.Bold(true).Bold(false) }, []byte{0x1B, 'E', 1, 0x1B, 'E', 0}},
		{"underline", func(b *Builder) *Builder { return b.Underline(true) }, []byte{0x1B, '-', 1}},
		{"normal size", func(b *Builder) *Builder { return b.Size(1, 1) }, []byte{0x1D, '!', 0x00}},
		{"double height", func(b *Builder) *Builder { return b.Size(1, 2) }, []byte{0x1D, '!', 0x01}},
		{"double width", func(b *Builder) *Builder { return b.Size(2, 1) }, []byte{0x1D, '!', 0x10}},
		{"eight by eight", func(b *Builder) *Builder { return b.Size(8, 8) }, []byte{0x1D, '!', 0x77}},
		{"barcode, code 128 set B", func(b *Builder) *Builder { return b.Barcode("A1/2", 60) }, []byte{
			0x1D, 'h', 60, 0x1D, 'w', 2, 0x1D, 'H', 2, 0x1D, 'k', 73, 6, '{', 'B', 'A', '1', '/', '2', '\n',
		}},
		{"QR code", func(b *Builder) *Builder { return b.QR("hi", 6, QRMedium) }, []byte{
			0x1D, '(', 'k', 4, 0, 49, 65, 50, 0,
			0x1D, '(', 'k', 3, 0, 49, 67, 6,
			0x1D, '(', 'k', 3, 0, 49, 69, 49,
			0x1D, '(', 'k', 5, 0, 49, 80, 48, 'h', 'i',
			0x1D, '(', 'k', 3, 0, 49, 81, 48,
		}},
		{"partial cut", func(b *Builder) *Builder { return b.Cut(true) }, []byte{0x1D, 'V', 66, 3}},
		{"full cut", func(b *Builder) *Builder { return b.Cut(false) }, []byte{0x1D, 'V', 65, 3}},
		{"drawer kick, DLE DC4", func(b *Builder) *Builder { return b.KickDrawer(DrawerPin2, 2) }, []byte{0x10, 0x14, 1, 0, 2}},
		{"second drawer", func(b *Builder) *Builder { return b.KickDrawer(DrawerPin5, 8) }, []byte{0x10, 0x14, 1, 1, 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.build(New()).Bytes()
			if err != nil {
				t.Fatalf("Bytes: %v", err)
			}
			if want := append(bytes.Clone(preamble), tt.want...); !bytes.Equal(got, want) {
				t.Errorf("\n got % X\nwant % X", got, want)
			}
		})
	}
}

// A QR code longer than 255 bytes needs the high byte of the store length.
func TestQRLongStoreLength(t *testing.T) {
	data := string(bytes.Repeat([]byte{'x'}, 300))
	got, err := New().QR(data, 4, QRLow).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	store := []byte{0x1D, '(', 'k', byte(303 & 0xFF), byte(303 >> 8), 49, 80, 48}
	if !bytes.Contains(got, store) {
		t.Errorf("no store header for 303 bytes in % X", got[:40])
	}
}

func TestInvalidCommands(t *testing.T) {
	tests := []struct {
		name  string
		build func(*Builder) *Builder
	}{
		{"size 0", func(b *Builder) *Builder { return b.Size(0, 1) }},
		{"size 9", func(b *Builder) *Builder { return b.Size(1, 9) }},
		{"feed 256", func(b *Builder) *Builder { return b.Feed(256) }},
		{"align 3", func(b *Builder) *Builder { return b.Align(3) }},
		{"empty barcode", func(b *Builder) *Builder { return b.Barcode("", 60) }},
		{"barcode with a brace", func(b *Builder) *Builder { return b.Barcode("a{b", 60) }},
		{"barcode with a control byte", func(b *Builder) *Builder { return b.Barcode("a\nb", 60) }},
		{"barcode height 0", func(b *Builder) *Builder { return b.Barcode("a", 0) }},
		{"QR module 17", func(b *Builder) *Builder { return b.QR("a", 17, QRLow) }},
		{"QR level", func(b *Builder) *Builder { return b.QR("a", 4, 52) }},
		{"QR too long", func(b *Builder) *Builder { return b.QR(string(make([]byte, maxQR+1)), 4, QRLow) }},
		{"drawer pin", func(b *Builder) *Builder { return b.KickDrawer(2, 2) }},
		{"drawer pulse 9", func(b *Builder) *Builder { return b.KickDrawer(DrawerPin2, 9) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Later commands are ignored: the stream is refused whole.
			got, err := tt.build(New()).Line("after").Bytes()
			if !errors.Is(err, ErrInvalid) || got != nil {
				t.Errorf("Bytes = %q, %v; want nil, ErrInvalid", got, err)
			}
		})
	}
}

func TestBytesIsACopy(t *testing.T) {
	b := New().Line("a")
	first, _ := b.Bytes()
	first[len(first)-1] = 'Z'
	second, _ := b.Bytes()
	if second[len(second)-1] != '\n' {
		t.Error("changing the returned bytes changed the builder")
	}
}
