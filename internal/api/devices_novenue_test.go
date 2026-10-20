package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mez-pos/mez/internal/auth"
)

// TestATokenAtAComputerWithNoVenueIsNotRefused is the till left on through
// "The PC died" (ADR-339): the new computer answers at the old address
// before the venue is back, and what it says must not be the answer that
// makes a till forget its pairing. Once there is a venue, a token it does
// not know is refused as it always was.
func TestATokenAtAComputerWithNoVenueIsNotRefused(t *testing.T) {
	h, _, _ := setupServer(t, testConfig())
	held := bearerOf("mzd_a-token-the-old-computer-gave")

	rec := send(h, http.MethodGet, "/api/v1/devices/me", "", held)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a token at a computer with no venue: status %d, body %s; want 503", rec.Code, rec.Body)
	}
	if got := errorOf(t, rec); got.Code != CodeNoVenue || got.Message == "" {
		t.Errorf("error = %+v, want %s and a sentence", got, CodeNoVenue)
	}
	if rec.Header().Get("WWW-Authenticate") != "" {
		t.Error("it asks for credentials, as a refusal does")
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After: nothing says to ask again")
	}

	// No token at all is still what it was: there is nothing to keep.
	if rec := send(h, http.MethodGet, "/api/v1/devices/me", "", nil); rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeUnauthenticated {
		t.Errorf("no token at a computer with no venue: status %d, body %s; want 401 unauthenticated", rec.Code, rec.Body)
	}

	if rec := from(h, atTheTill(), http.MethodPost, "/api/v1/setup", setupBody); rec.Code != http.StatusCreated {
		t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	rec = send(h, http.MethodGet, "/api/v1/devices/me", "", held)
	if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeUnauthenticated {
		t.Errorf("an unknown token once there is a venue: status %d, body %s; want 401 unauthenticated", rec.Code, rec.Body)
	}
}

// noDevices knows no token.
type noDevices struct{ Devices }

func (noDevices) Authenticate(ctx context.Context, token string) (auth.Device, error) {
	return auth.Device{}, auth.ErrUnauthenticated
}

// neverSure is a first run that cannot say whether there is a venue.
type neverSure struct{ FirstRun }

func (neverSure) Needed(ctx context.Context) (bool, error) {
	return false, errors.New("the disk is gone")
}

// TestATokenIsNotRefusedWhenTheVenueCannotBeLookedFor: not knowing whether
// there is a venue is not knowing whether the token is bad, and a till must
// not be told to forget it on a guess.
func TestATokenIsNotRefusedWhenTheVenueCannotBeLookedFor(t *testing.T) {
	srv := newTestServer(t, testConfig(), WithDevices(noDevices{}), WithFirstRun(neverSure{}))
	rec := send(srv.Handler(), http.MethodGet, "/api/v1/devices/me", "", bearerOf("mzd_whatever"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d, body %s; want 500", rec.Code, rec.Body)
	}
}
