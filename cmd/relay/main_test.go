package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/relay"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func envOf(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

// lockedBuffer is a stdout the serving goroutine and the test can share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestIssueKeyPrintsAKeyTheRelayAccepts(t *testing.T) {
	var out bytes.Buffer
	env := envOf(map[string]string{envSecret: testSecret})
	if err := run(context.Background(), []string{"issue-key", "cafe"}, env, &out, rand.Reader); err != nil {
		t.Fatalf("run: %v", err)
	}
	key := strings.TrimSpace(out.String())
	if strings.ContainsAny(key, " \n") || strings.Contains(key, testSecret) {
		t.Fatalf("output is not one key and nothing else: %q", out.String())
	}

	keys, err := relay.NewKeys(testSecret, nil)
	if err != nil {
		t.Fatal(err)
	}
	if venueID, _, err := keys.Verify(key); err != nil || venueID != "cafe" {
		t.Errorf("Verify = %q, %v", venueID, err)
	}
}

func TestRunRefuses(t *testing.T) {
	good := map[string]string{envSecret: testSecret}
	with := func(name, value string) map[string]string {
		return map[string]string{envSecret: testSecret, name: value}
	}
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{"an unknown command", []string{"frobnicate"}, good, "unknown command"},
		{"issue-key with no venue", []string{"issue-key"}, good, "usage"},
		{"issue-key with a bad venue id", []string{"issue-key", "My Cafe"}, good, "venue id"},
		{"issue-key with no secret", []string{"issue-key", "cafe"}, nil, envSecret},
		{"serving with no secret", nil, nil, envSecret},
		{"a short secret", nil, map[string]string{envSecret: "short"}, "at least"},
		{"a rate that is not a number", nil, with(envVenueRPM, "fast"), envVenueRPM},
		{"a zero burst", nil, with(envVenueBurst, "0"), envVenueBurst},
		{"an address rate that is not a number", nil, with(envIPRPM, "lots"), envIPRPM},
		{"a negative address burst", nil, with(envIPBurst, "-1"), envIPBurst},
		{"a trust setting that is not a boolean", nil, with(envTrustProxy, "maybe"), envTrustProxy},
		{"a log level that is not one", nil, with(envLogLevel, "loud"), envLogLevel},
		{"an address that cannot be listened on", nil, with(envListen, "not an address"), "listening"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), tt.args, envOf(tt.env), &out, rand.Reader)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run = %v, want an error mentioning %q", err, tt.want)
			}
			if strings.Contains(err.Error(), testSecret) || strings.Contains(out.String(), testSecret) {
				t.Error("the secret was printed")
			}
		})
	}
}

func TestVersionAndHelp(t *testing.T) {
	for _, tt := range []struct{ arg, want string }{
		{"version", version},
		{"help", "RELAY_SECRET"},
		{"--help", "issue-key"},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{tt.arg}, envOf(nil), &out, rand.Reader); err != nil {
			t.Errorf("%s: %v", tt.arg, err)
		}
		if !strings.Contains(out.String(), tt.want) {
			t.Errorf("%s printed %q, want it to mention %q", tt.arg, out.String(), tt.want)
		}
	}
}

func TestServeAnswersHealthAndStopsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := &lockedBuffer{}
	env := envOf(map[string]string{
		envSecret:     testSecret,
		envListen:     "127.0.0.1:0",
		envVenueRPM:   "90",
		envVenueBurst: "9",
		envIPRPM:      "30",
		envIPBurst:    "3",
		envLogLevel:   "debug",
	})
	listening := make(chan string, 1)
	stopped := make(chan error, 1)
	go func() { stopped <- serve(ctx, env, out, listening) }()

	var addr string
	select {
	case addr = <-listening:
	case err := <-stopped:
		t.Fatalf("serve returned before listening: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve never listened")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	var health struct{ Status, Version string }
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("health body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || health.Status != "ok" || health.Version != version {
		t.Errorf("health = %d %+v", resp.StatusCode, health)
	}

	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("serve stopped with %v, want a clean stop", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("serve did not stop")
	}

	// Structured, and saying what it was started with, but never the secret.
	logs := out.String()
	var first map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(logs, "\n", 2)[0]), &first); err != nil {
		t.Fatalf("the first log line is not JSON: %v\n%s", err, logs)
	}
	if first["ip_rpm"] != float64(30) || first["ip_burst"] != float64(3) {
		t.Errorf("the listening line did not carry the address limits: %v", first)
	}
	if first["venue_rpm"] != float64(90) || first["venue_burst"] != float64(9) {
		t.Errorf("startup line = %v, want the limits it was given", first)
	}
	if stamp, _ := first["time"].(string); !strings.HasSuffix(stamp, "Z") {
		t.Errorf("log time %q is not UTC", stamp)
	}
	if strings.Contains(logs, testSecret) {
		t.Error("the secret was logged")
	}
}
