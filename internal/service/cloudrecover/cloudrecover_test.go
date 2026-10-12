package cloudrecover

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/restore"
	"github.com/byjoellobo/mez/internal/service/cloudpair"
)

const (
	testPhone   = "+919876543210"
	testOTP     = "482913"
	testSession = "ses_not-a-real-session-token"
	testReadKey = "not-a-real-read-only-secret"
	testCode    = "ABCD2345"
	testVenue   = "ven_A1B2C3D4"
)

// standIn is mez Cloud's three recovery routes, for one account.
type standIn struct {
	mu     sync.Mutex
	asked  map[string]int
	grants int
}

func (s *standIn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	s.mu.Lock()
	s.asked[r.URL.Path]++
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/recover/start":
		_, _ = io.WriteString(w, `{"attempt":"att_1"}`)
	case "/v1/recover/verify":
		if body["code"] != testOTP {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = io.WriteString(w, `{"session":"`+testSession+`","expires_at":"2026-10-12T21:45:00Z","owner":{"name":"Priya Menon"},
			"venues":[{"id":"`+testVenue+`","name":"Corner Cafe","last_backup_at":"2026-10-12T16:02:30Z"},
			          {"id":"ven_E5F6G7H8","name":"Never Backed Up"}]}`)
	case "/v1/recover/grant":
		if r.Header.Get("Authorization") != "Bearer "+testSession {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		s.grants++
		s.mu.Unlock()
		_, _ = io.WriteString(w, `{"backup":{"bucket":"mez-venues","path":"v/corner-cafe","endpoint":"https://acct.r2.cloudflarestorage.com",
			"access_key_id":"AKIDREADONLY","secret_access_key":"`+testReadKey+`"},"pairing_code":"`+testCode+`","expires_at":"2026-10-12T22:30:00Z"}`)
	default:
		http.NotFound(w, r)
	}
}

func (s *standIn) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asked[path]
}

// world is a computer with no venue, a cloud, and somewhere to see what the
// service did with them.
type world struct {
	cloud  *standIn
	needed bool
	log    strings.Builder

	mu       sync.Mutex
	staged   []restore.StageOptions
	stageCtx []context.Context
	stageErr error
	photoErr error
	hold     chan struct{} // when not nil, Stage waits on it
	starts   int

	opts Options
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{cloud: &standIn{asked: map[string]int{}}, needed: true}
	srv := httptest.NewServer(w.cloud)
	t.Cleanup(srv.Close)
	client, err := cloud.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	w.opts = Options{
		Cloud: client, DataDir: t.TempDir(),
		Needed:     func(context.Context) (bool, error) { return w.needed, nil },
		StartAgain: func() { w.mu.Lock(); w.starts++; w.mu.Unlock() },
		Stage: func(ctx context.Context, o restore.StageOptions) (restore.Staged, error) {
			w.mu.Lock()
			w.staged, w.stageCtx = append(w.staged, o), append(w.stageCtx, ctx)
			hold, err, photoErr := w.hold, w.stageErr, w.photoErr
			w.mu.Unlock()
			if hold != nil {
				<-hold
			}
			if err != nil {
				return restore.Staged{}, err
			}
			return restore.Staged{VenueID: o.VenueID, VenueName: "Corner Cafe", StagedAt: time.Now(), Photos: 3, PhotosErr: photoErr}, nil
		},
		Log: slog.New(slog.NewJSONHandler(&w.log, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	return w
}

func (w *world) service(t *testing.T) *Service {
	t.Helper()
	s, err := New(w.opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func (w *world) signedIn(t *testing.T) *Service {
	t.Helper()
	s := w.service(t)
	attempt, err := s.Start(context.Background(), testPhone)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Verify(context.Background(), attempt, testOTP); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return s
}

// TestAVenueIsBroughtBack: the owner signs in, chooses, and the venue is
// staged with the cloud's read of its backup and its code, and the server
// asked to start again, once.
func TestAVenueIsBroughtBack(t *testing.T) {
	w := newWorld(t)
	s := w.service(t)
	ctx := context.Background()

	attempt, err := s.Start(ctx, " +91 98765-43210 ")
	if err != nil || attempt != "att_1" {
		t.Fatalf("Start = %q, %v", attempt, err)
	}
	account, err := s.Verify(ctx, attempt, "482 913")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if account.Owner != "Priya Menon" || len(account.Venues) != 2 || account.Venues[0].ID != testVenue || account.ExpiresAt.IsZero() {
		t.Errorf("signed in as %+v", account)
	}

	got, err := s.Restore(ctx, testVenue)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got.VenueID != testVenue || got.VenueName != "Corner Cafe" || got.Photos != 3 || got.PhotosIncomplete || len(got.Claim) != 2*claimBytes {
		t.Errorf("restored %+v", got)
	}
	if len(w.staged) != 1 {
		t.Fatalf("staged %d times, want once", len(w.staged))
	}
	o := w.staged[0]
	want := backup.Config{Kind: backup.KindS3, S3: backup.S3{
		Bucket: "mez-venues", Path: "v/corner-cafe", Endpoint: "https://acct.r2.cloudflarestorage.com", AccessKeyID: "AKIDREADONLY", SecretAccessKey: testReadKey,
	}}
	if o.DataDir != w.opts.DataDir || o.From != want || o.VenueID != testVenue || o.PairingCode != testCode {
		t.Errorf("staged with %+v", o)
	}
	// The claim is given to the page and only its hash is left to wait.
	if o.ClaimHash != HashClaim(got.Claim) || o.ClaimHash == got.Claim || o.ClaimHash == "" {
		t.Errorf("the hash left to wait is %q, for the claim %q", o.ClaimHash, got.Claim)
	}
	if w.starts != 1 {
		t.Errorf("the server was asked to start again %d times, want once", w.starts)
	}

	// Once. The server is about to start again on what was staged.
	if _, err := s.Restore(ctx, testVenue); !errors.Is(err, ErrUnderWay) {
		t.Errorf("a second restore = %v, want ErrUnderWay", err)
	}
	if w.cloud.grants != 1 || len(w.staged) != 1 || w.starts != 1 {
		t.Errorf("a second restore asked again: %d grants, %d stagings, %d starts", w.cloud.grants, len(w.staged), w.starts)
	}
	for _, secret := range []string{got.Claim, testCode, testReadKey, testSession} {
		if strings.Contains(w.log.String(), secret) {
			t.Errorf("a secret (%s) is in the log:\n%s", secret, w.log.String())
		}
	}

	// Two claims are never the same.
	other := newWorld(t)
	again, err := other.signedIn(t).Restore(ctx, testVenue)
	if err != nil || again.Claim == got.Claim {
		t.Errorf("another restore's claim: %q, %v", again.Claim, err)
	}
}

// TestRecoveryIsRefused: before anything is asked of the cloud that need
// not be.
func TestRecoveryIsRefused(t *testing.T) {
	ctx := context.Background()
	t.Run("a computer that has its venue", func(t *testing.T) {
		w := newWorld(t)
		s := w.signedIn(t)
		before := len(w.cloud.asked)
		w.needed = false
		if _, err := s.Start(ctx, testPhone); !errors.Is(err, ErrAlreadySetUp) {
			t.Errorf("Start = %v, want ErrAlreadySetUp", err)
		}
		if _, err := s.Verify(ctx, "att_1", testOTP); !errors.Is(err, ErrAlreadySetUp) {
			t.Errorf("Verify = %v, want ErrAlreadySetUp", err)
		}
		if _, err := s.Restore(ctx, testVenue); !errors.Is(err, ErrAlreadySetUp) {
			t.Errorf("Restore = %v, want ErrAlreadySetUp", err)
		}
		if w.cloud.count("/v1/recover/start") != 1 || w.cloud.count("/v1/recover/verify") != 1 || w.cloud.grants != 0 || len(w.staged) != 0 || before == 0 {
			t.Errorf("the cloud was asked on behalf of a computer with a venue: %+v", w.cloud.asked)
		}
	})
	t.Run("whether there is a venue cannot be read", func(t *testing.T) {
		w := newWorld(t)
		w.opts.Needed = func(context.Context) (bool, error) { return false, errors.New("disk") }
		if _, err := w.service(t).Start(ctx, testPhone); err == nil || errors.Is(err, ErrAlreadySetUp) {
			t.Errorf("Start = %v, want the error", err)
		}
	})
	t.Run("nobody signed in", func(t *testing.T) {
		w := newWorld(t)
		if _, err := w.service(t).Restore(ctx, testVenue); !errors.Is(err, cloud.ErrSessionOver) {
			t.Errorf("Restore = %v, want ErrSessionOver", err)
		}
	})
	t.Run("a wrong one-time code signs nobody in", func(t *testing.T) {
		w := newWorld(t)
		s := w.service(t)
		if _, err := s.Verify(ctx, "att_1", "000000"); !errors.Is(err, cloud.ErrOTPRefused) {
			t.Errorf("Verify = %v, want ErrOTPRefused", err)
		}
		if _, err := s.Restore(ctx, testVenue); !errors.Is(err, cloud.ErrSessionOver) {
			t.Errorf("Restore after it = %v, want ErrSessionOver", err)
		}
	})
	t.Run("a venue that is not the account's, or was never backed up", func(t *testing.T) {
		w := newWorld(t)
		s := w.signedIn(t)
		if _, err := s.Restore(ctx, "ven_ZZZZ9999"); !errors.Is(err, cloud.ErrNotListed) {
			t.Errorf("Restore = %v, want ErrNotListed", err)
		}
		if _, err := s.Restore(ctx, "ven_E5F6G7H8"); !errors.Is(err, cloud.ErrNeverBackedUp) {
			t.Errorf("Restore = %v, want ErrNeverBackedUp", err)
		}
		if len(w.staged) != 0 || w.starts != 0 {
			t.Errorf("staged %d, started %d", len(w.staged), w.starts)
		}
		// Neither spent the sign-in.
		if _, err := s.Restore(ctx, testVenue); err != nil {
			t.Errorf("the venue that is theirs, after: %v", err)
		}
	})
	t.Run("what New needs", func(t *testing.T) {
		w := newWorld(t)
		for _, spoil := range []func(*Options){
			func(o *Options) { o.Cloud = nil }, func(o *Options) { o.DataDir = "" },
			func(o *Options) { o.Needed = nil }, func(o *Options) { o.StartAgain = nil },
		} {
			o := w.opts
			spoil(&o)
			if _, err := New(o); err == nil {
				t.Errorf("New(%+v) was accepted", o)
			}
		}
	})
}

// TestAStagingThatFails: nothing starts again, and the owner tries once
// more without signing in again.
func TestAStagingThatFails(t *testing.T) {
	w := newWorld(t)
	s := w.signedIn(t)
	w.stageErr = restore.ErrWrongVenue
	if _, err := s.Restore(context.Background(), testVenue); !errors.Is(err, restore.ErrWrongVenue) {
		t.Fatalf("Restore = %v, want ErrWrongVenue", err)
	}
	if w.starts != 0 {
		t.Error("the server was asked to start again on nothing")
	}
	if strings.Contains(w.log.String(), testCode) || strings.Contains(w.log.String(), testReadKey) {
		t.Errorf("a secret is in the log:\n%s", w.log.String())
	}

	w.stageErr, w.photoErr = nil, errors.New("one photo would not come")
	got, err := s.Restore(context.Background(), testVenue)
	if err != nil || !got.PhotosIncomplete || got.Photos != 3 || w.starts != 1 {
		t.Errorf("the second try = %+v, %v, %d starts; want a venue back with not all its photos", got, err, w.starts)
	}
}

// TestASignInMadeWhileAVenueComesDownIsNotThrownAway: the restore ends the
// sign-in it used, and no other.
func TestASignInMadeWhileAVenueComesDownIsNotThrownAway(t *testing.T) {
	w := newWorld(t)
	s := w.signedIn(t)
	used := s.signedIn
	w.hold = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := s.Restore(context.Background(), testVenue)
		done <- err
	}()
	for deadline := time.Now().Add(5 * time.Second); ; {
		w.mu.Lock()
		n := len(w.staged)
		w.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the restore never began staging")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := s.Verify(context.Background(), "att_1", testOTP); err != nil {
		t.Fatalf("a second sign-in: %v", err)
	}
	close(w.hold)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	now := s.signedIn
	s.mu.Unlock()
	if now == nil || now == used {
		t.Errorf("after the restore the sign-in held is %v; want the one made since", now)
	}
	// And the staging was given an end.
	if _, has := w.stageCtx[0].Deadline(); !has {
		t.Error("the staging could go on for ever")
	}
}

// TestOneRestoreAtATime: a second asked for while the first is coming down
// is refused, and asks the cloud for nothing.
func TestOneRestoreAtATime(t *testing.T) {
	w := newWorld(t)
	s := w.signedIn(t)
	w.hold = make(chan struct{})

	first := make(chan error, 1)
	go func() {
		_, err := s.Restore(context.Background(), testVenue)
		first <- err
	}()
	// Until the first is inside Stage.
	for deadline := time.Now().Add(5 * time.Second); ; {
		w.mu.Lock()
		n := len(w.staged)
		w.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first restore never began staging")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := s.Restore(context.Background(), testVenue); !errors.Is(err, ErrUnderWay) {
		t.Errorf("a restore beside one under way = %v, want ErrUnderWay", err)
	}
	close(w.hold)
	if err := <-first; err != nil {
		t.Fatalf("the first restore: %v", err)
	}
	if w.cloud.grants != 1 || w.starts != 1 {
		t.Errorf("%d grants and %d starts, want one of each", w.cloud.grants, w.starts)
	}
}

// TestAPageThatGoesAwayDoesNotStopTheStaging: once the cloud has handed the
// venue over.
func TestAPageThatGoesAwayDoesNotStopTheStaging(t *testing.T) {
	w := newWorld(t)
	s := w.signedIn(t)
	w.hold = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := s.Restore(ctx, testVenue)
		done <- err
	}()
	for deadline := time.Now().Add(5 * time.Second); ; {
		w.mu.Lock()
		n := len(w.staged)
		w.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the restore never began staging")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel() // the browser is closed
	if err := w.stageCtx[0].Err(); err != nil {
		t.Errorf("the staging was told to stop with the page: %v", err)
	}
	close(w.hold)
	if err := <-done; err != nil || w.starts != 1 {
		t.Errorf("the restore = %v with %d starts; want it finished", err, w.starts)
	}
}

// --- after the start ---

// fakeCodes makes device codes that say how many it has made.
type fakeCodes struct {
	made []auth.NewCode
	err  error
}

func (f *fakeCodes) CreatePairingCode(_ context.Context, n auth.NewCode) (auth.PairingCode, error) {
	if f.err != nil {
		return auth.PairingCode{}, f.err
	}
	f.made = append(f.made, n)
	return auth.PairingCode{Code: "DEV1-000" + string(rune('0'+len(f.made))), ExpiresAt: time.Date(2026, 10, 12, 17, 0, 0, 0, time.UTC)}, nil
}

const testClaim = "the-claim-the-page-was-given"

func adopted() restore.Pending {
	return restore.Pending{VenueID: testVenue, VenueName: "Corner Cafe", PairingCode: testCode, ClaimHash: HashClaim(testClaim)}
}

func waitFor(t *testing.T, a *Arrival, want CloudState) Outcome {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		got, err := a.Outcome(testClaim)
		if err != nil {
			t.Fatalf("Outcome: %v", err)
		}
		if got.Cloud == want {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("the pairing is %q, and never became %q", got.Cloud, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestAnArrivalPairsWithTheCodeThatWaited: in the background, once, and
// says so to the page that holds the claim and to nobody else.
func TestAnArrivalPairsWithTheCodeThatWaited(t *testing.T) {
	var log strings.Builder
	release := make(chan struct{})
	var pairedVenue int64
	var pairedCode string
	a, err := Arrive(ArrivalOptions{
		Adopted: adopted(), VenueID: 7, Codes: &fakeCodes{},
		Pair: func(_ context.Context, venueID int64, code string) (cloudpair.Result, error) {
			<-release
			pairedVenue, pairedCode = venueID, code
			return cloudpair.Result{Owner: "Priya Menon", Hostname: "corner-cafe.mezpos.com"}, nil
		},
		Log: slog.New(slog.NewJSONHandler(&log, nil)),
	})
	if err != nil {
		t.Fatalf("Arrive: %v", err)
	}
	defer a.Close(context.Background())

	got, err := a.Outcome(testClaim)
	if err != nil || got != (Outcome{VenueID: testVenue, VenueName: "Corner Cafe", Cloud: CloudPairing}) {
		t.Errorf("while it is asked for: %+v, %v", got, err)
	}
	for _, claim := range []string{"", "another-claim", HashClaim(testClaim), testCode} {
		if got, err := a.Outcome(claim); !errors.Is(err, ErrNoClaim) || got != (Outcome{}) {
			t.Errorf("Outcome(%q) = %+v, %v; want ErrNoClaim and nothing", claim, got, err)
		}
	}

	close(release)
	got = waitFor(t, a, CloudPaired)
	if got.Hostname != "corner-cafe.mezpos.com" || pairedVenue != 7 || pairedCode != testCode {
		t.Errorf("paired %+v, venue %d with %q", got, pairedVenue, pairedCode)
	}
	if strings.Contains(log.String(), testCode) || strings.Contains(log.String(), testClaim) {
		t.Errorf("a secret is in the log:\n%s", log.String())
	}
}

func TestAnArrivalThatCannotPair(t *testing.T) {
	t.Run("the cloud refuses, or is not there", func(t *testing.T) {
		a, err := Arrive(ArrivalOptions{Adopted: adopted(), VenueID: 7, Codes: &fakeCodes{},
			Pair: func(context.Context, int64, string) (cloudpair.Result, error) {
				return cloudpair.Result{}, cloud.ErrUnreachable
			}})
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close(context.Background())
		if got := waitFor(t, a, CloudNotPaired); got.Hostname != "" || got.VenueName != "Corner Cafe" {
			t.Errorf("outcome %+v", got)
		}
	})
	t.Run("no code waited", func(t *testing.T) {
		p := adopted()
		p.PairingCode = ""
		a, err := Arrive(ArrivalOptions{Adopted: p, VenueID: 7, Codes: &fakeCodes{},
			Pair: func(context.Context, int64, string) (cloudpair.Result, error) {
				t.Error("pairing was asked for with no code")
				return cloudpair.Result{}, nil
			}})
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := a.Outcome(testClaim); got.Cloud != CloudNone {
			t.Errorf("outcome %+v, want none", got)
		}
		if err := a.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	t.Run("a server that cannot pair", func(t *testing.T) {
		a, err := Arrive(ArrivalOptions{Adopted: adopted(), VenueID: 7, Codes: &fakeCodes{}})
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := a.Outcome(testClaim); got.Cloud != CloudNotPaired {
			t.Errorf("outcome %+v, want not paired", got)
		}
	})
	t.Run("a restore nobody holds a claim to", func(t *testing.T) {
		p := adopted()
		p.ClaimHash = ""
		a, err := Arrive(ArrivalOptions{Adopted: p, VenueID: 7, Codes: &fakeCodes{}})
		if err != nil {
			t.Fatal(err)
		}
		for _, claim := range []string{"", testClaim} {
			if _, err := a.Outcome(claim); !errors.Is(err, ErrNoClaim) {
				t.Errorf("Outcome(%q) = %v, want ErrNoClaim", claim, err)
			}
			if _, err := a.DeviceCode(context.Background(), claim, "Front counter", auth.KindTerminal); !errors.Is(err, ErrNoClaim) {
				t.Errorf("DeviceCode(%q) = %v, want ErrNoClaim", claim, err)
			}
		}
	})
	t.Run("a start that adopted nothing", func(t *testing.T) {
		var none *Arrival
		if _, err := none.Outcome(testClaim); !errors.Is(err, ErrNoClaim) {
			t.Errorf("Outcome = %v, want ErrNoClaim", err)
		}
		if _, err := none.DeviceCode(context.Background(), testClaim, "Front counter", auth.KindTerminal); !errors.Is(err, ErrNoClaim) {
			t.Errorf("DeviceCode = %v, want ErrNoClaim", err)
		}
	})
	t.Run("what Arrive needs", func(t *testing.T) {
		for _, o := range []ArrivalOptions{
			{VenueID: 7, Codes: &fakeCodes{}}, {Adopted: adopted(), Codes: &fakeCodes{}}, {Adopted: adopted(), VenueID: 7},
		} {
			if _, err := Arrive(o); err == nil {
				t.Errorf("Arrive(%+v) was accepted", o)
			}
		}
	})
}

// TestClosingAnArrivalStopsAPairingStillAskedFor: a server asked to stop
// does not wait on a cloud that is not answering.
func TestClosingAnArrivalStopsAPairingStillAskedFor(t *testing.T) {
	a, err := Arrive(ArrivalOptions{Adopted: adopted(), VenueID: 7, Codes: &fakeCodes{},
		Pair: func(ctx context.Context, _ int64, _ string) (cloudpair.Result, error) {
			<-ctx.Done()
			return cloudpair.Result{}, ctx.Err()
		}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got, _ := a.Outcome(testClaim); got.Cloud != CloudNotPaired {
		t.Errorf("after closing: %+v, want not paired", got)
	}
}

// TestAClaimRunsOut: half an hour after the start, it buys nothing.
func TestAClaimRunsOut(t *testing.T) {
	now := time.Date(2026, 10, 12, 16, 30, 0, 0, time.UTC)
	p := adopted()
	p.PairingCode = ""
	a, err := Arrive(ArrivalOptions{Adopted: p, VenueID: 7, Codes: &fakeCodes{}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(ClaimLife - time.Second)
	if _, err := a.Outcome(testClaim); err != nil {
		t.Errorf("inside its life: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := a.Outcome(testClaim); !errors.Is(err, ErrNoClaim) {
		t.Errorf("Outcome after its life = %v, want ErrNoClaim", err)
	}
	if _, err := a.DeviceCode(context.Background(), testClaim, "Front counter", auth.KindTerminal); !errors.Is(err, ErrNoClaim) {
		t.Errorf("DeviceCode after its life = %v, want ErrNoClaim", err)
	}
}

// TestAStopDoesNotWaitOnAPairingPastItsExchange: one that no longer listens
// to being stopped is left, and the stop's time is not spent on it.
func TestAStopDoesNotWaitOnAPairingPastItsExchange(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	a, err := Arrive(ArrivalOptions{Adopted: adopted(), VenueID: 7, Codes: &fakeCodes{},
		Pair: func(context.Context, int64, string) (cloudpair.Result, error) {
			<-release
			return cloudpair.Result{}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := a.Close(context.Background()); err == nil {
		t.Error("Close said the pairing had stopped")
	}
	if took := time.Since(began); took > closeWait+2*time.Second {
		t.Errorf("Close waited %v on a pairing that was not going to stop", took)
	}
}

// TestADeviceCodeForTheBrowserThatAsked: one to a claim.
func TestADeviceCodeForTheBrowserThatAsked(t *testing.T) {
	codes := &fakeCodes{}
	p := adopted()
	p.PairingCode = ""
	a, err := Arrive(ArrivalOptions{Adopted: p, VenueID: 7, Codes: codes})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := a.DeviceCode(ctx, "another-claim", "Front counter", auth.KindTerminal); !errors.Is(err, ErrNoClaim) {
		t.Errorf("a stranger's DeviceCode = %v, want ErrNoClaim", err)
	}
	if len(codes.made) != 0 {
		t.Fatal("a code was made for a stranger")
	}

	// A refusal is not the one code.
	codes.err = &auth.InvalidDevice{Field: "name", Message: "a device needs a name"}
	var invalid *auth.InvalidDevice
	if _, err := a.DeviceCode(ctx, testClaim, "", auth.KindTerminal); !errors.As(err, &invalid) {
		t.Errorf("DeviceCode with no name = %v, want the refusal", err)
	}
	codes.err = nil

	first, err := a.DeviceCode(ctx, testClaim, "Front counter", auth.KindTerminal)
	if err != nil || first.Code == "" {
		t.Fatalf("DeviceCode = %+v, %v", first, err)
	}
	if codes.made[0] != (auth.NewCode{VenueID: 7, DeviceName: "Front counter", DeviceKind: auth.KindTerminal}) {
		t.Errorf("the code was made for %+v", codes.made[0])
	}
	again, err := a.DeviceCode(ctx, testClaim, "Something else", auth.KindKitchen)
	if err != nil || again != first || len(codes.made) != 1 {
		t.Errorf("asked again: %+v, %v, %d made; want the same one code", again, err, len(codes.made))
	}
}
