package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/byjoellobo/nofiltr-pos/internal/config"
)

// Rotation policy for the file log. These are constants rather than settings
// because nobody has ever needed to tune them from a counter, and every
// configuration key is one more thing that can be wrong. 01-architecture.md
// gives log rotation to the scheduler; when that arrives it drives the same
// writer rather than replacing it.
const (
	// FileName is the live log inside Config.LogDir().
	FileName = "nofiltr.log"

	// MaxFileBytes is the size at which the log rotates. Eight megabytes is
	// roughly a busy week, and small enough to attach to a support mail.
	MaxFileBytes = 8 << 20

	// KeepFiles is how many rotations survive, so the worst case on disk is
	// MaxFileBytes * (KeepFiles + 1).
	KeepFiles = 5
)

// Setup builds the process logger: structured, levelled, written to stdout and
// to a rotating file under cfg.LogDir() at the same time.
//
// Both sinks share one handler. A support engineer reading the file and a
// developer watching the terminal should be looking at the same lines, and one
// handler is also one format decision rather than two that can drift.
//
// The returned Closer flushes and closes the file. stdout may be nil, which
// means os.Stdout.
func Setup(cfg config.Config, stdout io.Writer) (*slog.Logger, io.Closer, error) {
	if stdout == nil {
		stdout = os.Stdout
	}

	file, err := newRotatingFile(cfg.LogDir(), FileName, MaxFileBytes, KeepFiles)
	if err != nil {
		return nil, nil, err
	}

	opts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
		// At debug someone is hunting something, and the file and line are
		// most of the answer. At info they are noise on every line.
		AddSource:   cfg.LogLevel <= slog.LevelDebug,
		ReplaceAttr: utcTime,
	}

	var handler slog.Handler
	if useJSON(cfg.LogFormat, stdout) {
		handler = slog.NewJSONHandler(io.MultiWriter(stdout, file), opts)
	} else {
		handler = slog.NewTextHandler(io.MultiWriter(stdout, file), opts)
	}

	log := slog.New(handler)
	if cfg.VenueID != "" {
		// Every line from this venue carries its id, so logs from two sites in
		// one support thread stay separable. 03-conventions.md asks for it.
		log = log.With("venue_id", cfg.VenueID)
	}

	// Anything reaching for the package-level slog functions, including code
	// this project did not write, lands in the same place.
	slog.SetDefault(log)

	return log, file, nil
}

// useJSON resolves config.LogFormatAuto against the writer.
//
// Auto means text when a person is watching a terminal and JSON when something
// is collecting it. Getting it wrong is never worse than inconvenient, which is
// why it may be guessed at all; log_format overrides it either way.
func useJSON(format string, w io.Writer) bool {
	switch format {
	case config.LogFormatJSON:
		return true
	case config.LogFormatText:
		return false
	default:
		return !isTerminal(w)
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// utcTime writes timestamps in UTC.
//
// Logs are correlated with rows in the database, which 03-conventions.md stores
// in UTC, and a venue that keeps trading across a daylight-saving change should
// not produce an hour of ambiguous log lines. The venue's own timezone is for
// rendering, not for recording.
func utcTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		a.Value = slog.TimeValue(a.Value.Time().UTC())
	}
	return a
}

// Bootstrap is the logger for the window before Setup runs: configuration has
// not been read, so there is no level, no format and nowhere on disk to write.
// It exists so that a failure during startup is still a structured line on
// stderr rather than a bare print. 03-conventions.md bans fmt.Println, and a
// message the operator sees at a black terminal is not the place to start.
func Bootstrap() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: utcTime,
	}))
}
