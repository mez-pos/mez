package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/mirror"
	"github.com/mez-pos/mez/web"
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
const (
	mirrorFollowEvery = time.Minute
	mirrorFollowFloor = time.Second
)

// leftCopies is the mirror directories already under dir: those of mirrors
// running now, and of any that was killed before it could remove its own.
func leftCopies(dir string) []string {
	found, err := filepath.Glob(filepath.Join(dir, mirror.DirPrefix+"*"))
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
	// CloudKey is mez Cloud's public key, as the venue's cloud.json has
	// it, and nil for a mirror the cloud signs nobody in to (ADR-331).
	CloudKey ed25519.PublicKey
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
// is interrupted, and keeps nothing afterwards. The mirror itself is
// internal/mirror's, where mez Cloud reaches it too (ADR-353); this is the
// command around it.
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

	// Asked here first for the words: a backup named and not there is told
	// how to find the ones that are. The mirror chooses again, from the
	// flag and not from this answer, so that with none it follows whatever
	// backup is newest, a new one begun since included.
	if _, err := chooseGeneration(ctx, from, opts.Generation); err != nil {
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

	// What the mirror remembers of its own is kept where a restart finds
	// it: the browsers paired to it and the locks wrong PINs have set.
	stateDir, err := opts.stateDir()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Reading the backup at %s\n", from.Destination())
	m, err := mirror.Open(ctx, mirror.OpenOptions{
		From: from, Generation: opts.Generation, StateDir: stateDir,
		CloudKey: opts.CloudKey, Build: buildInfo(), Log: log, Patience: shutdownTimeout,
	})
	if err != nil {
		return err
	}
	defer func() {
		// Whichever copy is being served when the mirror stops, and any
		// still finishing its last requests, are closed and removed.
		err = errors.Join(err, m.Close())
	}()

	// One more browser, paired with a code as any back office is. Those
	// paired before are remembered and need none.
	keeper := m.Keeper()
	code, codeEnds, err := keeper.NewCode()
	if err != nil {
		return err
	}
	known := len(keeper.Browsers())

	following := "It does not follow the backup: it stays as it was when started."
	if opts.Follow > 0 {
		following = fmt.Sprintf("While it is being used it looks at the backup every %s and shows what is newer.", opts.Follow)
	}
	if opts.CloudKey != nil {
		following += "\nmez Cloud can sign the account's owner in here, with the key it was given."
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

`, m.Venue().Name, localTime(m.AsOf()), addr, web.MountBackoffice, code, codeEnds.Local().Format("15:04"), known, stateDir, following)

	// Following stops before the copies are closed: it is what makes them.
	followed := make(chan struct{})
	followCtx, stopFollowing := context.WithCancel(ctx)
	go func() {
		defer close(followed)
		if opts.Follow > 0 {
			m.Follow(followCtx, opts.Follow)
		}
	}()
	defer func() {
		stopFollowing()
		<-followed
	}()

	if ready != nil {
		ready(mirrorUp{Addr: addr, Code: code, AsOf: m.AsOf(), Dir: m.Dir()})
	}
	return serve(ctx, log, listener, m, shutdownTimeout)
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
	cloudKey := set.String("cloud-key", "", "mez Cloud's public key, as `public_key` in the venue's cloud.json has it: lets the account's owner in on mez Cloud's word, with no PIN")
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
	if *cloudKey != "" {
		// Read as the pairing reads its own, so the two cannot disagree
		// about what a key looks like.
		key, err := cloud.Pairing{PublicKey: strings.TrimSpace(*cloudKey)}.Key()
		if err != nil {
			return mirrorOptions{}, fmt.Errorf("mirror: -cloud-key is not mez Cloud's public key as cloud.json has it (44 characters of base64): %w", err)
		}
		o.CloudKey = key
	}
	return o, nil
}
