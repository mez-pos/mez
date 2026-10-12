package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/device/printq"
	"github.com/byjoellobo/mez/internal/fsperm"
	"github.com/byjoellobo/mez/internal/mirror"
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

// mirrorListenAddr is where a mirror listens unless told: this computer
// only, and not the port a venue's own server takes. A mirror keeps no lock
// on a PIN that outlasts it (ADR-326), so it is not put on a network by
// default.
const mirrorListenAddr = "127.0.0.1:8090"

// mirrorFollowEvery is how often a mirror in use looks at its backup, and
// mirrorFollowFloor the least it may be told to. Each look lists the
// destination, which a bucket bills as a write (ADR-315), and each newer
// copy is a whole restore: a minute keeps a mirror close behind a venue
// that sends every half minute without asking twice for each thing sent.
//
// mirrorRetryAfter is how long a newer write that could not be made into a
// copy is left alone before it is fetched again. What the backup writes
// next is tried at once.
const (
	mirrorFollowEvery = time.Minute
	mirrorFollowFloor = time.Second
	mirrorRetryAfter  = 10 * time.Minute
)

// mirrorDirPrefix begins the name of every directory a mirror keeps its
// copy in, under the system's temporary directory.
const mirrorDirPrefix = "mez-mirror-"

// leftCopies is the mirror directories already under dir: those of mirrors
// running now, and of any that was killed before it could remove its own.
func leftCopies(dir string) []string {
	found, err := filepath.Glob(filepath.Join(dir, mirrorDirPrefix+"*"))
	if err != nil {
		return nil
	}
	return found
}

// mirrorOptions is everything `mez mirror` takes: where the backup is, as
// `mez restore` takes it, and where to listen.
type mirrorOptions struct {
	restoreOptions
	ListenAddr string
	// StateDir is where the mirror keeps what it remembers between runs.
	StateDir string
	// Follow is how often a mirror in use looks at its backup for
	// something newer. Zero is never.
	Follow time.Duration
}

// stateDir is where this mirror remembers the browsers paired to it and
// the locks wrong PINs set: as told, or mez/mirror under the user's
// configuration directory.
func (o mirrorOptions) stateDir() (string, error) {
	if o.StateDir != "" {
		return filepath.Abs(o.StateDir)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("mirror: this account has no configuration directory to remember paired browsers in; give one with -state-dir: %w", err)
	}
	return filepath.Join(base, "mez", "mirror"), nil
}

// mirrorUp is a mirror that is serving: where, what a browser pairs to it
// with, and how old its copy is.
type mirrorUp struct {
	Addr string
	Code string
	AsOf time.Time
	Dir  string
}

// runMirror is `mez mirror`: the back office over a copy of a venue's
// backup, to read (phase 14 Task 6a, ADR-312, ADR-326). It serves until it
// is interrupted, and keeps nothing afterwards.
func runMirror(args []string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveMirror(ctx, args, stdout, nil)
}

// serveMirror restores, serves until ctx ends, and removes what it restored.
// ready, when given, is told once it is serving.
func serveMirror(ctx context.Context, args []string, stdout io.Writer, ready func(mirrorUp)) (err error) {
	opts, err := parseMirrorFlags(args, stdout)
	if err != nil {
		return err
	}
	from, err := opts.destination(os.Getenv)
	if err != nil {
		return fmt.Errorf("mirror: %w", err)
	}
	// Litestream says what it is doing through Go's default logger, in its
	// own words, in the middle of what this prints.
	slog.SetDefault(quietLog())
	log := slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	generation, err := chooseGeneration(ctx, from, opts.Generation)
	if err != nil {
		return err
	}

	// Bound before anything is fetched: a port in use is the likeliest
	// failure, and a bucket is not read to find it out.
	listener, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("mirror: listening on %s: %w", opts.ListenAddr, err)
	}
	defer func() { _ = listener.Close() }()

	// A mirror that was killed, not stopped, could not remove its copy.
	// It is not removed here: another mirror may be running on it now.
	for _, left := range leftCopies(os.TempDir()) {
		fmt.Fprintf(stdout, "A mirror that was not stopped cleanly left a copy of a venue at %s\n  Remove it once no mirror is running.\n", left)
	}

	fmt.Fprintf(stdout, "Reading the backup at %s\n", from.Destination())
	source := &mirrorSource{from: from, listenAddr: opts.ListenAddr, log: log}
	first, err := source.open(ctx, generation)
	if err != nil {
		return err
	}
	held := &mirrorHolder{current: first, log: log, patience: shutdownTimeout}
	defer func() {
		// Whichever copy is being served when the mirror stops, and any
		// still finishing its last requests, are closed and removed.
		err = errors.Join(err, held.close())
	}()

	// What the mirror remembers of its own, kept where a restart finds it:
	// the browsers paired to it and the locks wrong PINs have set. It is
	// the same through every copy this mirror serves.
	stateDir, err := opts.stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("mirror: making %s: %w", stateDir, err)
	}
	if err := fsperm.Restrict(stateDir); err != nil {
		fmt.Fprintf(stdout, "  Could not restrict %s to its owner: %v\n", stateDir, err)
	}
	keeper, err := mirror.OpenKeeper(mirror.KeeperOptions{Path: filepath.Join(stateDir, first.venue.PublicID+".json")})
	if err != nil {
		return err
	}
	source.keeper, source.venue = keeper, first.venue.PublicID
	if err := first.serve(source); err != nil {
		return err
	}

	// One more browser, paired with a code as any back office is. Those
	// paired before are remembered and need none.
	code, codeEnds, err := keeper.NewCode()
	if err != nil {
		return err
	}
	known := len(keeper.Browsers())

	following := "It does not follow the backup: it stays as it was when started."
	if opts.Follow > 0 {
		following = fmt.Sprintf("While it is being used it looks at the backup every %s and shows what is newer.", opts.Follow)
	}
	addr := listener.Addr().String()
	fmt.Fprintf(stdout, `
A mirror of %s, as it was at %s.

  Open          http://%s%s
  Pairing code  %s   (for a browser not paired to this mirror before; good
                until %s, once. Start the mirror again for another)
  Remembered    %d browsers paired before, in %s

Then sign in with a PIN of someone who manages the venue. It is a copy to
read: nothing changed here reaches the venue, and nothing is printed.
%s
Ctrl-C stops it and removes the copy.

`, first.venue.Name, localTime(first.asOf), addr, web.MountBackoffice, code, codeEnds.Local().Format("15:04"), known, stateDir, following)

	// Following stops before the copies are closed: it is what makes them.
	followed := make(chan struct{})
	followCtx, stopFollowing := context.WithCancel(ctx)
	go func() {
		defer close(followed)
		if opts.Follow > 0 {
			source.follow(followCtx, held, opts.Follow, opts.Generation)
		}
	}()
	defer func() {
		stopFollowing()
		<-followed
	}()

	if ready != nil {
		ready(mirrorUp{Addr: addr, Code: code, AsOf: first.asOf, Dir: first.dir})
	}
	return serve(ctx, log, listener, held, shutdownTimeout)
}

// mirrorSource is where a mirror's copies come from: the backup, and what
// every copy of it is served with.
type mirrorSource struct {
	from       backup.Config
	listenAddr string
	log        *slog.Logger
	// keeper is the mirror's own memory, shared by every copy. venue is
	// the public id of the one venue the first copy held: a later copy
	// that holds another is not served.
	keeper *mirror.Keeper
	venue  string
}

// mirrorCopy is one restore of the backup: a directory of the mirror's
// own, the database in it, and the server over it.
type mirrorCopy struct {
	dir        string
	generation string
	// asOf is when the venue last wrote something this copy holds.
	asOf     time.Time
	venue    gen.Venue
	database *db.DB
	handler  http.Handler
	// inFlight is the requests being answered from this copy. It is closed
	// only once they have finished.
	inFlight sync.WaitGroup
}

// open restores one generation of the backup into a new directory, checks
// it, brings its schema up to this build's, and opens it. Nothing is left
// behind when it fails. The copy is not served until serve is called.
func (m *mirrorSource) open(ctx context.Context, generation backup.Generation) (_ *mirrorCopy, err error) {
	// A directory of the mirror's own. What is in it is a venue's sales, so
	// it is kept from every other account as a data directory is (ADR-259).
	dir, err := os.MkdirTemp("", mirrorDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("mirror: making a directory for the copy: %w", err)
	}
	// Its own name, and not the result's: a return of nil must not be what
	// the clean-up below is handed.
	c := &mirrorCopy{dir: dir, generation: generation.Name, asOf: generation.Last}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.close())
		}
	}()
	if err := fsperm.Restrict(dir); err != nil {
		// Through the log, which is safe to write to from the follower.
		m.log.Warn("the copy's directory could not be restricted to its owner", "dir", dir, "error", err)
	}

	dbPath := filepath.Join(dir, db.FileName)
	if err := backup.RestoreGeneration(ctx, m.from, generation.Name, dbPath); err != nil {
		return nil, fmt.Errorf("mirror: %w", err)
	}
	if _, err := backup.RestorePhotos(ctx, m.from, filepath.Join(dir, "photos")); err != nil {
		// The numbers are what a mirror is for. A menu without its pictures
		// is still a menu.
		m.log.Warn("not all the menu photos could be read from the backup", "error", err)
	}

	c.database, err = db.Open(ctx, db.Options{Path: dbPath, Log: m.log})
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
func (c *mirrorCopy) serve(m *mirrorSource) error {
	// A backup that now holds another venue is somebody else's: the
	// browsers paired here and the PINs counted here were for the first.
	if c.venue.PublicID != m.venue {
		return fmt.Errorf("mirror: the backup now holds the venue %s, and this mirror was started on %s", c.venue.PublicID, m.venue)
	}
	asOf := c.asOf
	srv, err := mirrorServer(c.database, m.keeper, c.venue.ID, filepath.Join(c.dir, "photos"), m.listenAddr, m.log,
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
		closing, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		err = c.database.Close(closing)
		cancel()
	}
	if rmErr := os.RemoveAll(c.dir); rmErr != nil {
		err = errors.Join(err, fmt.Errorf("mirror: the copy in %s could not be removed, and is a copy of the venue's sales: remove it by hand: %w", c.dir, rmErr))
	}
	return err
}

// mirrorHolder is what the listener serves: whichever copy is current. A
// newer copy takes the place of the one before it between two requests, and
// the one before is closed once the requests it was answering are done.
type mirrorHolder struct {
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
}

func (h *mirrorHolder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	c := h.current
	// Counted while the lock is held, so a copy is never taken up after
	// the swap that retires it has looked at the count.
	c.inFlight.Add(1)
	h.asked = time.Now()
	h.mu.Unlock()
	defer c.inFlight.Done()
	c.handler.ServeHTTP(w, r)
}

// state is the copy being served and when anyone last asked for anything.
func (h *mirrorHolder) state() (generation string, asOf, asked time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.current.generation, h.current.asOf, h.asked
}

// swap puts a newer copy in the old one's place. It reports false, and
// takes nothing, once the mirror is stopping.
func (h *mirrorHolder) swap(next *mirrorCopy) bool {
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
// retiring. The server has stopped taking requests by now, but one it
// could not finish in its own time may still be running: it is waited for
// as long as a retiring copy waits, and no longer. The mirror stops, and
// the copy goes, whatever a stalled client does.
func (h *mirrorHolder) close() error {
	h.mu.Lock()
	h.closed = true
	current := h.current
	h.mu.Unlock()

	// Each retiring copy waits patience and no more, so this ends.
	h.retiring.Wait()
	if !waited(&current.inFlight, h.patience) {
		h.log.Warn("the mirror stopped with requests still being answered", "after", h.patience.String())
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return errors.Join(append(h.problems, current.close())...)
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
func (m *mirrorSource) follow(ctx context.Context, held *mirrorHolder, every time.Duration, only string) {
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

		newest, err := m.newest(ctx, only)
		if err != nil {
			if ctx.Err() == nil {
				m.log.Warn("the mirror could not look at its backup, and goes on showing what it has", "error", err)
			}
			continue
		}
		if newest.Name == generation && !newest.Last.After(asOf) {
			continue
		}
		if newest.Name == refused.Name && newest.Last.Equal(refused.Last) && time.Since(refusedAt) < mirrorRetryAfter {
			continue
		}
		next, err := m.open(ctx, newest)
		if err == nil {
			if err = next.serve(m); err != nil {
				err = errors.Join(err, next.close())
			}
		}
		if err != nil {
			if ctx.Err() == nil {
				refused, refusedAt = newest, time.Now()
				m.log.Warn("the mirror could not take a newer copy of its backup, and goes on showing what it has",
					"as_of", asOf, "tries_again_after", mirrorRetryAfter.String(), "error", err)
			}
			continue
		}
		if !held.swap(next) {
			if err := next.close(); err != nil {
				m.log.Error("a copy made as the mirror stopped could not be put away", "error", err)
			}
			return
		}
	}
}

// newest is the generation a mirror should be showing: the one it was
// told to, or the newest that holds anything.
func (m *mirrorSource) newest(ctx context.Context, only string) (backup.Generation, error) {
	found, err := backup.Generations(ctx, m.from)
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

// mirrorServer is the API over a copy, as a mirror serves it: what the back
// office reads, and nothing that reaches out. The print queue is built and
// never started, so a ticket that was waiting when the backup was cut is
// not printed from here. There is no stream, tunnel, backup or gateway.
//
// Who is let in is the mirror's own affair: door reads people and their
// PINs from the copy and remembers browsers, sessions and wrong PINs in the
// keeper, so the copy is not written (ADR-327).
func mirrorServer(database *db.DB, keeper *mirror.Keeper, venueID int64, photosDir, listenAddr string, log *slog.Logger, info func() api.MirrorInfo) (*api.Server, error) {
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
	door, err := mirror.NewAuth(mirror.Options{Keeper: keeper, Staff: staff, Devices: devices, VenueID: venueID, Log: log})
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
	printerSettings, err := printers.New(database, prints, nil, buildInfo().Version)
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

	cfg := config.Config{
		ListenAddr: listenAddr, DataDir: filepath.Dir(photosDir),
		LogLevel: slog.LevelWarn, LogFormat: config.LogFormatText, Timezone: time.Local,
	}
	srv := api.New(cfg, log, buildInfo(),
		api.WithMirror(info),
		api.WithStatic(bundles),
		api.WithDBCheck(dbCheck(database)),
		api.WithDevices(door),
		api.WithFirstRun(firstRun),
		api.WithStaff(door),
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

// parseMirrorFlags reads the command line for mirror.
func parseMirrorFlags(args []string, out io.Writer) (mirrorOptions, error) {
	var o mirrorOptions
	set := flag.NewFlagSet("mez mirror", flag.ContinueOnError)
	set.SetOutput(out)
	set.Usage = func() {
		fmt.Fprint(out, strings.TrimLeft(`
mez mirror opens the back office over a copy of a venue's backup, to read:
the reports, the menu, the stock, as they were when the backup was last
written. Nothing can be changed in it, and the venue does not need to be on.

  mez mirror -from D:\mez-backup
  mez mirror -bucket corner-cafe -endpoint https://ACCOUNT.r2.cloudflarestorage.com -access-key-id KEY
      (with `+restoreSecretEnv+` set to the secret)

`, "\n"))
		set.PrintDefaults()
	}
	set.StringVar(&o.From, "from", "", "the folder the backup is in")
	set.StringVar(&o.ConfigFile, "config", "", "a saved backup.json naming the destination, in place of -from or -bucket")
	set.StringVar(&o.Bucket.Bucket, "bucket", "", "the bucket the backup is in")
	set.StringVar(&o.Bucket.Path, "path", "", "the folder in the bucket (default mez)")
	set.StringVar(&o.Bucket.Endpoint, "endpoint", "", "the service's address, for anything but Amazon S3")
	set.StringVar(&o.Bucket.Region, "region", "", "the bucket's region, for Amazon S3")
	set.StringVar(&o.Bucket.AccessKeyID, "access-key-id", "", "the bucket's access key id; the secret is read from "+restoreSecretEnv)
	set.StringVar(&o.Generation, "generation", "", "show this backup and not the newest, by its name from `mez restore -list`")
	set.StringVar(&o.ListenAddr, "listen-addr", mirrorListenAddr, "address to listen on, host:port")
	set.DurationVar(&o.Follow, "follow", mirrorFollowEvery, "how often, while it is being used, to look at the backup for something newer; 0 for never")
	set.StringVar(&o.StateDir, "state-dir", "", "where to remember the browsers paired to this mirror and the locks wrong PINs set (default: mez/mirror in your configuration directory)")
	if err := set.Parse(args); err != nil {
		return mirrorOptions{}, err
	}
	if set.NArg() > 0 {
		return mirrorOptions{}, fmt.Errorf("mirror: unexpected argument %q", set.Arg(0))
	}
	if o.Follow < 0 || (o.Follow > 0 && o.Follow < mirrorFollowFloor) {
		return mirrorOptions{}, fmt.Errorf("mirror: -follow %s: %s at the least, or 0 for never", o.Follow, mirrorFollowFloor)
	}
	if _, _, err := net.SplitHostPort(o.ListenAddr); err != nil {
		return mirrorOptions{}, fmt.Errorf("mirror: -listen-addr %q is not host:port", o.ListenAddr)
	}
	return o, nil
}
