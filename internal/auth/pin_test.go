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

// TestPINLengthByRole: a role that manages staff, devices or settings needs
// six digits; any other keeps four as the floor. Eight is the ceiling for
// both, and a PIN of letters is a format error whatever the role.
func TestPINLengthByRole(t *testing.T) {
	set := func(perms ...Permission) PermissionSet {
		out := PermissionSet{}
		for _, p := range perms {
			out[p] = struct{}{}
		}
		return out
	}
	tests := []struct {
		name string
		pin  string
		role PermissionSet
		want error
	}{
		{"a cashier with four", "2468", set(PermOrderOpen, PermPaymentTake), nil},
		{"a role with nothing, four", "2468", set(), nil},
		{"staff.manage with four", "2468", set(PermStaffManage), ErrPINTooShortForRole},
		{"device.manage with five", "24680", set(PermDeviceManage), ErrPINTooShortForRole},
		{"settings.edit with five", "24680", set(PermSettingsEdit, PermOrderOpen), ErrPINTooShortForRole},
		{"staff.manage with six", "246802", set(PermStaffManage), nil},
		{"everything with eight", "24680246", set(Permissions()...), nil},
		{"everything with nine", "246802468", set(Permissions()...), ErrPINFormat},
		{"a cashier with three", "246", set(PermOrderOpen), ErrPINFormat},
		{"staff.manage with letters", "abcdef", set(PermStaffManage), ErrPINFormat},
		{"reports and menu are not powerful", "2468", set(PermReportsView, PermMenuEdit, PermLineVoid), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckPINFor(tt.pin, tt.role); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("CheckPINFor(%q) = %v, want %v", tt.pin, err, tt.want)
			}
		})
	}
	if MinPINLengthFor(set(PermStaffManage)) != MinPowerfulPINLength || MinPINLengthFor(set(PermOrderOpen)) != MinPINLength {
		t.Error("MinPINLengthFor does not follow the role")
	}
}
