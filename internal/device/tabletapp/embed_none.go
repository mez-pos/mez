//go:build !tabletapp

package tabletapp

// Embedded is the app built into this binary: none, in a build without the
// tag tabletapp, which is every build but a release's (ADR-421).
func Embedded() (App, bool, error) {
	return App{}, false, nil
}
