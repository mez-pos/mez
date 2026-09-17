package config

import (
	"errors"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadPrecedence is the point of this package. Four layers, and the one an
// operator reached for most recently has to win: a flag beats the environment,
// which beats the file, which beats the defaults.
//
// It gets that wrong quietly, and a venue runs on a port nobody chose.
func TestLoadPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		env   map[string]string
		args  []string
		goos  string
		check func(t *testing.T, c Config)
	}{
		{
			name: "bare machine, nothing configured",
			goos: "linux",
			check: func(t *testing.T, c Config) {
				equal(t, "listen_addr", c.ListenAddr, ":7777")
				equal(t, "data_dir", c.DataDir, mustAbs(t, filepath.Join(".", "data")))
				equal(t, "log_level", c.LogLevel, slog.LevelInfo)
				equal(t, "log_format", c.LogFormat, LogFormatAuto)
				equal(t, "venue_id", c.VenueID, "")
				equal(t, "relay_url", c.RelayURL, "")
				equal(t, "relay_enabled", c.RelayEnabled(), false)
				equal(t, "dev_origin", c.DevOrigin, "")
				equal(t, "cors_enabled", c.CORSEnabled(), false)
				equal(t, "timezone", c.Timezone, time.Local)
				equal(t, "log_dir", c.LogDir(), filepath.Join(c.DataDir, "logs"))
			},
		},
		{
			name: "windows puts data in ProgramData",
			goos: "windows",
			env:  map[string]string{"PROGRAMDATA": filepath.Join("C:", "ProgramData")},
			check: func(t *testing.T, c Config) {
				equal(t, "data_dir", c.DataDir,
					mustAbs(t, filepath.Join("C:", "ProgramData", "Nofiltr POS")))
			},
		},
		{
			name: "windows without PROGRAMDATA still lands somewhere sane",
			goos: "windows",
			check: func(t *testing.T, c Config) {
				if !strings.Contains(c.DataDir, "Nofiltr POS") {
					t.Errorf("data_dir = %q, want it to contain %q", c.DataDir, "Nofiltr POS")
				}
			},
		},
		{
			name: "file beats defaults",
			goos: "linux",
			file: strings.Join([]string{
				"# the venue's own settings",
				`listen_addr = ":8080"`,
				`log_level   = "debug"`,
				`venue_id    = "blr-koramangala-01"`,
				`timezone    = "Asia/Kolkata"`,
			}, "\n"),
			check: func(t *testing.T, c Config) {
				equal(t, "listen_addr", c.ListenAddr, ":8080")
				equal(t, "log_level", c.LogLevel, slog.LevelDebug)
				equal(t, "venue_id", c.VenueID, "blr-koramangala-01")
				equal(t, "timezone", c.Timezone.String(), "Asia/Kolkata")
			},
		},
		{
			name: "environment beats file",
			goos: "linux",
			file: `listen_addr = ":8080"` + "\n" + `log_level = "debug"`,
			env: map[string]string{
				"NOFILTR_LISTEN_ADDR": ":9090",
			},
			check: func(t *testing.T, c Config) {
				equal(t, "listen_addr", c.ListenAddr, ":9090")
				equal(t, "log_level", c.LogLevel, slog.LevelDebug) // untouched by env
			},
		},
		{
			name: "flag beats environment and file",
			goos: "linux",
			file: `listen_addr = ":8080"`,
			env:  map[string]string{"NOFILTR_LISTEN_ADDR": ":9090"},
			args: []string{"-listen-addr", ":7000"},
			check: func(t *testing.T, c Config) {
				equal(t, "listen_addr", c.ListenAddr, ":7000")
			},
		},
		{
			name: "an unset flag does not overwrite a lower layer",
			goos: "linux",
			file: `venue_id = "blr-01"`,
			args: []string{"-log-level", "warn"},
			check: func(t *testing.T, c Config) {
				equal(t, "venue_id", c.VenueID, "blr-01")
				equal(t, "log_level", c.LogLevel, slog.LevelWarn)
			},
		},
		{
			name: "an empty environment value turns the relay back off",
			goos: "linux",
			file: `relay_url = "wss://relay.example.com/v1"`,
			env:  map[string]string{"NOFILTR_RELAY_URL": ""},
			check: func(t *testing.T, c Config) {
				equal(t, "relay_url", c.RelayURL, "")
				equal(t, "relay_enabled", c.RelayEnabled(), false)
			},
		},
		{
			name: "a relay in the file is accepted and enabled",
			goos: "linux",
			file: `relay_url = "wss://relay.example.com/v1"`,
			check: func(t *testing.T, c Config) {
				equal(t, "relay_url", c.RelayURL, "wss://relay.example.com/v1")
				equal(t, "relay_enabled", c.RelayEnabled(), true)
			},
		},
		{
			name: "data_dir is made absolute",
			goos: "linux",
			args: []string{"-data-dir", filepath.Join("var", "nofiltr")},
			check: func(t *testing.T, c Config) {
				if !filepath.IsAbs(c.DataDir) {
					t.Errorf("data_dir = %q, want an absolute path", c.DataDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.file != "" {
				writeConfig(t, dir, tt.file)
			}

			cfg, err := load(tt.args, lookupFrom(tt.env), dir, tt.goos)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

// TestLoadRejects covers the other half of the job. A configuration this
// package accepts is one the rest of the binary stops checking, so anything it
// cannot honour has to be an error here, with a message that names the setting.
func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string
		file string
		env  map[string]string
		args []string
		want string // substring the message must contain
	}{
		{
			name: "unknown key in the file",
			file: `listen_add = ":8080"`,
			want: `unknown key "listen_add"`,
		},
		{
			name: "table header",
			file: "[server]\nlisten_addr = \":8080\"",
			want: "tables are not supported",
		},
		{
			name: "duplicate key",
			file: "log_level = \"info\"\nlog_level = \"debug\"",
			want: "duplicate key",
		},
		{
			name: "line without an equals sign",
			file: "log_level debug",
			want: "want key = value",
		},
		{
			name: "array value",
			file: `venue_id = ["a", "b"]`,
			want: "arrays and inline tables are not supported",
		},
		{
			name: "missing value",
			file: "venue_id =",
			want: "missing value",
		},
		{
			name: "explicit config file that is not there",
			args: []string{"-config", filepath.Join("nowhere", "config.toml")},
			want: "reading config",
		},
		{
			name: "address without a port",
			args: []string{"-listen-addr", "localhost"},
			want: "want host:port",
		},
		{
			name: "named port",
			args: []string{"-listen-addr", ":http"},
			want: "port must be a number",
		},
		{
			name: "port out of range",
			args: []string{"-listen-addr", ":70000"},
			want: "port must be a number",
		},
		{
			name: "empty listen address",
			env:  map[string]string{"NOFILTR_LISTEN_ADDR": ""},
			want: "must not be empty",
		},
		{
			name: "unknown log format",
			env:  map[string]string{"NOFILTR_LOG_FORMAT": "xml"},
			want: "want auto, json or text",
		},
		{
			name: "unknown log level",
			env:  map[string]string{"NOFILTR_LOG_LEVEL": "chatty"},
			want: "want debug, info, warn or error",
		},
		{
			name: "relay with a scheme we cannot dial",
			env:  map[string]string{"NOFILTR_RELAY_URL": "ftp://relay.example.com"},
			want: "scheme must be ws, wss, http or https",
		},
		{
			name: "relay without a host",
			env:  map[string]string{"NOFILTR_RELAY_URL": "wss://"},
			want: "missing host",
		},
		{
			name: "dev origin with a path",
			env:  map[string]string{"NOFILTR_DEV_ORIGIN": "http://localhost:5173/app"},
			want: "want an origin with no path",
		},
		{
			name: "dev origin with a trailing slash never matches a browser",
			env:  map[string]string{"NOFILTR_DEV_ORIGIN": "http://localhost:5173/"},
			want: "want an origin with no path",
		},
		{
			name: "dev origin with a scheme a browser will not send",
			env:  map[string]string{"NOFILTR_DEV_ORIGIN": "ws://localhost:5173"},
			want: "scheme must be http or https",
		},
		{
			name: "unknown timezone",
			env:  map[string]string{"NOFILTR_TIMEZONE": "Mars/Olympus"},
			want: "unknown timezone",
		},
		{
			name: "empty data_dir",
			env:  map[string]string{"NOFILTR_DATA_DIR": "  "},
			want: "must not be empty",
		},
		{
			name: "unknown flag",
			args: []string{"-port", "8080"},
			want: "parsing flags",
		},
		{
			name: "stray positional argument",
			args: []string{"start"},
			want: `unexpected argument "start"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.file != "" {
				writeConfig(t, dir, tt.file)
			}

			_, err := load(tt.args, lookupFrom(tt.env), dir, "linux")
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestLoadHelpIsNotAFailure keeps -h distinguishable from a bad flag, so main
// can exit 0 on it rather than printing an error to an operator who only asked
// what the options are.
func TestLoadHelpIsNotAFailure(t *testing.T) {
	_, err := load([]string{"-h"}, lookupFrom(nil), t.TempDir(), "linux")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want it to wrap flag.ErrHelp", err)
	}
}

// TestNamedTimezoneResolves is the test for the embedded tzdata. Without the
// blank import in config.go this passes on a developer machine with Go
// installed and fails on the Windows box under a counter, which is the worst
// possible place to find out. See ADR-017.
func TestNamedTimezoneResolves(t *testing.T) {
	cfg, err := load([]string{"-timezone", "Asia/Kolkata"}, lookupFrom(nil), t.TempDir(), "linux")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// 2026-01-01 12:00 UTC is 17:30 in Kolkata. If tzdata were missing this
	// would either have errored above or silently be UTC.
	got := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).In(cfg.Timezone).Format("15:04")
	equal(t, "kolkata time", got, "17:30")
}

func TestParseTOML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty file",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "comments and blank lines",
			input: "# a note\n\n   # indented note\nlog_level = \"warn\"\n",
			want:  map[string]string{"log_level": "warn"},
		},
		{
			name:  "a hash inside a quoted value is not a comment",
			input: `venue_id = "cafe #2"  # the second one`,
			want:  map[string]string{"venue_id": "cafe #2"},
		},
		{
			name:  "trailing comment on a bare value",
			input: "listen_addr = :7777 # default",
			want:  map[string]string{"listen_addr": ":7777"},
		},
		{
			name:  "escaped quote inside a value",
			input: `venue_id = "the \"blue\" door"`,
			want:  map[string]string{"venue_id": `the "blue" door`},
		},
		{
			name:  "whitespace around the equals sign",
			input: "log_level    =   \"debug\"   ",
			want:  map[string]string{"log_level": "debug"},
		},
		{
			name:  "bare boolean and number stay strings",
			input: "a = true\nb = 42",
			want:  map[string]string{"a": "true", "b": "42"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTOML(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("parseTOML: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				equal(t, k, got[k], v)
			}
		})
	}
}

func TestParseTOMLRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single quotes", `venue_id = 'blr-01'`, "single-quoted"},
		{"unterminated string", `venue_id = "blr-01`, "unterminated string"},
		{"text after a string", `venue_id = "a" b`, "unexpected text after value"},
		{"inline table", `venue_id = {a = 1}`, "arrays and inline tables"},
		{"key with a space", `venue id = "x"`, "only letters, digits"},
		{"empty key", `= "x"`, "empty key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTOML(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("parseTOML succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), "line 1") {
				t.Errorf("error = %q, want it to name the line", err)
			}
		})
	}
}

// TestEveryKeyIsReachable guards the three transforms - file key, environment
// variable, flag - against drifting apart when a setting is added. It walks the
// key list rather than restating it, so a new key is covered the moment it
// exists.
func TestEveryKeyIsReachable(t *testing.T) {
	// Values that pass validation for every key, so the only thing under test
	// is whether the layer was read at all.
	sample := map[string]string{
		keyListenAddr: ":7001",
		keyDataDir:    filepath.Join("tmp", "nofiltr"),
		keyLogLevel:   "warn",
		keyLogFormat:  "text",
		keyVenueID:    "blr-01",
		keyRelayURL:   "wss://relay.example.com/v1",
		keyTimezone:   "Asia/Kolkata",
		keyDevOrigin:  "http://localhost:5173",
	}
	if len(sample) != len(keys) {
		t.Fatalf("sample covers %d keys, the package has %d: add the new one here", len(sample), len(keys))
	}

	for _, key := range keys {
		want := sample[key]

		t.Run(key+"/file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, key+" = "+quote(want))
			cfg, err := load(nil, lookupFrom(nil), dir, "linux")
			assertKey(t, key, want, cfg, err)
		})

		t.Run(key+"/env", func(t *testing.T) {
			env := map[string]string{envPrefix + strings.ToUpper(key): want}
			cfg, err := load(nil, lookupFrom(env), t.TempDir(), "linux")
			assertKey(t, key, want, cfg, err)
		})

		t.Run(key+"/flag", func(t *testing.T) {
			cfg, err := load([]string{"-" + flagName(key), want}, lookupFrom(nil), t.TempDir(), "linux")
			assertKey(t, key, want, cfg, err)
		})
	}
}

func assertKey(t *testing.T, key, want string, cfg Config, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	switch key {
	case keyListenAddr:
		equal(t, key, cfg.ListenAddr, want)
	case keyDataDir:
		equal(t, key, cfg.DataDir, mustAbs(t, want))
	case keyLogLevel:
		equal(t, key, cfg.LogLevel, slog.LevelWarn)
	case keyLogFormat:
		equal(t, key, cfg.LogFormat, want)
	case keyVenueID:
		equal(t, key, cfg.VenueID, want)
	case keyRelayURL:
		equal(t, key, cfg.RelayURL, want)
	case keyTimezone:
		equal(t, key, cfg.Timezone.String(), want)
	case keyDevOrigin:
		equal(t, key, cfg.DevOrigin, want)
	default:
		t.Fatalf("key %q has no assertion: add one", key)
	}
}

func equal[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

func writeConfig(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(contents), 0o600); err != nil {
		t.Fatalf("writing %s: %v", FileName, err)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs %q: %v", path, err)
	}
	return abs
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

// TestUsageDocumentsEverySetting guards the answer to -h.
//
// The flag package's output is captured so an unknown flag becomes an error
// rather than a print from inside a library, and that capture once swallowed
// the usage text entirely: -h printed nothing at all and exited 0. This is the
// test that keeps it printed, and keeps it complete when a setting is added.
func TestUsageDocumentsEverySetting(t *testing.T) {
	text := Usage()

	if text == "" {
		t.Fatal("Usage is empty: -h would print nothing")
	}
	for _, key := range keys {
		flag := "-" + flagName(key)
		if !strings.Contains(text, flag) {
			t.Errorf("usage does not mention %s:\n%s", flag, text)
		}
		if !strings.Contains(text, usage[key]) {
			t.Errorf("usage does not describe %s", flag)
		}
	}
	if !strings.Contains(text, "-config") {
		t.Error("usage does not mention -config, the flag that points at the file")
	}
}
