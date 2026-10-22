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
	"time"
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
	TypeNotice   Type = 6 // relay to venue: an Entitlement, on stream 0; only to a venue that asked
	TypeHealth   Type = 7 // venue to relay: a Health, on stream 0; only to a relay that asked (ADR-361)
)

// lastType is the highest type this build reads. A frame past it is not
// this protocol, or a newer one than this end speaks.
const lastType = TypeHealth

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
	if t < TypeRequest || t > lastType {
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

	// CloseNotEntitled: the relay no longer carries this venue (ADR-316).
	// The reason is a short code, Entitlement.Reason. The key is as good as
	// it was, so this is not a refusal of the key: whoever keeps the
	// entitlements can change their answer, and a venue comes back to ask
	// within the minute.
	CloseNotEntitled = 4003

	// CloseRevoked: the venue's key was turned off while it was connected
	// (ADR-359). A revoked key is otherwise checked only at the next dial
	// (relay.Keys.SetRevoked), so an operator who turns one off closes the
	// socket too. The venue's dial back is refused: it pairs again.
	CloseRevoked = 4004
)

// What the two ends say about entitlement on the dial, in headers, so that a
// venue is told where it stands whether or not it gets in.
const (
	// HeaderEntitlement is on the relay's answer to a dial, let in or not:
	// an Entitlement as JSON. A venue that does not know the header loses
	// nothing by it.
	HeaderEntitlement = "X-Mez-Entitlement"

	// HeaderNotices is on a venue's dial, with the value "1", when it reads
	// Notice frames. The relay sends none to a venue that did not say so: an
	// older one takes a frame it does not know for a broken line and hangs
	// up.
	HeaderNotices = "X-Mez-Tunnel-Notices"

	// CodeNotEntitled is the error code on the relay's answer to a dial it
	// turns away for want of entitlement, with status 403.
	CodeNotEntitled = "not_entitled"

	// HeaderHealth is on the relay's answer to a dial it lets in, with the
	// value "1", when it reads Health frames (ADR-361). A venue sends none
	// to a relay that did not say so: an older one takes a frame it does
	// not know for a broken line and hangs up.
	HeaderHealth = "X-Mez-Tunnel-Health"
)

// Health is what a venue says of its own health, on stream 0, to a relay
// that said it reads it (ADR-361). It is read from the venue's own records
// and settings, never asked of anything outside the venue; the cloud's
// clock alerts are drawn from it (Phase 15). Nothing in it is a guest's.
type Health struct {
	// At is the venue's clock when the summary was made, UTC.
	At time.Time `json:"at"`
	// Timezone is the venue's, an IANA name, which Hours is read in.
	Timezone string `json:"timezone"`
	// Printers is each printer the venue has, active or not.
	Printers []PrinterHealth `json:"printers"`
	Backup   BackupHealth    `json:"backup"`
	// Shift is the shift now open, or nil when none is.
	Shift *ShiftHealth `json:"shift,omitempty"`
	// Hours is when the venue is open, by weekday ("mon" to "sun"), as the
	// owner set them (ADR-360). A day not named is a day it is closed. Nil
	// when the venue has not set them, which the cloud reads as "unknown",
	// not "never open".
	Hours map[string]DayHours `json:"hours,omitempty"`
}

// PrinterHealth is one printer: when it last took a job, and when one last
// failed on it. A printer whose failure is newer than its success is one
// that is not printing.
type PrinterHealth struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	// LastSuccess is when a job was last printed whole, UTC. Nil if none
	// has been.
	LastSuccess *time.Time `json:"last_success,omitempty"`
	// LastFailure is when a job last failed or died on it, UTC. Nil if
	// none has.
	LastFailure *time.Time `json:"last_failure,omitempty"`
	// Error is the newest failure's words, when there is one.
	Error string `json:"error,omitempty"`
}

// BackupHealth is the venue's backup as the venue sees it.
type BackupHealth struct {
	// State is "off", "starting", "ok" or "failing" (internal/backup).
	State string `json:"state"`
	// LastSyncedAt is when the destination last had everything, UTC. Nil
	// if it has not since the server started.
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	// Error is what is wrong, while it is.
	Error string `json:"error,omitempty"`
}

// ShiftHealth is the open shift: enough to say "still open since".
type ShiftHealth struct {
	// ID is the shift's public id.
	ID         string    `json:"id"`
	TradingDay string    `json:"trading_day"`
	OpenedAt   time.Time `json:"opened_at"`
}

// DayHours is one weekday's opening and closing, "09:00" and "23:00" in
// the venue's timezone. A close at or before the open is the next day's:
// "17:00" to "02:00" is a bar.
type DayHours struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// EncodeHealth lays a Health out as a Health frame.
func EncodeHealth(h Health) ([]byte, error) {
	return EncodeHead(TypeHealth, 0, h)
}

// DecodeHealth reads the Health out of a Health frame.
func DecodeHealth(f Frame) (Health, error) {
	var h Health
	if f.Type != TypeHealth {
		return h, fmt.Errorf("%w: not a health frame", ErrMalformed)
	}
	if err := json.Unmarshal(f.Payload, &h); err != nil {
		return h, fmt.Errorf("%w: health: %v", ErrMalformed, err)
	}
	return h, nil
}

// Entitlement is where a venue stands with the relay, as the venue is told
// it: on the answer to its dial, and in a Notice when it changes.
type Entitlement struct {
	// State is the word whoever keeps the entitlements uses: "trial",
	// "active", "grace". Absent from a relay that keeps a file.
	State string `json:"state,omitempty"`
	// PaidUntil is the end of what has been paid for. Absent when nobody is
	// counting.
	PaidUntil *time.Time `json:"paid_until,omitempty"`
	// Until is when the relay stops carrying the venue. Absent when no date
	// is set, and when the venue is not carried at all.
	Until *time.Time `json:"until,omitempty"`
	// Remote says the relay carries the back office for this venue.
	Remote bool `json:"remote"`
	// Reason is a short code for why the venue stands where it does.
	Reason string `json:"reason,omitempty"`
	// LastReport is when the venue's last close-of-day message went to its
	// owner (ADR-364). Absent when none has, and from a relay that sends
	// none.
	LastReport *time.Time `json:"last_report,omitempty"`
	// MessagesURL is where the owner changes what they are sent about this
	// venue: the venue's page in the cloud's account area. Absent from a
	// relay that sends nothing. The venue shows it only if it is https.
	MessagesURL string `json:"messages_url,omitempty"`
}

// EncodeNotice lays an Entitlement out as a Notice frame.
func EncodeNotice(e Entitlement) ([]byte, error) {
	return EncodeHead(TypeNotice, 0, e)
}

// DecodeNotice reads the Entitlement out of a Notice frame.
func DecodeNotice(f Frame) (Entitlement, error) {
	var e Entitlement
	if f.Type != TypeNotice {
		return e, fmt.Errorf("%w: not a notice", ErrMalformed)
	}
	if err := json.Unmarshal(f.Payload, &e); err != nil {
		return e, fmt.Errorf("%w: notice: %v", ErrMalformed, err)
	}
	return e, nil
}
