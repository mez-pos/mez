package escpos

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// TestSend delivers a stream to a listener standing in for a printer, and
// fails cleanly when nothing is listening.
func TestSend(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- nil
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(conn)
		got <- data
	}()

	want, _ := TestPage(Paper80, time.Now(), "", false)
	if err := Send(context.Background(), ln.Addr().String(), want); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case data := <-got:
		if !bytes.Equal(data, want) {
			t.Errorf("printer got %d bytes, want %d", len(data), len(want))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the printer never got the bytes")
	}

	// A port nobody listens on: the listener is closed and its port freed.
	addr := ln.Addr().String()
	ln.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := Send(ctx, addr, want); err == nil {
		t.Error("Send to a closed port succeeded")
	}
}

func TestWithPort(t *testing.T) {
	tests := map[string]string{
		"192.168.1.50":      "192.168.1.50:9100",
		"192.168.1.50:9101": "192.168.1.50:9101",
		" printer.local ":   "printer.local:9100",
		"fe80::1":           "[fe80::1]:9100",
		"[fe80::1]:9100":    "[fe80::1]:9100",
	}
	for in, want := range tests {
		if got := withPort(in); got != want {
			t.Errorf("withPort(%q) = %q, want %q", in, got, want)
		}
	}
}
