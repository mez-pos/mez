package auth

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
)

var start = time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)

// clock is a settable test clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newDevices is a Devices over a fresh database holding venues 1 and 2.
func newDevices(t *testing.T) (*Devices, *db.DB, *clock) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, pub := range []string{"ven_AAAAAAAA", "ven_BBBBBBBB"} {
		err := d.WriteTx(ctx, func(q *gen.Queries) error {
			_, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: pub, Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: start, UpdatedAt: start,
			})
			return err
		})
		if err != nil {
			t.Fatalf("CreateVenue: %v", err)
		}
	}
	c := &clock{t: start}
	devs, err := NewDevices(DevicesOptions{DB: d, Now: c.now})
	if err != nil {
		t.Fatalf("NewDevices: %v", err)
	}
	return devs, d, c
}

func counter(venue int64) NewCode {
	return NewCode{VenueID: venue, DeviceName: "Counter 1", DeviceKind: KindTerminal}
}

func pair(t *testing.T, devs *Devices, venue int64) Paired {
	t.Helper()
	ctx := context.Background()
	code, err := devs.CreatePairingCode(ctx, counter(venue))
	if err != nil {
		t.Fatalf("CreatePairingCode: %v", err)
	}
	p, err := devs.Redeem(ctx, code.Code)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	return p
}

func TestPairAndAuthenticate(t *testing.T) {
	devs, _, c := newDevices(t)
	ctx := context.Background()

	code, err := devs.CreatePairingCode(ctx, NewCode{VenueID: 2, DeviceName: "  Kitchen pass  ", DeviceKind: KindKitchen})
	if err != nil {
		t.Fatalf("CreatePairingCode: %v", err)
	}
	if len(code.Code) != 9 || code.Code[4] != '-' || !code.ExpiresAt.Equal(start.Add(PairingCodeTTL)) {
		t.Fatalf("code = %+v", code)
	}

	c.advance(PairingCodeTTL - time.Second)
	p, err := devs.Redeem(ctx, code.Code)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if p.Device.VenueID != 2 || p.Device.Name != "Kitchen pass" || p.Device.Kind != KindKitchen || !strings.HasPrefix(p.Device.PublicID, "dev_") {
		t.Errorf("device = %+v", p.Device)
	}
	if !strings.HasPrefix(p.Token, "nfd_") || len(p.Token) != 47 {
		t.Errorf("token %q is not nfd_ and 43 characters", p.Token)
	}

	got, err := devs.Authenticate(ctx, p.Token)
	if err != nil || got != p.Device {
		t.Errorf("Authenticate = %+v, %v; want %+v", got, err, p.Device)
	}
}

// TestACodeFailsTheSameWayWhateverIsWrong is the guessing defence: unknown,
// expired and spent codes are indistinguishable to whoever presents them.
func TestACodeFailsTheSameWayWhateverIsWrong(t *testing.T) {
	devs, _, c := newDevices(t)
	ctx := context.Background()

	spent, _ := devs.CreatePairingCode(ctx, counter(1))
	if _, err := devs.Redeem(ctx, spent.Code); err != nil {
		t.Fatalf("first Redeem: %v", err)
	}
	expired, _ := devs.CreatePairingCode(ctx, counter(1))
	c.advance(PairingCodeTTL)

	tests := []struct {
		name string
		code string
	}{
		{"already used", spent.Code},
		{"expired, to the second", expired.Code},
		{"never issued", "ZZZZ-ZZZZ"},
		{"too short", "7K3M"},
		{"not in the alphabet", "7K3M-9QX!"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := devs.Redeem(ctx, tt.code)
			if err != ErrInvalidPairingCode {
				t.Errorf("err = %v, want exactly ErrInvalidPairingCode", err)
			}
		})
	}
}

// TestACodeIsReadAsTyped accepts what a person copying a code off a screen
// actually types.
func TestACodeIsReadAsTyped(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	// A fixed entropy source makes the code 0110-ABCD: the digits a person is
	// most likely to type as letters.
	entropy := []byte{0, 1, 1, 0, 10, 11, 12, 13}
	fixed, err := NewDevices(DevicesOptions{DB: devs.db, Now: devs.now, Entropy: bytes.NewReader(append(entropy, make([]byte, 256)...))})
	if err != nil {
		t.Fatal(err)
	}
	code, err := fixed.CreatePairingCode(ctx, counter(1))
	if err != nil || code.Code != "0110-ABCD" {
		t.Fatalf("code = %q, %v", code.Code, err)
	}
	if _, err := devs.Redeem(ctx, " oIlO abcd "); err != nil {
		t.Errorf("Redeem of the code as typed: %v", err)
	}
}

// TestACodePairsExactlyOneDevice races several devices for one code.
func TestACodePairsExactlyOneDevice(t *testing.T) {
	devs, d, _ := newDevices(t)
	ctx := context.Background()
	code, err := devs.CreatePairingCode(ctx, counter(1))
	if err != nil {
		t.Fatal(err)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := devs.Redeem(ctx, code.Code)
			switch {
			case err == nil:
				mu.Lock()
				wins++
				mu.Unlock()
			case !errors.Is(err, ErrInvalidPairingCode):
				t.Errorf("Redeem: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d devices paired with one code, want 1", wins)
	}
	var devices int
	if err := d.Reader().QueryRowContext(ctx, `SELECT count(*) FROM devices`).Scan(&devices); err != nil || devices != 1 {
		t.Errorf("%d device rows (%v), want 1", devices, err)
	}
}

func TestTokensThatAreNotADevicesToken(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	p := pair(t, devs, 1)

	forged := []byte(p.Token)
	forged[len(forged)-1] ^= 1
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no prefix", strings.TrimPrefix(p.Token, "nfd_")},
		{"another prefix", "nfs_" + strings.TrimPrefix(p.Token, "nfd_")},
		{"not base64", "nfd_" + strings.Repeat("!", 43)},
		{"too short", p.Token[:20]},
		{"one bit off", string(forged)},
		{"well formed and unknown", "nfd_" + strings.Repeat("A", 43)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := devs.Authenticate(ctx, tt.token); !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

func TestRevokingADeviceKillsItsToken(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	p := pair(t, devs, 1)
	other := pair(t, devs, 1)

	if err := devs.Revoke(ctx, 2, p.Device.ID); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("revoke from another venue: err = %v", err)
	}
	if _, err := devs.Authenticate(ctx, p.Token); err != nil {
		t.Fatalf("another venue's revoke took effect: %v", err)
	}
	if err := devs.Revoke(ctx, 1, p.Device.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := devs.Authenticate(ctx, p.Token); !errors.Is(err, ErrDeviceRevoked) {
		t.Errorf("revoked token: err = %v, want ErrDeviceRevoked", err)
	}
	if err := devs.Revoke(ctx, 1, p.Device.ID); err != nil {
		t.Errorf("revoking twice: %v", err)
	}
	if err := devs.Revoke(ctx, 1, 999); !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("revoking no device: err = %v", err)
	}
	if _, err := devs.Authenticate(ctx, other.Token); err != nil {
		t.Errorf("revoking one device broke another: %v", err)
	}
}

func TestLastSeenIsWrittenAtMostOnceAMinute(t *testing.T) {
	devs, d, c := newDevices(t)
	ctx := context.Background()
	p := pair(t, devs, 1)

	lastSeen := func() time.Time {
		t.Helper()
		row, err := d.Queries().DeviceByID(ctx, p.Device.ID)
		if err != nil {
			t.Fatal(err)
		}
		return row.LastSeenAt.Time.UTC()
	}
	auth := func() {
		t.Helper()
		if _, err := devs.Authenticate(ctx, p.Token); err != nil {
			t.Fatal(err)
		}
	}

	auth()
	if got := lastSeen(); !got.Equal(start) {
		t.Fatalf("last seen %v after the first request, want %v", got, start)
	}
	c.advance(59 * time.Second)
	auth()
	if got := lastSeen(); !got.Equal(start) {
		t.Errorf("last seen moved to %v inside a minute", got)
	}
	c.advance(time.Second)
	auth()
	if got := lastSeen(); !got.Equal(start.Add(time.Minute)) {
		t.Errorf("last seen %v after a minute, want %v", got, start.Add(time.Minute))
	}
}

func TestCreatePairingCodeRefusals(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	tests := []struct {
		name string
		n    NewCode
	}{
		{"no name", NewCode{VenueID: 1, DeviceName: "   ", DeviceKind: KindTerminal}},
		{"a name too long", NewCode{VenueID: 1, DeviceName: strings.Repeat("é", maxDeviceName+1), DeviceKind: KindTerminal}},
		{"a guest", NewCode{VenueID: 1, DeviceName: "Table 4", DeviceKind: "guest"}},
		{"an unknown kind", NewCode{VenueID: 1, DeviceName: "Toaster", DeviceKind: "toaster"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := devs.CreatePairingCode(ctx, tt.n); !errors.Is(err, ErrInvalidDevice) {
				t.Errorf("err = %v, want ErrInvalidDevice", err)
			}
		})
	}
	if _, err := devs.CreatePairingCode(ctx, counter(99)); err == nil {
		t.Error("a code was made for a venue that does not exist")
	}
	if _, err := devs.CreatePairingCode(ctx, NewCode{VenueID: 1, CreatedBy: 42, DeviceName: "x", DeviceKind: KindTerminal}); err == nil {
		t.Error("a code was made by a staff member who does not exist")
	}
}

// TestACodeCollisionIsRedrawn feeds the same code twice, then a fresh one.
func TestACodeCollisionIsRedrawn(t *testing.T) {
	devs, _, _ := newDevices(t)
	ctx := context.Background()
	same := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	src := bytes.NewReader(bytes.Join([][]byte{same, same, {8, 7, 6, 5, 4, 3, 2, 1}}, nil))
	fixed, err := NewDevices(DevicesOptions{DB: devs.db, Now: devs.now, Entropy: src})
	if err != nil {
		t.Fatal(err)
	}
	first, err := fixed.CreatePairingCode(ctx, counter(1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixed.CreatePairingCode(ctx, counter(1))
	if err != nil {
		t.Fatalf("second code: %v", err)
	}
	if first.Code != "1234-5678" || second.Code != "8765-4321" {
		t.Errorf("codes %s then %s, want the collision redrawn", first.Code, second.Code)
	}
	if _, err := NewDevices(DevicesOptions{}); err == nil {
		t.Error("NewDevices with no database succeeded")
	}
}
