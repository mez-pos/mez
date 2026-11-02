//go:build !tabletapp

package tabletapp

import "testing"

func TestABuildWithoutTheTagCarriesNoApp(t *testing.T) {
	app, ok, err := Embedded()
	if err != nil || ok || app.APK != nil {
		t.Errorf("Embedded() = %+v, %v, %v; this build has no tablet app", app, ok, err)
	}
}
