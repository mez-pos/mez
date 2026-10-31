package api

import (
	"context"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/service/firstrun"
)

// FirstRun is what the API needs to make the venue on a machine that has
// none. *firstrun.Service is the implementation.
type FirstRun interface {
	Needed(ctx context.Context) (bool, error)
	Run(ctx context.Context, answers firstrun.Answers, extras ...firstrun.Extra) (firstrun.Result, error)
}

// WithFirstRun mounts first run over HTTP (phase 12, Task 3a, ADR-266), for
// the wizard. Without it a venue is made by `mez setup` and nothing else.
func WithFirstRun(f FirstRun) Option {
	return func(s *Server) { s.firstRun = f }
}

// SetupToken is the token a browser must hold to make the venue (ADR-270).
// *firstrun.Token is the implementation.
type SetupToken interface {
	Valid(candidate string) bool
	Spend() error
}

// WithSetupToken makes first run over HTTP ask for the setup token as well
// as for a browser on this machine. cmd/mez always passes one; without it,
// being on the machine is enough, which is what the tests of the rest of
// first run are written against.
func WithSetupToken(t SetupToken) Option {
	return func(s *Server) { s.setupToken = t }
}

// HeaderSetupToken carries the setup token. A header and not a field of
// the body, so that it is never part of what a form could post, and not the
// query, so that it is never in an address or the access log.
const HeaderSetupToken = "X-Setup-Token"

// The first run error codes.
const (
	// CodeSetupLocked is first run asked for on this machine without the
	// setup token, or with one that is not it.
	CodeSetupLocked ErrorCode = "setup_locked"

	// CodeSetupNotHere is first run asked for from anywhere but a browser
	// on the server's own computer.
	CodeSetupNotHere ErrorCode = "setup_not_here"

	// CodeAlreadySetUp is first run on a machine that has its venue.
	CodeAlreadySetUp ErrorCode = "already_set_up"

	// CodeInvalidSetup is an answer first run cannot use. Field names it.
	CodeInvalidSetup ErrorCode = "invalid_setup"

	// CodeSetupNoCode is a venue that was made without its first pairing
	// code. The venue exists: `mez pair` at the machine makes the code.
	CodeSetupNoCode ErrorCode = "setup_no_code"
)

// setupLimit is what one address may spend on first run. The answers are
// hashed with argon2 before they are written, so each attempt costs the
// server real work; a person correcting a form needs a handful.
var setupLimit = limit{requests: 10, per: time.Minute}

// setupStatusResponse says whether the venue is still to be made, whether
// this caller is where it can be made from, and whether it holds what
// opens it: the setup token, sent with the question.
type setupStatusResponse struct {
	Needed   bool `json:"needed"`
	Here     bool `json:"here"`
	Unlocked bool `json:"unlocked"`
}

// setupRequest is the body of POST /api/v1/setup: the wizard's answers.
type setupRequest struct {
	Name            string          `json:"name"`
	LegalName       string          `json:"legal_name,omitempty"`
	Currency        string          `json:"currency"`
	Timezone        string          `json:"timezone"`
	FYStartMonth    int             `json:"fy_start_month"`
	TaxProfile      string          `json:"tax_profile"`
	TaxRegistration string          `json:"tax_registration,omitempty"`
	TaxPricing      string          `json:"tax_pricing,omitempty"`
	OwnerName       string          `json:"owner_name"`
	OwnerPIN        string          `json:"owner_pin"`
	DeviceName      string          `json:"device_name"`
	DeviceKind      auth.DeviceKind `json:"device_kind"`
}

// setupResponse is the venue that was made, the staff member to sign in
// as, and the code that pairs the first device. The PIN is not in it: the
// person who chose it is the one reading this.
type setupResponse struct {
	VenueID     string    `json:"venue_id"`
	OwnerID     string    `json:"owner_id"`
	PairingCode string    `json:"pairing_code"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (s *Server) mountSetup(mux *http.ServeMux) {
	if s.firstRun == nil {
		return
	}
	mux.HandleFunc("GET /api/v1/setup", s.setupStatus)
	mux.Handle("POST /api/v1/setup", s.rateLimit(setupLimit, http.HandlerFunc(s.runSetup)))
	mux.HandleFunc("/api/v1/setup", s.methodNotAllowed)
}

// setupStatus answers anyone: a till on the LAN asks it too, to tell "pair
// this device" from "the venue is not made yet, go to the computer".
func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	needed, err := s.firstRun.Needed(r.Context())
	if err != nil {
		s.internalError(w, r, "reading whether the venue is set up failed", err)
		return
	}
	here := s.atThisMachine(r)
	w.Header().Set("Cache-Control", "no-store")
	// A caller that is not here is told nothing about a token it sent: only
	// the machine itself may ask whether one is right.
	writeJSON(w, s.log, http.StatusOK, setupStatusResponse{Needed: needed, Here: here, Unlocked: here && s.holdsSetupToken(r)})
}

// holdsSetupToken reports whether a request carries the setup token, or
// whether this server asks for none.
func (s *Server) holdsSetupToken(r *http.Request) bool {
	return s.setupToken == nil || s.setupToken.Valid(r.Header.Get(HeaderSetupToken))
}

// spendSetupToken ends the token once a venue exists.
func (s *Server) spendSetupToken(r *http.Request) {
	if s.setupToken == nil {
		return
	}
	if err := s.setupToken.Spend(); err != nil {
		// It opens nothing now either way: first run refuses a second venue.
		s.log.Warn("the setup token could not be removed", "request_id", RequestID(r.Context()), "error", err)
	}
}

// runSetup makes the venue. It is the one endpoint that writes with nobody
// signed in and no device paired, because it is what makes the first of
// each; what it has in their place is atThisMachine.
func (s *Server) runSetup(w http.ResponseWriter, r *http.Request) {
	if !s.atThisMachine(r) {
		writeError(w, s.log, http.StatusForbidden, CodeSetupNotHere,
			"The venue is set up at the computer mez runs on. Open mez there.")
		return
	}
	// On the machine is not enough: so is every other user of it, and every
	// program (ADR-270).
	if !s.holdsSetupToken(r) {
		writeError(w, s.log, http.StatusForbidden, CodeSetupLocked,
			"Open mez from the installer, or from the setup link it prints when it starts, to set this venue up.")
		return
	}
	// A form on another site can post text/plain here without the browser
	// asking first. It cannot post JSON.
	if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
		writeError(w, s.log, http.StatusUnsupportedMediaType, CodeInvalidRequest, "the body must be application/json")
		return
	}
	var req setupRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}

	res, err := s.firstRun.Run(r.Context(), firstrun.Answers{
		VenueName: req.Name, LegalName: req.LegalName, Currency: req.Currency, Timezone: req.Timezone,
		FYStartMonth: req.FYStartMonth, TaxProfile: req.TaxProfile, TaxRegistration: req.TaxRegistration,
		TaxPricing: req.TaxPricing, OwnerName: req.OwnerName, OwnerPIN: req.OwnerPIN,
		DeviceName: req.DeviceName, DeviceKind: req.DeviceKind,
	}, firstrun.StarterMenu)
	var invalid *firstrun.Invalid
	switch {
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidSetup, invalid.Message, invalid.Field)
		return
	case errors.Is(err, firstrun.ErrAlreadySetUp):
		s.spendSetupToken(r)
		writeError(w, s.log, http.StatusConflict, CodeAlreadySetUp, "This venue is already set up.")
		return
	case errors.Is(err, firstrun.ErrNoCode):
		s.spendSetupToken(r)
		s.log.Error("first run made the venue without its pairing code",
			"request_id", RequestID(r.Context()), "venue_id", res.VenueID, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeSetupNoCode,
			"The venue was created, but the code to pair this device was not. Run `mez pair` on this computer.")
		return
	case err != nil:
		s.internalError(w, r, "first run failed", err)
		return
	}
	s.spendSetupToken(r)
	s.log.Info("the venue was set up", "request_id", RequestID(r.Context()), "venue_id", res.VenueID, "actor_id", res.OwnerID)

	// The code pairs a device, so it is a secret for as long as it lives.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusCreated, setupResponse{
		VenueID: res.VenuePublicID, OwnerID: res.OwnerPublicID,
		PairingCode: res.Code.Code, ExpiresAt: res.Code.ExpiresAt.UTC(),
	})
}

// forwardingHeaders are what a proxy or a tunnel adds to say whom it is
// asking for, and forwardingPrefixes the families of them. Canonical form,
// as net/http keys a header.
var (
	forwardingHeaders  = []string{"Forwarded", "Via", "Cf-Connecting-Ip", "True-Client-Ip"}
	forwardingPrefixes = []string{"X-Forwarded-", "X-Real-"}
)

// forwarded reports whether a request says it was passed on by a proxy.
func forwarded(h http.Header) bool {
	for _, name := range forwardingHeaders {
		if len(h.Values(name)) > 0 {
			return true
		}
	}
	for name := range h {
		for _, prefix := range forwardingPrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

// atThisMachine reports whether a request comes from a browser on the
// server's own computer, sent to the server by its own name. It is what
// "being sat at the machine" (ADR-058) comes to over HTTP, and every part
// of it is needed (ADR-266):
//
//   - The caller's address is loopback: nothing on the LAN or beyond it.
//   - The Host it asked for is loopback too. A page on another site can
//     point its own name at 127.0.0.1 and have the browser call "itself";
//     the address is then loopback and the Host is still the site's.
//   - It came through no proxy that says it is one. A proxy on this
//     machine, in front for TLS, makes every caller in the world arrive
//     from loopback.
//   - If the browser names the page that sent it, that page is ours. A
//     page on another site can post to localhost blind, and the browser
//     says so in Origin, and in Sec-Fetch-Site where it sends that.
//
// A guest's request through the relay never gets here: the tunnel puts
// every path under the guest app's mount.
//
// It is a browser's evidence and holds against a browser. A program on
// this machine, or a forwarder on it that adds no header, is let through:
// the machine is trusted, as it was by `mez setup`.
func (s *Server) atThisMachine(r *http.Request) bool {
	// Through the relay is the opposite of here, whatever address the relay
	// said the caller had: one run on this machine reports loopback.
	if relayed(r) {
		return false
	}
	if ip := net.ParseIP(remoteHost(r)); ip == nil || !ip.IsLoopback() {
		return false
	}
	if !loopbackHost(r.Host) {
		return false
	}
	if forwarded(r.Header) {
		return false
	}
	origin := r.Header.Get("Origin")
	if s.cfg.CORSEnabled() && origin == s.cfg.DevOrigin {
		return true
	}
	// "none" is an address typed or a shortcut opened, not a page.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// loopbackHost reports whether a Host header names this machine by
// "localhost" or a loopback address, with or without a port.
func loopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
