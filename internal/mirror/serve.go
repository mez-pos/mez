package mirror

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/dbhealth"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/fsperm"
	"github.com/byjoellobo/mez/internal/service/audit"
	"github.com/byjoellobo/mez/internal/service/firstrun"
	"github.com/byjoellobo/mez/internal/service/floor"
	"github.com/byjoellobo/mez/internal/service/menus"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
	"github.com/byjoellobo/mez/internal/service/people"
	"github.com/byjoellobo/mez/internal/service/printers"
	"github.com/byjoellobo/mez/internal/service/reports"
	"github.com/byjoellobo/mez/internal/service/settings"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
	svcstock "github.com/byjoellobo/mez/internal/service/stock"
	"github.com/byjoellobo/mez/web"
)

// DirPrefix begins the name of every directory a mirror keeps a copy in.
// A directory by this name that no mirror is running on was left by one
// that was killed, not closed.
const DirPrefix = "mez-mirror-"

// DefaultPatience is how long a copy being put away waits for the requests
// still being answered from it, unless told.
const DefaultPatience = 10 * time.Second

// retryAfter is how long a newer write that could not be made into a copy
// is left alone before it is fetched again. What the backup writes next is
// tried at once.
const retryAfter = 10 * time.Minute

// OpenOptions configure a Mirror. From and StateDir are required.
type OpenOptions struct {
	// From is where the backup is: a folder or a bucket. It is only read.
	From backup.Config
	// Generation names the backup to show and follow, as `mez restore
	// -list` names it. Empty is the newest that holds anything.
	Generation string
	// Venue is the public id of the venue the caller expects the backup
	// to hold, and a copy that holds another is refused. Empty takes
	// whichever venue the first copy holds, as `mez mirror` does with a
	// backup of the owner's own. A caller that serves backups it did not
	// write itself names it: the copy's id is the backup's word only.
	Venue string
	// StateDir is where the mirror remembers the browsers paired to it and
	// the locks wrong PINs set, in a file named by the venue's public id.
	// It is made if it is not there, and kept from other accounts.
	StateDir string
	// CopyDir is where each copy is restored, in a directory of its own
	// that begins with DirPrefix. Empty is the system's temporary directory.
	CopyDir string
	// CloudKey is mez Cloud's public key, and nil for a mirror the cloud
	// signs nobody in to (ADR-331).
	CloudKey ed25519.PublicKey
	// Build is what the mirror says of itself at /api/v1/health.
	Build api.BuildInfo
	// Log is told what goes wrong while the mirror serves and follows.
	// Nil is nowhere. Litestream, which restores each copy, says what it
	// is doing through Go's default logger and not this one: a process
	// that wants it quiet quiets that.
	Log *slog.Logger
	// Patience is how long a copy being put away waits for a request still
	// being answered from it. Zero is DefaultPatience.
	Patience time.Duration
}

// Mirror is the back office over a copy of a venue's backup, to read
// (ADR-326, ADR-327, ADR-353). It is an http.Handler: whoever opens one
// owns the listener, or the route it is mounted on. It serves one copy at
// a time, takes a newer one while Follow runs, and removes every copy it
// made when it is closed.
type Mirror struct {
	source    *source
	held      *holder
	keeper    *Keeper
	statePath string
}

// Open restores the backup, checks the copy, brings it up to this build's
// schema and opens it, with its door and what it remembers. Nothing is left
// behind when it fails. It holds exactly one venue or it is refused.
func Open(ctx context.Context, o OpenOptions) (_ *Mirror, err error) {
	if o.StateDir == "" {
		return nil, errors.New("mirror: a state directory is required")
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Patience <= 0 {
		o.Patience = DefaultPatience
	}
	src := &source{from: o.From, copyDir: o.CopyDir, log: o.Log, cloudKey: o.CloudKey, build: o.Build, patience: o.Patience}
	generation, err := src.newest(ctx, o.Generation)
	if err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}
	first, err := src.open(ctx, generation)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, first.close())
		}
	}()

	// The venue's id names the state file, and it is read from the copy:
	// a backup is written by whoever holds its key, so it is checked as an
	// id before it is a path, and against the one expected when there is.
	if !cloud.ValidVenuePublicID(first.venue.PublicID) {
		return nil, fmt.Errorf("mirror: the backup's venue has the id %q, which no venue has", first.venue.PublicID)
	}
	if o.Venue != "" && first.venue.PublicID != o.Venue {
		return nil, fmt.Errorf("mirror: the backup holds the venue %s, and %s was expected", first.venue.PublicID, o.Venue)
	}

	// What the mirror remembers of its own, kept where a restart finds it.
	// It is the same through every copy this mirror serves.
	if err := os.MkdirAll(o.StateDir, 0o700); err != nil {
		return nil, fmt.Errorf("mirror: making %s: %w", o.StateDir, err)
	}
	if err := fsperm.Restrict(o.StateDir); err != nil {
		o.Log.Warn("the mirror's state directory could not be restricted to its owner", "dir", o.StateDir, "error", err)
	}
	statePath := filepath.Join(o.StateDir, first.venue.PublicID+".json")
	keeper, err := OpenKeeper(KeeperOptions{Path: statePath})
	if err != nil {
		return nil, err
	}
	src.keeper, src.venue, src.only = keeper, first.venue.PublicID, o.Generation
	if err := first.serve(src); err != nil {
		return nil, err
	}
	return &Mirror{
		source:    src,
		held:      &holder{current: first, log: o.Log, patience: o.Patience},
		keeper:    keeper,
		statePath: statePath,
	}, nil
}

// ServeHTTP answers from the copy being served.
func (m *Mirror) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.held.ServeHTTP(w, r) }

// Keeper is what the mirror remembers: a pairing code for a browser comes
// from it.
func (m *Mirror) Keeper() *Keeper { return m.keeper }

// StatePath is the file the Keeper keeps.
func (m *Mirror) StatePath() string { return m.statePath }

// Venue is the venue in the copy being served.
func (m *Mirror) Venue() Venue {
	c := m.held.copy()
	return Venue{PublicID: c.venue.PublicID, Name: c.venue.Name}
}

// AsOf is when the venue last wrote something the copy being served holds.
func (m *Mirror) AsOf() time.Time { return m.held.copy().asOf }

// Dir is the directory the copy being served is in.
func (m *Mirror) Dir() string { return m.held.copy().dir }

// Asked is when a request last arrived, and zero if none has: a mirror
// nobody has asked anything for a while can be closed.
func (m *Mirror) Asked() time.Time {
	_, _, asked := m.held.state()
	return asked
}

// Follow keeps the mirror up with its backup until ctx ends, looking every
// so often while it is being asked things (ADR-327). An every of zero or
// less is never, and returns at once. Run one Follow a mirror. Stop it, and
// wait for it to return, before Close: a copy it is making is put away by
// it, and once the mirror is closed it returns at its next look.
func (m *Mirror) Follow(ctx context.Context, every time.Duration) {
	if every <= 0 {
		return
	}
	m.source.follow(ctx, m.held, every)
}

// Close puts away every copy the mirror made, once the requests still being
// answered from them are done or their patience runs out. A copy that could
// not be removed is said in the error: it is a copy of a venue's sales.
// Every Close answers once the first has finished, with what it found; a
// request that arrives after is answered 503 and never reaches a copy.
func (m *Mirror) Close() error { return m.held.close() }

// Venue is a mirror's venue as its copy has it.
type Venue struct {
	PublicID string
	Name     string
}

// source is where a mirror's copies come from: the backup, and what every
// copy of it is served with.
type source struct {
	from    backup.Config
	copyDir string
	log     *slog.Logger
	build   api.BuildInfo
	// patience is how long a copy's database is given to close.
	patience time.Duration
	// only is the generation the mirror was told to show, or empty.
	only string
	// keeper is the mirror's own memory, shared by every copy. venue is
	// the public id of the one venue the first copy held: a later copy
	// that holds another is not served.
	keeper *Keeper
	venue  string
	// cloudKey is mez Cloud's public key, or nil: every copy's door is
	// given it.
	cloudKey ed25519.PublicKey
}

// mirrorCopy is one restore of the backup: a directory of the mirror's own,
// the database in it, and the server over it.
type mirrorCopy struct {
	dir        string
	generation string
	// asOf is when the venue last wrote something this copy holds.
	asOf     time.Time
	venue    gen.Venue
	database *db.DB
	handler  http.Handler
	// patience is how long its database is given to close.
	patience time.Duration
	// inFlight is the requests being answered from this copy. It is closed
	// only once they have finished.
	inFlight sync.WaitGroup
}

// open restores one generation of the backup into a new directory, checks
// it, brings its schema up to this build's, and opens it. Nothing is left
// behind when it fails. The copy is not served until serve is called.
func (s *source) open(ctx context.Context, generation backup.Generation) (_ *mirrorCopy, err error) {
	// A directory of the mirror's own. What is in it is a venue's sales, so
	// it is kept from every other account as a data directory is (ADR-259).
	dir, err := os.MkdirTemp(s.copyDir, DirPrefix)
	if err != nil {
		return nil, fmt.Errorf("mirror: making a directory for the copy: %w", err)
	}
	// Its own name, and not the result's: a return of nil must not be what
	// the clean-up below is handed.
	c := &mirrorCopy{dir: dir, generation: generation.Name, asOf: generation.Last, patience: s.patience}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.close())
		}
	}()
	if err := fsperm.Restrict(dir); err != nil {
		// Through the log, which is safe to write to from the follower.
		s.log.Warn("the copy's directory could not be restricted to its owner", "dir", dir, "error", err)
	}

	dbPath := filepath.Join(dir, db.FileName)
	if err := backup.RestoreGeneration(ctx, s.from, generation.Name, dbPath); err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}
	if _, err := backup.RestorePhotos(ctx, s.from, filepath.Join(dir, "photos")); err != nil {
		// The numbers are what a mirror is for. A menu without its pictures
		// is still a menu.
		s.log.Warn("not all the menu photos could be read from the backup", "error", err)
	}

	c.database, err = db.Open(ctx, db.Options{Path: dbPath, Log: s.log})
	if err != nil {
		return nil, fmt.Errorf("mirror: opening the copy: %w", err)
	}
	// The copy may be from an older mez than this one. It is brought up to
	// this build's schema, as a restore is at its first start. It is the
	// last thing written to the copy: who is let in is kept beside it
	// (ADR-327).
	if err := c.database.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("mirror: the copy could not be read by this version of mez: %w", err)
	}
	venues, err := c.database.Queries().ListVenues(ctx)
	if err != nil {
		return nil, fmt.Errorf("mirror: reading the copy: %w", err)
	}
	if len(venues) != 1 {
		return nil, fmt.Errorf("mirror: the backup holds %d venues, and a mirror shows one", len(venues))
	}
	c.venue = venues[0]
	return c, nil
}

// serve builds the server over a copy. The keeper must be set by now.
func (c *mirrorCopy) serve(s *source) error {
	// A backup that now holds another venue is somebody else's: the
	// browsers paired here and the PINs counted here were for the first.
	if c.venue.PublicID != s.venue {
		return fmt.Errorf("mirror: the backup now holds the venue %s, and this mirror was started on %s", c.venue.PublicID, s.venue)
	}
	asOf := c.asOf
	srv, err := server(c.database, s, c.venue, filepath.Join(c.dir, "photos"),
		func() api.MirrorInfo { return api.MirrorInfo{AsOf: asOf} })
	if err != nil {
		return err
	}
	c.handler = srv.Handler()
	return nil
}

// close closes a copy's database and removes its directory. A directory
// that cannot be removed is said, whatever else went wrong: it is a copy
// of a venue's sales.
func (c *mirrorCopy) close() error {
	var err error
	if c.database != nil {
		patience := c.patience
		if patience <= 0 {
			patience = DefaultPatience
		}
		closing, cancel := context.WithTimeout(context.Background(), patience)
		err = c.database.Close(closing)
		cancel()
	}
	if rmErr := os.RemoveAll(c.dir); rmErr != nil {
		err = errors.Join(err, fmt.Errorf("mirror: the copy in %s could not be removed, and is a copy of the venue's sales: remove it by hand: %w", c.dir, rmErr))
	}
	return err
}

// holder is what is served: whichever copy is current. A newer copy takes
// the place of the one before it between two requests, and the one before
// is closed once the requests it was answering are done.
type holder struct {
	log *slog.Logger
	// patience is how long a copy that is being put away waits for the
	// requests still being answered from it. A client that has stalled
	// holds its request open for as long as it likes, and a copy of a
	// venue's sales is not left on disk for it.
	patience time.Duration

	mu      sync.Mutex
	current *mirrorCopy
	// asked is when a request last arrived: a mirror nobody is looking at
	// does not go and read its backup.
	asked time.Time
	// retiring is the copies that have been replaced and are not yet closed.
	retiring sync.WaitGroup
	closed   bool
	problems []error
	// shut is closed once the first close has finished, and shutErr is
	// what it found: every later close waits for it and answers the same.
	shut    chan struct{}
	shutErr error
}

func (h *holder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if h.closed {
		// A copy being put away is not taken up again: its database may
		// be closed and its directory gone.
		h.mu.Unlock()
		http.Error(w, "this mirror has been closed", http.StatusServiceUnavailable)
		return
	}
	c := h.current
	// Counted while the lock is held, so a copy is never taken up after
	// the swap that retires it has looked at the count.
	c.inFlight.Add(1)
	h.asked = time.Now()
	h.mu.Unlock()
	defer c.inFlight.Done()
	c.handler.ServeHTTP(w, r)
}

// copy is the copy being served.
func (h *holder) copy() *mirrorCopy {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.current
}

// state is the copy being served and when anyone last asked for anything.
func (h *holder) state() (generation string, asOf, asked time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.current.generation, h.current.asOf, h.asked
}

// swap puts a newer copy in the old one's place. It reports false, and
// takes nothing, once the mirror is stopping.
func (h *holder) swap(next *mirrorCopy) bool {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return false
	}
	old := h.current
	h.current = next
	h.retiring.Add(1)
	h.mu.Unlock()

	go func() {
		defer h.retiring.Done()
		// Whoever was being answered from the old copy is answered from
		// it, within reason.
		if !waited(&old.inFlight, h.patience) {
			h.log.Warn("a copy the mirror had finished with was put away with requests still being answered from it", "after", h.patience.String())
		}
		if err := old.close(); err != nil {
			h.log.Error("a copy the mirror had finished with could not be put away", "error", err)
			h.mu.Lock()
			h.problems = append(h.problems, err)
			h.mu.Unlock()
		}
	}()
	return true
}

// close puts away the copy being served, and waits for those already
// retiring. Whoever serves the mirror has stopped sending it requests by
// now, but one it could not finish in its own time may still be running:
// it is waited for as long as a retiring copy waits, and no longer. The
// mirror stops, and the copy goes, whatever a stalled client does. A
// later close waits for the first and answers what it found.
func (h *holder) close() error {
	h.mu.Lock()
	if h.closed {
		shut := h.shut
		h.mu.Unlock()
		<-shut
		return h.shutErr
	}
	h.closed = true
	h.shut = make(chan struct{})
	current := h.current
	h.mu.Unlock()

	// Each retiring copy waits patience and no more, so this ends.
	h.retiring.Wait()
	if !waited(&current.inFlight, h.patience) {
		h.log.Warn("the mirror stopped with requests still being answered", "after", h.patience.String())
	}
	// Put away without the lock: what is read of the mirror meanwhile is
	// not held up behind a database closing.
	closeErr := current.close()
	h.mu.Lock()
	h.shutErr = errors.Join(append(h.problems, closeErr)...)
	h.mu.Unlock()
	close(h.shut)
	return h.shutErr
}

// waited waits for wg, for at most patience, and reports whether it was
// done in time.
func waited(wg *sync.WaitGroup, patience time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(patience):
		return false
	}
}

// follow keeps the mirror up with its backup until ctx ends: every so
// often it asks the destination when it was last written, and when that is
// later than the copy being served, restores afresh and swaps (ADR-327).
//
// It asks only while the mirror is being used. Asking is a request to a
// bucket, and a restore is many; a mirror left open overnight makes none.
// Whatever goes wrong leaves the copy being served where it is, growing
// older, and is said in the log: the next look tries again.
func (s *source) follow(ctx context.Context, held *holder, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	var looked time.Time
	// refused is the last write that could not be made into a copy, and
	// when: a backup this build cannot read, or one that now holds another
	// venue, would otherwise be fetched whole at every look for as long as
	// anyone was looking.
	var refused backup.Generation
	var refusedAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		generation, asOf, asked := held.state()
		if !asked.After(looked) {
			continue
		}
		looked = time.Now()

		newest, err := s.newest(ctx, s.only)
		if err != nil {
			if ctx.Err() == nil {
				s.log.Warn("the mirror could not look at its backup, and goes on showing what it has", "error", err)
			}
			continue
		}
		if newest.Name == generation && !newest.Last.After(asOf) {
			continue
		}
		if newest.Name == refused.Name && newest.Last.Equal(refused.Last) && time.Since(refusedAt) < retryAfter {
			continue
		}
		next, err := s.open(ctx, newest)
		if err == nil {
			if err = next.serve(s); err != nil {
				err = errors.Join(err, next.close())
			}
		}
		if err != nil {
			if ctx.Err() == nil {
				refused, refusedAt = newest, time.Now()
				s.log.Warn("the mirror could not take a newer copy of its backup, and goes on showing what it has",
					"as_of", asOf, "tries_again_after", retryAfter.String(), "error", err)
			}
			continue
		}
		if !held.swap(next) {
			if err := next.close(); err != nil {
				s.log.Error("a copy made as the mirror stopped could not be put away", "error", err)
			}
			return
		}
	}
}

// newest is the generation a mirror should be showing: the one it was
// told to, or the newest that holds anything.
func (s *source) newest(ctx context.Context, only string) (backup.Generation, error) {
	found, err := backup.Generations(ctx, s.from)
	if err != nil {
		return backup.Generation{}, err
	}
	for _, g := range found {
		if g.Empty() || (only != "" && g.Name != only) {
			continue
		}
		return g, nil
	}
	return backup.Generation{}, backup.ErrNothingToRestore
}

// server is the API over a copy, as a mirror serves it: what the back
// office reads, and nothing that reaches out. The print queue is built and
// never started, so a ticket that was waiting when the backup was cut is
// not printed from here. There is no stream, tunnel, backup or gateway.
//
// Who is let in is the mirror's own affair: the door reads people and their
// PINs from the copy and remembers browsers, sessions and wrong PINs in the
// keeper, so the copy is not written (ADR-327). With a cloud key, mez
// Cloud's word lets the account's owner in too, checked and remembered by
// the same door (ADR-331).
func server(database *db.DB, s *source, venue gen.Venue, photosDir string, info func() api.MirrorInfo) (*api.Server, error) {
	log := s.log
	// These two only read here: the door asks them who works at the venue
	// and what its devices were.
	devices, err := auth.NewDevices(auth.DevicesOptions{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	staff, err := auth.NewStaffAuth(auth.StaffOptions{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	door, err := NewAuth(Options{
		Keeper: s.keeper, Staff: staff, Devices: devices, VenueID: venue.ID,
		CloudKey: s.cloudKey, VenuePublicID: venue.PublicID, Log: log,
	})
	if err != nil {
		return nil, err
	}
	prints, err := printq.New(printq.Options{DB: database, Log: log})
	if err != nil {
		return nil, err
	}
	orders, err := svcorder.New(svcorder.Options{DB: database, Log: log, Prints: prints})
	if err != nil {
		return nil, err
	}
	printerSettings, err := printers.New(database, prints, nil, s.build.Version)
	if err != nil {
		return nil, err
	}
	venueSettings, err := settings.New(settings.Options{DB: database})
	if err != nil {
		return nil, err
	}
	photos, err := menus.NewPhotoStore(photosDir)
	if err != nil {
		return nil, err
	}
	menuEditor, err := menus.New(menus.Options{DB: database, Photos: photos})
	if err != nil {
		return nil, err
	}
	stockBook, err := svcstock.New(database, nil)
	if err != nil {
		return nil, err
	}
	staffBook, err := people.New(database, nil)
	if err != nil {
		return nil, err
	}
	trail, err := audit.New(database, orders)
	if err != nil {
		return nil, err
	}
	rangeReports, err := reports.New(database)
	if err != nil {
		return nil, err
	}
	floorPlan, err := floor.New(floor.Options{DB: database, Orders: orders})
	if err != nil {
		return nil, err
	}
	shifts, err := svcshift.New(svcshift.Options{DB: database, Prints: prints})
	if err != nil {
		return nil, err
	}
	bundles, err := web.Bundles()
	if err != nil {
		return nil, err
	}
	// Only so that the back office's first question, whether this machine
	// still needs setting up, is answered no. The wizard's own steps are
	// writes, and refused with the rest.
	firstRun, err := firstrun.New(database, devices, nil)
	if err != nil {
		return nil, err
	}

	// Of the configuration, the API reads only whether to answer CORS,
	// which a mirror does not. Where it listens is its caller's.
	cfg := config.Config{
		DataDir:  filepath.Dir(photosDir),
		LogLevel: slog.LevelWarn, LogFormat: config.LogFormatText, Timezone: time.Local,
	}
	srv := api.New(cfg, log, s.build,
		api.WithMirror(info),
		api.WithStatic(bundles),
		api.WithDBCheck(dbhealth.Check(database)),
		api.WithDevices(door),
		api.WithFirstRun(firstRun),
		api.WithStaff(door),
		// Always, key or none: with none it refuses in the one voice a
		// venue that is not paired does.
		api.WithCloudSignIn(door.Cloud()),
		api.WithOrders(orders),
		api.WithMenus(database),
		api.WithPrints(prints),
		api.WithPrinters(printerSettings),
		api.WithSettings(venueSettings),
		api.WithMenuEditor(menuEditor),
		api.WithStock(stockBook),
		api.WithPeople(staffBook),
		api.WithAudit(trail),
		api.WithReports(rangeReports),
		api.WithFloor(floorPlan),
		api.WithShifts(shifts),
		// No WithGuests and no WithTableCodes: guests are not served from a
		// copy, and the codes they scan lead to the venue, not here.
	)
	return srv, nil
}
