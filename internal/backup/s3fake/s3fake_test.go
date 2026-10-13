package s3fake

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signed is a request as the SDK signs one, as far as the stand-in looks:
// the key's id in the Authorization header.
func signed(t *testing.T, method, url, key, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+key+"/20261013/auto/s3/aws4_request, SignedHeaders=host, Signature=0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// TestAReaderLooksAndDoesNotWrite: the key a restore is handed reads what
// the venue's own key wrote, and can change none of it (ADR-332).
func TestAReaderLooksAndDoesNotWrite(t *testing.T) {
	bucket := New("mez-dev", "writer").WithReader("reader")
	srv := httptest.NewServer(bucket)
	defer srv.Close()
	object := srv.URL + "/mez-dev/v/cafe/db/one"

	if resp := signed(t, http.MethodPut, object, "writer", "a backup"); resp.StatusCode != http.StatusOK {
		t.Fatalf("the venue's key writing: status %d", resp.StatusCode)
	}

	resp := signed(t, http.MethodGet, object, "reader", "")
	if got, _ := io.ReadAll(resp.Body); resp.StatusCode != http.StatusOK || string(got) != "a backup" {
		t.Errorf("the reader reading: status %d, %q", resp.StatusCode, got)
	}
	if resp := signed(t, http.MethodHead, object, "reader", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("the reader asking whether it is there: status %d", resp.StatusCode)
	}
	resp = signed(t, http.MethodGet, srv.URL+"/mez-dev?list-type=2&prefix=v/cafe/", "reader", "")
	if got, _ := io.ReadAll(resp.Body); resp.StatusCode != http.StatusOK || !strings.Contains(string(got), "v/cafe/db/one") {
		t.Errorf("the reader listing: status %d, %s", resp.StatusCode, got)
	}

	for name, try := range map[string]func() *http.Response{
		"writing over it": func() *http.Response { return signed(t, http.MethodPut, object, "reader", "something else") },
		"writing beside it": func() *http.Response {
			return signed(t, http.MethodPut, srv.URL+"/mez-dev/v/cafe/db/two", "reader", "x")
		},
		"deleting it": func() *http.Response {
			return signed(t, http.MethodPost, srv.URL+"/mez-dev?delete", "reader", `<Delete><Object><Key>v/cafe/db/one</Key></Object></Delete>`)
		},
	} {
		if resp := try(); resp.StatusCode != http.StatusForbidden {
			t.Errorf("the reader %s: status %d, want 403", name, resp.StatusCode)
		}
	}
	if keys := bucket.Keys(); len(keys) != 1 || keys[0] != "v/cafe/db/one" {
		t.Errorf("after the reader's tries the bucket holds %v", keys)
	}
	resp = signed(t, http.MethodGet, object, "writer", "")
	if got, _ := io.ReadAll(resp.Body); string(got) != "a backup" {
		t.Errorf("what the reader could not change now reads %q", got)
	}

	// A key nobody gave it, and a bucket with no reader at all.
	if resp := signed(t, http.MethodGet, object, "stranger", ""); resp.StatusCode != http.StatusForbidden {
		t.Errorf("a stranger's key: status %d", resp.StatusCode)
	}
	alone := httptest.NewServer(New("mez-dev", "writer"))
	defer alone.Close()
	for _, key := range []string{"reader", ""} {
		if resp := signed(t, http.MethodGet, alone.URL+"/mez-dev?list-type=2", key, ""); resp.StatusCode != http.StatusForbidden {
			t.Errorf("a bucket with no reader, asked with key %q: status %d", key, resp.StatusCode)
		}
	}
	if bucket.Count("PUT object") != 3 || bucket.CountPrefix("PUT", "v/cafe/") != 3 {
		t.Errorf("counted %d writes asked for, %d under the venue's folder; want 3 of each", bucket.Count("PUT object"), bucket.CountPrefix("PUT", "v/cafe/"))
	}
}
