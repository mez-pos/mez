package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/config"
)

// TestSetupWritesToBothSinks: the developer watching the terminal and the
// support engineer reading the file have to be looking at the same line.
func TestSetupWritesToBothSinks(t *testing.T) {
	cfg := testConfig(t)
	var stdout bytes.Buffer

	log, closer, err := Setup(cfg, &stdout)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Info("sold", "order_id", "A-17")
	if err := closer.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	onDisk := readLog(t, cfg)
	if !strings.Contains(stdout.String(), "sold") {
		t.Errorf("stdout = %q, want it to contain the line", stdout.String())
	}
	if !strings.Contains(onDisk, "sold") {
		t.Errorf("log file = %q, want it to contain the line", onDisk)
	}
	if stdout.String() != onDisk {
		t.Errorf("the two sinks disagree:\nstdout: %q\nfile:   %q", stdout.String(), onDisk)
	}
}

func TestSetupFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		json   bool
	}{
		// Auto resolves against the writer, and a buffer is not a terminal, so
		// it takes the collected-by-something branch.
		{"auto off a terminal is json", config.LogFormatAuto, true},
		{"json is json", config.LogFormatJSON, true},
		{"text is text", config.LogFormatText, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.LogFormat = tt.format

			var stdout bytes.Buffer
			log, closer, err := Setup(cfg, &stdout)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			log.Info("sold")
			closer.Close()

			line := strings.TrimSpace(stdout.String())
			isJSON := json.Valid([]byte(line))
			if isJSON != tt.json {
				t.Errorf("line = %q, json = %v, want json = %v", line, isJSON, tt.json)
			}
		})
	}
}

// TestSetupStampsTheVenue: two venues in one support thread have to stay
// separable, and 03-conventions.md asks for venue_id on the lines that have one.
func TestSetupStampsTheVenue(t *testing.T) {
	cfg := testConfig(t)
	cfg.VenueID = "blr-koramangala-01"

	var stdout bytes.Buffer
	log, closer, err := Setup(cfg, &stdout)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Info("sold")
	closer.Close()

	if got := field(t, stdout.String(), "venue_id"); got != "blr-koramangala-01" {
		t.Errorf("venue_id = %q, want %q", got, "blr-koramangala-01")
	}
}

// TestSetupOmitsAnEmptyVenue keeps venue_id="" off every line of a venue that
// has not been provisioned yet.
func TestSetupOmitsAnEmptyVenue(t *testing.T) {
	var stdout bytes.Buffer
	log, closer, err := Setup(testConfig(t), &stdout)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Info("sold")
	closer.Close()

	if strings.Contains(stdout.String(), "venue_id") {
		t.Errorf("line = %q, want no venue_id key", stdout.String())
	}
}

// TestSetupTimestampsInUTC: log lines are read next to database rows, which
// 03-conventions.md stores in UTC. A venue trading across a daylight-saving
// change must not produce an hour of ambiguous timestamps.
func TestSetupTimestampsInUTC(t *testing.T) {
	cfg := testConfig(t)
	var stdout bytes.Buffer

	log, closer, err := Setup(cfg, &stdout)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Info("sold")
	closer.Close()

	stamp := field(t, stdout.String(), slog.TimeKey)
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatalf("parsing %q: %v", stamp, err)
	}
	if _, offset := at.Zone(); offset != 0 {
		t.Errorf("time = %q, want a UTC offset of 0, got %d", stamp, offset)
	}
}

// TestSetupHonoursTheLevel: a venue set to warn should not be paying for debug
// lines on disk.
func TestSetupHonoursTheLevel(t *testing.T) {
	cfg := testConfig(t)
	cfg.LogLevel = slog.LevelWarn

	var stdout bytes.Buffer
	log, closer, err := Setup(cfg, &stdout)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Info("routine")
	log.Warn("printer offline")
	closer.Close()

	if strings.Contains(stdout.String(), "routine") {
		t.Errorf("line = %q, want the info line filtered out", stdout.String())
	}
	if !strings.Contains(stdout.String(), "printer offline") {
		t.Errorf("line = %q, want the warning kept", stdout.String())
	}
}

// TestSetupAddsSourceOnlyAtDebug: at debug someone is hunting something and the
// file and line are most of the answer; at info they are noise on every line.
func TestSetupAddsSourceOnlyAtDebug(t *testing.T) {
	for _, tt := range []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{"debug carries source", slog.LevelDebug, true},
		{"info does not", slog.LevelInfo, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.LogLevel = tt.level

			var stdout bytes.Buffer
			log, closer, err := Setup(cfg, &stdout)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			log.Log(nil, tt.level, "sold") //nolint:staticcheck // nil ctx is fine here
			closer.Close()

			if got := strings.Contains(stdout.String(), slog.SourceKey); got != tt.want {
				t.Errorf("line = %q, source = %v, want %v", stdout.String(), got, tt.want)
			}
		})
	}
}

// TestSetupCreatesTheLogDirectory: on a first run data_dir does not exist yet,
// and a POS that refuses to start because nobody made a folder is a bad POS.
func TestSetupCreatesTheLogDirectory(t *testing.T) {
	cfg := testConfig(t)

	_, closer, err := Setup(cfg, new(bytes.Buffer))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { closer.Close() })

	if _, err := os.Stat(filepath.Join(cfg.LogDir(), FileName)); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

// TestSetupReportsAnUnusableLogDirectory: a data_dir the operator cannot write
// to has to say so at startup rather than silently logging nowhere.
func TestSetupReportsAnUnusableLogDirectory(t *testing.T) {
	// A file where the directory should be: the one way to make MkdirAll fail
	// that behaves the same on Windows and on Unix.
	blocked := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("writing blocker: %v", err)
	}

	cfg := testConfig(t)
	cfg.DataDir = blocked

	if _, _, err := Setup(cfg, new(bytes.Buffer)); err == nil {
		t.Fatal("Setup succeeded with an unusable data_dir, want an error")
	}
}

// TestAutoFormatOnARedirectedStdout: a service writing to a file or a pipe is
// being collected by something, so auto has to resolve to JSON there. This is
// the branch that runs in production; the buffer in the other tests only
// approximates it.
func TestAutoFormatOnARedirectedStdout(t *testing.T) {
	redirected, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer redirected.Close()

	if isTerminal(redirected) {
		t.Fatal("a regular file reported itself as a terminal")
	}
	if !useJSON(config.LogFormatAuto, redirected) {
		t.Error("auto chose text for a redirected stdout, want json")
	}
}

// TestBootstrapIsUsableBeforeConfig covers the window where startup fails before
// there is a level, a format or anywhere on disk to write.
func TestBootstrapIsUsableBeforeConfig(t *testing.T) {
	log := Bootstrap()
	if log == nil {
		t.Fatal("Bootstrap returned nil")
	}
	// It writes to stderr, which is not capturable here; what matters is that
	// it does not panic and that a logger exists at all.
	log.Error("startup failed", "error", "nothing is configured yet")
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DataDir:   t.TempDir(),
		LogLevel:  slog.LevelInfo,
		LogFormat: config.LogFormatJSON,
		Timezone:  time.UTC,
	}
}

func readLog(t *testing.T, cfg config.Config) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cfg.LogDir(), FileName))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	return string(b)
}

// field pulls one key out of a JSON log line.
func field(t *testing.T, line, key string) string {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &record); err != nil {
		t.Fatalf("parsing %q: %v", line, err)
	}
	value, ok := record[key]
	if !ok {
		t.Fatalf("line %q has no %q", line, key)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s = %v, want a string", key, value)
	}
	return text
}
