package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// pairAs pairs a device of this name and kind at the venue.
func pairAs(t *testing.T, devs *Devices, venue int64, name string, kind DeviceKind) Paired {
	t.Helper()
	ctx := context.Background()
	code, err := devs.CreatePairingCode(ctx, NewCode{VenueID: venue, DeviceName: name, DeviceKind: kind})
	if err != nil {
		t.Fatal(err)
	}
	p, err := devs.Redeem(ctx, code.Code)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRosterListsTheVenuesDevicesAndWhoIsWaiting(t *testing.T) {
	devs, _, c := newDevices(t)
	ctx := context.Background()

	empty, err := devs.Roster(ctx, 1)
	if err != nil || len(empty.Devices) != 0 || len(empty.Waiting) != 0 {
		t.Fatalf("a venue with none: %+v, %v", empty, err)
	}

	till := pairAs(t, devs, 1, "Counter 1", KindTerminal)
	c.advance(time.Minute)
	office := pairAs(t, devs, 1, "Office", KindBackOffice)
	pairAs(t, devs, 2, "Another venue's till", KindTerminal)
	c.advance(time.Minute)

	// The till is heard from; the office is not.
	if _, err := devs.Authenticate(ctx, till.Token); err != nil {
		t.Fatal(err)
	}
	waiting, err := devs.CreatePairingCode(ctx, NewCode{VenueID: 1, DeviceName: "Pass", DeviceKind: KindKitchen})
	if err != nil {
		t.Fatal(err)
	}
	// A code used is not waiting.
	if _, err := devs.Redeem(ctx, mustCode(t, devs, 1, "Used", KindTerminal)); err != nil {
		t.Fatal(err)
	}

	r, err := devs.Roster(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, d := range r.Devices {
		names = append(names, d.Name)
	}
	if strings.Join(names, ",") != "Counter 1,Office,Used" {
		t.Fatalf("devices = %v: this venue's, in the order they were paired", names)
	}
	first := r.Devices[0]
	if first.PublicID != till.Device.PublicID || first.Kind != KindTerminal || first.VenueID != 1 ||
		!first.PairedAt.Equal(start) || first.LastSeenAt == nil || !first.LastSeenAt.Equal(start.Add(2*time.Minute)) || first.RevokedAt != nil {
		t.Errorf("the till = %+v", first)
	}
	if o := r.Devices[1]; o.PublicID != office.Device.PublicID || o.LastSeenAt != nil || o.Kind != KindBackOffice {
		t.Errorf("the office = %+v: never heard from since it paired", o)
	}
	if len(r.Waiting) != 1 || r.Waiting[0].Name != "Pass" || r.Waiting[0].Kind != KindKitchen || !r.Waiting[0].ExpiresAt.Equal(waiting.ExpiresAt) {
		t.Errorf("waiting = %+v", r.Waiting)
	}

	// A code that has run out is not waiting any more.
	c.advance(PairingCodeTTL)
	if r, _ = devs.Roster(ctx, 1); len(r.Waiting) != 0 {
		t.Errorf("an expired code is waiting: %+v", r.Waiting)
	}
}

func mustCode(t *testing.T, devs *Devices, venue int64, name string, kind DeviceKind) string {
	t.Helper()
	code, err := devs.CreatePairingCode(context.Background(), NewCode{VenueID: venue, DeviceName: name, DeviceKind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return code.Code
}

func TestRevokedDevicesAreListedLast(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	first := pairAs(t, devs, 1, "Old till", KindTerminal)
	pairAs(t, devs, 1, "New till", KindTerminal)
	if err := devs.RevokeByPublicID(ctx, 1, first.Device.PublicID, 0, 1); err != nil {
		t.Fatal(err)
	}
	r, err := devs.Roster(ctx, 1)
	if err != nil || len(r.Devices) != 2 {
		t.Fatalf("%+v, %v", r, err)
	}
	if r.Devices[0].Name != "New till" || r.Devices[0].RevokedAt != nil || r.Devices[1].Name != "Old till" || r.Devices[1].RevokedAt == nil {
		t.Errorf("devices = %+v: in service first, then the revoked", r.Devices)
	}
}

func TestRename(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	till := pairAs(t, devs, 1, "Counter 1", KindTerminal)
	other := pairAs(t, devs, 2, "Their till", KindTerminal)

	if err := devs.Rename(ctx, 1, till.Device.PublicID, "  Bar till  ", 1); err != nil {
		t.Fatal(err)
	}
	r, _ := devs.Roster(ctx, 1)
	if r.Devices[0].Name != "Bar till" {
		t.Errorf("name = %q: trimmed and saved", r.Devices[0].Name)
	}
	// The token still works: a rename is a label, not a new pairing.
	if dev, err := devs.Authenticate(ctx, till.Token); err != nil || dev.Name != "Bar till" {
		t.Errorf("after the rename: %+v, %v", dev, err)
	}

	for _, name := range []string{"", "   ", strings.Repeat("é", maxDeviceName+1)} {
		err := devs.Rename(ctx, 1, till.Device.PublicID, name, 1)
		var inv *InvalidDevice
		if !errors.Is(err, ErrInvalidDevice) || !errors.As(err, &inv) || inv.Field != "name" {
			t.Errorf("name %q: %v, want an invalid name", name, err)
		}
	}
	if err := devs.Rename(ctx, 1, other.Device.PublicID, "Mine now", 1); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("another venue's device: %v, want ErrDeviceNotFound", err)
	}
	if err := devs.Rename(ctx, 1, "dev_NOSUCH01", "x", 1); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("an unknown device: %v", err)
	}
	if r, _ := devs.Roster(ctx, 2); r.Devices[0].Name != "Their till" {
		t.Errorf("another venue's device was renamed: %q", r.Devices[0].Name)
	}

	// A revoked device keeps the name its history knows it by.
	if err := devs.RevokeByPublicID(ctx, 1, till.Device.PublicID, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := devs.Rename(ctx, 1, till.Device.PublicID, "Again", 1); !errors.Is(err, ErrDeviceRevoked) {
		t.Errorf("a revoked device: %v, want ErrDeviceRevoked", err)
	}
	if r, _ := devs.Roster(ctx, 1); r.Devices[0].Name != "Bar till" {
		t.Errorf("a revoked device was renamed: %q", r.Devices[0].Name)
	}
}

func TestRevokeByPublicID(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	office := pairAs(t, devs, 1, "Office", KindBackOffice)
	till := pairAs(t, devs, 1, "Counter 1", KindTerminal)
	other := pairAs(t, devs, 2, "Their till", KindTerminal)

	// Revoked from the office: it stops working at once.
	if err := devs.RevokeByPublicID(ctx, 1, till.Device.PublicID, office.Device.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := devs.Authenticate(ctx, till.Token); !errors.Is(err, ErrDeviceRevoked) {
		t.Errorf("a revoked till: %v, want ErrDeviceRevoked", err)
	}
	// Again: not an error.
	if err := devs.RevokeByPublicID(ctx, 1, till.Device.PublicID, office.Device.ID, 1); err != nil {
		t.Errorf("revoking again: %v", err)
	}

	// Never itself: the office would be locked out of pairing another.
	if err := devs.RevokeByPublicID(ctx, 1, office.Device.PublicID, office.Device.ID, 1); !errors.Is(err, ErrRevokeSelf) {
		t.Errorf("revoking itself: %v, want ErrRevokeSelf", err)
	}
	if _, err := devs.Authenticate(ctx, office.Token); err != nil {
		t.Errorf("the office stopped working after a refused revoke: %v", err)
	}

	// Only this venue's.
	if err := devs.RevokeByPublicID(ctx, 1, other.Device.PublicID, office.Device.ID, 1); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("another venue's device: %v, want ErrDeviceNotFound", err)
	}
	if _, err := devs.Authenticate(ctx, other.Token); err != nil {
		t.Errorf("another venue's device was revoked: %v", err)
	}
	if err := devs.RevokeByPublicID(ctx, 1, "dev_NOSUCH01", office.Device.ID, 1); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("an unknown device: %v", err)
	}
}

// A refused code says which field it is about.
func TestAPairingCodeRefusalNamesItsField(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	tests := []struct {
		n     NewCode
		field string
	}{
		{NewCode{VenueID: 1, DeviceName: " ", DeviceKind: KindTerminal}, "name"},
		{NewCode{VenueID: 1, DeviceName: "Table 4", DeviceKind: "guest"}, "kind"},
		{NewCode{VenueID: 1, DeviceName: "Toaster", DeviceKind: "toaster"}, "kind"},
	}
	for _, tt := range tests {
		_, err := devs.CreatePairingCode(ctx, tt.n)
		var inv *InvalidDevice
		if !errors.Is(err, ErrInvalidDevice) || !errors.As(err, &inv) || inv.Field != tt.field {
			t.Errorf("%+v: err = %v, want a refusal of %q", tt.n, err, tt.field)
		}
	}
}
