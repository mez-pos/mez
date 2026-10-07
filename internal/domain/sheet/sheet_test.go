package sheet

import (
	"strings"
	"testing"
)

func TestDecimal(t *testing.T) {
	tests := []struct {
		cell string
		want int64
	}{
		{"250", 250000}, {"1.5", 1500}, {"0.125", 125}, {".5", 500}, {"5.", 5000}, {"0", 0}, {"007", 7000},
		{" 250 ", 250000}, {"1,000", 1000000}, {"1,00,000.5", 100000500}, {"-0.25", -250},
	}
	for _, tt := range tests {
		got, wrong := Decimal(tt.cell, 3, "amount")
		if wrong != "" || got != tt.want {
			t.Errorf("%q: got %d %q, want %d", tt.cell, got, wrong, tt.want)
		}
	}
}

func TestDecimalRefuses(t *testing.T) {
	tests := []struct{ cell, says string }{
		{"", "not an amount"}, {"abc", "not an amount"}, {"1.2.3", "not an amount"}, {"1e3", "not an amount"}, {"--1", "not an amount"},
		{"12 g", "not an amount"}, {".", "not an amount"},
		{"0.1234", "3 decimal places"},
		// Twelve and a half, the way much of the world writes it: not guessed at.
		{"12,5", "point"}, {"1,50", "point"},
		{"99999999999999999999", "too large"},
	}
	for _, tt := range tests {
		if got, wrong := Decimal(tt.cell, 3, "amount"); !strings.Contains(wrong, tt.says) || got != 0 {
			t.Errorf("%q: got %d %q, want a problem saying %q", tt.cell, got, wrong, tt.says)
		}
	}
}

func TestMoney(t *testing.T) {
	if got, wrong := Money("₹ 1,150.50", "INR", "cost"); wrong != "" || got != 115050 {
		t.Errorf("got %d %q", got, wrong)
	}
	if got, wrong := Money("-5", "INR", "cost"); wrong != "" || got != -500 {
		t.Errorf("below zero is read, and is the caller's to refuse: got %d %q", got, wrong)
	}
	for cell, says := range map[string]string{"12,50": "write the cost with a point", "lots": "is not a cost", "1.005": "costs in INR have at most 2"} {
		if _, wrong := Money(cell, "INR", "cost"); !strings.Contains(wrong, says) {
			t.Errorf("%q: %q, want it to say %q", cell, wrong, says)
		}
	}
}

func TestHeadings(t *testing.T) {
	h := Headings{Columns: []string{"Name", "Counted in", "Low at"}, Also: []string{"Status"}, Required: []string{"Name", "Counted in"}}
	columns, wrong := h.Read([]string{" COUNTED_IN ", "", "name", "status"})
	if wrong != "" || strings.Join(columns, "|") != "Counted in||Name|Status" {
		t.Errorf("columns %q, problem %q", columns, wrong)
	}
	for name, tt := range map[string]struct {
		record []string
		says   string
	}{
		"a column it does not know": {[]string{"Name", "Counted in", "Notes"}, `"Notes". Its columns are Name, Counted in, Low at.`},
		"a column twice":            {[]string{"Name", "Counted in", "name"}, "twice"},
		"one missing":               {[]string{"Name"}, "no Counted in column"},
		"two missing":               {[]string{"Low at"}, "no Name and Counted in columns"},
	} {
		if _, wrong := h.Read(tt.record); !strings.Contains(wrong, tt.says) {
			t.Errorf("%s: %q, want it to say %q", name, wrong, tt.says)
		}
	}
}

func TestRowsCountsAsTheSpreadsheetDoesAndStopsReadingPastTheLimit(t *testing.T) {
	records := [][]string{{"Name"}, {""}, {"a"}, {" "}, {"b"}}
	var rows []int
	seen := Rows(records, []string{"Name"}, func(row int, cells map[string]string) { rows = append(rows, row) })
	if seen != 2 || len(rows) != 2 || rows[0] != 3 || rows[1] != 5 {
		t.Errorf("seen %d, rows %v", seen, rows)
	}

	many := [][]string{{"Name"}}
	for range MaxRows + 5 {
		many = append(many, []string{"x"})
	}
	read := 0
	if seen := Rows(many, []string{"Name"}, func(int, map[string]string) { read++ }); seen != MaxRows+5 || read != MaxRows {
		t.Errorf("seen %d, read %d", seen, read)
	}
	if got := CountProblem(MaxRows+5, "Import"); !strings.Contains(got, "1005 rows. Import at most 1000") {
		t.Errorf("too many: %q", got)
	}
	if CountProblem(0, "Import") == "" || CountProblem(3, "Import") != "" {
		t.Error("none is a problem and three is not")
	}
}

func TestReadAndYes(t *testing.T) {
	records, wrong := Read([]byte("\xef\xbb\xbfName;Low at\r\nBeans;\"1,5\"\r\n"))
	if wrong != "" || len(records) != 2 || records[1][1] != "1,5" {
		t.Errorf("records %q, problem %q", records, wrong)
	}
	if _, wrong := Read([]byte("PK\x03\x04\x00")); wrong != NotCSV {
		t.Errorf("a spreadsheet: %q", wrong)
	}
	for cell, want := range map[string][2]bool{"Yes": {true, true}, "": {false, true}, "n": {false, true}, "maybe": {false, false}} {
		if yes, ok := Yes(cell); yes != want[0] || ok != want[1] {
			t.Errorf("%q: %v %v", cell, yes, ok)
		}
	}
}
