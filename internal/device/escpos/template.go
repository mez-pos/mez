package escpos

// What a venue may put on its customer documents beyond what its tax profile
// requires (phase 10, Task 6f, ADR-135): a few lines under its name and a few
// at the bottom, and whether three optional parts of a receipt print. The zero
// Template is the receipt as it was before any of this: nothing added, every
// optional part shown.
const (
	// MaxTemplateLines is how many header lines and how many footer lines.
	MaxTemplateLines = 4
	// MaxTemplateLineChars is the longest line: one full line on 80mm paper.
	// On 58mm it wraps.
	MaxTemplateLineChars = 48
)

// Template is a venue's additions to a receipt or credit note. The tax
// profile's own fields (the seller's registration, the number, the date,
// place of supply, HSN, the tax and the total) are not in it and cannot be
// turned off.
type Template struct {
	// Header is printed under the venue's name and registration: an address,
	// a phone number. Footer is printed last: thanks, a Wi-Fi password.
	Header []string
	Footer []string
	// HideBarcode drops the invoice number's barcode, so a refund cannot scan
	// the guest's copy. HideOrderRef drops the order's id under the date; the
	// table number stays. HideModifiers drops the choices under each line.
	HideBarcode   bool
	HideOrderRef  bool
	HideModifiers bool
}

// Unprintable is the first character of s that would print as "?": one
// Printable has no spelling for, such as Devanagari or an emoji. A control
// character counts, a newline included, so check a line at a time. A "?" the
// writer typed is fine.
func Unprintable(s string) (rune, bool) {
	for _, r := range s {
		if r >= 0x20 && r <= 0x7E {
			continue
		}
		if substitutes[r] != "" {
			continue
		}
		return r, true
	}
	return 0, false
}

// textLines lays each line out on its own, wrapped to w. Blank lines are
// dropped: a template has none, and one would only waste paper.
func textLines(lines []string, w int) []string {
	var out []string
	for _, l := range lines {
		for _, s := range wrap(l, w) {
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
