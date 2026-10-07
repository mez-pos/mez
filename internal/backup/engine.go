// Package backup keeps a second copy of the venue's database somewhere
// other than the venue's computer, as it changes.
//
// It is Litestream, as a library inside the one binary (ADR-273): it reads
// the database's write-ahead log and writes what changed to the
// destination, within about a second of each sale. A venue PC that dies
// has lost that second, not the day. With nowhere to send it, the copies
// wait in the data directory and go when the destination comes back:
// a backup that cannot be reached is a status, never a reason not to sell.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
)

// How far back a restore can reach, and how often a whole copy is taken.
//
// A whole copy each night, kept a week: a mistake found on Thursday can be
// undone to Monday. Between whole copies the changes are kept too, so the
// most recent state is always the one restored unless an earlier is asked
// for. A week of nightly copies of a database of a few megabytes is
// nothing a USB stick notices.
const (
	snapshotEvery = 24 * time.Hour
	snapshotsKept = 7 * 24 * time.Hour
)

// How long closing waits for the last copy to go. It is the last thing
// on the shutdown stack and would otherwise take all that is left of ten
// seconds whenever there is no internet, which at closing time is often:
// the S3 client retries for minutes. A healthy last copy is a few
// kilobytes and takes well under a second. A destination the probe has
// already found gone gets one short try.
const (
	closeBudget            = 5 * time.Second
	unreachableCloseBudget = 2 * time.Second
	firstLookGrace         = time.Second
	closeTidy              = time.Second
)

// staleAfter is how long without reaching the destination before the
// backup is said to be failing. Litestream reaches it every second while
// it can, and backs off to minutes while it cannot.
const staleAfter = time.Minute

// State is how the backup is doing, in one word.
type State string

const (
	// StateOff is no destination: nothing is being backed up.
	StateOff State = "off"
	// StateStarting is a backup that has not reached its destination yet
	// and has not had long to.
	StateStarting State = "starting"
	// StateOK is a backup that reached its destination within the minute.
	StateOK State = "ok"
	// StateFailing is a backup that is not reaching its destination. Sales
	// carry on; what is not yet copied waits on this machine.
	StateFailing State = "failing"
)

// Status is what an owner needs to know about the backup.
type Status struct {
	State State
	// LastSyncedAt is when the destination last had everything, UTC. Zero
	// if it has not since the server started.
	LastSyncedAt time.Time
	// Error is what went wrong most recently, while it is still wrong.
	Error string
}

// ErrOff is asking a backup with no destination to do something.
var ErrOff = errors.New("no backup destination is set")

// Clock is the time, so a test can say what it is.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options configure Start. Only DBPath is required.
type Options struct {
	// DBPath is the database file being backed up.
	DBPath string
	// Config is where to. The zero value is nowhere.
	Config Config
	// PhotosDir is the menu's photos folder (ADR-110), copied beside the
	// database. Empty copies none.
	PhotosDir string
	// Log receives Litestream's lines. Nil discards them.
	Log *slog.Logger
	// Clock is for Status. Nil is the system's.
	Clock Clock
	// ProbeEvery is how often the destination is looked at. Zero is the
	// destination's own pace.
	ProbeEvery time.Duration
}

// Engine is a running backup, or the absence of one: with no destination
// every method still answers, so its callers need no second path.
type Engine struct {
	cfg     Config
	clock   Clock
	log     *slog.Logger
	started time.Time

	// Nil when off.
	dest   destination
	store  *litestream.Store
	ldb    *litestream.DB
	errors *errorKeeper

	// The first look at the database, in a goroutine of its own.
	stopLook context.CancelFunc
	lookDone chan struct{}

	// The probe's goroutine, and what it last found: "" is reachable.
	stopProbe   context.CancelFunc
	probeDone   chan struct{}
	unreachable lastError

	// The photos folder's copying, nil with none to copy, and what last
	// went wrong with it.
	photos      *photoSync
	photosError lastError

	closeOnce sync.Once
}

// Start begins backing up the database at opts.DBPath.
//
// It does not wait for the destination, and a destination that cannot be
// reached is not an error here: the engine starts, reports itself failing,
// and keeps trying. The database does not have to exist yet either.
//
// Close it after the database's own connections are closed, not before:
// Litestream's last act is to copy what they wrote last.
func Start(ctx context.Context, opts Options) (*Engine, error) {
	if opts.DBPath == "" {
		return nil, errors.New("backup: no database path given")
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	if err := opts.Config.Validate(filepath.Dir(opts.DBPath)); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}

	e := &Engine{cfg: opts.Config, clock: opts.Clock, log: opts.Log, started: opts.Clock.Now()}
	if !opts.Config.Enabled() {
		return e, nil
	}

	e.errors = &errorKeeper{Handler: opts.Log.Handler(), last: new(lastError)}
	log := slog.New(e.errors).With("subsystem", "backup")

	e.ldb = litestream.NewDB(opts.DBPath)
	e.dest = newDestination(opts.Config, log)
	replica := litestream.NewReplicaWithClient(e.ldb, e.dest.client)
	e.ldb.Replica = replica
	e.dest.attach(replica)

	e.store = litestream.NewStore([]*litestream.DB{e.ldb}, litestream.DefaultCompactionLevels)
	e.store.Logger = log
	e.ldb.SetLogger(log)
	e.store.SnapshotInterval = snapshotEvery
	e.store.SnapshotRetention = snapshotsKept
	// One try on the way down, inside the caller's budget. A venue with
	// no internet at closing time must still shut down promptly, and what
	// did not go is still on disk for the next start.
	e.store.SetShutdownSyncTimeout(0)

	if opts.PhotosDir != "" {
		e.photos = newPhotoSync(opts.PhotosDir, e.dest.photos, opts.Log)
	}

	// Open calls nobody, for a bucket too: Validate has made sure there
	// is nothing Litestream would have to go and ask.
	if err := e.store.Open(ctx); err != nil {
		return nil, fmt.Errorf("backup: starting: %w", err)
	}

	// Litestream first looks at the database a second after starting, and
	// a server stopped sooner than that would close having copied nothing.
	// So it is made to look now, off to one side: with a bucket and no
	// internet the look does not come back for minutes, because Litestream
	// asks the bucket where it had got to before it reads anything.
	lookCtx, stopLook := context.WithCancel(context.WithoutCancel(ctx))
	e.stopLook, e.lookDone = stopLook, make(chan struct{})
	go func() {
		defer close(e.lookDone)
		if _, err := e.store.SyncDB(lookCtx, e.ldb.Path(), false); err != nil && lookCtx.Err() == nil {
			opts.Log.Debug("the backup's first look at the database did not finish", "error", err)
		}
	}()

	if opts.ProbeEvery <= 0 {
		opts.ProbeEvery = e.dest.every
	}
	probeCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	e.stopProbe, e.probeDone = stop, make(chan struct{})
	go e.probeLoop(probeCtx, opts.ProbeEvery)

	opts.Log.Info("backing up the database", "kind", string(opts.Config.Kind), "destination", opts.Config.Destination())
	return e, nil
}

// probeLoop looks at the destination now and then, so that Status can say
// it is gone without touching a disk itself: a share that has stopped
// answering hangs this goroutine and not a page load.
func (e *Engine) probeLoop(ctx context.Context, every time.Duration) {
	defer close(e.probeDone)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		e.probe(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// probe asks whether the destination can be used.
func (e *Engine) probe(ctx context.Context) {
	if err := e.dest.reach(ctx); err != nil {
		// Being stopped mid-question is not the destination's fault.
		if ctx.Err() == nil {
			e.unreachable.set(err.Error())
			// What comes back may not be what went away.
			e.forgetPhotos()
		}
		return
	}
	e.unreachable.clear()
	// The photos go on the probe's clock: a photo added to the menu is at
	// the destination within the minute, and a quiet venue with no new
	// photo asks the destination nothing more than the probe just did.
	if err := e.copyPhotos(ctx, false); err != nil && ctx.Err() == nil {
		e.log.Warn("the menu's photos were not all backed up", "error", err)
	}
}

func (e *Engine) forgetPhotos() {
	if e.photos != nil {
		e.photos.forget()
	}
}

// copyPhotos copies up the photos the destination lacks, and keeps what
// went wrong for Status.
func (e *Engine) copyPhotos(ctx context.Context, fresh bool) error {
	if e.photos == nil {
		return nil
	}
	copied, err := e.photos.sync(ctx, fresh)
	if copied > 0 {
		e.log.Info("backed up the menu's photos", "photos", copied)
	}
	if err != nil {
		if ctx.Err() == nil {
			e.photosError.set("photos: " + err.Error())
		}
		return err
	}
	e.photosError.clear()
	return nil
}

// BackUpNow copies everything written so far to the destination and
// returns when it is there, or with why it is not.
func (e *Engine) BackUpNow(ctx context.Context) error {
	if e == nil || e.store == nil {
		return ErrOff
	}
	// In a goroutine, because Litestream takes its lock before it looks
	// at ctx, and with a bucket and no internet that lock is held for
	// minutes. Whoever pressed the button gets an answer when ctx says.
	done := make(chan error, 1)
	go func() {
		_, err := e.store.SyncDB(ctx, e.ldb.Path(), true)
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		err = fmt.Errorf("the destination did not answer in time: %w", ctx.Err())
	}
	if err != nil {
		e.errors.last.set(err.Error())
		return fmt.Errorf("backup: backing up now: %w", err)
	}
	e.ldb.RecordSuccessfulSync()
	// It has just been written to: whatever was wrong is over, and the
	// owner who asked should not wait for the probe to agree.
	e.errors.last.clear()
	e.unreachable.clear()
	// And the photos, asking the destination what it holds and not
	// trusting memory: the owner who presses this may have just changed
	// the drive.
	if err := e.copyPhotos(ctx, true); err != nil {
		return fmt.Errorf("backup: backing up the photos now: %w", err)
	}
	return nil
}

// Status says how the backup is doing. It touches no disk itself, so it is
// cheap enough to ask on every page load and cannot hang on a dead share.
func (e *Engine) Status() Status {
	if e == nil || e.store == nil {
		return Status{State: StateOff}
	}
	now := e.clock.Now()
	last := e.ldb.LastSuccessfulSyncAt()
	gone, _ := e.unreachable.get()
	// An error stands until the destination is reached after it. Litestream
	// counts a tick with nothing to send as reaching it, so an old error
	// is not left on show once things are moving again.
	reason, at := e.errors.last.get()
	stands := reason != "" && (last.IsZero() || at.After(last))

	photos, _ := e.photosError.get()

	switch {
	case gone != "":
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: gone}
	case stands:
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: reason}
	case photos != "":
		// The sales are safe and the pictures are not: still failing,
		// because "ok" would be read as everything.
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: photos}
	case !last.IsZero() && now.Sub(last) < staleAfter:
		return Status{State: StateOK, LastSyncedAt: last.UTC()}
	case last.IsZero() && now.Sub(e.started) < staleAfter:
		return Status{State: StateStarting}
	default:
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last)}
	}
}

func utcOrZero(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

// Close copies what is left and stops. Safe to call twice, and on an
// engine that is off or nil.
//
// ctx is the shutdown budget. A destination that cannot be reached costs
// one failed try, and what it did not take goes on the next start. The
// error is always nil; the signature is the shutdown stack's.
func (e *Engine) Close(ctx context.Context) error {
	if e == nil || e.store == nil {
		return nil
	}
	e.closeOnce.Do(func() {
		// Asked before the probe is stopped, so that one in flight has
		// not been cut off before it could say.
		gone, _ := e.unreachable.get()
		e.stopProbe()

		// In a goroutine, because Litestream's close does not give up when
		// ctx does: a write stuck on a share that has stopped answering
		// would hold the whole shutdown, and the service manager kills
		// what does not stop.
		// A destination already known to be out of reach is not waited on
		// for the whole of what is left: one short try, and the rest goes
		// on the next start.
		budget := closeBudget
		if gone != "" {
			budget = unreachableCloseBudget
		}
		// The caller's time, plus a moment: when the copy runs out of
		// budget Litestream still has its lock to let go of and its
		// handles to close, and that is not what should be cut off.
		wait, stopWaiting := context.WithTimeout(context.WithoutCancel(ctx), budget+closeTidy)
		defer stopWaiting()
		ctx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			// The first look is given a moment to finish, so that a
			// server stopped as soon as it started still leaves a copy.
			// One still waiting on a bucket is told to stop: it holds
			// Litestream's lock, and nothing below can run until it lets go.
			// With the destination known gone there is nothing to give
			// it a moment for.
			grace := firstLookGrace
			if gone != "" {
				grace = 0
			}
			select {
			case <-e.lookDone:
			case <-time.After(grace):
				e.stopLook()
				<-e.lookDone
			}
			e.stopLook()
			// Not returned. A drive that is unplugged at closing time is
			// the backup failing, which the status already says, and not
			// the server failing to stop: under a service a failed stop is
			// a crash.
			if err := e.store.Close(ctx); err != nil {
				e.log.Warn("the backup stopped without reaching its destination; what is left goes on the next start", "error", err)
			}
			<-e.probeDone
			// The photos last, the database being what cannot be made
			// again, and only to a destination that is there.
			if gone == "" {
				if err := e.copyPhotos(ctx, false); err != nil {
					e.log.Warn("the menu's photos were not all backed up before stopping; the rest go on the next start", "error", err)
				}
			}
		}()
		select {
		case <-done:
		case <-wait.Done():
			e.log.Warn("the backup did not stop in time and is left behind; what it had not copied goes on the next start")
		}
	})
	return nil
}

// lastError is the most recent thing that went wrong, and when.
type lastError struct {
	mu  sync.Mutex
	msg string
	at  time.Time
}

// set stamps with the real time, not the injected clock: it is compared
// with the time Litestream stamps its own successes with.
func (l *lastError) set(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msg, l.at = msg, time.Now()
}

func (l *lastError) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msg, l.at = "", time.Time{}
}

func (l *lastError) get() (string, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.msg, l.at
}

// errorKeeper stands between Litestream and the server's log. It
// remembers the last error Litestream logged, and it keeps Litestream's
// chatter out.
//
// Litestream's replication loop tells nobody but its log when a sync
// fails, and an owner looking at "failing" deserves the reason. It also
// logs a line at Info every second it is healthy, which is 86,000 lines a
// day in a venue's log, so below a warning its lines go on only when the
// server is logging at debug.
type errorKeeper struct {
	slog.Handler
	last *lastError
}

// Enabled hears everything from Info up whatever the server's level, so
// that an error and the recovery after it are both noticed.
func (k *errorKeeper) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo || k.Handler.Enabled(ctx, level)
}

func (k *errorKeeper) Handle(ctx context.Context, r slog.Record) error {
	switch {
	case r.Level >= slog.LevelError:
		msg := r.Message
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "error" {
				msg = a.Value.String()
				return false
			}
			return true
		})
		k.last.set(msg)
	case r.Message == "replica sync recovered":
		k.last.clear()
	}

	chatter := r.Level < slog.LevelWarn && !k.Handler.Enabled(ctx, slog.LevelDebug)
	if chatter || !k.Handler.Enabled(ctx, r.Level) {
		return nil
	}
	return k.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup keep the keeper around the handler they derive:
// the embedded methods alone would hand back the inner one, and every
// logger made with With would stop being heard.
func (k *errorKeeper) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &errorKeeper{Handler: k.Handler.WithAttrs(attrs), last: k.last}
}

func (k *errorKeeper) WithGroup(name string) slog.Handler {
	return &errorKeeper{Handler: k.Handler.WithGroup(name), last: k.last}
}
