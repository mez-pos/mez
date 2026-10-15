package api

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/restore"
	"github.com/byjoellobo/mez/internal/service/cloudrecover"
)

// "I had mez before", over HTTP (ADR-332 to ADR-335): the wizard's way to
// bring a venue back from its owner's mez Cloud account, on a computer with
// no venue. Three routes before the server starts again on the venue, behind
// what the wizard is behind; one after, behind the claim the third gave.

// Recovery is what the API needs to bring a venue back.
// *cloudrecover.Service is the implementation.
type Recovery interface {
	Start(ctx context.Context, phone string) (cloud.Attempt, error)
	Verify(ctx context.Context, attempt cloud.Attempt, otp string) (cloudrecover.Account, error)
	Restore(ctx context.Context, venueID string) (cloudrecover.Restored, error)
}

// Arrived is what the API needs once a venue has been brought back.
// *cloudrecover.Arrival is the implementation; on a start that adopted
// nothing it answers every claim with cloudrecover.ErrNoClaim.
type Arrived interface {
	Outcome(claim string) (cloudrecover.Outcome, error)
	DeviceCode(ctx context.Context, claim, deviceName string, kind auth.DeviceKind) (auth.PairingCode, error)
}

// WithRecovery mounts "I had mez before" beside first run. arrived may be
// nil: a server that never adopts a venue has nothing to tell.
func WithRecovery(r Recovery, arrived Arrived) Option {
	return func(s *Server) { s.recovery, s.arrived = r, arrived }
}

// HeaderRecoverClaim carries the claim a restore gave the page that asked
// for it. A header for the reasons the setup token is one: never part of
// what a form could post, never in an address or the access log.
const HeaderRecoverClaim = "X-Recover-Claim"

// Why a venue was not brought back.
const (
	// CodeRecoverPhoneInvalid is a phone number that could not be one, as
	// typed. Nothing was sent.
	CodeRecoverPhoneInvalid ErrorCode = "recover_phone_invalid"
	// CodeRecoverPhoneRefused is a number mez Cloud cannot send a code to.
	// It is not "no such account": that is never said.
	CodeRecoverPhoneRefused ErrorCode = "recover_phone_refused"
	// CodeRecoverCodeInvalid is a one-time code that could not be one.
	CodeRecoverCodeInvalid ErrorCode = "recover_code_invalid"
	// CodeRecoverCodeRefused is a one-time code mez Cloud does not accept.
	CodeRecoverCodeRefused ErrorCode = "recover_code_refused"
	// CodeRecoverSignedOut is nobody signed in, or a sign-in that ran out:
	// the owner asks for a new code.
	CodeRecoverSignedOut ErrorCode = "recover_signed_out"
	// CodeRecoverVenueNotFound is a venue that is not one of the account's.
	CodeRecoverVenueNotFound ErrorCode = "recover_venue_not_found"
	// CodeRecoverNoBackup is a venue mez Cloud holds no backup of.
	CodeRecoverNoBackup ErrorCode = "recover_no_backup"
	// CodeRecoverBackupGone is a venue whose backup is no longer kept.
	CodeRecoverBackupGone ErrorCode = "recover_backup_gone"
	// CodeRecoverRefused is mez Cloud not handing the venue over.
	CodeRecoverRefused ErrorCode = "recover_refused"
	// CodeRecoverAwaitingLink is mez Cloud waiting on the link it sent to
	// the account's email (ADR-348). Asked again once it is opened, the
	// venue is brought back.
	CodeRecoverAwaitingLink ErrorCode = "recover_awaiting_link"
	// CodeRecoverHeld is the account's recoveries held for now: for a week
	// after its email changed, or until it has one (ADR-348).
	CodeRecoverHeld ErrorCode = "recover_held"
	// CodeRecoverUnderWay is a second restore beside one that is running,
	// or after one that is about to take hold.
	CodeRecoverUnderWay ErrorCode = "recover_under_way"
	// CodeRecoverBackupFailed is a backup that could not be brought down
	// or was not what it should be. Nothing on this computer was changed.
	CodeRecoverBackupFailed ErrorCode = "recover_backup_failed"
	// CodeRecoverNothing is an outcome asked for with no claim, a wrong
	// one, or one that has run out.
	CodeRecoverNothing ErrorCode = "recover_nothing"
)

var (
	// recoverLimit is what one address may spend on signing in to bring a
	// venue back: a number, a code, a mistyped code, a venue. Each try that
	// reaches mez Cloud is a message sent to somebody's phone, or a guess
	// at the code in it.
	recoverLimit = limit{requests: 8, per: time.Minute}

	// outcomeLimit is for the page that waits on the start and asks how it
	// went every second or two. A claim is too long to guess; this is a
	// ceiling on asking, not what keeps a claim.
	outcomeLimit = limit{requests: 90, per: time.Minute}
)

// recoverStartRequest is the account's phone number, as typed, with its
// country code.
type recoverStartRequest struct {
	Phone string `json:"phone"`
}

// recoverStartResponse names the asking. It is given back with the code
// sent to the phone. Every number is answered alike: this does not say the
// number has an account.
type recoverStartResponse struct {
	Attempt string `json:"attempt"`
}

// recoverVerifyRequest is the code sent to the phone, as typed.
type recoverVerifyRequest struct {
	Attempt string `json:"attempt"`
	Code    string `json:"code"`
}

// recoverVenue is one of the account's venues. A venue with no
// last_backup_at has no backup and cannot be brought back.
type recoverVenue struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	LastBackupAt *time.Time `json:"last_backup_at,omitempty"`
}

// recoverVerifyResponse is the owner signed in: who, until when, and the
// venues to choose from.
type recoverVerifyResponse struct {
	Owner     string         `json:"owner"`
	ExpiresAt time.Time      `json:"expires_at"`
	Venues    []recoverVenue `json:"venues"`
}

// recoverRestoreRequest is the venue chosen, by its id from the list.
type recoverRestoreRequest struct {
	VenueID string `json:"venue_id"`
}

// recoverRestoreResponse is a venue brought down, with mez about to start
// again on it. The page waits, then asks for the outcome with claim, which
// is said here once and is a secret.
type recoverRestoreResponse struct {
	VenueID          string `json:"venue_id"`
	VenueName        string `json:"venue_name"`
	Photos           int    `json:"photos"`
	PhotosIncomplete bool   `json:"photos_incomplete"`
	Claim            string `json:"claim"`
}

// recoverOutcomeRequest may name the device this browser is to be: with a
// device_name, the answer carries a code that pairs it.
type recoverOutcomeRequest struct {
	DeviceName string          `json:"device_name,omitempty"`
	DeviceKind auth.DeviceKind `json:"device_kind,omitempty"`
}

// recoverOutcomeResponse is how the venue's return went. cloud is
// "pairing" until it is "paired" or "not_paired"; hostname is where the
// venue is reached from away once paired. pairing_code pairs this browser
// (POST /api/v1/devices/pair) and is the same code however often it is
// asked for: the name and kind it was first asked with are the device's,
// and one given later is not taken up.
type recoverOutcomeResponse struct {
	VenueID     string                  `json:"venue_id"`
	VenueName   string                  `json:"venue_name"`
	Cloud       cloudrecover.CloudState `json:"cloud"`
	Hostname    string                  `json:"hostname,omitempty"`
	PairingCode string                  `json:"pairing_code,omitempty"`
	ExpiresAt   *time.Time              `json:"expires_at,omitempty"`
}

// mountRecovery adds the four routes. None is reachable through the relay:
// they are not in remoteRoutes, and each asks atThisMachine besides.
func (s *Server) mountRecovery(mux *http.ServeMux) {
	if s.recovery == nil {
		return
	}
	// The limit is inside the door, not in front of it as first run's is.
	// Every caller here comes from this machine's one address, so a limit
	// in front would be one any program on it could spend for the owner. A
	// caller without the token is refused at no cost to anyone; the token
	// is too long to be found by trying.
	mux.Handle("POST /api/v1/setup/recover/start", s.atTheWizard(s.rateLimit(recoverLimit, http.HandlerFunc(s.recoverStart))))
	mux.HandleFunc("/api/v1/setup/recover/start", s.methodNotAllowed)
	mux.Handle("POST /api/v1/setup/recover/verify", s.atTheWizard(s.rateLimit(recoverLimit, http.HandlerFunc(s.recoverVerify))))
	mux.HandleFunc("/api/v1/setup/recover/verify", s.methodNotAllowed)
	mux.Handle("POST /api/v1/setup/recover/restore", s.atTheWizard(s.rateLimit(recoverLimit, http.HandlerFunc(s.recoverRestore))))
	mux.HandleFunc("/api/v1/setup/recover/restore", s.methodNotAllowed)
	// The outcome's limit is inside its door too, and its door is the
	// claim, which the handler reads.
	mux.HandleFunc("POST /api/v1/setup/recover/outcome", s.recoverOutcome)
	mux.HandleFunc("/api/v1/setup/recover/outcome", s.methodNotAllowed)
}

// hereWithJSON is the part of the wizard's door that every recovery route
// has: a browser on this machine, posting JSON. It answers the refusal
// itself and reports whether to go on.
func (s *Server) hereWithJSON(w http.ResponseWriter, r *http.Request) bool {
	if !s.atThisMachine(r) {
		writeError(w, s.log, http.StatusForbidden, CodeSetupNotHere,
			"A venue is brought back at the computer mez runs on. Open mez there.")
		return false
	}
	// A form on another site can post text/plain here without the browser
	// asking first. It cannot post JSON.
	if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
		writeError(w, s.log, http.StatusUnsupportedMediaType, CodeInvalidRequest, "the body must be application/json")
		return false
	}
	return true
}

// atTheWizard is what runSetup asks before it makes a venue, asked before a
// venue is brought back: this machine, JSON, and the setup token (ADR-270).
func (s *Server) atTheWizard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hereWithJSON(w, r) {
			return
		}
		// On the machine is not enough: so is every other user of it, and
		// every program.
		if !s.holdsSetupToken(r) {
			writeError(w, s.log, http.StatusForbidden, CodeSetupLocked,
				"Open mez from the installer, or from the setup link it prints when it starts, to bring this venue back.")
			return
		}
		// Nothing these say is to be kept by a browser or anything between.
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverStart(w http.ResponseWriter, r *http.Request) {
	var req recoverStartRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	attempt, err := s.recovery.Start(r.Context(), req.Phone)
	if err != nil {
		s.recoverError(w, r, "asking mez Cloud for a one-time code failed", err, false)
		return
	}
	writeJSON(w, s.log, http.StatusOK, recoverStartResponse{Attempt: string(attempt)})
}

func (s *Server) recoverVerify(w http.ResponseWriter, r *http.Request) {
	var req recoverVerifyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	account, err := s.recovery.Verify(r.Context(), cloud.Attempt(req.Attempt), req.Code)
	if err != nil {
		s.recoverError(w, r, "signing in to mez Cloud failed", err, false)
		return
	}
	venues := make([]recoverVenue, 0, len(account.Venues))
	for _, v := range account.Venues {
		venue := recoverVenue{ID: v.ID, Name: v.Name}
		if !v.LastBackupAt.IsZero() {
			at := v.LastBackupAt.UTC()
			venue.LastBackupAt = &at
		}
		venues = append(venues, venue)
	}
	writeJSON(w, s.log, http.StatusOK, recoverVerifyResponse{Owner: account.Owner, ExpiresAt: account.ExpiresAt.UTC(), Venues: venues})
}

func (s *Server) recoverRestore(w http.ResponseWriter, r *http.Request) {
	var req recoverRestoreRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	restored, err := s.recovery.Restore(r.Context(), req.VenueID)
	if err != nil {
		s.recoverError(w, r, "bringing a venue back failed", err, true)
		return
	}
	s.log.Info("a venue was brought back from its backup", "request_id", RequestID(r.Context()), "venue", restored.VenueID)
	// Accepted, and not yet so: the venue is in place when mez has started
	// again, which it is doing as this is read.
	writeJSON(w, s.log, http.StatusAccepted, recoverRestoreResponse{
		VenueID: restored.VenueID, VenueName: restored.VenueName,
		Photos: restored.Photos, PhotosIncomplete: restored.PhotosIncomplete, Claim: restored.Claim,
	})
}

// recoverOutcome answers the page that brought a venue back, after the
// start. It is not behind the setup token, which that start ended: the
// claim is what the page holds in its place (ADR-334).
func (s *Server) recoverOutcome(w http.ResponseWriter, r *http.Request) {
	if !s.hereWithJSON(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var req recoverOutcomeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	claim := r.Header.Get(HeaderRecoverClaim)
	nothing := func() {
		// One answer for no claim, a wrong one and one that ran out, on a
		// server that brought a venue back and on one that did not.
		writeError(w, s.log, http.StatusNotFound, CodeRecoverNothing,
			"mez can no longer tell this page how bringing the venue back went. Open mez again on this computer to see whether your venue is there.")
	}
	if s.arrived == nil {
		nothing()
		return
	}
	outcome, err := s.arrived.Outcome(claim)
	if err != nil {
		nothing()
		return
	}
	// Whoever holds the claim is held to a page's pace. Nobody else is
	// counted: they would be spending what the page has.
	if ok, retryAfter := s.limiter.allow(outcomeLimit.key(remoteHost(r)), outcomeLimit); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
		writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, "too many requests; wait and try again")
		return
	}
	out := recoverOutcomeResponse{VenueID: outcome.VenueID, VenueName: outcome.VenueName, Cloud: outcome.Cloud, Hostname: outcome.Hostname}
	if req.DeviceName != "" {
		code, err := s.arrived.DeviceCode(r.Context(), claim, req.DeviceName, req.DeviceKind)
		var invalid *auth.InvalidDevice
		switch {
		case errors.As(err, &invalid):
			writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, invalid.Message, "device_"+invalid.Field)
			return
		case errors.Is(err, cloudrecover.ErrNoClaim):
			nothing()
			return
		case err != nil:
			s.internalError(w, r, "making a pairing code for the browser that brought the venue back failed", err)
			return
		}
		expires := code.ExpiresAt.UTC()
		out.PairingCode, out.ExpiresAt = code.Code, &expires
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

// awaitingLinkMessage says where the link went, as mez Cloud showed the
// address, which is masked.
func awaitingLinkMessage(err error) string {
	to := "your mez Cloud account's email"
	if link := (*cloud.LinkSent)(nil); errors.As(err, &link) {
		to = link.Email
	}
	return "mez Cloud has sent a link to " + to + ". Open it, on your phone or anywhere, then come back here and choose your venue again."
}

// heldMessage says why recovery is held and until when. The date is in
// this computer's own time zone, which a new machine usually has right
// even when its clock is not yet trusted (ADR-332).
func heldMessage(err error) string {
	held := (*cloud.Held)(nil)
	if !errors.As(err, &held) {
		held = &cloud.Held{}
	}
	switch {
	case held.Reason == cloud.HoldEmailChanged && !held.Until.IsZero():
		return "mez Cloud is holding this account's venues until " + held.Until.Local().Format("2 January 2006 at 15:04") +
			", because its email address was set or changed in the last week. If you did not change it, write to mez Cloud's support from your old address."
	case held.Reason == cloud.HoldNoEmail:
		return "Your mez Cloud account has no email address, and a venue is handed over only after a link sent to one is opened. Add one in your mez Cloud account: a venue can be brought back a week after."
	default:
		return "mez Cloud is holding this account's venues for now. Look at your mez Cloud account, or write to support."
	}
}

// recoverError says why a venue was not brought back, in words an owner at
// a new computer can act on. What the cloud, the backup or `mez restore`
// said underneath stays in the log: their words are a command's.
// restoring says the venue's backup was being brought down, which is what
// an error nobody named is then taken to be about.
func (s *Server) recoverError(w http.ResponseWriter, r *http.Request, what string, err error, restoring bool) {
	underneath := func() { s.log.Warn(what, "request_id", RequestID(r.Context()), "error", err) }
	switch {
	case errors.Is(err, cloud.ErrPhoneShape):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeRecoverPhoneInvalid,
			"Type the phone number of your mez Cloud account with its country code, like +91 98765 43210.", "phone")
	case errors.Is(err, cloud.ErrPhoneRefused):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeRecoverPhoneRefused,
			"mez Cloud cannot send a code to that number. Check it, with its country code.", "phone")
	case errors.Is(err, cloud.ErrOTPShape):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeRecoverCodeInvalid,
			"The code sent to your phone is 6 digits.", "code")
	case errors.Is(err, cloud.ErrOTPRefused):
		writeFieldError(w, s.log, http.StatusConflict, CodeRecoverCodeRefused,
			"mez Cloud does not accept that code. It may be mistyped, used already, or too old. If no code arrived, check the number and ask for another.", "code")
	case errors.Is(err, cloud.ErrSessionOver):
		writeError(w, s.log, http.StatusConflict, CodeRecoverSignedOut,
			"Your sign-in to mez Cloud has run out. Ask for a new code and sign in again.")
	case errors.Is(err, cloud.ErrNotListed):
		writeError(w, s.log, http.StatusNotFound, CodeRecoverVenueNotFound, "That venue is not one of this account's.")
	case errors.Is(err, cloud.ErrNeverBackedUp), errors.Is(err, backup.ErrNothingToRestore):
		writeError(w, s.log, http.StatusConflict, CodeRecoverNoBackup,
			"mez Cloud holds no backup of that venue, so there is nothing to bring back. Set it up as a new venue.")
	case errors.Is(err, cloud.ErrBackupGone):
		writeError(w, s.log, http.StatusGone, CodeRecoverBackupGone,
			"mez Cloud no longer keeps that venue's backup: its subscription ended too long ago. If you kept a backup of your own, it can be restored from that.")
	case errors.Is(err, cloud.ErrAwaitingLink):
		writeError(w, s.log, http.StatusConflict, CodeRecoverAwaitingLink, awaitingLinkMessage(err))
	case errors.Is(err, cloud.ErrRecoveryHeld):
		writeError(w, s.log, http.StatusConflict, CodeRecoverHeld, heldMessage(err))
	case errors.Is(err, cloud.ErrGrantRefused):
		writeError(w, s.log, http.StatusConflict, CodeRecoverRefused,
			"mez Cloud will not hand that venue over. Look at the venue in your mez Cloud account, or write to support.")
	case errors.Is(err, cloud.ErrBusy):
		w.Header().Set("Retry-After", "60")
		writeError(w, s.log, http.StatusTooManyRequests, CodeRateLimited, "mez Cloud is busy. Try again in a minute.")
	case errors.Is(err, cloud.ErrUnreachable):
		underneath()
		writeError(w, s.log, http.StatusBadGateway, CodeCloudUnreachable,
			"mez Cloud could not be reached. Check this computer's internet and try again.")
	case errors.Is(err, cloud.ErrInvalid):
		underneath()
		writeError(w, s.log, http.StatusBadGateway, CodeCloudUnreachable,
			"mez Cloud answered with something this version of mez cannot use. Update mez and try again.")
	case errors.Is(err, cloudrecover.ErrAlreadySetUp), errors.Is(err, restore.ErrVenueThere):
		writeError(w, s.log, http.StatusConflict, CodeAlreadySetUp,
			"This computer already has a venue, and one is never brought back over another.")
	case errors.Is(err, cloudrecover.ErrUnderWay):
		writeError(w, s.log, http.StatusConflict, CodeRecoverUnderWay,
			"A venue is already being brought back on this computer. Wait for it to finish.")
	case !restoring:
		// Nothing was being brought down: this is a fault of this
		// computer's own, and is said as any other is.
		s.internalError(w, r, what, err)
	default:
		// The backup would not come down, or was not this venue's. Nothing
		// here was changed, and trying again asks the cloud afresh.
		underneath()
		writeError(w, s.log, http.StatusBadGateway, CodeRecoverBackupFailed,
			"The venue's backup could not be brought down to this computer. Nothing here was changed. Check the internet and try again.")
	}
}
