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
// What is not here yet: the mirror (ADR-325 on). It is in package main, and
// handing it out means moving it first, which is the task that builds the
// cloud's mirror.
//
// Nothing in this module imports cloudkit but its own test. The venue's
// binary carries none of the relay's server (internal/cloud's rule), and
// this package would bring it in.
package cloudkit

import (
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
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

// MinRelaySecretLen is the least a relay's signing secret may be.
const MinRelaySecretLen = relay.MinSecretLen
