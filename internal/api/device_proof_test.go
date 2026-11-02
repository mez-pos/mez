package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	proofNonce = "0123456789abcdef0123456789abcdef"
	thePC      = "192.168.1.45:7777"
)

// askProof asks h for device id's proof, the tablet saying it asked at, the
// request reaching the server at local (its own end of the connection).
func askProof(h http.Handler, id, nonce, at, local string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+id+"/proof?nonce="+nonce+"&at="+url.QueryEscape(at), nil)
	if local != "" {
		addr, err := net.ResolveTCPAddr("tcp", local)
		if err == nil {
			req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, addr))
		}
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// tabletsProof is what the app makes from the token it holds (Proof.kt), made
// here from first principles: HMAC-SHA256 keyed by the token's SHA-256 in hex.
func tabletsProof(token, device, nonce, at string) string {
	sum := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(sum[:])))
	mac.Write([]byte("mez-proof-v1\n" + device + "\n" + nonce + "\n" + at))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestTheProofIsWhatTheTabletMakes: the PC that holds the pairing answers
// what the tablet holding the token expects, for the address it asked
// (ADR-417).
func TestTheProofIsWhatTheTabletMakes(t *testing.T) {
	h, devs := pairingServer(t)
	p := pairOK(t, h, newCode(t, devs))

	rec := askProof(h, p.Device.ID, proofNonce, thePC, thePC, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a proof may be cached")
	}
	var got proofResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("proof response: %v", err)
	}
	if got.Proof != tabletsProof(p.Token, p.Device.ID, proofNonce, thePC) {
		t.Errorf("proof %q is not what the tablet makes", got.Proof)
	}
	if strings.Contains(rec.Body.String(), p.Token) {
		t.Error("the answer carries the token")
	}
}

// TestNoProofForAnotherAddress: a device on the wifi that passes the
// tablet's question on to the PC reaches it at the PC's address, not the one
// the tablet asked, and is answered nothing (ADR-417).
func TestNoProofForAnotherAddress(t *testing.T) {
	h, devs := pairingServer(t)
	p := pairOK(t, h, newCode(t, devs))
	tests := []struct{ name, at, local string }{
		{"another host", "192.168.1.99:7777", thePC},
		{"another port", "192.168.1.45:8080", thePC},
		{"no port", "192.168.1.45", thePC},
		{"a name", "mez.local:7777", thePC},
		{"nothing said", "", thePC},
		{"no connection to compare", thePC, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := askProof(h, p.Device.ID, proofNonce, tt.at, tt.local, nil)
			if rec.Code != http.StatusMisdirectedRequest || errorOf(t, rec).Field != "at" {
				t.Errorf("status %d, body %s, want 421 on at", rec.Code, rec.Body)
			}
		})
	}
	// IPv4 written as IPv6 is the same address.
	if rec := askProof(h, p.Device.ID, proofNonce, thePC, "[::ffff:192.168.1.45]:7777", nil); rec.Code != http.StatusOK {
		t.Errorf("a mapped address: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestProofRefusals(t *testing.T) {
	h, devs := pairingServer(t)
	p := pairOK(t, h, newCode(t, devs))
	gone := pairOK(t, h, newCode(t, devs))
	revoke(t, devs, gone)
	tests := []struct {
		name, id, nonce string
		status          int
		code            ErrorCode
	}{
		{"a short nonce", p.Device.ID, "abc123", http.StatusBadRequest, CodeInvalidRequest},
		{"an uppercase nonce", p.Device.ID, strings.ToUpper(proofNonce), http.StatusBadRequest, CodeInvalidRequest},
		{"no device by that id", "dev_NOTHERE1", proofNonce, http.StatusNotFound, CodeNotFound},
		{"a revoked device", gone.Device.ID, proofNonce, http.StatusNotFound, CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := askProof(h, tt.id, tt.nonce, thePC, thePC, nil)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Errorf("status %d, body %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
		})
	}
}

// TestProofIsAtTheVenueOnly: the relay's gate refuses it (ADR-308), and a
// server whose devices cannot prove has no such route.
func TestProofIsAtTheVenueOnly(t *testing.T) {
	h, devs := pairingServer(t)
	p := pairOK(t, h, newCode(t, devs))
	rec := askProof(h, p.Device.ID, proofNonce, thePC, thePC, map[string]string{HeaderVia: ViaRelay})
	if errorOf(t, rec).Code != CodeRemoteNotAllowed {
		t.Errorf("through the relay: status %d, body %s", rec.Code, rec.Body)
	}

	without := newTestServer(t, testConfig(), WithDevices(brokenDevices{})).Handler()
	if rec := askProof(without, p.Device.ID, proofNonce, thePC, thePC, nil); rec.Code != http.StatusNotFound {
		t.Errorf("devices that cannot prove: status %d, body %s", rec.Code, rec.Body)
	}
}

// TestProofMessageVector pins the bytes a proof is made over to a vector
// made outside both the server and the app (Python's hmac), the same one
// the app's ProofTest holds (ADR-417).
func TestProofMessageVector(t *testing.T) {
	token := "nfd_" + strings.Repeat("A", 43)
	sum := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(sum[:])))
	mac.Write(proofMessage("dev_7K3M9Q2A", proofNonce, thePC))
	if got := hex.EncodeToString(mac.Sum(nil)); got != "0c74779a56dbe468fcfe51165e054306b51b0c28d7385ec47de8992f8d542adf" {
		t.Errorf("proof = %s", got)
	}
}
