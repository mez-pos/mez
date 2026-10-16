package mirror

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/db"
)

// backedUp is the venue seed makes, backed up by the venue's own backup
// into a folder: what a mirror is opened on.
func backedUp(t *testing.T) backup.Config {
	t.Helper()
	ctx := context.Background()
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, db.FileName)
	d, err := db.Open(ctx, db.Options{Path: path, LogKeptBy: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	seed(t, d)
	to := backup.Config{Kind: backup.KindFolder, Folder: filepath.Join(t.TempDir(), "backups")}
	photos := filepath.Join(dataDir, "photos")
	if err := os.MkdirAll(photos, 0o750); err != nil {
		t.Fatal(err)
	}
	engine, err := backup.Start(ctx, backup.Options{DBPath: path, Config: to, PhotosDir: photos})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.BackUpNow(ctx); err != nil {
		t.Fatalf("BackUpNow: %v", err)
	}
	if err := d.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatal(err)
	}
	return to
}

// quiet is a log that says nothing.
func quiet() *slog.Logger { return slog.New(slog.DiscardHandler) }

// entries is what is in dir.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(found))
	for _, e := range found {
		names = append(names, e.Name())
	}
	return names
}

// TestAMirrorIsOpenedServedAndPutAway is the mirror as anything that is not
// `mez mirror` holds it: opened on a backup into a directory of the
// caller's, asked through its handler, and closed with nothing left.
func TestAMirrorIsOpenedServedAndPutAway(t *testing.T) {
	ctx := context.Background()
	to := backedUp(t)
	copies, state := t.TempDir(), filepath.Join(t.TempDir(), "state")

	m, err := Open(ctx, OpenOptions{From: to, StateDir: state, CopyDir: copies, Build: api.BuildInfo{Version: "test"}})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := m.Venue(); got != (Venue{PublicID: "ven_MIRROR01", Name: "Cafe"}) {
		t.Errorf("Venue() = %+v", got)
	}
	if got, want := m.StatePath(), filepath.Join(state, "ven_MIRROR01.json"); got != want {
		t.Errorf("StatePath() = %s, want %s", got, want)
	}
	if filepath.Dir(m.Dir()) != copies || !strings.HasPrefix(filepath.Base(m.Dir()), DirPrefix) {
		t.Errorf("the copy is at %s, want a %s directory in %s", m.Dir(), DirPrefix, copies)
	}
	if m.AsOf().IsZero() {
		t.Error("AsOf() is zero for a copy of a backup that was written")
	}
	if !m.Asked().IsZero() {
		t.Errorf("Asked() = %v before anything was asked", m.Asked())
	}

	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get(api.HeaderMirror) == "" {
		t.Errorf("health: %d, %s %q; want 200 and the mirror's time", res.StatusCode, api.HeaderMirror, res.Header.Get(api.HeaderMirror))
	}
	if m.Asked().IsZero() {
		t.Error("Asked() is still zero after a request")
	}
	res, err = http.Post(srv.URL+"/api/v1/menu/categories", "application/json", strings.NewReader(`{"name":"Tea"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusConflict || !strings.Contains(string(body), string(api.CodeMirrorReadOnly)) {
		t.Errorf("a change: %d %s; want 409 %s", res.StatusCode, body, api.CodeMirrorReadOnly)
	}

	// Following that is told never returns at once.
	followed := make(chan struct{})
	go func() { m.Follow(ctx, 0); close(followed) }()
	select {
	case <-followed:
	case <-time.After(5 * time.Second):
		t.Fatal("Follow with no interval did not return")
	}

	// Two closes at once: each answers once the copy is gone.
	closes := make(chan error, 2)
	for range 2 {
		go func() { closes <- m.Close() }()
	}
	for range 2 {
		if err := <-closes; err != nil {
			t.Fatalf("Close: %v", err)
		}
		if left := entries(t, copies); len(left) != 0 {
			t.Errorf("a Close answered with %v still there", left)
		}
	}
	if err := m.Close(); err != nil {
		t.Errorf("a Close after: %v", err)
	}
	// And what arrives after is not answered from a copy put away.
	res, err = http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("a request to a closed mirror: %d, want 503", res.StatusCode)
	}
}

func TestOpenRefuses(t *testing.T) {
	ctx := context.Background()
	to := backedUp(t)
	empty := backup.Config{Kind: backup.KindFolder, Folder: t.TempDir()}
	tests := []struct {
		name string
		o    OpenOptions
		is   error
		says string
	}{
		{"no state directory", OpenOptions{From: to}, nil, "state directory is required"},
		{"a folder with no backup in it", OpenOptions{From: empty, StateDir: t.TempDir()}, backup.ErrNothingToRestore, ""},
		{"a backup of another venue than expected", OpenOptions{From: to, StateDir: t.TempDir(), Venue: "ven_SOMEONE1"}, nil, "ven_SOMEONE1 was expected"},
		{"a backup named and not there", OpenOptions{From: to, StateDir: t.TempDir(), Generation: "20260101T000000Z-nothere"}, backup.ErrNothingToRestore, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copies := t.TempDir()
			tt.o.CopyDir = copies
			m, err := Open(ctx, tt.o)
			if err == nil || m != nil {
				t.Fatalf("Open: %v, %v; want an error and no mirror", m, err)
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want %v", err, tt.is)
			}
			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("err = %v, want it to say %q", err, tt.says)
			}
			if left := entries(t, copies); len(left) != 0 {
				t.Errorf("a mirror that was refused left %v", left)
			}
		})
	}
}

// TestACopyThatCannotBeMadeIsPutAway: a generation that is not there, as
// a line dropped mid-restore would leave it. The error comes back as one,
// and the directory made for the copy is gone. While following, this is
// what leaves the old copy serving and not the mirror dead.
func TestACopyThatCannotBeMadeIsPutAway(t *testing.T) {
	copies := t.TempDir()
	s := &source{from: backedUp(t), copyDir: copies, log: quiet()}

	c, err := s.open(context.Background(), backup.Generation{Name: "20260101T000000Z-nothere"})
	if err == nil || c != nil {
		t.Fatalf("open: %+v, %v; want an error and no copy", c, err)
	}
	if left := entries(t, copies); len(left) != 0 {
		t.Errorf("a copy that could not be made left %v", left)
	}
}

// TestAMirrorStopsWhateverAStalledClientDoes: a request that never
// finishes holds neither a retiring copy nor the mirror's stopping for
// longer than its patience, and the copy is removed all the same.
func TestAMirrorStopsWhateverAStalledClientDoes(t *testing.T) {
	ctx := context.Background()
	to := backedUp(t)
	s := &source{from: to, copyDir: t.TempDir(), log: quiet()}
	generations, err := backup.Generations(ctx, to)
	if err != nil || len(generations) == 0 {
		t.Fatal(err)
	}
	first, err := s.open(ctx, generations[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.open(ctx, generations[0])
	if err != nil {
		t.Fatal(err)
	}
	stalled := make(chan struct{})
	hang := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-stalled })
	first.handler, second.handler = hang, hang
	held := &holder{current: first, log: quiet(), patience: 300 * time.Millisecond}

	// One request stalls on each copy: on the first, then the first is
	// replaced, then on the second.
	serveOne := func() {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		go held.ServeHTTP(nil, req)
	}
	serveOne()
	time.Sleep(50 * time.Millisecond)
	if !held.swap(second) {
		t.Fatal("swap refused")
	}
	serveOne()
	time.Sleep(50 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- held.close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the mirror did not stop behind two stalled requests")
	}
	for _, c := range []*mirrorCopy{first, second} {
		if _, err := os.Stat(c.dir); !os.IsNotExist(err) {
			t.Errorf("the copy at %s is still there (%v)", c.dir, err)
		}
	}
	close(stalled)
	// And once stopping, it takes no newer copy.
	if held.swap(first) {
		t.Error("a stopped mirror took a copy")
	}
}
