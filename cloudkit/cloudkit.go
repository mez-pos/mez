// Package cloudkit is the door mez Cloud comes through (ADR-341, ADR-344).
//
// mez Cloud is a service in a repository of its own. It answers what a venue
// asks when it pairs or asks for itself back, signs what a venue checks when
// its owner comes in from away, and runs the relay. Each of those has one
// definition, and it is this module's, under internal/. Go lets no other
// module import internal/, so this package hands those definitions out, by
// their own names, and nothing else: there is no code here but one function,
// and nothing that is not used on the cloud's side of a conversation.
//
// It is an API, and the only one this module has. A change here is a change
// to another repository, to be made in the same sitting or not made.
//
// A backup's restore is here (ADR-352): an owner's download of their
// venue's data is one. So is the mirror (ADR-353): the back office over a
// copy of a venue's backup, which the cloud serves when the venue is off.
//
// Nothing in this module imports cloudkit but its own test. The venue's
// binary carries none of the relay's server (internal/cloud's rule), and
// this package would bring it in.
package cloudkit

import (
	"context"
	"io"

	"crypto/ed25519"
	"log/slog"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/mirror"
	"github.com/byjoellobo/mez/internal/relay"
)

// Pairing: POST PairPath, a PairRequest, answered with a PairAnswer
// (ADR-318). A code not known, spent or too old is any 4xx but 429; 429 is
// "ask again shortly".
const (
	PairPath = cloud.PairPath
	// CodeLen is how long a pairing code is, in CleanCode's alphabet.
	CodeLen = cloud.CodeLen
)

type (
	// PairRequest is what the venue sends: the code, and who is asking.
	PairRequest = cloud.PairRequest
	// PairingVenue is what a venue says of itself as it pairs. Its ID is
	// how the cloud knows a venue it has seen before (ADR-313).
	PairingVenue = cloud.PairingVenue
	// PairAnswer is everything a venue is given for a code. The venue
	// checks it whole and refuses one it cannot use.
	PairAnswer = cloud.PairAnswer
	// Venue is the venue's name on the relay and where it is reached.
	Venue = cloud.Venue
	// Relay is the relay's address and the venue's key to it.
	Relay = cloud.Relay
	// CloudKey is the cloud's public key as a pairing carries it.
	CloudKey = cloud.CloudKey
	// Owner is the account, by the name it gave.
	Owner = cloud.Owner
	// Bucket is where a backup goes: an S3 bucket's name, folder, address
	// and key. A pairing carries one that writes; a hand-over, one that
	// only reads and stops within the hour (ADR-332).
	Bucket = backup.S3
)

// CleanCode is a pairing code as it is compared: what a person typed, with
// its spaces and dashes gone and its letters raised. The cloud cleans the
// code it is sent the same way the venue cleaned it before sending.
func CleanCode(typed string) (string, error) { return cloud.CleanCode(typed) }

// CheckPairAnswer is the venue's own check of a pairing's answer, whole
// (ADR-347). An answer it refuses is one the venue would refuse after the
// code was spent: ask it before committing the pairing.
func CheckPairAnswer(a PairAnswer) error { return cloud.CheckPairAnswer(a) }

// ValidVenuePublicID says whether a venue's id, as a pairing sends it, could
// be one: "ven_" and capitals and digits (ADR-347). It is not ValidVenueID,
// which is a slug on the relay.
func ValidVenuePublicID(s string) bool { return cloud.ValidVenuePublicID(s) }

// NewCode draws a pairing code from the alphabet CleanCode takes, with no
// bias (ADR-346).
func NewCode(entropy io.Reader) (string, error) { return cloud.NewCode(entropy) }

// Recovery: three requests (ADR-332). A number with no account is answered
// as one with. The session a VerifyAnswer carries is the bearer token of the
// grant.
const (
	RecoverStartPath  = cloud.RecoverStartPath
	RecoverVerifyPath = cloud.RecoverVerifyPath
	RecoverGrantPath  = cloud.RecoverGrantPath
	// OTPLen is how long the one-time code sent to the owner's phone is.
	OTPLen = cloud.OTPLen
)

type (
	StartRequest  = cloud.StartRequest
	StartAnswer   = cloud.StartAnswer
	VerifyRequest = cloud.VerifyRequest
	VerifyAnswer  = cloud.VerifyAnswer
	GrantRequest  = cloud.GrantRequest
	GrantVenue    = cloud.GrantVenue
	GrantAnswer   = cloud.GrantAnswer
	// AccountVenue is one of an account's venues, with when its backup was
	// last written. Zero is one the cloud holds no backup of.
	AccountVenue = cloud.AccountVenue
)

// CheckGrantAnswer is the venue's own check of a hand-over's answer, whole
// (ADR-349). Ask it before committing the hand-over: an answer it refuses is
// one the venue would refuse after the owner's link was spent.
func CheckGrantAnswer(a GrantAnswer) error { return cloud.CheckGrantAnswer(a) }

// A grant that is neither given nor refused (ADR-348): StatusAwaitingLink
// with a GrantWaiting while the link sent to the account's email is
// unopened, and StatusHeld with a GrantHeld, beside the error envelope,
// while the account's recoveries are held. The cloud decides them in this
// order: 410 for a backup no longer kept, then the hold, then the link.
const (
	StatusAwaitingLink = cloud.StatusAwaitingLink
	StatusHeld         = cloud.StatusHeld
	// HoldEmailChanged is the email set or changed under a week ago, and
	// its Until is required; HoldNoEmail is an account with none, and has
	// no Until.
	HoldEmailChanged = cloud.HoldEmailChanged
	HoldNoEmail      = cloud.HoldNoEmail
)

type (
	GrantWaiting = cloud.GrantWaiting
	EmailLink    = cloud.EmailLink
	GrantHeld    = cloud.GrantHeld
	Hold         = cloud.Hold
	HoldReason   = cloud.HoldReason
)

// The assertion: what the cloud signs for an owner it has signed in, and a
// venue accepts as a sign-in by the one person made at pairing (ADR-323,
// ADR-324). It is presented as "Authorization: <AssertionScheme> <signed>".
const (
	AssertionScheme = cloud.AssertionScheme
	AssertionLife   = cloud.AssertionLife
	SubjectOwner    = cloud.SubjectOwner
)

// Assertion is what is vouched for: a venue, a person, five minutes and a
// nonce the venue takes once.
type Assertion = cloud.Assertion

var (
	// NewAssertion makes one for a venue, by its public id, good from now.
	NewAssertion = cloud.NewAssertion
	// SignAssertion signs it with the cloud's private key.
	SignAssertion = cloud.SignAssertion
	// VerifyAssertion is the venue's check, for the cloud's own tests. It
	// returns the key that signed: the one held, or the newest the links
	// led to from it, which the venue then keeps.
	VerifyAssertion = cloud.VerifyAssertion
)

// Rotation (ADR-345). A venue learns the cloud's next key from links: each
// new public key signed once by the one before it. An assertion is
// signed with the newest key and presented with the links from the oldest
// key a venue may still hold, oldest first, at most MaxKeyChain of them.
const MaxKeyChain = cloud.MaxKeyChain

var (
	// SignKeyLink is the link that hands on the next key, signed with the
	// key before it. It is made once, when the key is rotated, and kept.
	SignKeyLink = cloud.SignKeyLink
	// PresentAssertion is a signed assertion with its links, as it goes in
	// "Authorization: <AssertionScheme> <presented>".
	PresentAssertion = cloud.PresentAssertion
)

// The relay, to run in the cloud's own process with entitlements read from
// its database (ADR-312, ADR-316).
type (
	// RelayServer is a relay. Its Handler is mounted; Close ends it.
	RelayServer = relay.Server
	// RelayOptions are what one is made with.
	RelayOptions = relay.Options
	// RelayLimit is an allowance of requests.
	RelayLimit = relay.Limit
	// Keys mints and checks venue keys under one secret.
	Keys = relay.Keys
	// Key is a venue key, checked.
	Key = relay.Key
	// Entitlements is asked, of a venue, until when it is carried.
	Entitlements = relay.Entitlements
	// Entitlement is the answer: the date the relay enforces, whether the
	// back office crosses, and what is passed on to the venue untouched.
	Entitlement = relay.Entitlement
)

var (
	// NewRelay makes a relay.
	NewRelay = relay.New
	// NewKeys makes the keys of one secret, less the ones revoked.
	NewKeys = relay.NewKeys
	// ValidVenueID says whether a slug can name a venue on a relay, and
	// ReservedVenueID whether it is one no venue may take.
	ValidVenueID    = relay.ValidVenueID
	ReservedVenueID = relay.Reserved
	// NoEnd is the Until of a venue carried with no date set.
	NoEnd = relay.NoEnd
)

// The reasons a relay gives of itself (ADR-316), which a venue knows the
// words for: a date that has passed with no reason given, and a venue
// nobody has turned on.
const (
	ReasonExpired   = relay.ReasonExpired
	ReasonNotListed = relay.ReasonNotListed
)

// MinRelaySecretLen is the least a relay's signing secret may be.
const MinRelaySecretLen = relay.MinSecretLen

// A venue's backup, read back by the cloud for its owner (ADR-352): the
// newest whole copy in its bucket restored to a new database file and
// checked whole, as mez's own restore does it, and its menu's photos beside
// it. The bucket is read and nothing in it is changed.
var (
	// ErrNothingToRestore is a bucket with no whole copy in it.
	ErrNothingToRestore = backup.ErrNothingToRestore
	// ErrWouldOverwrite is a restore asked to write where a file already
	// is. A restore never writes over one.
	ErrWouldOverwrite = backup.ErrWouldOverwrite
)

// RestoreBackup writes the newest state of the backup in a bucket to a new
// database file at outputPath, and checks it whole before returning.
func RestoreBackup(ctx context.Context, b Bucket, outputPath string) error {
	return backup.Restore(ctx, backup.Config{Kind: backup.KindS3, S3: b}, outputPath)
}

// RestorePhotos copies the menu's photos backed up in a bucket into dir,
// and says how many it copied.
func RestorePhotos(ctx context.Context, b Bucket, dir string) (int, error) {
	return backup.RestorePhotos(ctx, backup.Config{Kind: backup.KindS3, S3: b}, dir)
}

// The mirror (ADR-326, ADR-327, ADR-353): the back office over a copy of a
// venue's backup, to read, as `mez mirror` serves it. A Mirror is an
// http.Handler that answers as the venue's back office would, refuses every
// change before any handler, and says on every answer, in HeaderMirror,
// how old its copy is. Let people in by pairing a browser with a code from
// its Keeper and a PIN from the copy, or, with CloudKey, by an assertion
// presented as at the venue (ADR-331).
type (
	// Mirror is one venue's mirror. Follow keeps it up with its bucket
	// while it is asked things; Close removes every copy it made.
	Mirror = mirror.Mirror
	// MirrorVenue is the venue in a mirror's copy.
	MirrorVenue = mirror.Venue
	// BuildInfo is what a mirror says of itself at /api/v1/health.
	BuildInfo = api.BuildInfo
)

const (
	// HeaderMirror is on every answer a mirror gives: the moment of the
	// last write its copy holds, RFC 3339 in UTC.
	HeaderMirror = api.HeaderMirror
	// MirrorDirPrefix begins the name of every directory a mirror makes in
	// its CopyDir. One there that no open mirror holds was left by a
	// process that was killed, and is a copy of a venue's sales.
	MirrorDirPrefix = mirror.DirPrefix
)

// MirrorOptions are what a mirror is opened with. Bucket and StateDir are
// required.
type MirrorOptions struct {
	// Bucket is where the venue's backup is. It is only read: the reading
	// key is enough.
	Bucket Bucket
	// StateDir is where the mirror remembers the browsers paired to it,
	// the assertions it has taken and the locks wrong PINs set, in a file
	// named by the venue's public id. Made if it is not there.
	StateDir string
	// CopyDir is where each copy is restored. Empty is the system's
	// temporary directory.
	CopyDir string
	// CloudKey is the cloud's public key, for the owner let in by an
	// assertion; nil lets nobody in that way.
	CloudKey ed25519.PublicKey
	Build    BuildInfo
	// Log is told what goes wrong while the mirror serves and follows.
	Log *slog.Logger
	// Patience is how long a copy being put away waits for a request still
	// being answered from it. Zero is ten seconds.
	Patience time.Duration
}

// OpenMirror restores the newest copy in the bucket, checks it, brings it
// up to this build's schema and opens it as a mirror. It holds exactly one
// venue or it is refused, and nothing is left behind when it fails.
func OpenMirror(ctx context.Context, o MirrorOptions) (*Mirror, error) {
	return mirror.Open(ctx, mirror.OpenOptions{
		From:     backup.Config{Kind: backup.KindS3, S3: o.Bucket},
		StateDir: o.StateDir, CopyDir: o.CopyDir, CloudKey: o.CloudKey,
		Build: o.Build, Log: o.Log, Patience: o.Patience,
	})
}
