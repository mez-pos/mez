package wire

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{"end with no payload", Frame{Type: TypeEnd, Stream: 1}},
		{"body", Frame{Type: TypeBody, Stream: 42, Payload: []byte("flat white")}},
		{"largest stream id", Frame{Type: TypeReset, Stream: ^uint64(0), Payload: []byte("why")}},
		{"a full chunk", Frame{Type: TypeBody, Stream: 7, Payload: bytes.Repeat([]byte{0xAB}, MaxChunk)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := Encode(tt.frame)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := Decode(msg)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got.Type != tt.frame.Type || got.Stream != tt.frame.Stream || !bytes.Equal(got.Payload, tt.frame.Payload) {
				t.Errorf("round trip changed the frame: got type %d stream %d, %d bytes", got.Type, got.Stream, len(got.Payload))
			}
		})
	}
}

func TestEncodeRefusesOversizePayload(t *testing.T) {
	if _, err := Encode(Frame{Type: TypeBody, Stream: 1, Payload: make([]byte, MaxFrame)}); err == nil {
		t.Fatal("Encode accepted a payload over the frame limit")
	}
}

func TestDecodeRefusesMalformed(t *testing.T) {
	tests := []struct {
		name string
		msg  []byte
	}{
		{"empty", nil},
		{"shorter than a header", []byte{byte(TypeBody), 0, 0}},
		{"type zero", make([]byte, headerLen)},
		{"type past the last", append([]byte{byte(TypeReset) + 1}, make([]byte, 8)...)},
		{"over the frame limit", append([]byte{byte(TypeBody)}, make([]byte, MaxFrame)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.msg); !errors.Is(err, ErrMalformed) {
				t.Errorf("Decode = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestRequestHeadRoundTrip(t *testing.T) {
	want := RequestHead{Method: "POST", Path: "/t/abc?x=1", Header: http.Header{"Accept": {"text/html"}}}
	msg, err := EncodeHead(TypeRequest, 9, want)
	if err != nil {
		t.Fatalf("EncodeHead: %v", err)
	}
	f, err := Decode(msg)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := DecodeRequest(f)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if f.Stream != 9 || got.Method != want.Method || got.Path != want.Path || got.Header.Get("Accept") != "text/html" {
		t.Errorf("got %+v on stream %d, want %+v on stream 9", got, f.Stream, want)
	}
}

func TestDecodeRequestRefuses(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{"wrong frame type", Frame{Type: TypeBody, Payload: []byte(`{"method":"GET","path":"/"}`)}},
		{"not json", Frame{Type: TypeRequest, Payload: []byte("GET /")}},
		{"no method", Frame{Type: TypeRequest, Payload: []byte(`{"path":"/"}`)}},
		{"no path", Frame{Type: TypeRequest, Payload: []byte(`{"method":"GET"}`)}},
		{"path not rooted", Frame{Type: TypeRequest, Payload: []byte(`{"method":"GET","path":"t/abc"}`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRequest(tt.frame); !errors.Is(err, ErrMalformed) {
				t.Errorf("DecodeRequest = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestDecodeResponse(t *testing.T) {
	tests := []struct {
		name    string
		frame   Frame
		want    int
		wantErr bool
	}{
		{"ok", Frame{Type: TypeResponse, Payload: []byte(`{"status":200}`)}, 200, false},
		{"last valid status", Frame{Type: TypeResponse, Payload: []byte(`{"status":599}`)}, 599, false},
		{"informational is not an answer", Frame{Type: TypeResponse, Payload: []byte(`{"status":101}`)}, 0, true},
		{"zero status", Frame{Type: TypeResponse, Payload: []byte(`{}`)}, 0, true},
		{"status past the range", Frame{Type: TypeResponse, Payload: []byte(`{"status":600}`)}, 0, true},
		{"wrong frame type", Frame{Type: TypeRequest, Payload: []byte(`{"status":200}`)}, 0, true},
		{"not json", Frame{Type: TypeResponse, Payload: []byte(`200`)}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeResponse(tt.frame)
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("DecodeResponse = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil || got.Status != tt.want {
				t.Errorf("DecodeResponse = %d, %v; want %d", got.Status, err, tt.want)
			}
		})
	}
}
