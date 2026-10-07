package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ManagerOptions configure a Manager. DataDir and DBPath are required.
type ManagerOptions struct {
	// DataDir is where backup.json is kept.
	DataDir string
	// DBPath is the database being backed up.
	DBPath string
	// PhotosDir is the menu's photos folder. Empty copies none.
	PhotosDir string
	// Log receives the backup's lines. Nil discards them.
	Log *slog.Logger
	// ProbeEvery is how often the destination is looked at. Zero is the
	// destination's own pace.
	ProbeEvery time.Duration
}

// Manager is the venue's backup for as long as the server runs: the
// destination in backup.json, the Engine copying to it, and the changing of
// one for another when the owner saves a new destination.
//
// A new destination takes effect at once where that is safe, and at the
// next start where it is not (ADR-276). Stopping Litestream closes its
// handle on the database file, and on every system but Windows closing any
// handle on a file drops every lock the process holds on it: the server
// would go on writing a database it no longer has locked, and the next
// `mez pair` at that machine could take the log out from under it. So off
// Windows a running backup is never stopped while the server runs. A
// backup that is off has nothing to stop, and starts at once anywhere.
type Manager struct {
	opts ManagerOptions
	// swapsLive says whether a running backup may be stopped and another
	// started under the open database.
	swapsLive bool

	// saving is held across the whole of Set, so two saves cannot cross:
	// a channel of one and not a mutex, so that a shutdown waiting on a
	// save that is waiting on a dead share can stop waiting.
	saving chan struct{}
	// lastAgain is when a backup was last started again for a destination
	// that had lost what it had.
	lastAgain time.Time

	mu sync.RWMutex
	// saved is what backup.json holds; running is what the engine was
	// started with. They differ while a restart is owed.
	saved, running Config
	engine         *Engine
	started        bool
	closed         bool
}

// NewManager reads the destination kept in the data directory. It starts
// nothing. A file that cannot be read as a destination is an error: the
// owner who wrote it believes the venue is being backed up.
func NewManager(opts ManagerOptions) (*Manager, error) {
	if opts.DataDir == "" || opts.DBPath == "" {
		return nil, errors.New("backup: a data directory and a database path are required")
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	cfg, err := LoadConfig(opts.DataDir)
	if err != nil {
		return nil, err
	}
	return &Manager{opts: opts, swapsLive: runtime.GOOS == "windows", saved: cfg, running: cfg, saving: make(chan struct{}, 1)}, nil
}

// Start begins backing up to the saved destination. Call it once the
// database is migrated; see Engine for why Close comes after the
// database's own.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("backup: already started")
	}
	engine, err := m.startEngine(ctx, m.saved)
	if err != nil {
		return err
	}
	m.engine, m.running, m.started = engine, m.saved, true
	return nil
}

func (m *Manager) startEngine(ctx context.Context, cfg Config) (*Engine, error) {
	return Start(ctx, Options{
		DBPath: m.opts.DBPath, Config: cfg, Log: m.opts.Log, PhotosDir: m.opts.PhotosDir,
		OnLost: m.startAgain, ProbeEvery: m.opts.ProbeEvery,
	})
}

// startAgain replaces a backup whose destination has lost what was sent
// to it (a new drive in the old one's place, an emptied bucket) with a
// new one to the same destination, which sends the whole database.
//
// Only where a running backup may be stopped. Elsewhere it waits for the
// restart, and Status says so in the meantime.
func (m *Manager) startAgain() {
	if !m.swapsLive {
		return
	}
	ctx := context.Background()
	if !m.lock(ctx) {
		return
	}
	defer m.unlock()

	m.mu.RLock()
	old, cfg, closed := m.engine, m.running, m.closed
	m.mu.RUnlock()
	// Asked of the engine as it stands now: a save may have replaced the
	// one that called while this waited its turn.
	if closed || !old.Lost() {
		return
	}
	// Not over and over. Each time is the whole database sent again, to
	// a bucket somebody pays for, and a destination that loses what it is
	// sent every minute is not one that sending again will mend.
	if !m.lastAgain.IsZero() && time.Since(m.lastAgain) < againAtMost {
		m.opts.Log.Warn("the backup's destination has lost what it was sent again, too soon after the last time; it stays failing until mez is restarted or the destination is saved again")
		return
	}
	m.lastAgain = time.Now()
	if err := old.Close(ctx); err != nil {
		m.opts.Log.Warn("could not stop the backup to start it again", "error", err)
		return
	}
	engine, err := m.startEngine(ctx, cfg)
	if err != nil {
		m.opts.Log.Error("could not start the backup again; it is off until mez is restarted", "error", err)
		engine, _ = m.startEngine(ctx, Config{})
		cfg = Config{}
	}
	m.mu.Lock()
	m.engine, m.running = engine, cfg
	m.mu.Unlock()
}

// Config is the destination as saved, secret and all. It is for the
// service that shows it to decide what of it to show.
func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saved
}

// RestartNeeded says whether the saved destination is waiting on a
// restart of the server to take effect.
func (m *Manager) RestartNeeded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started && m.saved != m.running
}

// Status is how the running backup is doing. While a restart is owed it
// is the old destination's status: that is what is being copied to.
func (m *Manager) Status() Status {
	return m.current().Status()
}

// Running reports whether a backup is running, reaching its destination
// or not. It is what the database asks as it closes: a running backup
// reads the write-ahead log, and the log is then its to fold in.
func (m *Manager) Running() bool {
	if m == nil {
		return false
	}
	return m.current().Running()
}

// BackUpNow copies everything to the running destination now.
func (m *Manager) BackUpNow(ctx context.Context) error {
	return m.current().BackUpNow(ctx)
}

func (m *Manager) current() *Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.engine
}

// Set saves a new destination and, where it is safe, starts copying to
// it. The destination is checked first, by writing to it; one that cannot
// be written to is refused and nothing changes.
//
// It reports whether the server must be restarted for the change to take
// effect.
func (m *Manager) Set(ctx context.Context, cfg Config) (restartNeeded bool, err error) {
	if !m.lock(ctx) {
		return false, fmt.Errorf("backup: saving the destination: %w", ctx.Err())
	}
	defer m.unlock()

	if err := Check(ctx, cfg, m.opts.DataDir); err != nil {
		return false, err
	}
	// From here on it is not the caller's to cancel. A browser that goes
	// away mid-save must not cut short the old backup's last copy, or
	// leave the venue between two destinations.
	ctx = context.WithoutCancel(ctx)

	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return false, ErrStopping
	}

	m.mu.RLock()
	started, running, old := m.started, m.running, m.engine
	m.mu.RUnlock()

	if err := SaveConfig(m.opts.DataDir, cfg); err != nil {
		return false, err
	}
	m.mu.Lock()
	m.saved = cfg
	m.mu.Unlock()
	m.opts.Log.Info("the backup's destination was changed", "backup", cfg)

	switch {
	case !started:
		// Saved before Start, which will read it.
		m.mu.Lock()
		m.running = cfg
		m.mu.Unlock()
		return false, nil
	case cfg == running:
		return false, nil
	case running.Enabled() && !m.swapsLive:
		// A running Litestream that must not be stopped here.
		return true, nil
	}

	// Stop the old before starting the new: two Litestreams on one
	// database would each believe the log was theirs. The old one's last
	// act is to copy what it has, so the destination being left is whole.
	if err := old.Close(ctx); err != nil {
		return false, fmt.Errorf("backup: stopping the old backup: %w", err)
	}
	engine, err := m.startEngine(ctx, cfg)
	if err != nil {
		// Validated and checked, so not expected. The venue is left with
		// no backup running and is told so; the saved file is the new one
		// and the next start will try it again.
		off, _ := m.startEngine(ctx, Config{})
		m.mu.Lock()
		m.engine, m.running = off, Config{}
		m.mu.Unlock()
		return true, fmt.Errorf("backup: starting the new backup: %w", err)
	}
	m.mu.Lock()
	m.engine, m.running = engine, cfg
	m.mu.Unlock()
	return false, nil
}

// Close stops the running backup. Safe on a Manager that was never
// started, and on a nil one.
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	// Said first, so that a save still trying its destination gives up
	// when it comes to the swap.
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	// A save in the middle of its swap is waited for, or the engine it
	// starts would outlive this; but only for as long as the shutdown has.
	if m.lock(ctx) {
		defer m.unlock()
	}
	return m.current().Close(ctx)
}

// lock takes the manager's turn, or gives up when ctx does.
func (m *Manager) lock(ctx context.Context) bool {
	select {
	case m.saving <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Manager) unlock() { <-m.saving }

// againAtMost is how often a backup is started again by itself for a
// destination that has lost what it had.
const againAtMost = 10 * time.Minute

// DataDir is where this Manager's backup.json is.
func (m *Manager) DataDir() string { return filepath.Clean(m.opts.DataDir) }
