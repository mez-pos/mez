package escpos

import (
	"fmt"
	"strings"
	"time"
)

// TestPage is what `mez --test-print` prints: everything a receipt
// uses, so an owner can see before service that their printer does all of
// it. A line that comes out wrong names the command that failed.
func TestPage(p Paper, at time.Time, version string, kick bool) ([]byte, error) {
	w := p.Columns
	b := New().Align(Center).Bold(true).Size(2, 2).Line("MEZ").Size(1, 1).Line("Printer test").Bold(false)
	b.Line(at.Format("02/01/2006 15:04:05"))
	if version != "" {
		b.Line("mez " + version)
	}
	b.Align(Left).Line(rule('=', w))

	para(b, fmt.Sprintf("Paper: %d columns. The ruler below should fill one line exactly:", w), w)
	b.Line(ruler(w))

	b.Line(rule('-', w)).Line("Alignment")
	b.Align(Left).Line("left")
	b.Align(Center).Line("centre")
	b.Align(Right).Line("right")
	b.Align(Left)

	b.Line(rule('-', w)).Line("Emphasis")
	b.Bold(true).Line("bold").Bold(false)
	b.Underline(true).Line("underline").Underline(false)

	b.Line(rule('-', w)).Line("Sizes")
	b.Size(1, 2).Line("double height").Size(2, 1).Line("double width")
	b.Size(2, 2).Line("double both").Size(1, 1)

	b.Line(rule('-', w)).Line("Columns")
	for _, s := range columns("2 x Filter coffee with oat milk and extra hot", "1,20,000.00", w) {
		b.Line(s)
	}

	b.Line(rule('-', w))
	para(b, "Barcode (Code 128): should scan as NF/26-27/000001", w)
	b.Align(Center).Barcode("NF/26-27/000001", 60).Align(Left)

	b.Line(rule('-', w))
	para(b, "QR code: should read the words "+testQR, w)
	b.Align(Center).QR(testQR, 6, QRMedium).Feed(1).Align(Left)

	b.Line(rule('-', w))
	if kick {
		para(b, "Cash drawer: kicked. It should be open.", w)
		b.KickDrawer(DrawerPin2, 2)
	} else {
		para(b, "Cash drawer: not kicked (add -drawer).", w)
	}
	para(b, "Cut: this slip should be cut below.", w)
	b.Feed(3)
	return b.Cut(true).Bytes()
}

// para writes text wrapped to the paper.
func para(b *Builder, text string, w int) {
	for _, l := range wrap(text, w) {
		b.Line(l)
	}
}

// ruler is w characters with a digit every ten, so a wrong column count is
// read off the paper: "1234567890" repeated, cut to the width.
func ruler(w int) string {
	return strings.Repeat("1234567890", w/10+1)[:w]
}

// testQR is plain words, not a link: a phone shows them, and there is no
// address to go stale.
const testQR = "mez printer test OK"
