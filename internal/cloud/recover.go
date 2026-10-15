package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/byjoellobo/mez/internal/backup"
)

// Recovery is the venue's other meeting with mez Cloud that it starts
// itself: "I had mez before" (ADR-312, ADR-332). A machine that died took
// its database and its pairing with it, so there is nothing here to be
// known by. The owner is known by their account instead: a phone number, and
// a one-time code the cloud sends to it. What that buys is the list of the
// account's venues, and for one of them a key that reads its backup and a
// pairing code to pair the restored venue with.
//
// Three requests, each of which writes nothing here. Restoring and pairing
// are the caller's.
const (
	RecoverStartPath  = "/v1/recover/start"
	RecoverVerifyPath = "/v1/recover/verify"
	RecoverGrantPath  = "/v1/recover/grant"

	// OTPLen is how long the one-time code sent to the owner's phone is.
	OTPLen = 6

	// minPhone and maxPhone are how many digits a phone number has after
	// its plus sign, by E.164.
	minPhone = 8
	maxPhone = 15

	// maxVenues is the most venues an account's answer may list, and
	// maxToken the longest an attempt or a session may be. Both are far
	// over what a cloud sends, and under what a wrong answer could.
	maxVenues    = 50
	maxToken     = 512
	maxVenueName = 100

	venueIDPrefix = "ven_"
)

// Why a recovery did not go on.
var (
	// ErrPhoneShape is a phone number that could not be one, as typed. Said
	// before anything is sent.
	ErrPhoneShape = errors.New("a phone number starts with + and its country code, like +91 98765 43210")

	// ErrPhoneRefused is the cloud not taking the number at all. It is not
	// "no such account": a number with no account is answered as one with,
	// and no code arrives.
	ErrPhoneRefused = errors.New("mez Cloud cannot send a code to that number")

	// ErrOTPShape is a one-time code that could not be one, as typed.
	ErrOTPShape = fmt.Errorf("the code sent to the phone is %d digits", OTPLen)

	// ErrOTPRefused is the cloud not accepting the one-time code: wrong,
	// used, too old, or for a number with no account. They read alike, as
	// a pairing code's refusals do.
	ErrOTPRefused = errors.New("mez Cloud does not accept that code: it may be mistyped, used already, or too old")

	// ErrSessionOver is the sign-in having run out before a venue was
	// chosen. The owner asks for another code.
	ErrSessionOver = errors.New("the sign-in to mez Cloud has run out; ask for a new code")

	// ErrNotListed is a venue asked for that the account's list did not
	// hold. It is not sent.
	ErrNotListed = errors.New("that venue is not one of this account's")

	// ErrNeverBackedUp is a venue the cloud holds no backup of. It is not
	// sent: there is nothing to hand over.
	ErrNeverBackedUp = errors.New("mez Cloud holds no backup of that venue")

	// ErrBackupGone is a venue whose backup the cloud no longer keeps: its
	// subscription ended longer ago than backups are kept for (ADR-313).
	ErrBackupGone = errors.New("mez Cloud no longer keeps that venue's backup")

	// ErrGrantRefused is the cloud not handing a venue's backup over, for
	// a reason it keeps.
	ErrGrantRefused = errors.New("mez Cloud will not hand over that venue's backup")

	// ErrAwaitingLink is a grant waiting on the link the cloud has sent to
	// the account's email (ADR-342, ADR-348). Nothing is wrong: asked
	// again once the link is opened, the venue is handed over. A *LinkSent
	// says where it went.
	ErrAwaitingLink = errors.New("mez Cloud has sent a link to the account's email, and hands the venue over once it is opened")

	// ErrRecoveryHeld is the account's recoveries held for now, which
	// asking again does not change (ADR-348). A *Held says why and until
	// when.
	ErrRecoveryHeld = errors.New("mez Cloud is holding this account's recoveries for now")
)

// The statuses a grant is answered with when it is not refused and not
// given: the cloud answers with these and the venue reads them (ADR-348).
const (
	// StatusAwaitingLink is a grant waiting on the link: GrantWaiting.
	StatusAwaitingLink = http.StatusAccepted
	// StatusHeld is the account's recoveries held: GrantHeld, beside the
	// error a refusal carries.
	StatusHeld = http.StatusLocked
)

// HoldReason says why an account's recoveries are held. A reason this
// version does not know is kept as it came and read as held.
type HoldReason string

const (
	// HoldEmailChanged is the account's email set or changed less than a
	// week ago. The hold ends at Until.
	HoldEmailChanged HoldReason = "email_changed"
	// HoldNoEmail is an account with no email for a link to go to. It
	// ends a week after one is set, and has no Until yet.
	HoldNoEmail HoldReason = "no_email"
)

// maxEmail is the longest an address may be, by the mail standards, and
// maxHoldReason far over any reason's name.
const (
	maxEmail      = 254
	maxHoldReason = 40
)

// GrantWaiting is the body of a StatusAwaitingLink: where the link went,
// as the cloud chose to show it (masked, m***@example.net), and when.
type GrantWaiting struct {
	Link EmailLink `json:"link"`
}

// EmailLink is a link sent to the account's email.
type EmailLink struct {
	Email  string    `json:"email"`
	SentAt time.Time `json:"sent_at"`
}

// GrantHeld is what a StatusHeld carries, beside the error envelope every
// refusal has.
type GrantHeld struct {
	Hold Hold `json:"hold"`
}

// Hold is why an account's recoveries are held, and until when. Until is
// absent for a hold with no end yet.
type Hold struct {
	Reason HoldReason `json:"reason"`
	Until  time.Time  `json:"until,omitzero"`
}

// LinkSent is a grant waiting on the link: errors.Is(err, ErrAwaitingLink)
// and errors.As(err, &linkSent) both find it.
type LinkSent struct {
	// Email is the address as the cloud shows it, masked.
	Email  string
	SentAt time.Time
}

func (e *LinkSent) Error() string { return ErrAwaitingLink.Error() }
func (e *LinkSent) Unwrap() error { return ErrAwaitingLink }

// Held is the account's recoveries held: errors.Is(err, ErrRecoveryHeld)
// and errors.As(err, &held) both find it.
type Held struct {
	Reason HoldReason
	// Until is when the cloud says the hold ends, to be shown. Zero is a
	// hold with no end yet.
	Until time.Time
}

func (e *Held) Error() string { return ErrRecoveryHeld.Error() }
func (e *Held) Unwrap() error { return ErrRecoveryHeld }

// CleanPhone is a phone number as it is sent: a plus sign and its digits,
// with the spaces, hyphens, dots and brackets a person writes one with taken
// away. The country code is not guessed: there is no venue yet to say which
// country this is.
func CleanPhone(typed string) (string, error) {
	typed = strings.TrimFunc(typed, unseen)
	if !strings.HasPrefix(typed, "+") {
		return "", ErrPhoneShape
	}
	var b strings.Builder
	b.WriteByte('+')
	for _, r := range typed[1:] {
		switch {
		case r == '-' || r == '.' || r == '(' || r == ')' || unseen(r):
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			return "", ErrPhoneShape
		}
	}
	// No country code begins with a zero.
	if n := b.Len() - 1; n < minPhone || n > maxPhone || b.String()[1] == '0' {
		return "", ErrPhoneShape
	}
	return b.String(), nil
}

// CleanOTP is a one-time code as it is sent: its digits, with the spaces
// and hyphens a messaging app shows one with taken away.
func CleanOTP(typed string) (string, error) {
	var b strings.Builder
	for _, r := range typed {
		switch {
		case r == '-' || unseen(r):
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			return "", ErrOTPShape
		}
	}
	if b.Len() != OTPLen {
		return "", ErrOTPShape
	}
	return b.String(), nil
}

// unseen is what a paste brings with it and a person does not see: spaces
// of every kind, a line's end, and the marks a messaging app wraps a number
// in to say which way it reads.
func unseen(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) }

// hidden is what has no place in a name or a bucket's address: a control
// character, or a mark that turns the reading round. Not every mark that is
// not seen: a name in Malayalam or Kannada is written with joiners.
func hidden(r rune) bool {
	return unicode.IsControl(r) || (r >= '‪' && r <= '‮') || (r >= '⁦' && r <= '⁩')
}

// Attempt is one asking for a one-time code, as the cloud names it. It is
// given back with the code. It opens nothing by itself, so it may be held
// by the page that asked.
type Attempt string

// AccountVenue is a venue of the owner's account, as the cloud lists it.
type AccountVenue struct {
	// ID is the venue's own public id, the one it paired with (ADR-318) and
	// the one the restored database will hold.
	ID   string `json:"id"`
	Name string `json:"name"`
	// LastBackupAt is when the cloud last saw its backup written. Zero is
	// a venue it holds no backup of.
	LastBackupAt time.Time `json:"last_backup_at,omitzero"`
}

// Recovery is an owner signed in to their account from a machine with no
// venue on it, for as long as it takes to choose one. It holds the session,
// which is a secret: it is kept in memory, and never written or logged.
type Recovery struct {
	Owner  Owner
	Venues []AccountVenue
	// ExpiresAt is when the cloud says the session ends. It is for a screen
	// to show and is not judged here: a new machine's clock is not yet one
	// to judge by.
	ExpiresAt time.Time

	session string
	client  *Client
}

// String keeps the session out of an error or a %v that prints the whole.
// On the value and not the pointer, so that a copy is covered as well.
func (r Recovery) String() string {
	return fmt.Sprintf("{recovery for %s, %d venues}", r.Owner.Name, len(r.Venues))
}

// GoString covers %#v, which goes around String.
func (r Recovery) GoString() string { return "cloud.Recovery" + r.String() }

// LogValue is what a log line says of a recovery: whose, and not the session.
func (r Recovery) LogValue() slog.Value {
	return slog.GroupValue(slog.String("owner", r.Owner.Name), slog.Int("venues", len(r.Venues)))
}

// Grant is what the cloud hands over for one venue to be brought back: a
// key that reads that venue's backup, and a pairing code to pair the
// restored venue with (Client.Pair), which gives it a new relay key and
// revokes the old. Both are secrets, and both are short-lived.
type Grant struct {
	Venue AccountVenue
	// Backup is the venue's folder in the cloud's bucket, with a key that
	// reads it and writes nothing. The key the restored venue backs up
	// with is the one its pairing gives.
	Backup backup.S3
	// PairingCode is good once, as any pairing code is, and for this venue
	// alone.
	PairingCode string
	// ExpiresAt is when the cloud says the key stops reading.
	ExpiresAt time.Time
}

// String keeps the two secrets out of an error or a %v.
func (g Grant) String() string {
	return fmt.Sprintf("{grant for venue %s}", g.Venue.ID)
}

// GoString covers %#v.
func (g Grant) GoString() string { return "cloud.Grant" + g.String() }

// LogValue is what a log line says of a grant: which venue, and no secret.
func (g Grant) LogValue() slog.Value {
	return slog.GroupValue(slog.String("venue", g.Venue.ID), slog.Time("expires_at", g.ExpiresAt))
}

// MarshalJSON is a grant as it may be written anywhere: which venue and
// until when, and neither secret. A grant is never an answer to a page. What
// a page is told of one is the caller's to choose, field by field.
func (g Grant) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Venue     AccountVenue `json:"venue"`
		ExpiresAt time.Time    `json:"expires_at"`
	}{g.Venue, g.ExpiresAt})
}

// RecoverStart asks the cloud to send a one-time code to the account's
// phone. A number with no account is answered the same as one with, so that
// this cannot be used to ask whose number has a venue; no code arrives.
func (c *Client) RecoverStart(ctx context.Context, typedPhone string) (Attempt, error) {
	phone, err := CleanPhone(typedPhone)
	if err != nil {
		return "", err
	}
	status, raw, err := c.send(ctx, RecoverStartPath, "", StartRequest{Phone: phone})
	if err != nil {
		return "", err
	}
	switch {
	case status == http.StatusOK:
	case status == http.StatusTooManyRequests:
		return "", ErrBusy
	case status >= 400 && status < 500:
		return "", ErrPhoneRefused
	default:
		return "", fmt.Errorf("%w: it answered %d", ErrUnreachable, status)
	}
	var a StartAnswer
	if err := decodeAnswer(raw, &a); err != nil {
		return "", err
	}
	if !validToken(a.Attempt) {
		return "", invalid("mez Cloud's answer names no attempt")
	}
	return Attempt(a.Attempt), nil
}

// StartRequest is what the venue sends to have a one-time code sent: the
// account's phone number, in E.164.
type StartRequest struct {
	Phone string `json:"phone"`
}

// StartAnswer names the attempt the code is to be given back against. It
// is the same for a number with an account and one without.
type StartAnswer struct {
	Attempt string `json:"attempt"`
}

// VerifyRequest gives the one-time code back.
type VerifyRequest struct {
	Attempt string `json:"attempt"`
	Code    string `json:"code"`
}

// GrantRequest asks for one of the account's venues. The session goes in
// the Authorization header, as a bearer token.
type GrantRequest struct {
	Venue GrantVenue `json:"venue"`
}

// GrantVenue names the venue asked for, by its own identifier.
type GrantVenue struct {
	ID string `json:"id"`
}

// VerifyAnswer is what the cloud sends back for a one-time code it accepts.
type VerifyAnswer struct {
	Session   string         `json:"session"`
	ExpiresAt time.Time      `json:"expires_at"`
	Owner     Owner          `json:"owner"`
	Venues    []AccountVenue `json:"venues"`
}

// RecoverVerify gives the one-time code back and is signed in to the
// account: who it is, and its venues. A code that could not be one is
// refused here without being sent, since the cloud counts wrong ones.
func (c *Client) RecoverVerify(ctx context.Context, attempt Attempt, typedOTP string) (*Recovery, error) {
	if !validToken(string(attempt)) {
		return nil, ErrSessionOver
	}
	otp, err := CleanOTP(typedOTP)
	if err != nil {
		return nil, err
	}
	status, raw, err := c.send(ctx, RecoverVerifyPath, "", VerifyRequest{Attempt: string(attempt), Code: otp})
	if err != nil {
		return nil, err
	}
	switch {
	case status == http.StatusOK:
	case status == http.StatusTooManyRequests:
		return nil, ErrBusy
	case status >= 400 && status < 500:
		// Whatever it says of the code, the answer here is one.
		return nil, ErrOTPRefused
	default:
		return nil, fmt.Errorf("%w: it answered %d", ErrUnreachable, status)
	}
	var a VerifyAnswer
	if err := decodeAnswer(raw, &a); err != nil {
		return nil, err
	}
	if err := a.validate(); err != nil {
		return nil, err
	}
	return &Recovery{Owner: a.Owner, Venues: a.Venues, ExpiresAt: a.ExpiresAt.UTC(), session: a.Session, client: c}, nil
}

func (a VerifyAnswer) validate() error {
	if !validToken(a.Session) {
		return invalid("mez Cloud's answer opens no session")
	}
	if a.ExpiresAt.IsZero() {
		return invalid("mez Cloud's answer does not say when the session ends")
	}
	if !validName(a.Owner.Name, maxOwnerName) {
		return invalid("the owner's name is empty, or not a name")
	}
	if len(a.Venues) > maxVenues {
		return invalid("mez Cloud's answer lists more venues than an account has")
	}
	seen := make(map[string]bool, len(a.Venues))
	for _, v := range a.Venues {
		if !validVenueID(v.ID) {
			return invalid("mez Cloud's answer lists a venue with an id that is not one")
		}
		if seen[v.ID] {
			return invalid("the venue %s is listed twice", v.ID)
		}
		seen[v.ID] = true
		if !validName(v.Name, maxVenueName) {
			return invalid("the venue %s has no name, or one that is not a name", v.ID)
		}
	}
	return nil
}

// GrantAnswer is what the cloud sends back for a venue it hands over.
type GrantAnswer struct {
	Backup      backup.S3 `json:"backup"`
	PairingCode string    `json:"pairing_code"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Grant asks for what brings one of the account's venues back. A venue the
// list did not hold, or one with no backup, is refused here without being
// sent.
//
// What comes back is checked whole before it is returned, and nothing is
// written or restored: that is the caller's.
//
// Two answers are neither a grant nor a refusal (ADR-348): a *LinkSent,
// while the link sent to the account's email is unopened, and a *Held,
// while the account's recoveries are held. The session stays good through
// both, and the first ends by asking again once the link is opened.
func (r *Recovery) Grant(ctx context.Context, venueID string) (Grant, error) {
	if r == nil || r.client == nil || r.session == "" {
		return Grant{}, ErrSessionOver
	}
	var venue AccountVenue
	for _, v := range r.Venues {
		if v.ID == venueID {
			venue = v
		}
	}
	if venue.ID == "" {
		return Grant{}, ErrNotListed
	}
	if venue.LastBackupAt.IsZero() {
		return Grant{}, ErrNeverBackedUp
	}

	status, raw, err := r.client.send(ctx, RecoverGrantPath, r.session, GrantRequest{Venue: GrantVenue{ID: venue.ID}})
	if err != nil {
		return Grant{}, err
	}
	switch {
	case status == http.StatusOK:
	case status == StatusAwaitingLink:
		return Grant{}, readWaiting(raw)
	case status == StatusHeld:
		return Grant{}, readHeld(raw)
	case status == http.StatusUnauthorized:
		return Grant{}, ErrSessionOver
	case status == http.StatusGone:
		return Grant{}, ErrBackupGone
	case status == http.StatusTooManyRequests:
		return Grant{}, ErrBusy
	case status >= 400 && status < 500:
		return Grant{}, ErrGrantRefused
	default:
		return Grant{}, fmt.Errorf("%w: it answered %d", ErrUnreachable, status)
	}
	var a GrantAnswer
	if err := decodeAnswer(raw, &a); err != nil {
		return Grant{}, err
	}
	// What is wrong with it is not repeated: the words would be the cloud's,
	// as long as it cared to make them.
	if err := (backup.Config{Kind: backup.KindS3, S3: a.Backup}).Validate(""); err != nil {
		return Grant{}, invalid("the backup's bucket is not one a restore could read")
	}
	for _, field := range []string{a.Backup.Bucket, a.Backup.Path, a.Backup.Region, a.Backup.Endpoint, a.Backup.AccessKeyID, a.Backup.SecretAccessKey} {
		if strings.ContainsFunc(field, hidden) {
			return Grant{}, invalid("the backup's bucket is not one a restore could read")
		}
	}
	// A bucket an owner names may be a box on their own network, in the
	// clear. One the cloud names is the cloud's, and a key is sent to it.
	if a.Backup.Endpoint != "" {
		if err := secureURL(a.Backup.Endpoint); err != nil {
			return Grant{}, invalid("the backup's bucket is not at an address a key may be sent to")
		}
	}
	// As at pairing: the key is for this venue's folder, so there must be
	// one. The default would be every venue's.
	if a.Backup.Path == "" {
		return Grant{}, invalid("the backup's bucket has no folder for this venue")
	}
	code, err := CleanCode(a.PairingCode)
	if err != nil || code != a.PairingCode {
		return Grant{}, invalid("mez Cloud's answer holds no pairing code")
	}
	if a.ExpiresAt.IsZero() {
		return Grant{}, invalid("mez Cloud's answer does not say how long the key reads for")
	}
	return Grant{Venue: venue, Backup: a.Backup, PairingCode: code, ExpiresAt: a.ExpiresAt.UTC()}, nil
}

// readWaiting is a grant waiting on the link, read from its body. An
// address that could not be shown on a screen is an answer not understood.
func readWaiting(raw []byte) error {
	var a GrantWaiting
	if err := decodeAnswer(raw, &a); err != nil {
		return err
	}
	if !validShownEmail(a.Link.Email) {
		return invalid("mez Cloud's answer does not say where the link went")
	}
	if a.Link.SentAt.IsZero() {
		return invalid("mez Cloud's answer does not say when the link was sent")
	}
	return &LinkSent{Email: a.Link.Email, SentAt: a.Link.SentAt.UTC()}
}

// readHeld is an account's recoveries held, read from the body that comes
// with the error. A reason this version does not know is still a hold: it
// is kept, and a screen says less about it. A hold for a changed email
// must say when it ends, since that is what the owner is told.
func readHeld(raw []byte) error {
	var a GrantHeld
	if err := decodeAnswer(raw, &a); err != nil {
		return err
	}
	if !validHoldReason(a.Hold.Reason) {
		return invalid("mez Cloud's answer does not say why recovery is held")
	}
	if a.Hold.Reason == HoldEmailChanged && a.Hold.Until.IsZero() {
		return invalid("mez Cloud's answer does not say when the hold ends")
	}
	return &Held{Reason: a.Hold.Reason, Until: a.Hold.Until.UTC()}
}

// validShownEmail is an address as the cloud shows one, masked or not:
// something before an @ and something after, on one line, nothing unseen.
func validShownEmail(s string) bool {
	at := strings.LastIndexByte(s, '@')
	return at > 0 && at < len(s)-1 && len(s) <= maxEmail &&
		!strings.ContainsFunc(s, unseen) && !strings.ContainsFunc(s, hidden)
}

// validHoldReason is a reason's name: small letters, digits, underscores
// and hyphens, so that one the cloud adds later is still read as a hold.
func validHoldReason(r HoldReason) bool {
	if r == "" || len(r) > maxHoldReason {
		return false
	}
	for i := 0; i < len(r); i++ {
		if c := r[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// decodeAnswer reads a 200's body, which must be JSON and no larger than
// an answer is.
func decodeAnswer(raw []byte, into any) error {
	if len(raw) > maxAnswer {
		return invalid("mez Cloud's answer is larger than an answer is")
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return invalid("mez Cloud's answer could not be read")
	}
	return nil
}

// validToken is an attempt or a session as a cloud would name one: some
// printable ASCII with no spaces, so that it goes in a header as it is.
func validToken(s string) bool {
	if s == "" || len(s) > maxToken {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// validVenueID is a venue's public id by its look: "ven_" and capitals and
// digits. This package does not import the one that makes them.
func validVenueID(s string) bool {
	code, found := strings.CutPrefix(s, venueIDPrefix)
	if !found || len(code) < 4 || len(code) > 32 {
		return false
	}
	for i := 0; i < len(code); i++ {
		if c := code[i]; (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// validName is a name a screen can show on one line: something, trimmed,
// no longer than most, with nothing in it that is not seen.
func validName(name string, most int) bool {
	return strings.TrimSpace(name) != "" && name == strings.TrimSpace(name) && len(name) <= most && !strings.ContainsFunc(name, hidden)
}
