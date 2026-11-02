package main

import (
	"log/slog"

	"github.com/mez-pos/mez/internal/device/tabletapp"
)

// carriedApp is the tablet app this binary carries for the venue's tablets,
// or nil, said at the start either way (ADR-421). One the build put in
// wrongly is said loudly and left out: the till starts without it, and the
// Devices page offers no app rather than a broken one.
func carriedApp(log *slog.Logger) *tabletapp.App {
	app, ok, err := tabletapp.Embedded()
	switch {
	case err != nil:
		log.Error("the tablet app in this build cannot be read; serving none", "error", err)
		return nil
	case !ok:
		log.Info("built without the tablet app", "hint", "a release carries it (ADR-421)")
		return nil
	}
	log.Info("carrying the tablet app", "version", app.Version, "version_code", app.Code, "bytes", len(app.APK))
	return &app
}
