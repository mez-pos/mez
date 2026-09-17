package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/api"
	"github.com/byjoellobo/nofiltr-pos/internal/config"
)

// TestServeFinishesInFlightRequests is the test this task exists for. Cancelling
// the context must stop the server accepting without cutting off a request that
// is already running: a cashier who hit Charge as the machine restarted has to
// see that payment complete or fail, never vanish.
func TestServeFinishesInFlightRequests(t *testing.T) {
	listener := listen(t)
	addr := listener.Addr().String()

	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(200 * time.Millisecond) // the slow part of taking a payment
		io.WriteString(w, "charged")
	})

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, quietLogger(), listener, h, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	responded := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/pay")
		if err != nil {
			responded <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		responded <- result{body: string(body), err: err}
	}()

	<-started
	cancel() // the SIGINT, landing mid-request

	got := <-responded
	if got.err != nil {
		t.Fatalf("in-flight request failed: %v", got.err)
	}
	if got.body != "charged" {
		t.Errorf("body = %q, want %q: the request was cut off rather than drained", got.body, "charged")
	}

	if err := <-served; err != nil {
		t.Errorf("serve: %v", err)
	}
}

// TestServeStopsAcceptingAfterCancel: draining is only half of it. The socket
// has to be released, or the restart that follows cannot bind.
func TestServeStopsAcceptingAfterCancel(t *testing.T) {
	listener := listen(t)
	addr := listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, quietLogger(), listener, apiHandler(), time.Second) }()

	// Prove it is up before proving it goes down.
	if _, err := http.Get("http://" + addr + "/"); err != nil {
		t.Fatalf("server was never reachable: %v", err)
	}

	cancel()
	if err := <-served; err != nil {
		t.Fatalf("serve: %v", err)
	}

	if _, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
		t.Error("the port still accepts connections after shutdown")
	}
}

// TestServeReportsADrainThatRanOut keeps a hung request from looking like a
// clean stop. The operator needs to know the process was cut short.
func TestServeReportsADrainThatRanOut(t *testing.T) {
	listener := listen(t)
	addr := listener.Addr().String()

	release := make(chan struct{})
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, quietLogger(), listener, h, 50*time.Millisecond) }()

	go http.Get("http://" + addr + "/stuck") //nolint:errcheck // the response never comes, that is the point
	<-started
	cancel()

	err := <-served
	if err == nil {
		t.Fatal("serve returned nil, want an error naming the drain")
	}
	if !strings.Contains(err.Error(), "draining requests") {
		t.Errorf("error = %q, want it to say the drain ran out", err)
	}
}

// TestServeReportsALostListener: if the socket dies under us, that is a failure
// to report, not a quiet exit.
func TestServeReportsALostListener(t *testing.T) {
	listener := listen(t)
	listener.Close()

	err := serve(context.Background(), quietLogger(), listener, apiHandler(), time.Second)
	if err == nil {
		t.Fatal("serve returned nil on a dead listener, want an error")
	}
	if !strings.Contains(err.Error(), "serving") {
		t.Errorf("error = %q, want it to name the failure", err)
	}
}

// TestTheWiredHandlerServesHealth is the seam that used to assert a 404. It
// exists to catch main and internal/api drifting apart: serve owns the
// lifecycle, api owns the routes, and nothing else checks that they are
// actually joined.
func TestTheWiredHandlerServesHealth(t *testing.T) {
	listener := listen(t)
	addr := listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- serve(ctx, quietLogger(), listener, apiHandler(), time.Second) }()

	resp, err := http.Get("http://" + addr + "/api/v1/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d: the API is not wired into serve", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("content-type = %q, want JSON", got)
	}

	cancel()
	<-served
}

// apiHandler builds the same tree run() does, so these tests exercise what
// ships rather than a stand-in.
func apiHandler() http.Handler {
	cfg := config.Config{ListenAddr: "127.0.0.1:0", DataDir: ".", Timezone: time.UTC}
	return api.New(cfg, quietLogger(), api.BuildInfo{Version: version}).Handler()
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	// Port 0: the operating system picks one, so the tests do not fight each
	// other or whatever is already on :7777.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
}
