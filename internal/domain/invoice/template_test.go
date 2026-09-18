package invoice

import (
	"errors"
	"testing"
)

func TestTemplateFormat(t *testing.T) {
	india := FinancialYear{StartYear: 2026, Rule: IndiaFY}
	calendar := FinancialYear{StartYear: 2026, Rule: CalendarFY}

	tests := []struct {
		name     string
		template string
		fy       FinancialYear
		seq      int64
		want     string
	}{
		{"the default", DefaultTemplate, india, 1, "NF/2026-27/000001"},
		{"the default, calendar year", DefaultTemplate, calendar, 42, "NF/2026/000042"},
		{"short year for GST", "{prefix}/{fy:short}/{seq:06d}", india, 123, "NF/26-27/000123"},
		{"unpadded", "{prefix}-{seq}", india, 7, "NF-7"},
		{"a number past its padding is written in full", "{seq:03d}", india, 12345, "12345"},
		{"literal text around tokens", "INV {fy} #{seq:04d}.", india, 9, "INV 2026-27 #0009."},
		{"seq only", "{seq}", india, 1, "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := ParseTemplate(tt.template)
			if err != nil {
				t.Fatalf("ParseTemplate(%q): %v", tt.template, err)
			}
			got, err := tmpl.Format("NF", tt.fy, tt.seq)
			if err != nil || got != tt.want {
				t.Errorf("Format = %q, %v; want %q", got, err, tt.want)
			}
			if tmpl.String() != tt.template {
				t.Errorf("String() = %q, want %q", tmpl.String(), tt.template)
			}
		})
	}
}

func TestParseTemplateRefuses(t *testing.T) {
	tests := []string{
		"",                      // no seq
		"{prefix}/{fy}",         // no seq
		"{seq}/{seq}",           // two seqs: numbers could collide
		"{seq:06d}{seq}",        // two seqs
		"{prefix}/{year}/{seq}", // unknown token
		"{prefix/{seq}",         // a token swallowed by another
		"{seq",                  // unclosed
		"INV}{seq}",             // stray }
		"{seq}}",                // stray }
		"{seq:6d}",              // not zero-padded
		"{seq:00d}",             // width zero
		"{seq:019d}",            // too wide
		"{seq:0xd}",             // not a width
		"{seq:06}",              // no d
		"{seq:0+6d}",            // a sign is not a width
	}

	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseTemplate(s); !errors.Is(err, ErrInvalidTemplate) {
				t.Errorf("ParseTemplate(%q) err = %v, want ErrInvalidTemplate", s, err)
			}
		})
	}
}

func TestFormatRefuses(t *testing.T) {
	fy := FinancialYear{StartYear: 2026, Rule: IndiaFY}
	tmpl, err := ParseTemplate(DefaultTemplate)
	if err != nil {
		t.Fatal(err)
	}

	for _, seq := range []int64{0, -1} {
		if _, err := tmpl.Format("NF", fy, seq); !errors.Is(err, ErrInvalidSeq) {
			t.Errorf("Format(seq %d) err = %v, want ErrInvalidSeq", seq, err)
		}
	}
	if _, err := (Template{}).Format("NF", fy, 1); !errors.Is(err, ErrInvalidTemplate) {
		t.Errorf("zero Template: err = %v, want ErrInvalidTemplate", err)
	}
}
