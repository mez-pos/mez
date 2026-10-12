package api

import (
	"net/http"
	"net/url"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
)

// TunnelState is where the venue's link to the relay stands. It is a
// vocabulary the back office shows, and it is generated into TypeScript.
type TunnelState string

// The tunnel's states. Off is a venue with no relay set, which is most
// venues and not a fault.
const (
	TunnelOff        TunnelState = "off"
	TunnelConnecting TunnelState = "connecting"
	TunnelConnected  TunnelState = "connected"
	TunnelWaiting    TunnelState = "waiting" // the last attempt failed; another is coming
	TunnelRefused    TunnelState = "refused" // the relay does not accept this venue's key
	// TunnelNotEntitled: the key is good and the relay is not carrying this
	// venue. It asks again within the minute (ADR-316).
	TunnelNotEntitled TunnelState = "not_entitled"
	TunnelReplaced    TunnelState = "replaced" // another machine is connected with the same key
	TunnelStopped     TunnelState = "stopped"
)

// TunnelStates is every state, for the generated type.
func TunnelStates() []TunnelState {
	return []TunnelState{TunnelOff, TunnelConnecting, TunnelConnected, TunnelWaiting, TunnelRefused, TunnelNotEntitled, TunnelReplaced, TunnelStopped}
}

// TunnelStatus is the tunnel as of now, as whatever runs it reports it.
type TunnelStatus struct {
	State TunnelState
	// Since is when it entered this state. Zero when off.
	Since time.Time
	// RetryAt is when the next attempt is due. Zero unless one is.
	RetryAt time.Time
	// Detail is why the last attempt ended, in the client's own words.
	Detail string
	// BackOffice says the venue set remote_backoffice: while the tunnel is
	// connected, the back office can be opened through the relay (ADR-306).
	// It is the setting, not proof that anyone can get in.
	BackOffice bool
	// Cloud is where the relay last said this venue stands with it. Nil
	// when it has said nothing.
	Cloud *TunnelCloud
}

// TunnelCloud is where a venue stands with its relay, as the relay told it
// (ADR-313, ADR-316): what has been paid for, until when it is carried,
// and why. The words are the relay's and nothing in this server acts on
// them; they are for the Devices screen to say.
type TunnelCloud struct {
	// State is "trial", "active", "grace" and the like from a relay that
	// keeps accounts, and empty from one that keeps a file.
	State string
	// PaidUntil is the end of what has been paid for, Until when the relay
	// stops carrying the venue. Either may be zero.
	PaidUntil time.Time
	Until     time.Time
	// BackOffice says the relay carries the back office for this venue.
	BackOffice bool
	// Reason is the relay's short code for why.
	Reason string
}

// TunnelPairing is the account this venue is paired to, and where the
// pairing says it is reached (ADR-318). Neither key is in it.
type TunnelPairing struct {
	Owner    string
	Hostname string
	PairedAt time.Time
}

// WithPairing supplies the venue's pairing to mez Cloud, for the Devices
// screen to say. It reports false for a venue that is not paired, or that
// is and does not use its pairing. A function, as the tunnel's status is,
// so that a venue paired while running is said so without a restart.
func WithPairing(paired func() (TunnelPairing, bool)) Option {
	return func(s *Server) { s.pairing = paired }
}

// WithTunnel supplies the relay tunnel's status, for the health endpoint and
// the back office. It is a function and not the client so that this package
// still knows nothing about the tunnel, and so that the answer is a read of
// what the client already knows: it must not dial anything. A venue with no
// relay does not supply one, and reads off.
func WithTunnel(status func() TunnelStatus) Option {
	return func(s *Server) { s.tunnel = status }
}

// tunnelStatus is the tunnel now. A state outside the vocabulary is read as
// stopped: the back office switches on these words.
func (s *Server) tunnelStatus() TunnelStatus {
	if s.tunnel == nil {
		return TunnelStatus{State: TunnelOff}
	}
	st := s.tunnel()
	switch st.State {
	case TunnelOff:
		return TunnelStatus{State: TunnelOff}
	case TunnelConnecting, TunnelConnected, TunnelWaiting, TunnelRefused, TunnelNotEntitled, TunnelReplaced, TunnelStopped:
		return st
	default:
		st.State = TunnelStopped
		return st
	}
}

// tunnelCheck is the tunnel in health's three words. Off is unknown, since
// there is nothing to fail. With a relay set, anything short of connected
// is failing: a guest scanning a table's code right now would be turned away.
func (s *Server) tunnelCheck() string {
	switch s.tunnelStatus().State {
	case TunnelOff:
		return CheckUnknown
	case TunnelConnected:
		return CheckOK
	default:
		return CheckFailing
	}
}

// tunnelResponse is GET /api/v1/tunnel. since and retry_at are absent when
// there is none; detail is the client's own account of the last failure, for
// whoever is helping the owner, and is absent while connected. back_office
// is always present: whether the venue lets its back office be opened
// through the relay (ADR-306), which is false wherever there is no relay.
//
// Health does not say it. Health is read by anyone who can reach the
// server, and whether a venue's back office answers from outside is not a
// thing to tell them.
type tunnelResponse struct {
	State      TunnelState `json:"state"`
	Since      *time.Time  `json:"since,omitempty"`
	RetryAt    *time.Time  `json:"retry_at,omitempty"`
	Detail     string      `json:"detail,omitempty"`
	BackOffice bool        `json:"back_office"`
	// BackOfficeURL is the address to open the back office at from outside:
	// the venue's public_url with the back office's own path. Absent unless
	// back_office is true and there is such an address to give.
	BackOfficeURL string `json:"back_office_url,omitempty"`
	// Cloud is where the relay says this venue stands with it. Absent when
	// the relay has said nothing: one from before it had anything to say,
	// one not yet reached, or no relay.
	Cloud *tunnelCloud `json:"cloud,omitempty"`
	// PairedTo is the mez Cloud account this venue is paired to. Absent for
	// a venue that is not paired, which is most of them.
	PairedTo *tunnelPairing `json:"paired_to,omitempty"`
}

// tunnelPairing is who a venue is paired to and where that reaches it:
// the owner's name as their account gives it, the venue's hostname, and
// when it was paired.
type tunnelPairing struct {
	Owner    string    `json:"owner"`
	Hostname string    `json:"hostname"`
	PairedAt time.Time `json:"paired_at"`
}

// tunnelCloud is what the relay last told this venue about its standing
// (ADR-316). state is the relay's word ("trial", "active", "grace",
// "suspended", "cancelled", "closed"), absent from a relay that keeps a
// file. paid_until is the end of what has been paid for and until is when
// the relay stops carrying the venue; each is absent when there is none.
// back_office is whether the relay carries the back office, which a venue
// can have turned on and the relay not. reason is the relay's short code.
type tunnelCloud struct {
	State      string     `json:"state,omitempty"`
	PaidUntil  *time.Time `json:"paid_until,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
	BackOffice bool       `json:"back_office"`
	Reason     string     `json:"reason,omitempty"`
}

// mountTunnel adds the tunnel's one route. It is read by whoever manages the
// venue's devices: it is the venue's connection to the outside, and it is
// shown on that screen.
func (s *Server) mountTunnel(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil {
		return
	}
	mux.Handle("GET /api/v1/tunnel",
		s.requireDevice(s.requireStaff(s.requirePermission(auth.PermDeviceManage, http.HandlerFunc(s.showTunnel)))))
	mux.HandleFunc("/api/v1/tunnel", s.methodNotAllowed)
}

// backOfficeURL is where the back office is opened from outside, or empty
// when the venue has not said where it is reached (public_url), or is
// reached under a path. The back office asks for its files by their path
// from the root, so it opens at a hostname of the venue's own and not under
// a relay's /v/<venue>/ (ADR-309). An address that would not open is not
// one to show an owner.
func (s *Server) backOfficeURL() string {
	if s.publicURL == "" {
		return ""
	}
	u, err := url.Parse(s.publicURL)
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + remoteBundle + "/"
}

func (s *Server) showTunnel(w http.ResponseWriter, _ *http.Request) {
	st := s.tunnelStatus()
	out := tunnelResponse{State: st.State, BackOffice: st.BackOffice}
	if st.BackOffice {
		out.BackOfficeURL = s.backOfficeURL()
	}
	if !st.Since.IsZero() {
		since := st.Since.UTC()
		out.Since = &since
	}
	if !st.RetryAt.IsZero() {
		retry := st.RetryAt.UTC()
		out.RetryAt = &retry
	}
	if st.State != TunnelConnected && st.State != TunnelOff {
		out.Detail = st.Detail
	}
	if c := st.Cloud; c != nil && st.State != TunnelOff {
		out.Cloud = &tunnelCloud{State: c.State, BackOffice: c.BackOffice, Reason: c.Reason}
		if !c.PaidUntil.IsZero() {
			paid := c.PaidUntil.UTC()
			out.Cloud.PaidUntil = &paid
		}
		if !c.Until.IsZero() {
			until := c.Until.UTC()
			out.Cloud.Until = &until
		}
	}
	if s.pairing != nil {
		if p, ok := s.pairing(); ok {
			out.PairedTo = &tunnelPairing{Owner: p.Owner, Hostname: p.Hostname, PairedAt: p.PairedAt.UTC()}
		}
	}
	// It changes by the second while reconnecting: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, out)
}
