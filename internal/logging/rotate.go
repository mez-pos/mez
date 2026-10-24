package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// rotatingFile is an io.WriteCloser that keeps a log from growing without
// bound: when a write would take the file past maxBytes it is renamed to
// name.1, the previous name.1 becomes name.2, and the oldest beyond keep is
// deleted.
//
// Numbered suffixes rather than timestamps, because the question a support call
// actually asks is "what happened just before this" and .1 answers it without
// anyone parsing a filename.
//
// It is hand-written for the same reason as the TOML reader: a rotating file is
// about eighty lines of standard library, and every dependency is a thing a
// cafe owner's antivirus can object to. See ADR-021.
// rotateRetryAfter is how long a failed rotation waits before another try.
const rotateRetryAfter = time.Minute

type rotatingFile struct {
	dir      string
	name     string
	maxBytes int64
	keep     int

	mu   sync.Mutex
	file *os.File
	size int64
	// retryRotateAt is when a failed rotation may be tried again. Every
	// line meanwhile is written to the oversized file: a locked or a full
	// disk does not mend in the time between two lines, and trying on each
	// holds the log's lock through a close, a rename and an open.
	retryRotateAt time.Time
	// closed is set by Close. A nil file with closed false is a handle lost
	// to a rotation whose reopen failed, and Write tries again.
	closed bool
}

func newRotatingFile(dir, name string, maxBytes int64, keep int) (*rotatingFile, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("rotating log %s: maxBytes must be positive, got %d", name, maxBytes)
	}
	if keep < 1 {
		return nil, fmt.Errorf("rotating log %s: keep must be at least 1, got %d", name, keep)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", dir, err)
	}

	r := &rotatingFile{dir: dir, name: name, maxBytes: maxBytes, keep: keep}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

// path is the live log. path(1) is the most recent rotation.
func (r *rotatingFile) path(n int) string {
	if n == 0 {
		return filepath.Join(r.dir, r.name)
	}
	return filepath.Join(r.dir, fmt.Sprintf("%s.%d", r.name, n))
}

// open attaches to the live log, appending to whatever is already there. A
// restart continues the file rather than truncating it: the lines from just
// before a crash are the ones worth having.
func (r *rotatingFile) open() error {
	f, err := os.OpenFile(r.path(0), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("opening log %s: %w", r.path(0), err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("sizing log %s: %w", r.path(0), err)
	}
	r.file, r.size = f, info.Size()
	return nil
}

func (r *rotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, errors.New("write to a closed log file")
	}
	// A rotation that could not reopen the file left nothing to write to.
	// Trying once more here is what keeps a full disk at 02:00 from
	// silencing the log for the rest of the day.
	if r.file == nil {
		if err := r.open(); err != nil {
			return 0, err
		}
	}

	// The size guard is what stops a single line longer than maxBytes from
	// rotating forever: an empty file always accepts the write.
	rotateErr := error(nil)
	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes && !time.Now().Before(r.retryRotateAt) {
		if rotateErr = r.rotate(); rotateErr != nil {
			r.retryRotateAt = time.Now().Add(rotateRetryAfter)
		}
	}

	n, err := r.file.Write(p)
	r.size += int64(n)
	if err != nil {
		return n, err
	}
	// A failed rotation is reported but never costs the caller its line. slog
	// discards handler errors, so losing the write here would lose it silently.
	return n, rotateErr
}

// rotate closes the live log, shifts the numbered ones along, and reopens.
//
// The file is closed first because Windows will not always rename a handle that
// is still open, and a rotation that works on the developer's Linux box and
// fails on the venue's counter is the failure this project keeps trying to
// avoid.
func (r *rotatingFile) rotate() error {
	if err := r.file.Close(); err != nil {
		// The handle is gone either way. Without a reopen here every later
		// line would fail with "closed log file" for the life of the
		// process, which is the one outcome rotation must never have.
		r.file = nil
		return r.reopenAfter(fmt.Errorf("closing log for rotation: %w", err))
	}
	r.file = nil

	// Drop the oldest, then walk backwards so nothing is overwritten.
	if err := remove(r.path(r.keep)); err != nil {
		return r.reopenAfter(err)
	}
	for n := r.keep - 1; n >= 1; n-- {
		if err := rename(r.path(n), r.path(n+1)); err != nil {
			return r.reopenAfter(err)
		}
	}
	if err := rename(r.path(0), r.path(1)); err != nil {
		return r.reopenAfter(err)
	}
	return r.open()
}

// reopenAfter puts the writer back into a usable state after a failed
// rotation. Logging has to survive a full disk or a locked file: the venue is
// mid-service and losing every subsequent line is worse than an oversized log.
func (r *rotatingFile) reopenAfter(cause error) error {
	if err := r.open(); err != nil {
		return fmt.Errorf("rotating log: %w (and reopening failed: %v)", cause, err)
	}
	return fmt.Errorf("rotating log: %w", cause)
}

func (r *rotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	if err != nil {
		return fmt.Errorf("closing log %s: %w", r.path(0), err)
	}
	return nil
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

func rename(from, to string) error {
	if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("renaming %s to %s: %w", from, to, err)
	}
	return nil
}
