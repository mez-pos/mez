package invoice

import (
	"errors"
	"strings"
	"testing"
)

func TestNewSeries(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		prefix   string
		fy       FYRule
		template string
		max      int
		wantErr  error
	}{
		{"the default, no cap", "NF", "NF", IndiaFY, DefaultTemplate, 0, nil},
		{"GST with a short year", "NF", "NF", IndiaFY, "{prefix}/{fy:short}/{seq:06d}", 16, nil},
		{"no prefix", "NF", "", CalendarFY, "{fy}-{seq:05d}", 0, nil},
		// The headline trap: the documented default is 17 characters with
		// a two-letter prefix, one over GST's cap. It fails at setup.
		{"the default is too long for GST", "NF", "NF", IndiaFY, DefaultTemplate, 16, ErrTooLong},
		{"no code", " ", "NF", IndiaFY, DefaultTemplate, 0, ErrInvalidSeries},
		{"negative cap", "NF", "NF", IndiaFY, DefaultTemplate, -1, ErrInvalidSeries},
		{"bad financial year", "NF", "NF", FYRule{StartMonth: 4, StartDay: 31}, DefaultTemplate, 0, ErrInvalidFYRule},
		{"bad template", "NF", "NF", IndiaFY, "{prefix}", 0, ErrInvalidTemplate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSeries(tt.code, tt.prefix, tt.fy, tt.template, tt.max)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (s.Code != tt.code || s.Template.String() != tt.template) {
				t.Errorf("series = %+v", s)
			}
		})
	}
}

func TestSeriesNumberCapsLength(t *testing.T) {
	// A three-letter prefix leaves exactly room for six digits in 16.
	s, err := NewSeries("NFX", "NFX", IndiaFY, "{prefix}/{fy:short}/{seq:06d}", 16)
	if err != nil {
		t.Fatal(err)
	}
	fy := FinancialYear{StartYear: 2026, Rule: IndiaFY}

	got, err := s.Number(fy, 999999)
	if err != nil || got != "NFX/26-27/999999" {
		t.Errorf("Number(999999) = %q, %v", got, err)
	}
	// The seven-digit number outgrows its padding and the cap with it. It
	// must fail rather than be truncated into a duplicate.
	if _, err := s.Number(fy, 1000000); !errors.Is(err, ErrTooLong) {
		t.Errorf("Number(1000000) err = %v, want ErrTooLong", err)
	}
	if _, err := s.Number(fy, 0); !errors.Is(err, ErrInvalidSeq) {
		t.Errorf("Number(0) err = %v, want ErrInvalidSeq", err)
	}
}

func TestKeys(t *testing.T) {
	s, err := NewSeries("NF", "NF", IndiaFY, DefaultTemplate, 0)
	if err != nil {
		t.Fatal(err)
	}
	k := s.KeyFor(7, FinancialYear{StartYear: 2026, Rule: IndiaFY})
	if k != (Key{VenueID: 7, Series: "NF", FYStart: 2026}) || k.Validate() != nil {
		t.Errorf("KeyFor = %+v, Validate = %v", k, k.Validate())
	}
	for _, bad := range []Key{{}, {VenueID: 7, FYStart: 2026}, {VenueID: 7, Series: "NF"}, {Series: "NF", FYStart: 2026}} {
		if err := bad.Validate(); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%+v: err = %v, want ErrInvalidKey", bad, err)
		}
	}
}

func TestAudit(t *testing.T) {
	a := Key{VenueID: 1, Series: "NF", FYStart: 2026}
	b := Key{VenueID: 1, Series: "NF", FYStart: 2027}
	issued := func(k Key, seqs ...int64) []Issued {
		out := make([]Issued, len(seqs))
		for i, s := range seqs {
			out[i] = Issued{Key: k, Seq: s}
		}
		return out
	}
	join := func(parts ...[]Issued) []Issued {
		var out []Issued
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	tests := []struct {
		name    string
		in      []Issued
		wantErr error
		wantSeq int64
		wantKey Key
	}{
		{"nothing issued", nil, nil, 0, Key{}},
		{"in order", issued(a, 1, 2, 3), nil, 0, Key{}},
		{"out of order is fine", issued(a, 3, 1, 2), nil, 0, Key{}},
		{"each year restarts at 1", join(issued(a, 1, 2), issued(b, 1)), nil, 0, Key{}},
		{"missing the first", issued(a, 2, 3), ErrGap, 1, a},
		{"missing one in the middle", issued(a, 1, 2, 4), ErrGap, 3, a},
		{"a number twice", issued(a, 1, 2, 2, 3), ErrDuplicate, 2, a},
		{"a gap in the second year only", join(issued(a, 1, 2), issued(b, 1, 3)), ErrGap, 2, b},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Audit(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				return
			}
			var ae *AuditError
			if !errors.As(err, &ae) || ae.Seq != tt.wantSeq || ae.Key != tt.wantKey {
				t.Errorf("error = %v, want number %d in %+v", err, tt.wantSeq, tt.wantKey)
			}
			if !strings.Contains(err.Error(), "FY") {
				t.Errorf("message %q does not name the year", err.Error())
			}
		})
	}
}
