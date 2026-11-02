package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

// proofBy is what a tablet holding token expects for message (ADR-417): the
// HMAC keyed by the token's SHA-256 in hex, made here without the package's
// own hashing so that the two are checked against each other.
func proofBy(token string, message []byte) []byte {
	sum := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(sum[:])))
	mac.Write(message)
	return mac.Sum(nil)
}

func TestProveAnswersWhatTheTabletMakes(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	p := pair(t, devs, 1)
	other := pair(t, devs, 1)
	message := []byte("mez-proof-v1\nnonce\n192.168.1.45:7777")

	got, err := devs.Prove(ctx, p.Device.PublicID, message)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if !hmac.Equal(got, proofBy(p.Token, message)) {
		t.Error("the proof is not what the tablet holding the token makes")
	}
	if hmac.Equal(got, proofBy(other.Token, message)) {
		t.Error("the proof is what another device's token makes")
	}
	again, err := devs.Prove(ctx, p.Device.PublicID, []byte("mez-proof-v1\nnonce\n192.168.1.46:7777"))
	if err != nil || hmac.Equal(got, again) {
		t.Errorf("another message proved the same, or not at all: %v", err)
	}
}

func TestNoProofForAPairingThatIsNotHere(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	p := pair(t, devs, 1)
	if err := devs.Revoke(ctx, 1, p.Device.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	for name, id := range map[string]string{"revoked": p.Device.PublicID, "unknown": "dev_NOTHERE1", "empty": ""} {
		if _, err := devs.Prove(ctx, id, []byte("m")); !errors.Is(err, ErrNoProof) {
			t.Errorf("%s: err = %v, want ErrNoProof", name, err)
		}
	}
}
