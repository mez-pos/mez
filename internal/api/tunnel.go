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
	TunnelWaiting    TunnelState = "waiting"  // the last attempt failed; another is coming
	TunnelRefused    TunnelState = "refused"  // the relay does not accept this venue's key
	TunnelReplaced   TunnelState = "replaced" // another machine is connected with the same key
	TunnelStopped    TunnelState = "stopped"
)

// TunnelStates is every state, for the generated type.
func TunnelStates() []TunnelState {
	return []TunnelState{TunnelOff, TunnelConnecting, TunnelConnected, TunnelWaiting, TunnelRefused, TunnelReplaced, TunnelStopped}
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
	case TunnelConnecting, TunnelConnected, TunnelWaiting, TunnelRefused, TunnelReplaced, TunnelStopped:
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
	// It changes by the second while reconnecting: never from a cache.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, out)
}
