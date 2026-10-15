package cloud

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// A key link is how a paired venue learns mez Cloud's next signing key
// without pairing again (ADR-345). The cloud makes one when it rotates: the
// new public key, signed once by the key before it. Every assertion after
// that is presented with the links that lead to the key that signed it, and
// the venue follows them from the key its pairing holds.
//
// A link reads "k1.<key>.<signature>", both in base64url with no padding.
// The signature is over keyLinkContext and the key part as it is written,
// so a signature the same key made over anything else, an assertion
// included, is not a link.
//
// An assertion presented with links reads "<assertion>~<link>~<link>", the
// oldest link first.
const (
	// MaxKeyChain is the most links one assertion may be presented with.
	// A venue further behind than this pairs again.
	MaxKeyChain = 16

	keyLinkVersion = "k1"
	keyLinkContext = "mez-cloud-key-v1\n"
	chainSeparator = "~"

	// keyLinkLen is the one length a link can be: the version, a 32 byte
	// key and a 64 byte signature, each in base64url with no padding.
	keyLinkLen = len(keyLinkVersion) + 1 + 43 + 1 + 86

	// maxPresented bounds what is read of an assertion and its links.
	maxPresented = maxAssertion + MaxKeyChain*(len(chainSeparator)+keyLinkLen)
)

// SignKeyLink is the link that hands on next, signed with the key before
// it. The venue's binary never calls it, as it never calls SignAssertion.
func SignKeyLink(prev ed25519.PrivateKey, next ed25519.PublicKey) (string, error) {
	if len(prev) != ed25519.PrivateKeySize {
		return "", errors.New("cloud: the signing key is not an Ed25519 private key")
	}
	if len(next) != ed25519.PublicKeySize {
		return "", errors.New("cloud: the next key is not an Ed25519 public key")
	}
	if next.Equal(prev.Public()) {
		return "", errors.New("cloud: a key does not hand on to itself")
	}
	body := base64URL(next)
	sig := ed25519.Sign(prev, []byte(keyLinkContext+body))
	return keyLinkVersion + "." + body + "." + base64URL(sig), nil
}

// PresentAssertion is a signed assertion as it is sent, with the links
// that lead from the oldest key a venue may hold to the key that signed
// it, oldest first. With none, it is the assertion alone.
func PresentAssertion(signed string, chain ...string) string {
	if len(chain) == 0 {
		return signed
	}
	return signed + chainSeparator + strings.Join(chain, chainSeparator)
}

// followChain walks links from held and returns the newest key it
// reaches. A link before the first one held signed is passed over: it is
// for a venue that holds an older key. From that one on, each link must be
// signed by the key the one before handed on, so nothing is passed over
// in the middle of a walk. A link that is not a link at all refuses the
// whole.
func followChain(held ed25519.PublicKey, links []string) (ed25519.PublicKey, error) {
	if len(links) > MaxKeyChain {
		return nil, ErrAssertion
	}
	current, walking := held, false
	for _, link := range links {
		next, sig, ok := parseKeyLink(link)
		if !ok {
			return nil, ErrAssertion
		}
		switch {
		case ed25519.Verify(current, []byte(keyLinkContext+next.body), sig):
			current, walking = next.key, true
		case walking:
			return nil, ErrAssertion
		}
	}
	return current, nil
}

type linkedKey struct {
	key  ed25519.PublicKey
	body string
}

// parseKeyLink reads a link's parts, signed or not.
func parseKeyLink(link string) (linkedKey, []byte, bool) {
	if len(link) != keyLinkLen {
		return linkedKey{}, nil, false
	}
	parts := strings.Split(link, ".")
	if len(parts) != 3 || parts[0] != keyLinkVersion {
		return linkedKey{}, nil, false
	}
	key, err := assertionEncoding.DecodeString(parts[1])
	if err != nil || len(key) != ed25519.PublicKeySize {
		return linkedKey{}, nil, false
	}
	sig, err := assertionEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return linkedKey{}, nil, false
	}
	return linkedKey{key: ed25519.PublicKey(key), body: parts[1]}, sig, true
}

func base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
