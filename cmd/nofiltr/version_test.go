package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuild(t *testing.T) {
	const sha = "9f1c2b3d4e5f60718293a4b5c6d7e8f90a1b2c3d"

	stamps := func(settings ...[2]string) []debug.BuildSetting {
		out := make([]debug.BuildSetting, 0, len(settings))
		for _, s := range settings {
			out = append(out, debug.BuildSetting{Key: s[0], Value: s[1]})
		}
		return out
	}

	tests := []struct {
		name       string
		version    string
		commit     string
		date       string
		settings   []debug.BuildSetting
		wantVer    string
		wantSHA    string
		wantDate   string
		wantWhyNot string
	}{
		{
			name:     "the Makefile stamped everything",
			version:  "v1.2.0",
			commit:   sha,
			date:     "2026-09-17T10:00:00+05:30",
			wantVer:  "v1.2.0",
			wantSHA:  sha,
			wantDate: "2026-09-17T10:00:00+05:30",
		},
		{
			name:    "go run, so the toolchain's stamps answer instead",
			version: "dev",
			settings: stamps(
				[2]string{"vcs.revision", sha},
				[2]string{"vcs.time", "2026-09-17T04:30:00Z"},
				[2]string{"vcs.modified", "false"},
			),
			wantVer:    "dev",
			wantSHA:    sha,
			wantDate:   "2026-09-17T04:30:00Z",
			wantWhyNot: "without the fallback a developer's health endpoint cannot say what it is running",
		},
		{
			name:    "a dirty tree says so",
			version: "dev",
			settings: stamps(
				[2]string{"vcs.revision", sha},
				[2]string{"vcs.modified", "true"},
			),
			wantVer: "dev-dirty",
			wantSHA: sha,
		},
		{
			name:    "a tagged build from a dirty tree keeps its version",
			version: "v1.2.0-dirty",
			commit:  sha,
			date:    "2026-09-17T10:00:00+05:30",
			settings: stamps(
				[2]string{"vcs.modified", "true"},
			),
			wantVer:  "v1.2.0-dirty",
			wantSHA:  sha,
			wantDate: "2026-09-17T10:00:00+05:30",
		},
		{
			name:    "git was missing at build time",
			version: "dev",
			commit:  unknown,
			date:    unknown,
			wantVer: "dev",
			// Empty, so health omits the fields rather than reporting the word
			// "unknown" as though it were a commit.
			wantSHA:  "",
			wantDate: "",
		},
		{
			name:    "nothing anywhere",
			wantVer: "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBuild(tt.version, tt.commit, tt.date, tt.settings)

			if got.Version != tt.wantVer {
				t.Errorf("version = %q, want %q. %s", got.Version, tt.wantVer, tt.wantWhyNot)
			}
			if got.Commit != tt.wantSHA {
				t.Errorf("commit = %q, want %q", got.Commit, tt.wantSHA)
			}
			if got.BuildDate != tt.wantDate {
				t.Errorf("build_date = %q, want %q", got.BuildDate, tt.wantDate)
			}
		})
	}
}

// TestBuildInfoIsAlwaysUsable: whatever it was built with, the binary has to be
// able to answer health. A version of "" would render as an empty field on the
// one endpoint a support call starts from.
func TestBuildInfoIsAlwaysUsable(t *testing.T) {
	if got := buildInfo(); got.Version == "" {
		t.Error("version is empty")
	}
}
