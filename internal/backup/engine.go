// Package backup keeps a second copy of the venue's database somewhere
// other than the venue's computer, as it changes.
//
// It is Litestream, as a library inside the one binary (ADR-273): it reads
// the database's write-ahead log and writes what changed to the
// destination, every half minute unless the venue set another pace
// (ADR-315). A venue PC that dies has lost that half minute, not the day. With nowhere to send it, the copies
// wait in the data directory and go when the destination comes back:
// a backup that cannot be reached is a status, never a reason not to sell.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	closeTidy              = time.Second
)

// State is how the backup is doing, in one word.
type State string

const (
	// StateOff is no destination: nothing is being backed up.
	StateOff State = "off"
	// StateStarting is a backup that has not reached its destination yet
	// and has not had long to.
	StateStarting State = "starting"
	// StateOK is a backup that reached its destination when it last meant to.
	StateOK State = "ok"
	// StateFailing is a backup that is not reaching its destination. Sales
	// carry on; what is not yet copied waits on this machine.
	StateFailing State = "failing"
)

// States is every State, for the API's vocabulary.
func States() []State { return []State{StateOff, StateStarting, StateOK, StateFailing} }

// Status is what an owner needs to know about the backup.
type Status struct {
	State State
	// LastSyncedAt is when the destination last had everything, UTC. Zero
	// if it has not since the server started.
	LastSyncedAt time.Time
	// Error is what went wrong most recently, while it is still wrong.
	Error string
}

// ErrStopping is asking a backup that is being stopped, for the server's
// shutdown or to make way for another, to do something.
var ErrStopping = errors.New("the backup is stopping")

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
	// SyncInterval is how often what changed is sent. Zero is
	// DefaultSyncInterval. It is taken as given: CheckSyncInterval is for
	// whoever reads it from a person.
	SyncInterval time.Duration
	// OnLost is told, once and on a goroutine of its own, that the
	// destination no longer holds what was sent to it and the backup must
	// be started again to send it all. See Engine.Lost.
	OnLost func()
}

// Engine is a running backup, or the absence of one: with no destination
// every method still answers, so its callers need no second path.
type Engine struct {
	cfg      Config
	clock    Clock
	log      *slog.Logger
	started  time.Time
	interval time.Duration

	// Nil or empty when off.
	generation string
	dest       destination
	store      *litestream.Store
	ldb        *litestream.DB
	errors     *errorKeeper

	// Whether Litestream has been started, which waits for the
	// destination to be seen (open), and the context it runs under.
	openMu        sync.Mutex
	opened        bool
	runCtx        context.Context
	litestreamLog *slog.Logger

	// The probe's goroutine, and what it last found: "" is reachable.
	stopProbe   context.CancelFunc
	probeDone   chan struct{}
	unreachable lastError

	// The photos folder's copying, nil with none to copy, and what last
	// went wrong with it.
	photos      *photoSync
	photosError lastError

	// asked is every "back up now" in flight. Close waits for them before
	// it stops Litestream: one that reached Litestream after it had
	// closed would open it again, with nobody left to close it. stopAsked
	// tells them to give up, and stopping refuses new ones.
	askMu     sync.Mutex
	stopping  bool
	asked     sync.WaitGroup
	askCtx    context.Context
	stopAsked context.CancelFunc

	// sent is whether the destination has been seen to hold a whole copy
	// in this generation. Once it has, not finding one is the
	// destination having lost it.
	sent atomic.Bool

	// tidiedAt is when old generations were last looked for, the probe's
	// goroutine's alone, and tidying the looking that is under way.
	tidiedAt time.Time
	tidying  sync.WaitGroup
	// seenOnce is the noting, once, that the destination holds a copy.
	seenOnce sync.Once

	// lostOnce is the saying, once, that the destination has lost what it
	// had, and onLost who is told.
	lostOnce sync.Once
	onLost   func()

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
	if opts.SyncInterval <= 0 {
		opts.SyncInterval = pace.sync
	}

	e := &Engine{cfg: opts.Config, clock: opts.Clock, log: opts.Log, started: opts.Clock.Now(), interval: opts.SyncInterval}
	if !opts.Config.Enabled() {
		return e, nil
	}

	e.errors = &errorKeeper{Handler: opts.Log.Handler(), last: new(lastError), lost: new(atomic.Bool)}
	log := slog.New(e.errors).With("subsystem", "backup")

	e.onLost = opts.OnLost
	e.askCtx, e.stopAsked = context.WithCancel(context.Background())
	e.ldb = litestream.NewDB(opts.DBPath)
	// Before Litestream opens them: what it remembers of one destination
	// is no use for another.
	generation, sent, err := workingFilesFor(e.ldb.MetaPath(), opts.Config, opts.Clock.Now(), opts.Log)
	if err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}
	e.generation = generation
	e.sent.Store(sent)
	e.dest = newDestination(opts.Config, generation, log)
	replica := litestream.NewReplicaWithClient(e.ldb, e.dest.client)
	e.ldb.Replica = replica
	e.dest.attach(replica)
	e.errors.lostIn = e.ldb.MetaPath()

	e.ldb.SetLogger(log)
	e.litestreamLog = log
	// Litestream gives up on the database after a second of waiting for
	// it, and what it does when it gives up as it starts is close its
	// handle (see open). A till can hold the database for longer than
	// that: a day's close, a big import.
	e.ldb.BusyTimeout = litestreamBusyTimeout
	// Both, or neither does anything: Litestream cuts one file each time it
	// looks at the database and sends each file by itself, so sending every
	// half minute what was cut every second is the same thirty requests.
	e.ldb.MonitorInterval = e.interval
	replica.SyncInterval = e.interval

	if opts.PhotosDir != "" {
		e.photos = newPhotoSync(opts.PhotosDir, e.dest.photos, opts.Log)
	}

	// Litestream is not started here. It is started by the first look at
	// the destination that finds it there (see open), which for a folder
	// is a moment from now and for a bucket with no internet is whenever
	// the internet comes back.
	e.runCtx = context.WithoutCancel(ctx)
	there, gone := e.dest.every, pace.probeGone
	if opts.ProbeEvery > 0 {
		there, gone = opts.ProbeEvery, opts.ProbeEvery
	}
	probeCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	e.stopProbe, e.probeDone = stop, make(chan struct{})
	go e.probeLoop(probeCtx, there, gone)

	opts.Log.Info("backing up the database", "kind", string(opts.Config.Kind), "destination", opts.Config.Destination(),
		"every", e.interval.String())
	return e, nil
}

// open starts Litestream on the database. It is called once the
// destination has been seen to be there, and not before (ADR-279).
//
// Litestream, as it starts, asks the destination where it had got to. If
// the destination does not answer it gives up, closes its handle on the
// database file, and tries again a second later, for as long as the
// destination is away. On every system but Windows, closing any handle on
// a file drops every lock the process holds on it: a venue that started
// its day with no internet would have had the server's locks on its own
// database taken away, over and over, with the server none the wiser, and
// the next `mez pair` at that machine free to take the log out from under
// it. A Litestream that is only started when the destination answers
// does not go through that door.
//
// It is also why a server started with no internet captures nothing for
// a bucket until the bucket first answers. That was so before, and was a
// cost; now it is also the point.
func (e *Engine) open(ctx context.Context) error {
	e.openMu.Lock()
	defer e.openMu.Unlock()
	if e.opened {
		return nil
	}

	store := litestream.NewStore([]*litestream.DB{e.ldb}, pace.levels)
	store.SetL0Retention(pace.l0Retention)
	store.L0RetentionCheckInterval = pace.l0RetentionCheck
	store.Logger = e.litestreamLog
	e.ldb.SetLogger(e.litestreamLog)
	store.SnapshotInterval = snapshotEvery
	store.SnapshotRetention = snapshotsKept
	// One try on the way down, inside the caller's budget. A venue with
	// no internet at closing time must still shut down promptly, and what
	// did not go is still on disk for the next start.
	store.SetShutdownSyncTimeout(0)

	// Open itself calls nobody and opens nothing. It is the first look at
	// the database that opens Litestream's handle on it and asks the
	// destination where it had got to; so that look is made here, once,
	// on this engine's own time and not on that of whoever happened to
	// ask, whose patience running out part way through would be a failure
	// like any other.
	if err := store.Open(ctx); err != nil {
		return fmt.Errorf("starting litestream: %w", err)
	}
	lookCtx, cancel := context.WithTimeout(e.runCtx, firstLookTimeout)
	_, err := store.SyncDB(lookCtx, e.ldb.Path(), false)
	cancel()
	if err != nil && !errors.Is(err, litestream.ErrDBNotReady) {
		// It did not start, and if it got as far as opening its handle it
		// has closed it again. Left running it would do that every second.
		// Stopped, it does it once, and is tried again at the next look.
		stopCtx, stop := context.WithTimeout(e.runCtx, firstLookTimeout)
		_ = store.Close(stopCtx)
		stop()
		return fmt.Errorf("litestream's first look at the database: %w", err)
	}
	e.store = store
	e.opened = true
	return nil
}

// firstLookTimeout bounds Litestream's first look at the database, and
// litestreamBusyTimeout how long it waits for the database when a till has
// it.
const (
	firstLookTimeout      = 30 * time.Second
	litestreamBusyTimeout = 10 * time.Second
)

// isOpen reports whether Litestream has been started.
func (e *Engine) isOpen() bool {
	e.openMu.Lock()
	defer e.openMu.Unlock()
	return e.opened
}

// probeLoop looks at the destination now and then, so that Status can say
// it is gone without touching a disk itself: a share that has stopped
// answering hangs this goroutine and not a page load.
//
// It looks every there while the destination answers and every gone while
// it does not: a bucket bills each look it answers and none it does not.
func (e *Engine) probeLoop(ctx context.Context, there, gone time.Duration) {
	defer close(e.probeDone)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		e.probe(ctx)
		next := there
		if was, _ := e.unreachable.get(); was != "" {
			next = gone
		}
		timer.Reset(next)
	}
}

// probe asks whether the destination can be used.
func (e *Engine) probe(ctx context.Context) {
	if e.errors.lost.Load() {
		e.sayLost()
	}
	whole, err := e.dest.reach(ctx)
	if err != nil {
		// Being stopped mid-question is not the destination's fault.
		if ctx.Err() == nil {
			// Said in the log when it starts, and not at every look after:
			// on a server with no screen the log is the only place a
			// backup that has stopped can be seen.
			if was, _ := e.unreachable.get(); was == "" {
				e.log.Warn("the backup cannot reach its destination; sales carry on and are copied when it can", "destination", e.cfg.Destination(), "error", err)
			}
			e.unreachable.set(err.Error())
			// What comes back may not be what went away.
			e.forgetPhotos()
		}
		return
	}
	if was, _ := e.unreachable.get(); was != "" {
		e.log.Info("the backup can reach its destination again", "destination", e.cfg.Destination())
	}
	e.unreachable.clear()
	if err := e.open(ctx); err != nil {
		e.errors.last.set(err.Error())
		e.log.Warn("the backup could not be started; it is tried again at the next look", "error", err)
		return
	}
	// Litestream does not look again at what it has sent. A drive swapped
	// for an empty one while the server runs is sent every change from
	// then on and never the database they are changes to, and Litestream
	// reports all well. So it is looked at here.
	switch {
	case whole && !e.sent.Load():
		e.seenWhole()
	case !whole && e.sent.Load():
		e.errors.lost.Store(true)
		e.sayLost()
		return
	}
	e.tidy(ctx)
	// The photos go on the probe's clock: a photo added to the menu is at
	// the destination by the next look, and a quiet venue with no new
	// photo asks the destination nothing more than the probe just did.
	if err := e.copyPhotos(ctx, false); err != nil && ctx.Err() == nil {
		e.log.Warn("the menu's photos were not all backed up", "error", err)
	}
}

// tidy removes, once a day at most, the generations this data directory
// began here that were replaced more than a week ago. Only once this
// generation has been seen whole at the destination: until then the one
// before it is the venue's backup.
func (e *Engine) tidy(ctx context.Context) {
	now := e.clock.Now()
	if !e.sent.Load() || (!e.tidiedAt.IsZero() && now.Sub(e.tidiedAt) < tidyEvery) {
		return
	}
	e.tidiedAt = now
	// To one side. Removing a folder from a share that has stopped
	// answering does not come back when it is told to, and the probe that
	// would say the share is gone is the one waiting. Close waits for it
	// only as long as it waits for anything.
	e.tidying.Add(1)
	go func() {
		defer e.tidying.Done()
		ctx, cancel := context.WithTimeout(ctx, tidyTimeout)
		defer cancel()
		if _, err := tidyGenerations(ctx, e.ldb.MetaPath(), e.cfg, e.generation, now, e.log); err != nil && ctx.Err() == nil {
			e.log.Warn("could not remove an old generation of the backup; it is tried again tomorrow", "error", err)
		}
	}()
}

// tidyTimeout bounds one tidying of old generations.
const tidyTimeout = 5 * time.Minute

// seenWhole notes, the first time, that the destination holds a copy in
// this generation: for this engine, for the starts after it, and for the
// tidying of the generation before.
func (e *Engine) seenWhole() {
	e.seenOnce.Do(func() {
		if err := markSent(e.ldb.MetaPath(), e.cfg, e.generation); err != nil {
			e.log.Warn("could not note that the backup has reached its destination", "error", err)
		}
		if err := noteSent(e.ldb.MetaPath(), destinationMark(e.cfg), e.generation, e.clock.Now()); err != nil {
			e.log.Warn("could not note when the backup reached its destination", "error", err)
		}
		// Last. The tidying waits on this, and must find the time written:
		// read a moment sooner, the list said this generation had not
		// arrived, and the one before it was counted as replaced by the
		// one before that.
		e.sent.Store(true)
	})
}

// tidyEvery is how often old generations are looked for.
const tidyEvery = 24 * time.Hour

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
	if e == nil || !e.cfg.Enabled() {
		return ErrOff
	}
	// In a goroutine, because Litestream takes its lock before it looks
	// at ctx, and with a bucket and no internet that lock is held for
	// minutes. Whoever pressed the button gets an answer when ctx says.
	e.askMu.Lock()
	if e.stopping {
		e.askMu.Unlock()
		return fmt.Errorf("backup: backing up now: %w", ErrStopping)
	}
	e.asked.Add(1)
	e.askMu.Unlock()

	// The caller's time, or the engine's, whichever ends first.
	callerCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(e.askCtx, cancel)()

	// A backup that has not yet found its destination is looked for now:
	// whoever asks has, as likely as not, just plugged the drive in.
	if !e.isOpen() {
		if _, err := e.dest.reach(ctx); err != nil {
			e.asked.Done()
			e.unreachable.set(err.Error())
			return fmt.Errorf("backup: backing up now: %w", err)
		}
		if err := e.open(ctx); err != nil {
			e.asked.Done()
			e.errors.last.set(err.Error())
			return fmt.Errorf("backup: backing up now: %w", err)
		}
	}

	done := make(chan error, 1)
	go func() {
		defer e.asked.Done()
		_, err := e.store.SyncDB(ctx, e.ldb.Path(), true)
		if lostWorkingFile(err, e.ldb.MetaPath()) {
			e.errors.lost.Store(true)
			e.sayLost()
			err = errors.New(lostReason)
		}
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		if callerCtx.Err() == nil {
			return fmt.Errorf("backup: backing up now: %w", ErrStopping)
		}
		err = fmt.Errorf("the destination did not answer in time: %w", ctx.Err())
	}
	if err != nil {
		e.errors.last.set(err.Error())
		return fmt.Errorf("backup: backing up now: %w", err)
	}
	// Litestream is content when it has nothing new to send, and with
	// nothing new to send it has not touched the destination: a drive
	// pulled out on a quiet afternoon answered "back up now" with
	// success. So the destination is looked at, and what is said to be
	// there has to be there.
	whole, err := e.dest.reach(ctx)
	switch {
	case err != nil:
		e.unreachable.set(err.Error())
		e.forgetPhotos()
		return fmt.Errorf("backup: backing up now: %w", err)
	case !whole && e.sent.Load():
		e.errors.lost.Store(true)
		e.sayLost()
		return fmt.Errorf("backup: backing up now: %s", lostReason)
	case !whole:
		return errors.New("backup: backing up now: the destination was reached and holds no whole copy of the database yet; try again in a moment")
	}
	if !e.sent.Load() {
		e.seenWhole()
	}
	e.ldb.RecordSuccessfulSync()
	// It has just been seen whole: whatever was wrong is over, and the
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

// SyncInterval is how often what changed is sent: the most that a machine
// dying can lose. Zero with no backup running.
func (e *Engine) SyncInterval() time.Duration {
	if e == nil || !e.cfg.Enabled() {
		return 0
	}
	return e.interval
}

// Status says how the backup is doing. It touches no disk itself, so it is
// cheap enough to ask on every page load and cannot hang on a dead share.
func (e *Engine) Status() Status {
	if e == nil || !e.cfg.Enabled() {
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
	case e.errors.lost.Load():
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: lostReason}
	case gone != "":
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: gone}
	case stands:
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: reason}
	case photos != "":
		// The sales are safe and the pictures are not: still failing,
		// because "ok" would be read as everything.
		return Status{State: StateFailing, LastSyncedAt: utcOrZero(last), Error: photos}
	case !last.IsZero() && now.Sub(last) < staleAfter(e.interval):
		return Status{State: StateOK, LastSyncedAt: last.UTC()}
	case last.IsZero() && now.Sub(e.started) < staleAfter(e.interval):
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
	if e == nil || !e.cfg.Enabled() {
		return nil
	}
	var left error
	e.closeOnce.Do(func() {
		// Asked before the probe is stopped, so that one in flight has
		// not been cut off before it could say.
		gone, _ := e.unreachable.get()
		e.stopProbe()
		e.askMu.Lock()
		e.stopping = true
		e.askMu.Unlock()

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
			// Every "back up now" still out is told to give up, and waited
			// for, so that none comes back to a Litestream that has closed
			// and opens it again.
			e.stopAsked()
			e.asked.Wait()

			// A backup that never found its destination gets one more
			// look, unless it is already known to be away: a server
			// stopped a moment after it started should still leave a copy.
			// The probe is waited for first, and only here: it is the
			// other thing that starts Litestream.
			if !e.isOpen() {
				<-e.probeDone
				if gone != "" || wait.Err() != nil {
					return
				}
				if _, err := e.dest.reach(ctx); err != nil {
					e.log.Warn("the backup never reached its destination; everything goes on the next start", "error", err)
					return
				}
				// Not if this is no longer being waited for: a Litestream
				// started now would have nobody to stop it.
				if wait.Err() != nil {
					return
				}
				if err := e.open(ctx); err != nil {
					e.log.Warn("the backup could not be started before stopping; everything goes on the next start", "error", err)
					return
				}
			}

			// The last copy, before anything that might not come back:
			// the probe may be waiting on a share that has stopped
			// answering, and the last sales are not to wait behind it.
			// Not returned. A drive that is unplugged at closing time is
			// the backup failing, which the status already says, and not
			// the server failing to stop: under a service a failed stop is
			// a crash.
			if err := e.store.Close(ctx); err != nil {
				e.log.Warn("the backup stopped without reaching its destination; what is left goes on the next start", "error", err)
			}
			<-e.probeDone
			e.tidying.Wait()
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
			left = errors.New("backup: did not stop in time and is left behind")
		}
	})
	// Said to the caller as well: a server that starts again in this same
	// process (ADR-334) must not start a second copier over the first.
	return left
}

// lostWorkingFile reports whether err is Litestream failing to find one of
// its own small files in its working folder.
//
// That is what a destination that has lost what it had looks like from
// here: a new drive in the old one's place, a bucket emptied, a folder
// changed for another and back. Litestream sees the destination is behind
// and goes to send the changes since, which it tidied away long ago.
func lostWorkingFile(err error, workingDir string) bool {
	return workingDir != "" && errors.Is(err, fs.ErrNotExist) && strings.Contains(err.Error(), workingDir)
}

// lostReason is what Status says of a destination that has lost what it
// had.
const lostReason = "the destination no longer holds what was sent to it; the whole database is sent again when the backup next starts"

// Running reports whether this engine is backing up: false for one that is
// off or nil. It says nothing of whether the destination can be reached.
func (e *Engine) Running() bool { return e != nil && e.cfg.Enabled() }

// Lost reports whether the destination no longer holds what was sent to
// it, so that this backup cannot go on and another must be started in
// its place.
//
// Litestream can forget where it had got to while it runs, and does for
// some errors (its AutoRecoverEnabled), but not for this one; and doing
// it for it, under its own goroutines, sent backups that restored short.
// So it is done the plain way: this engine is stopped, its working files
// removed, and a new one started, by whoever OnLost tells.
func (e *Engine) Lost() bool {
	return e != nil && e.cfg.Enabled() && e.errors.lost.Load()
}

// sayLost marks the working files as no use to the next start, and tells
// whoever asked to be told. Once.
func (e *Engine) sayLost() {
	e.lostOnce.Do(func() {
		e.log.Warn("the destination no longer holds what was last sent to it; the backup has to be started again to send the whole database")
		if err := forgetWorkingFiles(e.ldb.MetaPath()); err != nil {
			e.log.Warn("could not mark the backup to start again", "error", err)
		}
		if e.onLost != nil {
			go e.onLost()
		}
	})
}

// markerSuffix names the file, beside Litestream's working folder, that
// says which destination and which generation the folder's contents are
// about.
const markerSuffix = "-for"

// workingFilesFor makes Litestream's working folder fit to back up to
// cfg, before Litestream is started on it, and names the generation to
// back up into.
//
// If the folder is what the last run left for this same destination, the
// run carries on in its generation. If it was kept for another
// destination, or for one since found to have lost what it had, or is
// not there at all, it is cleared and a new generation begun: Litestream
// starts from nothing and sends the whole database.
//
// Nothing is lost by clearing it. It is Litestream's memory of what it
// has sent, and the database is what is sent.
func workingFilesFor(workingDir string, cfg Config, now time.Time, log *slog.Logger) (generation string, sent bool, err error) {
	marker := workingDir + markerSuffix
	want := destinationMark(cfg)

	have, err := os.ReadFile(marker)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", false, fmt.Errorf("reading %s: %w", marker, err)
	}
	// Three lines: the destination, the generation, and "sent" once a
	// whole copy has been seen there.
	if lines := strings.Split(strings.TrimSpace(string(have)), "\n"); len(lines) >= 2 && lines[0] == want && generationName.MatchString(lines[1]) {
		if _, statErr := os.Stat(workingDir); statErr == nil {
			return lines[1], len(lines) > 2 && lines[2] == sentWord, nil
		}
	}

	generation, err = newGeneration(now)
	if err != nil {
		return "", false, err
	}
	log.Info("the backup begins again with the whole database", "destination", cfg.Destination(), "generation", generation)
	if err := os.RemoveAll(workingDir); err != nil {
		return "", false, fmt.Errorf("clearing %s: %w", workingDir, err)
	}
	// Made here, so that it is there for the next start to find even if
	// this one is stopped before Litestream has looked at the database.
	if err := os.MkdirAll(workingDir, 0o750); err != nil {
		return "", false, fmt.Errorf("making %s: %w", workingDir, err)
	}
	if err := writeWhole(marker, []byte(want+"\n"+generation+"\n")); err != nil {
		return "", false, fmt.Errorf("writing %s: %w", marker, err)
	}
	// So that it can be tidied away, one day, by the data directory that
	// began it and by no other.
	// Not a reason not to back up. A generation that is not noted is one
	// that is never tidied away, which costs a folder and loses nothing.
	if err := noteMade(workingDir, want, generation); err != nil {
		log.Warn("could not note the backup's new generation; it will not be tidied away when it is old", "generation", generation, "error", err)
	}
	return generation, false, nil
}

// sentWord is the marker's third line.
const sentWord = "sent"

// markSent notes, for the starts after this one, that the destination has
// held a whole copy in this generation.
func markSent(workingDir string, cfg Config, generation string) error {
	return writeWhole(workingDir+markerSuffix, []byte(destinationMark(cfg)+"\n"+generation+"\n"+sentWord+"\n"))
}

// forgetWorkingFiles makes the next workingFilesFor clear the folder,
// whatever the destination.
func forgetWorkingFiles(workingDir string) error {
	err := os.Remove(workingDir + markerSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// destinationMark is a destination as one line: which place, not how it
// is got into. A new secret for the same bucket is the same place.
func destinationMark(cfg Config) string {
	sum := sha256.Sum256([]byte(string(cfg.Kind) + "\n" + cfg.Destination()))
	return hex.EncodeToString(sum[:])
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
	// lostIn is Litestream's working folder, and lost is set when an
	// error says a file is missing from it. See Engine.Lost.
	lostIn string
	lost   *atomic.Bool
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
				if err, ok := a.Value.Any().(error); ok && lostWorkingFile(err, k.lostIn) {
					k.lost.Store(true)
				}
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
	return &errorKeeper{Handler: k.Handler.WithAttrs(attrs), last: k.last, lostIn: k.lostIn, lost: k.lost}
}

func (k *errorKeeper) WithGroup(name string) slog.Handler {
	return &errorKeeper{Handler: k.Handler.WithGroup(name), last: k.last, lostIn: k.lostIn, lost: k.lost}
}
