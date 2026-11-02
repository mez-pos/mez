// Package tabletapp is the mez app for Android tablets that this server
// carries, to serve to the venue's tablets from the venue's own network
// (ADR-421). The release builds the app, signs it and embeds it under the
// build tag tabletapp; a build without the tag carries none, and the server
// says so.
//
// Imports: nothing of mez's.
package tabletapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

// App is the tablet app as built into this binary.
type App struct {
	// Version is the app's version name: the version of mez it was built
	// with, as `git describe` writes it.
	Version string
	// Code is Android's version code, higher for each release (ADR-421).
	Code int64
	// APK is the signed package.
	APK []byte
	// SHA256 is the package's digest, in hex.
	SHA256 string
}

// meta is app.json, written beside the package by the build that made it.
type meta struct {
	Version string `json:"version"`
	Code    int64  `json:"version_code"`
}

// maxCode is the largest version code Android takes.
const maxCode = 2_100_000_000

// zipHeader opens every APK, which is a zip.
var zipHeader = []byte("PK\x03\x04")

// versionPattern is what a version name may be: what `git describe` writes
// of a tag, and nothing that needs quoting in a file name or a header.
var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)

// Read makes an App of a package and its app.json, refusing anything that
// is not one: a build that put the wrong file in place is found here, at the
// server's start, and not on a tablet. The App keeps apk itself, not a copy:
// a package of megabytes is held once, where it is in the binary.
func Read(apk, metaJSON []byte) (App, error) {
	if len(apk) == 0 {
		return App{}, errors.New("tablet app: the package is empty")
	}
	if !bytes.HasPrefix(apk, zipHeader) {
		return App{}, errors.New("tablet app: the file is not a package (an APK is a zip)")
	}
	m, err := readMeta(metaJSON)
	if err != nil {
		return App{}, err
	}
	if !versionPattern.MatchString(m.Version) {
		return App{}, fmt.Errorf("tablet app: app.json's version %q is not a version", m.Version)
	}
	if m.Code < 1 || m.Code > maxCode {
		return App{}, fmt.Errorf("tablet app: app.json's version code %d is not one Android takes", m.Code)
	}
	sum := sha256.Sum256(apk)
	return App{
		Version: m.Version,
		Code:    m.Code,
		APK:     apk,
		SHA256:  hex.EncodeToString(sum[:]),
	}, nil
}

// readMeta decodes app.json: one object. A field it does not know is let
// be, so that a build adding one does not leave a venue with no app; the
// two it needs are checked by Read.
func readMeta(metaJSON []byte) (meta, error) {
	dec := json.NewDecoder(bytes.NewReader(metaJSON))
	var m meta
	if err := dec.Decode(&m); err != nil {
		return meta{}, fmt.Errorf("tablet app: reading app.json: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return meta{}, errors.New("tablet app: app.json holds more than one object")
	}
	return m, nil
}
