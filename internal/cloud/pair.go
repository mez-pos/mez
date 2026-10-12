package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/backup"
)

const (
	// pairPath is where a code is exchanged, under the cloud's address.
	pairPath = "/v1/pair"

	// pairTimeout bounds the one request. The owner is standing at the
	// counter with a code in their hand.
	pairTimeout = 20 * time.Second

	// maxAnswer is the most that is read of the cloud's answer. A pairing
	// is a few hundred bytes.
	maxAnswer = 64 << 10
)

// Why a code was not exchanged.
var (
	// ErrCodeRefused is the cloud not knowing the code, or knowing it and
	// it being spent or too old. The three read alike on purpose: a code is
	// typed by whoever is at the counter, and which it was is not theirs to
	// be told. The owner makes another.
	ErrCodeRefused = errors.New("mez Cloud does not accept that code: it may be mistyped, used already, or older than an hour")

	// ErrBusy is the cloud asking to be asked again shortly.
	ErrBusy = errors.New("mez Cloud is busy; try again in a minute")

	// ErrUnreachable is no usable answer at all: no internet, or a cloud
	// that is down. Nothing was spent: the same code can be tried again.
	ErrUnreachable = errors.New("mez Cloud could not be reached")
)

// About is what the venue says of itself as it pairs.
type About struct {
	// VenueID is the venue's own identifier, the one in its database. It is
	// how the cloud knows a venue it has seen before, whatever it is named
	// now and whichever machine it is on (ADR-313: one trial a venue).
	VenueID string
	// Name is what the venue is called, for the account's list.
	Name string
	// Version is this build's.
	Version string
}

// Client exchanges pairing codes with one cloud.
type Client struct {
	url  string
	http *http.Client
	now  func() time.Time
}

// NewClient is a Client for the cloud at baseURL, which must be https, or
// this machine's own for a test.
func NewClient(baseURL string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if err := secureURL(baseURL); err != nil {
		return nil, fmt.Errorf("cloud: %w", err)
	}
	return &Client{
		url: baseURL,
		http: &http.Client{
			Timeout: pairTimeout,
			// The code is in the body. A cloud that answers "ask over
			// there" is not followed there with it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}, nil
}

// send posts one question to the cloud as JSON and reads its answer, one
// byte past the most an answer may be so that the caller can tell. bearer,
// when there is one, is a recovery's session (ADR-332).
//
// No answer at all is ErrUnreachable. What the answer's status means is the
// caller's to say: it differs by what was asked.
func (c *Client) send(ctx context.Context, path, bearer string, question any) (int, []byte, error) {
	body, err := json.Marshal(question)
	if err != nil {
		return 0, nil, fmt.Errorf("cloud: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("cloud: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Not the error itself to the caller's screen: it names addresses.
		return 0, nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: reading its answer: %v", ErrUnreachable, err)
	}
	return resp.StatusCode, raw, nil
}

// pairRequest is what the venue sends: the code, and who is asking.
type pairRequest struct {
	Code  string       `json:"code"`
	Venue pairingVenue `json:"venue"`
}

type pairingVenue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// pairAnswer is what the cloud sends back for a code it accepts.
type pairAnswer struct {
	Venue  Venue     `json:"venue"`
	Relay  Relay     `json:"relay"`
	Backup backup.S3 `json:"backup"`
	Cloud  struct {
		PublicKey string `json:"public_key"`
	} `json:"cloud"`
	Owner Owner `json:"owner"`
}

// Pair exchanges a code for a pairing. The code is cleaned first, and one
// that could not be a code is refused here without being sent.
//
// What comes back is checked whole before it is returned, and nothing is
// written: keeping it is the caller's, once it is sure.
func (c *Client) Pair(ctx context.Context, typed string, about About) (Pairing, error) {
	code, err := CleanCode(typed)
	if err != nil {
		return Pairing{}, err
	}
	status, raw, err := c.send(ctx, pairPath, "", pairRequest{Code: code, Venue: pairingVenue{ID: about.VenueID, Name: about.Name, Version: about.Version}})
	if err != nil {
		return Pairing{}, err
	}

	switch {
	case status == http.StatusOK:
	case status == http.StatusTooManyRequests:
		return Pairing{}, ErrBusy
	case status >= 400 && status < 500:
		// Whatever it says of the code, the answer here is one.
		return Pairing{}, ErrCodeRefused
	default:
		return Pairing{}, fmt.Errorf("%w: it answered %d", ErrUnreachable, status)
	}
	if len(raw) > maxAnswer {
		return Pairing{}, invalid("mez Cloud's answer is larger than a pairing is")
	}

	var a pairAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return Pairing{}, invalid("mez Cloud's answer could not be read")
	}
	p := Pairing{
		PairedAt: c.now().UTC().Truncate(time.Second), CloudURL: c.url,
		Venue: a.Venue, Relay: a.Relay, Backup: a.Backup, PublicKey: a.Cloud.PublicKey, Owner: a.Owner,
	}
	if err := p.Validate(); err != nil {
		return Pairing{}, err
	}
	return p, nil
}
