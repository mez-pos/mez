// Command relay is the optional cloud relay that lets a guest's phone reach a
// venue server which has no public address.
//
// It stores nothing. A venue dials wss://relay/agent with a venue key, the
// relay maps venue_id to that open socket, and a guest request is forwarded
// down it and streamed back.
//
// Optional is the important word. A self-hoster can run their own relay, point
// cloudflared at the machine, or skip guest ordering entirely, and the POS is
// fully functional either way. See ADR-008.
//
// This is a placeholder. Phase 11 builds the real relay.
package main

import (
	"log/slog"
	"os"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	slog.Info("relay", "version", version, "status", "not implemented, arrives in phase 11")
}
