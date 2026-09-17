// Command nofiltr is the Nofiltr POS server. One binary on the venue's own
// computer: HTTP API, the three SPA bundles, the WebSocket hub, the print queue
// and the scheduler.
//
// It must boot on a bare machine with no external services and keep selling
// with no internet connection.
//
// This is a placeholder. Phase 01 builds the real entrypoint: config, logging,
// graceful shutdown and the embedded bundles.
package main

import (
	"log/slog"
	"os"
)

// version is stamped at build time with -ldflags "-X main.version=...".
// See the Makefile.
var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	slog.Info("nofiltr", "version", version, "status", "not implemented, arrives in phase 01")
}
