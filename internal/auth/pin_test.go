package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerifyPIN(t *testing.T) {
	hash, err := HashPIN("2468")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash %q does not carry its parameters", hash)
	}
	again, _ := HashPIN("2468")
	if again == hash {
		t.Error("two hashes of one PIN are equal: the salt is not random")
	}
	for pin, want := range map[string]bool{"2468": true, "2469": false, "24680": false, "": false} {
		got, err := verifyPIN(hash, pin)
		if err != nil || got != want {
			t.Errorf("verifyPIN(%q) = %v, %v; want %v", pin, got, err, want)
		}
	}
}

func TestPINFormat(t *testing.T) {
	for _, pin := range []string{"", "123", "123456789", "12a4", "12 34", "١٢٣٤"} {
		if _, err := HashPIN(pin); !errors.Is(err, ErrPINFormat) {
			t.Errorf("HashPIN(%q): err = %v, want ErrPINFormat", pin, err)
		}
	}
	for _, pin := range []string{"0000", "12345678"} {
		if _, err := HashPIN(pin); err != nil {
			t.Errorf("HashPIN(%q): %v", pin, err)
		}
	}
}

// TestAStoredHashCannotBeTrustedBlindly feeds verifyPIN the hashes a broken
// or tampered row could hold. None may match, and none may make it allocate
// what the hash asks for without limit.
func TestAStoredHashCannotBeTrustedBlindly(t *testing.T) {
	good, _ := HashPIN("2468")
	parts := strings.Split(good, "$")
	tests := []string{
		"",
		"plain",
		"$argon2i$v=19$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=16$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=99999999,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=19456,t=0,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=19456,t=2,p=0$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=19456,t=2,p=1$!!!$" + parts[5],
		"$argon2id$v=19$m=19456,t=2,p=1$" + parts[4] + "$",
		"$argon2id$v=19$m=19456,t=2,p=1$" + parts[4],
	}
	for _, h := range tests {
		ok, err := verifyPIN(h, "2468")
		if ok || err == nil {
			t.Errorf("verifyPIN(%q) = %v, %v; want no match and an error", h, ok, err)
		}
	}
	dummy, err := dummyHash()
	if err != nil {
		t.Fatalf("dummyHash: %v", err)
	}
	if ok, err := verifyPIN(dummy, "00000000"); err != nil || !ok {
		t.Errorf("the dummy hash does not verify its own PIN: %v, %v", ok, err)
	}
}

func TestKeyedMutexForgetsIdleKeys(t *testing.T) {
	var k keyedMutex
	unlock := k.lock("a")
	unlock()
	unlockB := k.lock("b")
	if len(k.locks) != 1 {
		t.Errorf("%d keys held, want 1", len(k.locks))
	}
	unlockB()
	if len(k.locks) != 0 {
		t.Errorf("%d keys held after every unlock, want 0", len(k.locks))
	}
}
