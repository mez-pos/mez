package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// testBundles mimics what Vite produces: an index.html that names a
// content-hashed asset, plus the asset.
func testBundles() map[string]fs.FS {
	bundle := func(name string) fs.FS {
		return fstest.MapFS{
			"index.html":            {Data: []byte("<!doctype html><title>" + name + "</title>")},
			"assets/app-ab12cd.js":  {Data: []byte("console.log('" + name + "')")},
			"assets/app-ab12cd.css": {Data: []byte("body{}")},
			"favicon.ico":           {Data: []byte("icon")},
		}
	}
	return map[string]fs.FS{
		"/":            bundle("terminal"),
		"/backoffice/": bundle("backoffice"),
		"/order/":      bundle("guest"),
	}
}

func staticServer(t *testing.T) *Server {
	t.Helper()
	return newTestServer(t, testConfig(), WithStatic(testBundles()))
}

// TestEachBundleAnswersItsMount is the routing the task exists for: three apps,
// three mounts, one binary.
func TestEachBundleAnswersItsMount(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"terminal at the root", "/", "terminal"},
		{"back office", "/backoffice/", "backoffice"},
		{"guest ordering", "/order/", "guest"},
	}

	s := staticServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, s, tt.path)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
			}
			if !strings.Contains(resp.Body.String(), tt.want) {
				t.Errorf("body = %q, want the %s bundle", resp.Body, tt.want)
			}
			if got := resp.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
				t.Errorf("content-type = %q, want HTML", got)
			}
		})
	}
}

// TestDeepRoutesReachTheApp: the router lives in the browser, so a URL a
// cashier bookmarked or reloaded has to arrive at index.html rather than a 404.
func TestDeepRoutesReachTheApp(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"a terminal route", "/tables/12", "terminal"},
		{"a back office route", "/backoffice/reports/z", "backoffice"},
		{"a guest route", "/order/table/7", "guest"},
		{"a mount without its trailing slash", "/order", "guest"},
	}

	s := staticServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, s, tt.path)

			// A subtree mount answers its bare form with a redirect to the
			// slashed one, which is net/http doing the right thing. Follow it
			// rather than asserting a particular 3xx, since which one net/http
			// picks is its business.
			if resp.Code >= 300 && resp.Code < 400 {
				location := resp.Header().Get("Location")
				if location == "" {
					t.Fatalf("status = %d with no Location", resp.Code)
				}
				resp = get(t, s, location)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
			}
			if !strings.Contains(resp.Body.String(), tt.want) {
				t.Errorf("body = %q, want the %s bundle", resp.Body, tt.want)
			}
		})
	}
}

// TestAMissingFileIsNotTheApp is the narrow half of the fallback rule. Serving
// index.html for a missing .js hands the browser HTML with a 200 and a MIME
// error, which reads like a bundler problem and is not one.
func TestAMissingFileIsNotTheApp(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"a missing script", "/assets/gone-99.js"},
		{"a missing stylesheet", "/assets/gone-99.css"},
		{"a missing image", "/logo.png"},
		{"a missing file under a mount", "/backoffice/assets/gone-99.js"},
	}

	s := staticServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, s, tt.path)

			if resp.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d (body %q)", resp.Code, http.StatusNotFound, resp.Body)
			}
			if strings.Contains(resp.Body.String(), "doctype") {
				t.Error("a missing asset was answered with the app")
			}
		})
	}
}

func TestAssetsAreServedWithTheirOwnTypes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"script", "/assets/app-ab12cd.js", "javascript"},
		{"stylesheet", "/assets/app-ab12cd.css", "text/css"},
	}

	s := staticServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, s, tt.path)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
			}
			if got := resp.Header().Get("Content-Type"); !strings.Contains(got, tt.want) {
				t.Errorf("content-type = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCaching: a cached index.html pins a tablet to the build it was opened on,
// because index.html is what names the hashed assets. The hashed files
// themselves cannot go stale, by construction.
func TestCaching(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"the app shell is revalidated", "/", "no-cache"},
		{"a deep route is too", "/tables/12", "no-cache"},
		{"hashed assets are immutable", "/assets/app-ab12cd.js", "immutable"},
	}

	s := staticServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := get(t, s, tt.path).Header().Get("Cache-Control")
			if !strings.Contains(got, tt.want) {
				t.Errorf("cache-control = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

// TestTheAPIIsNotSwallowedByTheSPA is the one that would bite hardest. With a
// bundle mounted at /, an unknown endpoint falls through to it unless /api is
// claimed first, and a terminal parsing JSON would get a page of HTML.
func TestTheAPIIsNotSwallowedByTheSPA(t *testing.T) {
	s := staticServer(t)

	t.Run("health still answers", func(t *testing.T) {
		resp := get(t, s, "/api/v1/health")
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
		if !strings.Contains(resp.Header().Get("Content-Type"), "application/json") {
			t.Errorf("content-type = %q, want JSON", resp.Header().Get("Content-Type"))
		}
	})

	t.Run("an unknown endpoint keeps the envelope", func(t *testing.T) {
		resp := get(t, s, "/api/v1/orders")

		if resp.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
		}
		if strings.Contains(resp.Body.String(), "doctype") {
			t.Fatal("an unknown API path was answered with the terminal app")
		}
		if !strings.Contains(resp.Body.String(), CodeNotFound) {
			t.Errorf("body = %q, want the error envelope", resp.Body)
		}
	})

	t.Run("an unversioned api path too", func(t *testing.T) {
		resp := get(t, s, "/api/whatever")
		if strings.Contains(resp.Body.String(), "doctype") {
			t.Error("an /api path was answered with the terminal app")
		}
	})
}

// TestStaticKeepsTheMiddleware: static responses are requests too, and an
// upgrade that stops logging them or stops giving them ids is a blind spot.
func TestStaticKeepsTheMiddleware(t *testing.T) {
	resp := get(t, staticServer(t), "/")

	if resp.Header().Get(HeaderRequestID) == "" {
		t.Error("no request id on a static response")
	}
}

// TestWithoutBundlesTheRootIsStillTheEnvelope pins the API-only build, which is
// what every other test in this package runs against.
func TestWithoutBundlesTheRootIsStillTheEnvelope(t *testing.T) {
	resp := get(t, newTestServer(t, testConfig()), "/")

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}
	if !strings.Contains(resp.Body.String(), CodeNotFound) {
		t.Errorf("body = %q, want the error envelope", resp.Body)
	}
}

// TestABundleWithoutAnIndexFailsSafely: web.Bundles checks for index.html at
// startup, so reaching this means the filesystem changed under a running
// process. It must be a 500, never a panic.
func TestABundleWithoutAnIndexFailsSafely(t *testing.T) {
	s := newTestServer(t, testConfig(), WithStatic(map[string]fs.FS{
		"/": fstest.MapFS{"assets/app.js": {Data: []byte("{}")}},
	}))

	resp := get(t, s, "/some/route")

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(resp.Body.String(), CodeInternal) {
		t.Errorf("body = %q, want the error envelope", resp.Body)
	}
}

// TestNoPathEscapesTheBundle: the mounts are the only things this binary serves
// off disk, and a POS holds a SQLite database one directory up.
func TestNoPathEscapesTheBundle(t *testing.T) {
	escapes := []string{
		"/../config.toml",
		"/..%2fconfig.toml",
		"/assets/../../config.toml",
		"/backoffice/../../config.toml",
	}

	s := staticServer(t)

	for _, attempt := range escapes {
		t.Run(attempt, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, attempt, nil)
			resp := do(t, s, r)

			if body := resp.Body.String(); strings.Contains(body, "listen_addr") {
				t.Fatalf("path traversal returned configuration: %q", body)
			}
			// Whatever it answers - a redirect, the app, a 404 - it must not be
			// a file from outside the bundle.
			if resp.Code == http.StatusOK && strings.Contains(resp.Body.String(), "[") {
				t.Errorf("suspicious body for %q: %q", attempt, resp.Body)
			}
		})
	}
}
