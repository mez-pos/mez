//go:build tabletapp

package tabletapp

import _ "embed" // the package and its app.json, below

// apk and metaJSON are the package and its app.json, which the build that
// made the package wrote into app/ (ADR-421). Held as they are in the
// binary, never copied to the heap. This file only compiles once they are
// there: with none, the directives below fail with "pattern ... no matching
// files found", which is the right error for asking to carry an app that
// was never built.
var (
	//go:embed app/mez.apk
	apk []byte
	//go:embed app/app.json
	metaJSON []byte
)

// Embedded is the app built into this binary. An error is a package or an
// app.json that is not what the build should have put there.
func Embedded() (App, bool, error) {
	app, err := Read(apk, metaJSON)
	if err != nil {
		return App{}, false, err
	}
	return app, true, nil
}
