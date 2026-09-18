package money

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestAmountMarshalsAsMinorUnits(t *testing.T) {
	tests := []struct {
		name string
		in   Amount
		want string
	}{
		{name: "rupees", in: Amount{Minor: 123400, Currency: "INR"}, want: `{"minor":123400,"currency":"INR"}`},
		{name: "zero", in: Amount{Minor: 0, Currency: "INR"}, want: `{"minor":0,"currency":"INR"}`},
		{name: "negative", in: Amount{Minor: -5, Currency: "INR"}, want: `{"minor":-5,"currency":"INR"}`},
		{name: "the zero value", in: Amount{}, want: `{"minor":0,"currency":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshalling %+v: %v", tt.in, err)
			}
			if string(b) != tt.want {
				t.Errorf("json.Marshal(%+v) = %s, want %s", tt.in, b, tt.want)
			}
			// The encoded minor value must be an integer literal. A float
			// field holding whole rupees would still encode as 123400 and
			// slip past the comparison above, so check the token itself.
			minor, ok := strings.CutPrefix(string(b), `{"minor":`)
			if !ok {
				t.Fatalf("json.Marshal(%+v) = %s, which does not start with a minor field", tt.in, b)
			}
			minor, _, _ = strings.Cut(minor, ",")
			if _, err := strconv.ParseInt(minor, 10, 64); err != nil {
				t.Errorf("json.Marshal(%+v) encoded minor as %s, which is not an integer: %v", tt.in, minor, err)
			}
		})
	}
}

func TestAmountUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Amount
		wantErr bool
	}{
		{name: "rupees", in: `{"minor":123400,"currency":"INR"}`, want: Amount{Minor: 123400, Currency: "INR"}},
		{name: "negative", in: `{"minor":-5,"currency":"INR"}`, want: Amount{Minor: -5, Currency: "INR"}},

		// The whole point of the type. A wire format that accepts 1234.56
		// would put a float back into the money path through the back door.
		{name: "a fractional minor unit is not money", in: `{"minor":1234.56,"currency":"INR"}`, wantErr: true},
		{name: "exponent notation", in: `{"minor":1e3,"currency":"INR"}`, wantErr: true},
		{name: "a string minor", in: `{"minor":"1234","currency":"INR"}`, wantErr: true},
		{name: "a bare number is not an amount", in: `1234`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Amount
			err := json.Unmarshal([]byte(tt.in), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("json.Unmarshal(%s) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("json.Unmarshal(%s) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// TestAmountSurvivesAStructRoundTrip guards the shape an API response actually
// has: an Amount nested in a larger object, out and back, unchanged.
func TestAmountSurvivesAStructRoundTrip(t *testing.T) {
	type line struct {
		Name  string `json:"name"`
		Price Amount `json:"price"`
	}

	want := line{Name: "Filter coffee", Price: Amount{Minor: 12000, Currency: "INR"}}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if string(b) != `{"name":"Filter coffee","price":{"minor":12000,"currency":"INR"}}` {
		t.Errorf("marshalled to %s", b)
	}

	var got line
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshalling %s: %v", b, err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}
