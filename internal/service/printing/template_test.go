package printing

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/device/escpos"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"nothing", "", nil},
		{"one line", "Unit 4", []string{"Unit 4"}},
		{"two lines", "Unit 4\nTel 080", []string{"Unit 4", "Tel 080"}},
		{"trimmed, blanks dropped", "\n  Unit 4  \n\n   \nTel 080\n", []string{"Unit 4", "Tel 080"}},
		{"windows line ends", "Unit 4\r\nTel 080", []string{"Unit 4", "Tel 080"}},
		{"no more than four", "1\n2\n3\n4\n5\n6", []string{"1", "2", "3", "4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitLines(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	// What JoinLines stores, SplitLines reads back.
	lines := []string{"Unit 4", "Tel 080"}
	if got := SplitLines(JoinLines(lines)); !reflect.DeepEqual(got, lines) {
		t.Errorf("round trip = %q", got)
	}
	if got := JoinLines(nil); got != "" {
		t.Errorf("JoinLines(nil) = %q: no lines is nothing stored", got)
	}
}

func TestTemplateOf(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want escpos.Template
	}{
		{"nothing set is the receipt as it always was", nil, escpos.Template{}},
		{"all set", map[string]string{
			SettingReceiptHeader: "Unit 4\nTel 080", SettingReceiptFooter: "Thanks",
			SettingReceiptHideBarcode: "true", SettingReceiptHideOrderRef: "true", SettingReceiptHideModifiers: "true",
		}, escpos.Template{
			Header: []string{"Unit 4", "Tel 080"}, Footer: []string{"Thanks"},
			HideBarcode: true, HideOrderRef: true, HideModifiers: true,
		}},
		{"one part hidden", map[string]string{SettingReceiptHideOrderRef: " true "}, escpos.Template{HideOrderRef: true}},
		{"a switch that is not true is off", map[string]string{
			SettingReceiptHideBarcode: "false", SettingReceiptHideOrderRef: "yes", SettingReceiptHideModifiers: "1",
		}, escpos.Template{}},
		{"unreadable text is kept, to print as it will", map[string]string{
			SettingReceiptFooter: strings.Repeat("स", 3),
		}, escpos.Template{Footer: []string{"ससस"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TemplateOf(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TemplateOf = %+v, want %+v", got, tt.want)
			}
		})
	}
}
