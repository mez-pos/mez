// Package sheet reads a CSV file as a spreadsheet saves one: the part of an
// import that is the same whatever is being imported (ADR-288, ADR-298).
//
// It knows nothing of menus or stock. It turns bytes into rows of cells by
// column, and says in an owner's words what is wrong with a file that is
// not one. What a cell means is its caller's.
package sheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxRows is the most rows one file may hold. A venue's whole menu or
// store is well under it, and it bounds what one request can ask of the
// database.
const MaxRows = 1000

// Problem is why a row cannot be used. Row is its row in the spreadsheet,
// the header being row 1; row 0 is a problem with the file as a whole. It
// has no column, unless it is about one that every row needs.
type Problem struct {
	Row     int
	Column  string
	Message string
}

// Report is how a cell's reader says what is wrong with it.
type Report func(column, message string)

// NotCSV is what is said of a file that is not UTF-8 text.
const NotCSV = "This is not a CSV file the import can read. In your spreadsheet, save it as CSV UTF-8 and choose that file."

// Read splits a file into rows of cells, the header first. The string is
// what is wrong with the file, or empty.
//
// It takes the file as it comes: a byte order mark, either line ending, and
// a comma, a semicolon or a tab between cells, whichever the header row has
// most of. A spreadsheet set to a language that writes decimals with a comma
// saves CSV with semicolons.
func Read(data []byte) ([][]string, string) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, "The file is empty."
	}
	// A spreadsheet's own format, or text in an encoding that is not UTF-8,
	// would read as rows of nonsense.
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, NotCSV
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter(data)
	reader.FieldsPerRecord = -1
	var records [][]string
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line, _ := reader.FieldPos(0)
			return nil, fmt.Sprintf("The file could not be read near line %d: a quotation mark there is not closed, or is in the middle of a cell.", line)
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, "The file is empty."
	}
	return records, ""
}

func delimiter(data []byte) rune {
	first, _, _ := bytes.Cut(data, []byte("\n"))
	best, most := ',', bytes.Count(first, []byte(","))
	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(first, []byte(string(d))); n > most {
			best, most = d, n
		}
	}
	return best
}

// Headings is what a file's first row may hold.
type Headings struct {
	// Columns are the file's columns by the names its template gives them:
	// what a file is told its columns are when it has one that is not.
	Columns []string
	// Also are columns taken and not listed: one another file of the same
	// rows has, which this one does not read.
	Also []string
	// Required must be in the file. The rest may be left out.
	Required []string
}

// Read reads a header row into the column each position holds, or "" for a
// position with no heading. A heading is matched without regard to case,
// spaces, hyphens or underscores. The second string is what is wrong with
// the row, or empty.
func (h Headings) Read(record []string) ([]string, string) {
	known := make(map[string]string, len(h.Columns)+len(h.Also))
	for _, c := range h.Columns {
		known[Fold(c)] = c
	}
	for _, c := range h.Also {
		known[Fold(c)] = c
	}
	columns := make([]string, len(record))
	have := make(map[string]bool, len(record))
	for i, cell := range record {
		heading := strings.TrimSpace(cell)
		if heading == "" {
			continue
		}
		column, ok := known[Fold(heading)]
		switch {
		case !ok:
			return nil, fmt.Sprintf("The import does not know the column %q. Its columns are %s.", heading, strings.Join(h.Columns, ", "))
		case have[column]:
			return nil, fmt.Sprintf("The column %s is in the file twice.", column)
		}
		columns[i], have[column] = column, true
	}

	var missing []string
	for _, c := range h.Required {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	switch len(missing) {
	case 0:
		return columns, ""
	case 1:
		return nil, fmt.Sprintf("The file has no %s column. The first row must be the headings from the template.", missing[0])
	}
	last := len(missing) - 1
	return nil, fmt.Sprintf("The file has no %s and %s columns. The first row must be the headings from the template.",
		strings.Join(missing[:last], ", "), missing[last])
}

// Fold is a heading, or a word typed into a cell, as it is compared:
// "Gluten free", "gluten_free" and "GLUTEN-FREE" are one.
func Fold(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// Cells is a row as its cells by column, trimmed. It is empty for a row
// with nothing in it.
func Cells(record, columns []string) map[string]string {
	cells := make(map[string]string, len(columns))
	for i, column := range columns {
		if column == "" || i >= len(record) {
			continue
		}
		if cell := strings.TrimSpace(record[i]); cell != "" {
			cells[column] = cell
		}
	}
	return cells
}

// Rows calls read for each row under the header that has anything in it,
// with its row as the spreadsheet numbers it: the header is row 1, and an
// empty row is counted and not read. It answers how many rows were read,
// and stops reading, though not counting, past MaxRows.
func Rows(records [][]string, columns []string, read func(row int, cells map[string]string)) int {
	seen := 0
	for i, record := range records[1:] {
		cells := Cells(record, columns)
		if len(cells) == 0 {
			continue
		}
		seen++
		if seen <= MaxRows {
			read(i+2, cells)
		}
	}
	return seen
}

// CountProblem is what is wrong with how many rows a file has, or empty.
// do is what the file is for, as "Import" or "Send".
func CountProblem(seen int, do string) string {
	switch {
	case seen == 0:
		return "The file has no rows under its headings."
	case seen > MaxRows:
		return fmt.Sprintf("The file has %d rows. %s at most %d at a time.", seen, do, MaxRows)
	}
	return ""
}

// IsLineBreak is a control character or a Unicode line or paragraph
// separator. Format characters are left alone: the zero-width joiners that
// Devanagari and Malayalam need are among them.
func IsLineBreak(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

// Yes reads a yes or no. Empty is no. The second value is false for a cell
// that is neither.
func Yes(cell string) (yes, ok bool) {
	switch strings.ToLower(cell) {
	case "yes", "y", "true", "1":
		return true, true
	case "", "no", "n", "false", "0":
		return false, true
	}
	return false, false
}

// ExamplePrefix begins the name of every example row a template comes with.
const ExamplePrefix = "Example:"

// IsExample reports whether a name is one of a template's examples, left in
// the file: it begins with ExamplePrefix, whatever its case. Such a row is
// left out of an import and counted, so "Example: Masala dosa" is never an
// item on a menu because an owner did not delete a row (ADR-305).
func IsExample(name string) bool {
	name = strings.TrimSpace(name)
	return len(name) >= len(ExamplePrefix) && strings.EqualFold(name[:len(ExamplePrefix)], ExamplePrefix)
}
