package escpos

import (
	"reflect"
	"testing"
)

func TestReplayReadsWhatABuilderWrote(t *testing.T) {
	b := New()
	b.Align(Center).Bold(true).Size(1, 2).Line("TOTAL").Size(1, 1).Bold(false)
	b.Align(Left).Line("left")
	b.Feed(1)
	b.Align(Center).Barcode("INV/1", 60)
	b.Line("after").Align(Right).Line("right").Cut(true)
	data, err := b.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	page, err := Replay(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{
		{Text: "TOTAL", Align: Center, Width: 1, Height: 2, Bold: true},
		{Text: "left", Align: Left, Width: 1, Height: 1},
		{Text: "", Align: Left, Width: 1, Height: 1},
		{Text: "INV/1", Align: Center, Width: 1, Height: 1, Barcode: true},
		{Text: "after", Align: Center, Width: 1, Height: 1},
		{Text: "right", Align: Right, Width: 1, Height: 1},
	}
	if !reflect.DeepEqual(page.Lines, want) {
		t.Fatalf("lines = %+v\nwant    %+v", page.Lines, want)
	}
	if !reflect.DeepEqual(page.Codes, []string{"barcode:INV/1"}) || !reflect.DeepEqual(page.Cuts, []string{"partial"}) || page.Kicks != 0 {
		t.Errorf("codes %v, cuts %v, kicks %d", page.Codes, page.Cuts, page.Kicks)
	}
}

func TestReplayCountsTheDrawerKick(t *testing.T) {
	b := New()
	b.KickDrawer(DrawerPin2, 2).Line("x")
	data, _ := b.Bytes()
	if page, err := Replay(data); err != nil || page.Kicks != 1 {
		t.Fatalf("%+v, %v", page, err)
	}
}

func TestReplayRefusesWhatItDoesNotKnow(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"an unknown ESC command", []byte{esc, 'Z', 1}},
		{"an unknown GS command", []byte{gs, 'Z', 1}},
		{"a barcode type it does not write", []byte{gs, 'k', 4, 3, 'a', 'b', 'c'}},
		{"a stream that ends inside a command", []byte{esc, 'a'}},
		{"a control byte in text", []byte("ab\x07c\n")},
		{"a byte outside ASCII", []byte("caf\xe9\n")},
		{"text with no line feed at the end", []byte("abc")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if page, err := Replay(tt.data); err == nil {
				t.Errorf("replayed %+v with no error", page)
			}
		})
	}
	if page, err := Replay(nil); err != nil || len(page.Lines) != 0 {
		t.Errorf("an empty stream: %+v, %v", page, err)
	}
}
