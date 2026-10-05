// Package wire is the framing spoken on the socket between the relay and a
// venue's tunnel client. Both ends import it, and nothing else.
//
// One socket carries every guest of a venue at once, so each WebSocket binary
// message is one frame belonging to one stream:
//
//	byte 0      type
//	bytes 1-8   stream id, big endian
//	the rest    payload
//
// A stream is one HTTP exchange. The relay opens it with a Request, sends the
// request body as Body frames and closes its half with End. The venue answers
// with a Response, Body frames and its own End. Either side abandons a stream
// with a Reset. Heads are JSON; bodies are the bytes themselves, so nothing
// is inflated on its way to a phone on a slow link.
package wire

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Type says what a frame carries.
type Type byte

// The frame types. They are a contract with every venue already installed:
// add to the end, never renumber.
const (
	TypeRequest  Type = 1 // relay to venue: a RequestHead, opens the stream
	TypeResponse Type = 2 // venue to relay: a ResponseHead
	TypeBody     Type = 3 // either way: a piece of the sender's body
	TypeEnd      Type = 4 // either way: the sender's body is complete
	TypeReset    Type = 5 // either way: the stream is abandoned; payload is why
)

const (
	headerLen = 9

	// MaxChunk is the most body one frame carries. Small enough that one
	// guest's download cannot hold the socket for long, large enough that the
	// guest bundle is two or three frames.
	MaxChunk = 32 << 10

	// MaxFrame is the largest message either end accepts. A head with its
	// headers has to fit, and so does a full chunk.
	MaxFrame = headerLen + 64<<10
)

// ErrMalformed is a frame that cannot be read. The socket that sent it is
// not speaking this protocol and is closed.
var ErrMalformed = errors.New("wire: malformed frame")

// Frame is one message on the socket.
type Frame struct {
	Type    Type
	Stream  uint64
	Payload []byte
}

// RequestHead is a guest's request as the venue sees it.
type RequestHead struct {
	Method string `json:"method"`
	// Path is the request URI below the venue's prefix, query included, as
	// the guest sent it: "/t/abc?x=1".
	Path   string      `json:"path"`
	Header http.Header `json:"header"`
}

// ResponseHead is the venue's answer before its body.
type ResponseHead struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
}

// Encode lays a frame out as one message.
func Encode(f Frame) ([]byte, error) {
	if len(f.Payload) > MaxFrame-headerLen {
		return nil, fmt.Errorf("wire: payload of %d bytes is over the frame limit", len(f.Payload))
	}
	out := make([]byte, headerLen+len(f.Payload))
	out[0] = byte(f.Type)
	binary.BigEndian.PutUint64(out[1:headerLen], f.Stream)
	copy(out[headerLen:], f.Payload)
	return out, nil
}

// Decode reads one message as a frame. The payload aliases msg.
func Decode(msg []byte) (Frame, error) {
	if len(msg) < headerLen || len(msg) > MaxFrame {
		return Frame{}, fmt.Errorf("%w: %d bytes", ErrMalformed, len(msg))
	}
	t := Type(msg[0])
	if t < TypeRequest || t > TypeReset {
		return Frame{}, fmt.Errorf("%w: unknown type %d", ErrMalformed, msg[0])
	}
	return Frame{Type: t, Stream: binary.BigEndian.Uint64(msg[1:headerLen]), Payload: msg[headerLen:]}, nil
}

// EncodeHead is Encode for a frame whose payload is a head.
func EncodeHead(t Type, stream uint64, head any) ([]byte, error) {
	payload, err := json.Marshal(head)
	if err != nil {
		return nil, fmt.Errorf("wire: encoding a head: %w", err)
	}
	return Encode(Frame{Type: t, Stream: stream, Payload: payload})
}

// DecodeRequest reads the head out of a Request frame.
func DecodeRequest(f Frame) (RequestHead, error) {
	var head RequestHead
	if f.Type != TypeRequest {
		return head, fmt.Errorf("%w: not a request", ErrMalformed)
	}
	if err := json.Unmarshal(f.Payload, &head); err != nil {
		return head, fmt.Errorf("%w: request head: %v", ErrMalformed, err)
	}
	if head.Method == "" || head.Path == "" || head.Path[0] != '/' {
		return head, fmt.Errorf("%w: request head without a method or a rooted path", ErrMalformed)
	}
	return head, nil
}

// DecodeResponse reads the head out of a Response frame.
func DecodeResponse(f Frame) (ResponseHead, error) {
	var head ResponseHead
	if f.Type != TypeResponse {
		return head, fmt.Errorf("%w: not a response", ErrMalformed)
	}
	if err := json.Unmarshal(f.Payload, &head); err != nil {
		return head, fmt.Errorf("%w: response head: %v", ErrMalformed, err)
	}
	// 1xx is not a final answer, and net/http refuses anything outside 100-999.
	if head.Status < 200 || head.Status > 599 {
		return head, fmt.Errorf("%w: response status %d", ErrMalformed, head.Status)
	}
	return head, nil
}

// WebSocket close codes the relay uses beyond the standard ones. A tunnel
// client reads them to decide how soon to come back.
const (
	// CloseReplaced: another connection presented this venue's key and took
	// its place. Reconnecting at once would only take it back, and two
	// machines with one key would do that to each other forever.
	CloseReplaced = 4001

	// CloseProtocol: the venue sent something this package cannot read.
	CloseProtocol = 4002
)
