package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
)

// SystemLevel is the least severe line the operating system's log is sent.
//
// The file log has every request in it. The event log is where a person looks
// when the till will not open, beside every other program's entries, so it
// carries only what went wrong and what the service wrapper says about
// starting and stopping (ADR-263).
const SystemLevel = slog.LevelWarn

// SystemLog is the operating system's own log: the Windows event log when the
// server runs as a service. It is the three methods of the service library's
// logger that are used here, so that this package does not import it.
type SystemLog interface {
	Error(v ...any) error
	Warning(v ...any) error
	Info(v ...any) error
}

// Option changes what Setup builds.
type Option func(*options)

type options struct {
	system SystemLog
}

// WithSystemLog also sends lines at SystemLevel and above to system. A nil
// system is no system log, which is the server run from a terminal.
func WithSystemLog(system SystemLog) Option {
	return func(o *options) { o.system = system }
}

// systemHandler passes every record to next and copies the serious ones to
// the system log.
type systemHandler struct {
	next slog.Handler
	// text renders a record's attributes into out, carrying whatever With
	// and WithGroup have added, as next does.
	text slog.Handler
	out  *systemOut
}

// systemOut is the one buffer and the one system log every handler derived
// from a logger shares.
type systemOut struct {
	mu  sync.Mutex
	buf bytes.Buffer
	log SystemLog
}

func newSystemHandler(next slog.Handler, system SystemLog) slog.Handler {
	out := &systemOut{log: system}
	text := slog.NewTextHandler(&out.buf, &slog.HandlerOptions{
		Level:       SystemLevel,
		ReplaceAttr: attrsOnly,
	})
	return &systemHandler{next: next, text: text, out: out}
}

// attrsOnly drops the time, the level and the message from the rendered
// line: the event log records the first two itself, and the message is put
// in front by hand so that an entry reads as a sentence.
func attrsOnly(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.MessageKey:
		return slog.Attr{}
	}
	return a
}

func (h *systemHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *systemHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.next.Handle(ctx, r)
	if r.Level < SystemLevel {
		return err
	}
	return errors.Join(err, h.out.send(ctx, h.text, r))
}

func (h *systemHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &systemHandler{next: h.next.WithAttrs(attrs), text: h.text.WithAttrs(attrs), out: h.out}
}

func (h *systemHandler) WithGroup(name string) slog.Handler {
	return &systemHandler{next: h.next.WithGroup(name), text: h.text.WithGroup(name), out: h.out}
}

// send writes one record to the system log as "message key=value ...".
func (o *systemOut) send(ctx context.Context, text slog.Handler, r slog.Record) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.buf.Reset()
	if err := text.Handle(ctx, r); err != nil {
		return err
	}
	line := r.Message
	if attrs := strings.TrimSpace(o.buf.String()); attrs != "" {
		line += " " + attrs
	}
	if r.Level >= slog.LevelError {
		return o.log.Error(line)
	}
	return o.log.Warning(line)
}
