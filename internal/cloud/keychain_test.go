package cloud

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"
)

// rotation is a cloud that has rotated its key: keys[0] is the one a venue
// was paired with, and links[i] hands keys[i] on to keys[i+1].
type rotation struct {
	pubs  []ed25519.PublicKey
	privs []ed25519.PrivateKey
	links []string
}

func rotated(t *testing.T, keys int) rotation {
	t.Helper()
	var r rotation
	for i := 0; i < keys; i++ {
		pub, priv := signingKey(t)
		r.pubs, r.privs = append(r.pubs, pub), append(r.privs, priv)
		if i == 0 {
			continue
		}
		link, err := SignKeyLink(r.privs[i-1], pub)
		if err != nil {
			t.Fatalf("SignKeyLink: %v", err)
		}
		r.links = append(r.links, link)
	}
	return r
}

// signedBy is a fresh assertion for the test venue, signed with priv.
func signedBy(t *testing.T, priv ed25519.PrivateKey) string {
	t.Helper()
	signed, err := SignAssertion(priv, assertion(t))
	if err != nil {
		t.Fatalf("SignAssertion: %v", err)
	}
	return signed
}

func TestAVenueFollowsTheChainFromTheKeyItHolds(t *testing.T) {
	r := rotated(t, 4)
	newest := r.privs[3]
	now := assertedAt.Add(time.Second)

	tests := []struct {
		name  string
		held  int
		links []string
		by    ed25519.PrivateKey
		want  int
	}{
		{"no rotation, no links", 0, nil, r.privs[0], 0},
		{"one rotation behind", 2, r.links[2:], newest, 3},
		{"the whole chain, from the first key", 0, r.links, newest, 3},
		{"the whole chain, holding a key in the middle", 1, r.links, newest, 3},
		{"the whole chain, holding the newest already", 3, r.links, newest, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			presented := PresentAssertion(signedBy(t, tt.by), tt.links...)
			a, signer, err := VerifyAssertion(r.pubs[tt.held], presented, assertedVenue, now)
			if err != nil {
				t.Fatalf("VerifyAssertion: %v", err)
			}
			if !signer.Equal(r.pubs[tt.want]) {
				t.Errorf("signed by key %d, want key %d", indexOf(r.pubs, signer), tt.want)
			}
			if a.Audience != assertedVenue {
				t.Errorf("audience %q", a.Audience)
			}
		})
	}
}

func TestAChainIsRefusedWhenItDoesNotLeadToTheSigner(t *testing.T) {
	r := rotated(t, 3)
	stranger, strangerPriv := signingKey(t)
	now := assertedAt.Add(time.Second)

	// A link the stranger signed, handing on to a key of the stranger's.
	_, thief := signingKey(t)
	foreign, err := SignKeyLink(strangerPriv, thief.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		held      ed25519.PublicKey
		presented string
	}{
		// The cloud signs with its newest key, so the venue checks with the
		// newest the chain reaches and no other.
		{"signed by the held key, presented with links past it", r.pubs[0], PresentAssertion(signedBy(t, r.privs[0]), r.links...)},
		{"signed by a newer key, with no links", r.pubs[0], signedBy(t, r.privs[2])},
		{"signed by a newer key, a link missing", r.pubs[0], PresentAssertion(signedBy(t, r.privs[2]), r.links[1])},
		{"a link signed by a key the venue never held", r.pubs[0], PresentAssertion(signedBy(t, thief), foreign)},
		{"held by a venue the chain never reaches", stranger, PresentAssertion(signedBy(t, r.privs[2]), r.links...)},
		{"the links before the assertion", r.pubs[0], strings.Join(r.links, "~") + "~" + signedBy(t, r.privs[2])},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := VerifyAssertion(tt.held, tt.presented, assertedVenue, now); !errors.Is(err, ErrAssertion) {
				t.Errorf("err = %v, want ErrAssertion", err)
			}
		})
	}
}

// A presentation has one spelling: a link that is not a link refuses it
// whole, even beside a chain that would have led to the signer.
func TestAMalformedLinkRefusesTheWhole(t *testing.T) {
	r := rotated(t, 2)
	signed := signedBy(t, r.privs[1])
	good := r.links[0]
	now := assertedAt.Add(time.Second)

	tests := []struct {
		name      string
		presented string
	}{
		{"an empty link at the end", signed + "~" + good + "~"},
		{"an empty link in the middle", signed + "~~" + good},
		{"another version", signed + "~k2" + good[2:]},
		{"one character short", signed + "~" + good[:len(good)-1]},
		{"one character long", signed + "~" + good + "A"},
		{"padding", signed + "~" + good[:len(good)-2] + "=="},
		{"a line break inside it", signed + "~" + good[:40] + "\n" + good[41:]},
		{"a stray bit in the key's last character", signed + "~" + flipLast(t, good, 1)},
		{"an assertion where a link goes", signed + "~" + signed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := VerifyAssertion(r.pubs[0], tt.presented, assertedVenue, now); !errors.Is(err, ErrAssertion) {
				t.Errorf("err = %v, want ErrAssertion", err)
			}
		})
	}
}

// flipLast changes the last character of part i of a link to one that
// decodes to the same bytes under a lenient decoder.
func flipLast(t *testing.T, link string, part int) string {
	t.Helper()
	parts := strings.Split(link, ".")
	p := parts[part]
	last := p[len(p)-1]
	// 43 characters carry 258 bits for 256: the last two bits are stray.
	next := strings.IndexByte(base64Alphabet, last) ^ 1
	parts[part] = p[:len(p)-1] + string(base64Alphabet[next])
	return strings.Join(parts, ".")
}

const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// A signature the cloud's key made over anything else is not a link. An
// assertion's signature over a key's bytes, as an assertion's payload, does
// not hand the venue that key.
func TestALinkIsSignedUnderItsOwnContext(t *testing.T) {
	held, heldPriv := signingKey(t)
	next, nextPriv := signingKey(t)
	body := base64URL(next)
	wrong := ed25519.Sign(heldPriv, []byte(assertionContext+body))
	forged := keyLinkVersion + "." + body + "." + base64URL(wrong)
	if len(forged) != keyLinkLen {
		t.Fatalf("the forged link is %d long, want %d", len(forged), keyLinkLen)
	}
	presented := PresentAssertion(signedBy(t, nextPriv), forged)
	if _, _, err := VerifyAssertion(held, presented, assertedVenue, assertedAt); !errors.Is(err, ErrAssertion) {
		t.Errorf("a link signed under the assertion's context was followed: %v", err)
	}
}

func TestTheChainHasALimit(t *testing.T) {
	r := rotated(t, MaxKeyChain+2)
	now := assertedAt.Add(time.Second)

	atLimit := PresentAssertion(signedBy(t, r.privs[MaxKeyChain]), r.links[:MaxKeyChain]...)
	if _, signer, err := VerifyAssertion(r.pubs[0], atLimit, assertedVenue, now); err != nil || !signer.Equal(r.pubs[MaxKeyChain]) {
		t.Errorf("%d links: %v", MaxKeyChain, err)
	}
	if len(atLimit) > maxPresented {
		t.Errorf("%d links and an assertion are %d long, over the %d read", MaxKeyChain, len(atLimit), maxPresented)
	}

	over := PresentAssertion(signedBy(t, r.privs[MaxKeyChain+1]), r.links...)
	if _, _, err := VerifyAssertion(r.pubs[0], over, assertedVenue, now); !errors.Is(err, ErrAssertion) {
		t.Errorf("%d links: err = %v, want ErrAssertion", MaxKeyChain+1, err)
	}
}

// The time is still the time: a chained assertion out of its five minutes
// is told apart, as an unchained one is.
func TestAChainedAssertionKeepsItsTime(t *testing.T) {
	r := rotated(t, 2)
	presented := PresentAssertion(signedBy(t, r.privs[1]), r.links...)
	late := assertedAt.Add(AssertionLife + AssertionLeeway)
	if _, signer, err := VerifyAssertion(r.pubs[0], presented, assertedVenue, late); !errors.Is(err, ErrAssertionTime) || signer != nil {
		t.Errorf("err = %v and a signer %v, want ErrAssertionTime and none", err, signer)
	}
	if _, _, err := VerifyAssertion(r.pubs[0], presented, "ven_SOMEONE1", assertedAt); !errors.Is(err, ErrAssertion) {
		t.Errorf("another venue: err = %v, want ErrAssertion", err)
	}
}

func TestSignKeyLinkRefuses(t *testing.T) {
	pub, priv := signingKey(t)
	next, _ := signingKey(t)
	tests := []struct {
		name string
		prev ed25519.PrivateKey
		next ed25519.PublicKey
	}{
		{"no signing key", nil, next},
		{"a short signing key", priv[:31], next},
		{"no next key", priv, nil},
		{"a short next key", priv, next[:31]},
		{"a key handed on to itself", priv, pub},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if link, err := SignKeyLink(tt.prev, tt.next); err == nil {
				t.Errorf("signed %q", link)
			}
		})
	}
	link, err := SignKeyLink(priv, next)
	if err != nil || len(link) != keyLinkLen || !strings.HasPrefix(link, keyLinkVersion+".") {
		t.Errorf("SignKeyLink = %q, %v", link, err)
	}
}

func TestPresentAssertionWithNoLinksIsTheAssertion(t *testing.T) {
	if got := PresentAssertion("v1.a.b"); got != "v1.a.b" {
		t.Errorf("PresentAssertion = %q", got)
	}
	if got := PresentAssertion("v1.a.b", "k1.c.d", "k1.e.f"); got != "v1.a.b~k1.c.d~k1.e.f" {
		t.Errorf("PresentAssertion = %q", got)
	}
}

func TestWithKeyChangesOnlyTheKey(t *testing.T) {
	next, _ := signingKey(t)
	p := Pairing{PublicKey: "old", Owner: Owner{Name: "Priya"}, Venue: Venue{Slug: "cafe"}}
	got := p.WithKey(next)
	if p.PublicKey != "old" {
		t.Error("WithKey changed the pairing it was called on")
	}
	key, err := got.Key()
	if err != nil || !key.Equal(next) {
		t.Errorf("Key() = %v, %v", key, err)
	}
	if got.Owner != p.Owner || got.Venue != p.Venue {
		t.Errorf("WithKey changed more than the key: %+v", got)
	}
}

func indexOf(keys []ed25519.PublicKey, k ed25519.PublicKey) int {
	for i, key := range keys {
		if key.Equal(k) {
			return i
		}
	}
	return -1
}

// Once the walk has begun, nothing is passed over: links out of order, or
// a stranger's link between two of the cloud's, refuse the whole. Before
// it begins, links for older keys are passed over, as they must be.
func TestAWalkOnceBegunPassesNothingOver(t *testing.T) {
	r := rotated(t, 3)
	_, strangerPriv := signingKey(t)
	other, _ := signingKey(t)
	junk, err := SignKeyLink(strangerPriv, other)
	if err != nil {
		t.Fatal(err)
	}
	newest := signedBy(t, r.privs[2])
	now := assertedAt.Add(time.Second)

	tests := []struct {
		name  string
		links []string
		ok    bool
	}{
		{"in order", r.links, true},
		{"a stranger's link before the walk begins", []string{junk, r.links[0], r.links[1]}, true},
		{"out of order", []string{r.links[1], r.links[0]}, false},
		{"a stranger's link in the middle", []string{r.links[0], junk, r.links[1]}, false},
		{"a stranger's link at the end", []string{r.links[0], r.links[1], junk}, false},
		{"a link repeated", []string{r.links[0], r.links[0], r.links[1]}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := VerifyAssertion(r.pubs[0], PresentAssertion(newest, tt.links...), assertedVenue, now)
			if tt.ok && err != nil {
				t.Errorf("refused: %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrAssertion) {
				t.Errorf("err = %v, want ErrAssertion", err)
			}
		})
	}
}
