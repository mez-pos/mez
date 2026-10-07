package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/byjoellobo/mez/internal/config"
)

// Rotation policy for the file log. These are constants rather than settings
// because nobody has ever needed to tune them from a counter, and every
// configuration key is one more thing that can be wrong. 01-architecture.md
// gives log rotation to the scheduler; when that arrives it drives the same
// writer rather than replacing it.
const (
	// FileName is the live log inside Config.LogDir().
	FileName = "mez.log"

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
// means os.Stdout. A Windows service has no console and should pass
// io.Discard; if it does not, or a console goes away under a running
// process, the file is still written: it is the first sink, and a failing
// stdout is swallowed rather than allowed to stop the line.
func Setup(cfg config.Config, stdout io.Writer, opts ...Option) (*slog.Logger, io.Closer, error) {
	if stdout == nil {
		stdout = os.Stdout
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	file, err := newRotatingFile(cfg.LogDir(), FileName, MaxFileBytes, KeepFiles)
	if err != nil {
		return nil, nil, err
	}

	handlerOpts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
		// At debug someone is hunting something, and the file and line are
		// most of the answer. At info they are noise on every line.
		AddSource:   cfg.LogLevel <= slog.LevelDebug,
		ReplaceAttr: utcTime,
	}

	// io.MultiWriter stops at the first writer that fails. The file is the
	// sink a support call reads, so it goes first, and the console is
	// wrapped so that a process started without one (a service, a closed
	// terminal) cannot leave mez.log empty.
	sinks := io.MultiWriter(file, bestEffort{stdout})
	var handler slog.Handler
	if useJSON(cfg.LogFormat, stdout) {
		handler = slog.NewJSONHandler(sinks, handlerOpts)
	} else {
		handler = slog.NewTextHandler(sinks, handlerOpts)
	}

	if o.system != nil {
		handler = newSystemHandler(handler, o.system)
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

// bestEffort is a writer whose failures do not count. It wraps the console:
// a dead stdout is nobody's loss, and reporting it would stop the writers
// after it in an io.MultiWriter, the file among them.
type bestEffort struct{ w io.Writer }

func (b bestEffort) Write(p []byte) (int, error) {
	_, _ = b.w.Write(p)
	return len(p), nil
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

// IsTerminal reports whether w is a console a person is reading, and not a
// file, a pipe or nothing.
func IsTerminal(w io.Writer) bool { return isTerminal(w) }

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
