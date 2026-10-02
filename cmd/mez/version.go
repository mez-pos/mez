package main

import (
	"runtime/debug"
	"strings"

	"github.com/byjoellobo/mez/internal/api"
)

// Stamped at build time with -ldflags "-X main.version=...". See the Makefile.
//
// They are variables rather than constants because the linker can only write to
// variables, and they stay in package main because that is the only place -X
// can address without the import path appearing in the Makefile.
var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

// unknown is what the Makefile writes when git is not available, which happens
// when the source arrives as a tarball rather than as a clone.
const unknown = "unknown"

// buildInfo is what this binary reports about itself at /api/v1/health.
func buildInfo() api.BuildInfo {
	return resolveBuild(version, commit, buildDate, vcsSettings())
}

func vcsSettings() []debug.BuildSetting {
	stamped, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return stamped.Settings
}

// resolveBuild prefers the ldflags and falls back to the VCS stamps the Go
// toolchain embeds on its own.
//
// The fallback covers every `go run`, `go install` and plain `go build`, which
// is the difference between a developer's health endpoint saying "dev" and
// saying which commit they are actually running. It is a separate function from
// buildInfo so that it can be tested: the stamped values cannot be set from
// inside a test binary.
func resolveBuild(version, commit, date string, settings []debug.BuildSetting) api.BuildInfo {
	info := api.BuildInfo{
		Version:   strings.TrimSpace(version),
		Commit:    clean(commit),
		BuildDate: clean(date),
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit != "" && info.BuildDate != "" {
		return info
	}

	var revision, at string
	var modified bool
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			at = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	if info.Commit == "" {
		info.Commit = revision
	}
	if info.BuildDate == "" {
		info.BuildDate = at
	}
	// A binary built from a dirty tree is not the commit it names. The Makefile
	// says so through git describe --dirty; this is the same honesty for the
	// builds that did not go through it.
	if modified && info.Version == "dev" && revision != "" {
		info.Version = "dev-dirty"
	}
	return info
}

// clean drops the placeholder the Makefile writes when git is missing, so that
// health omits the field rather than reporting the word "unknown" as though it
// were a commit.
func clean(value string) string {
	value = strings.TrimSpace(value)
	if value == unknown {
		return ""
	}
	return value
}
