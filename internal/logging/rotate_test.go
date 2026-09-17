package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotationKeepsWhatCameBefore(t *testing.T) {
	dir := t.TempDir()
	r, err := newRotatingFile(dir, "test.log", 32, 3)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	write(t, r, "first line that fills the file\n") // 31 bytes, fits
	write(t, r, "second line\n")                    // pushes past 32, rotates first

	equal(t, "live log", read(t, filepath.Join(dir, "test.log")), "second line\n")
	equal(t, "rotated log", read(t, filepath.Join(dir, "test.log.1")), "first line that fills the file\n")
}

func TestRotationDropsTheOldest(t *testing.T) {
	dir := t.TempDir()
	const keep = 2
	r, err := newRotatingFile(dir, "test.log", 16, keep)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	// Ten rotations through a two-deep history.
	for i := range 10 {
		write(t, r, fmt.Sprintf("line %d padded out\n", i))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != keep+1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("found %d files (%s), want %d: the history is unbounded",
			len(entries), strings.Join(names, ", "), keep+1)
	}
	if _, err := os.Stat(filepath.Join(dir, "test.log.3")); err == nil {
		t.Error("test.log.3 exists, want the oldest rotation removed")
	}
}

// TestAnOversizedLineIsNotLost is the loop guard. A stack trace longer than the
// rotation size must be written once, not trigger a rotation per attempt and
// then get dropped.
func TestAnOversizedLineIsNotLost(t *testing.T) {
	dir := t.TempDir()
	r, err := newRotatingFile(dir, "test.log", 16, 2)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	huge := strings.Repeat("x", 200) + "\n"
	write(t, r, huge)

	equal(t, "live log", read(t, filepath.Join(dir, "test.log")), huge)
}

// TestRestartAppendsRatherThanTruncates: the lines written just before a crash
// are the ones worth having, so reopening must not erase them.
func TestRestartAppendsRatherThanTruncates(t *testing.T) {
	dir := t.TempDir()

	first, err := newRotatingFile(dir, "test.log", 1024, 2)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	write(t, first, "before the crash\n")
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := newRotatingFile(dir, "test.log", 1024, 2)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	t.Cleanup(func() { second.Close() })
	write(t, second, "after the restart\n")

	equal(t, "live log", read(t, filepath.Join(dir, "test.log")),
		"before the crash\nafter the restart\n")
}

// TestSizeSurvivesAReopen: if the reopened file reported size 0 it would accept
// a full maxBytes again and the log would grow past its budget on every restart.
func TestSizeSurvivesAReopen(t *testing.T) {
	dir := t.TempDir()

	first, err := newRotatingFile(dir, "test.log", 24, 2)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	write(t, first, "twenty bytes here!!\n") // 20
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := newRotatingFile(dir, "test.log", 24, 2)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	t.Cleanup(func() { second.Close() })
	write(t, second, "more\n") // 20 + 5 > 24, so this must rotate

	if _, err := os.Stat(filepath.Join(dir, "test.log.1")); err != nil {
		t.Errorf("no rotation happened: the reopened writer forgot the existing size")
	}
}

// TestAFailedRotationKeepsWriting is the path that matters at 19:40 on a busy
// Friday: something on disk is wrong, and the rotation cannot happen. The line
// still has to land and the writer still has to work afterwards, because an
// oversized log is better than a venue with no log at all.
func TestAFailedRotationKeepsWriting(t *testing.T) {
	dir := t.TempDir()
	const keep = 2

	// A non-empty directory where the oldest rotation belongs. os.Remove
	// refuses it on both Windows and Unix, so rotate() fails at its first step.
	blocked := filepath.Join(dir, "test.log.2")
	if err := os.MkdirAll(blocked, 0o750); err != nil {
		t.Fatalf("creating blocker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "occupied"), []byte("x"), 0o600); err != nil {
		t.Fatalf("filling blocker: %v", err)
	}

	r, err := newRotatingFile(dir, "test.log", 16, keep)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	if _, err := r.Write([]byte("first line here\n")); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// This one wants a rotation it cannot have.
	n, err := r.Write([]byte("second line\n"))
	if err == nil {
		t.Error("write returned nil, want the rotation failure reported")
	} else if !strings.Contains(err.Error(), "rotating log") {
		t.Errorf("error = %q, want it to name the rotation", err)
	}
	if n != len("second line\n") {
		t.Errorf("wrote %d bytes of %d: the line was lost to a failed rotation", n, len("second line\n"))
	}

	// And the writer is still usable for everything after it.
	if _, err := r.Write([]byte("third line\n")); err == nil {
		t.Log("rotation recovered on the next write")
	}
	if got := read(t, filepath.Join(dir, "test.log")); !strings.Contains(got, "second line") {
		t.Errorf("live log = %q, want it to contain the line the rotation failed on", got)
	}
}

func TestWriteAfterCloseIsAnError(t *testing.T) {
	dir := t.TempDir()
	r, err := newRotatingFile(dir, "test.log", 1024, 2)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second close: %v, want nil: closing twice is not a failure", err)
	}

	if _, err := r.Write([]byte("late\n")); err == nil {
		t.Error("write to a closed log succeeded, want an error")
	}
}

func TestRotatingFileRejectsNonsenseSettings(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int64
		keep     int
		want     string
	}{
		{"zero size rotates forever", 0, 2, "maxBytes must be positive"},
		{"negative size", -1, 2, "maxBytes must be positive"},
		{"keeping nothing", 1024, 0, "keep must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newRotatingFile(t.TempDir(), "test.log", tt.maxBytes, tt.keep)
			if err == nil {
				t.Fatal("newRotatingFile succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestRotatingFileCreatesItsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data", "logs")

	r, err := newRotatingFile(dir, "test.log", 1024, 2)
	if err != nil {
		t.Fatalf("newRotatingFile: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	if _, err := os.Stat(filepath.Join(dir, "test.log")); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

func write(t *testing.T, w *rotatingFile, s string) {
	t.Helper()
	n, err := w.Write([]byte(s))
	if err != nil {
		t.Fatalf("write %q: %v", s, err)
	}
	if n != len(s) {
		t.Fatalf("wrote %d bytes of %d", n, len(s))
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func equal(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}
