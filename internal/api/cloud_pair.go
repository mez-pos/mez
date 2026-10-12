package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
)

// Why a venue was not paired to mez Cloud (ADR-318, ADR-320).
const (
	// CodePairingCodeInvalid is a code that could not be one, as typed.
	// Nothing was sent.
	CodePairingCodeInvalid ErrorCode = "pairing_code_invalid"
	// CodePairingCodeRefused is a code mez Cloud does not accept: mistyped,
	// used already, or older than an hour. Which is not said.
	CodePairingCodeRefused ErrorCode = "pairing_code_refused"
	// CodeCloudUnreachable is mez Cloud not answering, or answering
	// something that is not a pairing. The code is not spent by the first;
	// it may be by the second.
	CodeCloudUnreachable ErrorCode = "cloud_unreachable"
	// CodeRelayInSettings is a venue whose settings file names a relay of
	// its own, which a pairing would be set aside for (ADR-319).
	CodeRelayInSettings ErrorCode = "relay_in_settings"
	// CodePairingNotKept is a pairing mez Cloud gave that this computer
	// could not write down. The code is spent.
	CodePairingNotKept ErrorCode = "pairing_not_kept"
	// CodePairingUnavailable is a server that cannot pair.
	CodePairingUnavailable ErrorCode = "pairing_unavailable"
)

// CloudPairing pairs the venue to a mez Cloud account.
// *cloudpair.Service is the implementation.
type CloudPairing interface {
	Pair(ctx context.Context, venueID int64, code string) (cloudpair.Result, error)
}

// WithCloudPairing lets the venue be paired to a mez Cloud account from the
// back office.
func WithCloudPairing(p CloudPairing) Option {
	return func(s *Server) { s.cloudPairing = p }
}

// WithPublicURL supplies the address guests reach the venue at, for a
// server where it can change while running: a venue paired to mez Cloud
// has one from that moment (ADR-320). It is used in place of the address
// WithTableCodes and WithCharges were given.
func WithPublicURL(address func() string) Option {
	return func(s *Server) { s.publicURLNow = address }
}

// public is the address guests reach this venue at, or empty when the venue
// has not said and is not paired.
func (s *Server) public() string {
	if s.publicURLNow != nil {
		return strings.TrimRight(s.publicURLNow(), "/")
	}
	return s.publicURL
}

// cloudPairRequest is a pairing code as the owner typed it. Spaces, hyphens
// and lower case are taken as written.
type cloudPairRequest struct {
	Code string `json:"code"`
}

// cloudPairResponse is a venue that has been paired: who to, where it is
// reached, and when. backup says what was done about the venue's backup:
// "set" to mez Cloud's, "kept" where the venue already backs up somewhere
// of its own, "failed" when mez Cloud's could not be written to, "none" on
// a server with no backup. restart_needed says some of it takes hold only
// when mez is next started. Neither of the keys the pairing holds is here.
type cloudPairResponse struct {
	Owner         string                  `json:"owner"`
	Hostname      string                  `json:"hostname"`
	PairedAt      time.Time               `json:"paired_at"`
	Backup        cloudpair.BackupOutcome `json:"backup"`
	RestartNeeded bool                    `json:"restart_needed"`
}

// cloudPairLimit is what one address may spend on pairing the venue to an
// account: a person typing a code, mistyping it and typing it again, and no
// more. Each try that reaches mez Cloud is a guess at somebody's code, made
// from this venue's address. It is a limit of its own and not the one for
// pairing a device, so that neither spends the other's.
var cloudPairLimit = limit{requests: 6, per: time.Minute}

// mountCloudPairing adds the one route. It is the venue's own to use: a
// code is typed at the venue, and it is not reachable through the relay.
func (s *Server) mountCloudPairing(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil {
		return
	}
	pair := s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, http.HandlerFunc(s.pairToCloud))))
	mux.Handle("POST /api/v1/cloud/pair", s.rateLimit(cloudPairLimit, pair))
	mux.HandleFunc("/api/v1/cloud/pair", s.methodNotAllowed)
}

func (s *Server) pairToCloud(w http.ResponseWriter, r *http.Request) {
	if s.cloudPairing == nil {
		writeError(w, s.log, http.StatusNotFound, CodePairingUnavailable, "This server cannot be paired to mez Cloud.")
		return
	}
	var req cloudPairRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	dev, _ := DeviceFrom(r.Context())

	res, err := s.cloudPairing.Pair(r.Context(), dev.VenueID, req.Code)
	// What a pairing holds must not be cached anywhere, and neither should
	// a refusal that names what was tried.
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case err == nil:
		writeJSON(w, s.log, http.StatusOK, cloudPairResponse{
			Owner: res.Owner, Hostname: res.Hostname, PairedAt: res.PairedAt.UTC(),
			Backup: res.Backup, RestartNeeded: res.RestartNeeded,
		})
	case errors.Is(err, cloud.ErrCodeShape):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodePairingCodeInvalid,
			"A pairing code is 8 letters and digits. Check it against the one in your mez Cloud account.", "code")
	case errors.Is(err, cloud.ErrCodeRefused):
		writeFieldError(w, s.log, http.StatusConflict, CodePairingCodeRefused,
			"mez Cloud does not accept that code. It may be mistyped, used already, or older than an hour. Make a new one in your account and try again.", "code")
	case errors.Is(err, cloud.ErrBusy):
		w.Header().Set("Retry-After", "60")
		writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, "mez Cloud is busy. Try again in a minute; the code is still good.")
	case errors.Is(err, cloud.ErrUnreachable):
		writeError(w, s.log, http.StatusBadGateway, CodeCloudUnreachable,
			"mez Cloud could not be reached. Check this computer's internet and try again; the code is still good.")
	case errors.Is(err, cloud.ErrInvalid):
		// What it said is in the log. It is not the owner's to mend.
		s.log.Error("mez Cloud's answer to a pairing code could not be used", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusBadGateway, CodeCloudUnreachable,
			"mez Cloud answered with something this version of mez cannot use. Update mez, then make a new code and try again.")
	case errors.Is(err, cloudpair.ErrRelayInSettings):
		writeError(w, s.log, http.StatusConflict, CodeRelayInSettings,
			"This venue already has a relay of its own in its settings file. Remove relay_url and relay_key from it, restart mez, and pair again. The code was not used.")
	case errors.Is(err, cloudpair.ErrNotKept):
		writeError(w, s.log, http.StatusInternalServerError, CodePairingNotKept,
			"mez Cloud accepted the code, but the pairing could not be saved on this computer. Make a new code and try again.")
	case errors.Is(err, cloudpair.ErrNoVenue):
		writeError(w, s.log, http.StatusNotFound, CodeVenueNotFound, "No such venue.")
	default:
		s.internalError(w, r, "pairing to mez Cloud failed", err)
	}
}
