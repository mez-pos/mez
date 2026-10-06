package logging

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// fakeSystem records what the system log was sent, by severity.
type fakeSystem struct {
	lines []string
	err   error
}

func (f *fakeSystem) Error(v ...any) error   { return f.add("error", v) }
func (f *fakeSystem) Warning(v ...any) error { return f.add("warning", v) }
func (f *fakeSystem) Info(v ...any) error    { return f.add("info", v) }

func (f *fakeSystem) add(kind string, v []any) error {
	f.lines = append(f.lines, kind+": "+v[0].(string))
	return f.err
}

// TestSystemLogTakesWhatWentWrong: the event log is read by someone whose
// till will not open. It carries the warnings and the errors, in words, and
// none of the lines that only say a request was served.
func TestSystemLogTakesWhatWentWrong(t *testing.T) {
	tests := []struct {
		name string
		emit func(log *slog.Logger)
		want []string
	}{
		{"info stays in the file", func(l *slog.Logger) {
			l.Info("listening", "addr", ":7777")
		}, nil},
		{"a warning is a warning", func(l *slog.Logger) {
			l.Warn("printer offline", "printer", "bar")
		}, []string{"warning: printer offline printer=bar"}},
		{"an error is an error", func(l *slog.Logger) {
			l.Error("startup failed", "error", errors.New("port in use"))
		}, []string{`error: startup failed error="port in use"`}},
		{"a line with nothing attached is its message", func(l *slog.Logger) {
			l.Warn("serving placeholder frontends")
		}, []string{"warning: serving placeholder frontends"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := &fakeSystem{}
			log, closer, err := Setup(testConfig(t), io.Discard, WithSystemLog(system))
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			defer closer.Close()

			tt.emit(log)

			if strings.Join(system.lines, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("system log = %q, want %q", system.lines, tt.want)
			}
		})
	}
}

// TestSystemLogCarriesWhatTheLoggerCarries: a logger made with With, as the
// venue's id is, says so in the event log as it does in the file.
func TestSystemLogCarriesWhatTheLoggerCarries(t *testing.T) {
	cfg := testConfig(t)
	cfg.VenueID = "blr-01"
	system := &fakeSystem{}
	log, closer, err := Setup(cfg, io.Discard, WithSystemLog(system))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()

	log.With("device_id", "dev_1").WithGroup("job").Warn("not printed", "id", 7)

	want := "warning: not printed venue_id=blr-01 device_id=dev_1 job.id=7"
	if len(system.lines) != 1 || system.lines[0] != want {
		t.Errorf("system log = %q, want %q", system.lines, want)
	}
}

// TestSystemLogFailingDoesNotLoseTheFile: an event log that is full or gone
// must not cost the venue its own log.
func TestSystemLogFailingDoesNotLoseTheFile(t *testing.T) {
	cfg := testConfig(t)
	var stdout bytes.Buffer
	log, closer, err := Setup(cfg, &stdout, WithSystemLog(&fakeSystem{err: errors.New("event log is full")}))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Error("drawer did not open")
	closer.Close()

	if !strings.Contains(readLog(t, cfg), "drawer did not open") {
		t.Errorf("log file = %q, want the line in it", readLog(t, cfg))
	}
}

// TestNoSystemLogIsTheUsualCase: run from a terminal there is none, and
// Setup builds what it always did.
func TestNoSystemLogIsTheUsualCase(t *testing.T) {
	var stdout bytes.Buffer
	log, closer, err := Setup(testConfig(t), &stdout, WithSystemLog(nil))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	log.Error("sold")
	closer.Close()

	if !strings.Contains(stdout.String(), "sold") {
		t.Errorf("stdout = %q, want the line", stdout.String())
	}
}
