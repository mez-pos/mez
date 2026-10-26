package announce

import (
	"context"
	"testing"
)

func TestTheNameAndItsAddress(t *testing.T) {
	if Hostname != "mez.local" {
		t.Fatalf("hostname = %q", Hostname)
	}
	tests := []struct {
		port int
		want string
	}{
		{7777, "http://mez.local:7777"},
		{80, "http://mez.local"},
		{8080, "http://mez.local:8080"},
	}
	for _, tt := range tests {
		if got := URL(tt.port); got != tt.want {
			t.Errorf("URL(%d) = %q, want %q", tt.port, got, tt.want)
		}
	}
}

// TestStartAndCloseAreSafe: the announcer starts where the machine lets it
// and refuses to start twice; closing one that never started, or twice, is
// nothing. A machine with no multicast is a reported refusal, not a crash.
func TestStartAndCloseAreSafe(t *testing.T) {
	a := New(nil)
	ctx := context.Background()
	if err := a.Close(ctx); err != nil {
		t.Fatalf("closing before starting: %v", err)
	}
	if err := a.Start(ctx); err != nil {
		t.Skipf("this machine refused a multicast socket: %v", err)
	}
	if err := a.Start(ctx); err == nil {
		t.Fatal("started twice")
	}
	if err := a.Close(ctx); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if err := a.Close(ctx); err != nil {
		t.Fatalf("closing twice: %v", err)
	}
}
