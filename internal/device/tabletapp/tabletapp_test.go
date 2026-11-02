package tabletapp

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// zip is the start of every APK: a zip's local file header.
const zip = "PK\x03\x04"

func TestReadTakesAPackageAndItsVersion(t *testing.T) {
	apk := []byte(zip + "the rest of the package")
	app, err := Read(apk, []byte(`{"version":"v0.2.0-beta.3","version_code":20036}`))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(apk)
	if app.Version != "v0.2.0-beta.3" || app.Code != 20036 || string(app.APK) != string(apk) || app.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("read %+v", app)
	}
}

func TestReadRefusesWhatIsNotAnApp(t *testing.T) {
	good := `{"version":"v0.2.0","version_code":20099}`
	cases := []struct {
		name string
		apk  string
		meta string
		want string
	}{
		{"no package", "", good, "empty"},
		{"not a zip", "<html>", good, "not a package"},
		{"no app.json", zip, "", "app.json"},
		{"app.json not json", zip, "version: 1", "app.json"},
		{"two objects", zip, good + good, "app.json"},
		{"no version", zip, `{"version_code":20099}`, "version"},
		{"a version with a space", zip, `{"version":"v0.2 beta","version_code":20099}`, "version"},
		{"a version with a quote", zip, `{"version":"v0.2\"","version_code":20099}`, "version"},
		{"a version ending in a new line", zip, `{"version":"v0.2.0\n","version_code":20099}`, "version"},
		{"a version too long", zip, `{"version":"v` + strings.Repeat("1", 64) + `","version_code":20099}`, "version"},
		{"no version code", zip, `{"version":"v0.2.0"}`, "version code"},
		{"a negative version code", zip, `{"version":"v0.2.0","version_code":-1}`, "version code"},
		{"a version code past Android's", zip, `{"version":"v0.2.0","version_code":2100000001}`, "version code"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Read([]byte(c.apk), []byte(c.meta))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Read: %v, want an error naming %q", err, c.want)
			}
		})
	}
}

// TestReadLetsAFieldItDoesNotKnowBe: a build that writes more into
// app.json than this server reads still carries its app.
func TestReadLetsAFieldItDoesNotKnowBe(t *testing.T) {
	app, err := Read([]byte(zip+"x"), []byte(`{"version":"v0.2.0","version_code":20099,"built_at":"2026-11-02"}`))
	if err != nil || app.Code != 20099 {
		t.Errorf("Read: %+v, %v", app, err)
	}
}
