package escpos

import (
	"strings"
	"unicode/utf8"
)

// Paper is how many characters a line holds in the printer's standard font
// (Font A, 12 dots wide).
type Paper struct {
	Columns int
}

// The two roll widths cafes use.
var (
	Paper80 = Paper{Columns: 48} // 80mm, 576 dots
	Paper58 = Paper{Columns: 32} // 58mm, 384 dots
)

// substitutes are the characters a menu, a name or a currency brings that
// PC437 cannot print, spelled in ASCII. A name with "é" prints "e" rather
// than a stray byte the printer shows as a Greek letter.
var substitutes = map[rune]string{
	'₹': "Rs.", '£': "GBP ", '€': "EUR ", '¥': "JPY ",
	'\u2013': "-", '\u2014': "-", '‘': "'", '’': "'", '“': `"`, '”': `"`, '…': "...", '×': "x", '·': "-",
	' ': " ", '\t': " ",
	'à': "a", 'á': "a", 'â': "a", 'ä': "a", 'ã': "a", 'å': "a", 'æ': "ae",
	'ç': "c", 'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'ö': "o", 'õ': "o", 'ø': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
	'À': "A", 'Á': "A", 'Â': "A", 'Ä': "A", 'Ã': "A", 'Å': "A", 'Æ': "AE",
	'Ç': "C", 'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I", 'Ñ': "N",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Ö': "O", 'Õ': "O", 'Ø': "O",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U", 'Ý': "Y",
}

// Printable is s as bytes every ESC/POS printer prints the same: ASCII 0x20
// to 0x7E. Accented Latin letters and the usual typographic marks become
// their plain spelling; anything else (Devanagari, emoji, a control byte)
// becomes "?", so an unprintable name shows as a gap, never as a command.
func Printable(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7E:
			b.WriteRune(r)
		case substitutes[r] != "":
			b.WriteString(substitutes[r])
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// wrap breaks text into lines of at most width characters, at spaces where
// it can and mid-word where a word is longer than the line.
func wrap(text string, width int) []string {
	text = strings.Join(strings.Fields(Printable(text)), " ")
	if width < 1 || text == "" {
		return []string{text}
	}
	var lines []string
	for len(text) > width {
		cut := strings.LastIndexByte(text[:width+1], ' ')
		if cut <= 0 {
			lines = append(lines, text[:width])
			text = text[width:]
			continue
		}
		lines = append(lines, text[:cut])
		text = text[cut+1:]
	}
	return append(lines, text)
}

// columns lays out left and right on lines of width: left wraps in the
// space right leaves, and right sits flush right on the first line. A
// right side too wide to share a line goes on a line of its own. Leading
// spaces on left are an indent, kept on every line it wraps to.
func columns(left, right string, width int) []string {
	right = Printable(right)
	indent := strings.Repeat(" ", len(left)-len(strings.TrimLeft(left, " ")))
	room := width - len(right) - 1
	if room-len(indent) < width/3 {
		lines := indentAll(wrap(left, width-len(indent)), indent)
		return append(lines, pad(right, width))
	}
	lines := indentAll(wrap(left, room-len(indent)), indent)
	lines[0] = lines[0] + strings.Repeat(" ", width-len(lines[0])-len(right)) + right
	return lines
}

func indentAll(lines []string, indent string) []string {
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return lines
}

// pad right-aligns s in width.
func pad(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return strings.Repeat(" ", width-n) + s
	}
	return s
}

// rule is a full-width line of c.
func rule(c byte, width int) string { return strings.Repeat(string(c), width) }
