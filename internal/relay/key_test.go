package relay

import (
	"bytes"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func mustKeys(t *testing.T, secret string, revoked ...string) *Keys {
	t.Helper()
	keys, err := NewKeys(secret, revoked)
	if err != nil {
		t.Fatalf("NewKeys: %v", err)
	}
	return keys
}

func mustIssue(t *testing.T, keys *Keys, venueID string) string {
	t.Helper()
	key, err := keys.Issue(rand.Reader, venueID)
	if err != nil {
		t.Fatalf("Issue(%q): %v", venueID, err)
	}
	return key
}

func TestNewKeysRefusesShortSecret(t *testing.T) {
	if _, err := NewKeys(strings.Repeat("x", MinSecretLen-1), nil); err == nil {
		t.Fatal("NewKeys accepted a secret under the minimum length")
	}
}

func TestIssueAndVerify(t *testing.T) {
	keys := mustKeys(t, testSecret)
	key := mustIssue(t, keys, "blue-tokai-12")

	if !strings.HasPrefix(key, "mzr_blue-tokai-12.") {
		t.Errorf("key %q does not say what it is and whose", key)
	}
	venueID, keyID, err := keys.Verify(key)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if venueID != "blue-tokai-12" || len(keyID) != keyIDBytes*2 {
		t.Errorf("Verify = %q, %q", venueID, keyID)
	}
	if other := mustIssue(t, keys, "blue-tokai-12"); other == key {
		t.Error("two keys for one venue are the same key")
	}
}

func TestIssueRefusesBadVenueIDAndShortRandom(t *testing.T) {
	keys := mustKeys(t, testSecret)
	if _, err := keys.Issue(rand.Reader, "Blue Tokai"); err == nil {
		t.Error("Issue accepted a venue id with a space and capitals")
	}
	if _, err := keys.Issue(bytes.NewReader([]byte{1, 2}), "cafe"); err == nil {
		t.Error("Issue made a key from a source that ran dry")
	}
}

func TestVerifyRefuses(t *testing.T) {
	keys := mustKeys(t, testSecret)
	good := mustIssue(t, keys, "cafe")
	parts := strings.Split(strings.TrimPrefix(good, keyPrefix), ".")

	tests := []struct {
		name string
		key  string
		want error
	}{
		{"empty", "", ErrKeyMalformed},
		{"no prefix", strings.TrimPrefix(good, keyPrefix), ErrKeyMalformed},
		{"a device token", "nfd_abc", ErrKeyMalformed},
		{"two parts", keyPrefix + "cafe." + parts[2], ErrKeyMalformed},
		{"four parts", good + ".extra", ErrKeyMalformed},
		{"venue id not allowed", keyPrefix + "CAFE." + parts[1] + "." + parts[2], ErrKeyMalformed},
		{"key id wrong length", keyPrefix + "cafe.abc." + parts[2], ErrKeyMalformed},
		{"another venue's name on this mac", keyPrefix + "bar." + parts[1] + "." + parts[2], ErrKeyInvalid},
		{"another key id on this mac", keyPrefix + "cafe.000000000000." + parts[2], ErrKeyInvalid},
		{"mac truncated", good[:len(good)-1], ErrKeyInvalid},
		{"signed by another relay", mustIssue(t, mustKeys(t, strings.Repeat("z", MinSecretLen)), "cafe"), ErrKeyInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := keys.Verify(tt.key); !errors.Is(err, tt.want) {
				t.Errorf("Verify = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyRevoked(t *testing.T) {
	issuer := mustKeys(t, testSecret)
	first := mustIssue(t, issuer, "cafe")
	second := mustIssue(t, issuer, "cafe")
	_, firstID, err := issuer.Verify(first)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	t.Run("by key id, the venue's other key still works", func(t *testing.T) {
		keys := mustKeys(t, testSecret, " "+firstID+" ", "")
		if _, _, err := keys.Verify(first); !errors.Is(err, ErrKeyRevoked) {
			t.Errorf("revoked key: Verify = %v, want ErrKeyRevoked", err)
		}
		if _, _, err := keys.Verify(second); err != nil {
			t.Errorf("the other key: Verify = %v", err)
		}
	})
	t.Run("by venue id, every key of the venue is refused", func(t *testing.T) {
		keys := mustKeys(t, testSecret, "cafe")
		for _, key := range []string{first, second} {
			if _, _, err := keys.Verify(key); !errors.Is(err, ErrKeyRevoked) {
				t.Errorf("Verify = %v, want ErrKeyRevoked", err)
			}
		}
		if _, _, err := keys.Verify(mustIssue(t, issuer, "bar")); err != nil {
			t.Errorf("another venue: Verify = %v", err)
		}
	})
}

func TestValidVenueID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"cafe", true},
		{"blue-tokai-12", true},
		{"7", true},
		{strings.Repeat("a", 63), true},
		{"", false},
		{strings.Repeat("a", 64), false},
		{"-cafe", false},
		{"Cafe", false},
		{"cafe.bar", false},
		{"cafe/bar", false},
		{"cafe bar", false},
		{"café", false},
	}
	for _, tt := range tests {
		if got := ValidVenueID(tt.id); got != tt.want {
			t.Errorf("ValidVenueID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestBucket(t *testing.T) {
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	// One token a second, three at once.
	b := newBucket(Limit{PerMinute: 60, Burst: 3}, start)

	for i := range 3 {
		if ok, _ := b.take(start); !ok {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}
	ok, wait := b.take(start)
	if ok || wait != time.Second {
		t.Fatalf("past the burst: take = %v, %v; want refused for 1s", ok, wait)
	}
	if ok, wait := b.take(start.Add(400 * time.Millisecond)); ok || wait != 600*time.Millisecond {
		t.Errorf("0.4s later: take = %v, %v; want refused for 600ms", ok, wait)
	}
	if ok, _ := b.take(start.Add(time.Second)); !ok {
		t.Error("a second later the refilled token was refused")
	}
	if ok, _ := b.take(start.Add(time.Second)); ok {
		t.Error("one second bought two requests")
	}

	// A long quiet spell refills to the burst and no further.
	later := start.Add(time.Hour)
	for i := range 3 {
		if ok, _ := b.take(later); !ok {
			t.Fatalf("after an hour, request %d of the burst was refused", i+1)
		}
	}
	if ok, _ := b.take(later); ok {
		t.Error("an idle hour banked more than the burst")
	}
	// A clock that steps backwards buys nothing and breaks nothing.
	if ok, _ := b.take(start); ok {
		t.Error("a backwards clock produced a token")
	}
}
