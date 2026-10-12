package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/backup"
)

// startMirror runs `mez mirror` over a backup until the test ends, and
// returns where it is serving and what it printed.
func startMirror(t *testing.T, to backup.Config, stateDir string, more ...string) (mirrorUp, *bytes.Buffer, func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var said bytes.Buffer
	up := make(chan mirrorUp, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveMirror(ctx, append([]string{"-from", to.Folder, "-listen-addr", "127.0.0.1:0", "-state-dir", stateDir}, more...), &said, func(m mirrorUp) { up <- m })
	}()
	// Stopped once, by the test or at its end, whichever comes first.
	var once sync.Once
	var stopped error
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case stopped = <-done:
			case <-time.After(shutdownTimeout + 5*time.Second):
				t.Error("the mirror did not stop")
			}
		})
		return stopped
	}
	select {
	case m := <-up:
		t.Cleanup(func() { _ = stop() })
		return m, &said, stop
	case err := <-done:
		cancel()
		t.Fatalf("the mirror did not start: %v\n%s", err, said.String())
	case <-time.After(60 * time.Second):
		cancel()
		t.Fatalf("the mirror did not start in a minute:\n%s", said.String())
	}
	return mirrorUp{}, nil, nil
}

// ask sends one request to a mirror and returns the status, the body and
// the headers.
func ask(t *testing.T, method, url, body string, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw, resp.Header
}

// filesIn is every file under dir with its size, for telling whether a
// directory was touched.
func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		out = append(out, rel+" "+info.ModTime().UTC().Format(time.RFC3339Nano)+" "+string(rune('0'+info.Size()%10)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// TestAMirrorShowsTheVenueAndChangesNothing is `mez mirror` on the evening
// it is for: the venue's computer is off, and there is its backup folder.
// The owner pairs a browser with the code it prints, signs in with their
// own PIN, and reads what the venue held. Nothing they try to change is
// changed, in the copy's venue or in the backup.
func TestAMirrorShowsTheVenueAndChangesNothing(t *testing.T) {
	ctx := context.Background()
	dataDir, to, photo := backedUpVenue(t)
	before := filesIn(t, to.Folder)

	// What the venue itself holds, to hold the mirror to.
	source := opened(t, dataDir)
	var ownerID, venueName string
	if err := source.Reader().QueryRowContext(ctx, `SELECT s.public_id, v.name FROM staff s JOIN venues v ON v.id = s.venue_id WHERE s.name = 'Priya'`).Scan(&ownerID, &venueName); err != nil {
		t.Fatal(err)
	}
	var staffNames []string
	rows, err := source.Reader().QueryContext(ctx, `SELECT name FROM staff ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		staffNames = append(staffNames, n)
	}
	_ = rows.Close()
	var sourceDevices, sourceSessions int
	if err := source.Reader().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM devices), (SELECT COUNT(*) FROM staff_sessions)`).Scan(&sourceDevices, &sourceSessions); err != nil {
		t.Fatal(err)
	}
	generations, err := backup.Generations(ctx, to)
	if err != nil || len(generations) == 0 {
		t.Fatalf("the backup's generations: %v, %v", generations, err)
	}

	stateDir := t.TempDir()
	up, said, stop := startMirror(t, to, stateDir)
	base := "http://" + up.Addr
	if !strings.Contains(said.String(), "A mirror of "+venueName) || !strings.Contains(said.String(), up.Code) {
		t.Errorf("the mirror did not say whose it is and how to pair:\n%s", said.String())
	}
	if !up.AsOf.Equal(generations[0].Last) {
		t.Errorf("as of %s, and the backup was last written %s", up.AsOf, generations[0].Last)
	}
	if strings.Contains(up.Dir, dataDir) || strings.HasPrefix(up.Dir, to.Folder) {
		t.Errorf("the copy is kept in %s, inside the venue or its backup", up.Dir)
	}

	// The back office itself is served.
	if status, _, _ := ask(t, http.MethodGet, base+"/backoffice/", "", nil); status != http.StatusOK {
		t.Errorf("GET /backoffice/: status %d", status)
	}

	// Pair the browser, sign in as the owner with the venue's own PIN.
	status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/devices/pair", `{"code":"`+up.Code+`"}`, nil)
	if status != http.StatusCreated {
		t.Fatalf("pairing to the mirror: status %d, body %s", status, raw)
	}
	var paired struct {
		Token  string `json:"token"`
		Device struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"device"`
	}
	if err := json.Unmarshal(raw, &paired); err != nil {
		t.Fatal(err)
	}
	if paired.Device.Name != "Mirror" || paired.Device.Kind != "back_office" {
		t.Errorf("paired as %+v", paired.Device)
	}
	device := map[string]string{"Authorization": "Bearer " + paired.Token}
	status, raw, _ = ask(t, http.MethodPost, base+"/api/v1/sessions", `{"staff_id":"`+ownerID+`","pin":"246813"}`, device)
	if status != http.StatusCreated {
		t.Fatalf("signing in to the mirror: status %d, body %s", status, raw)
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	owner := map[string]string{"Authorization": "Bearer " + paired.Token, api.HeaderStaffToken: in.Token}

	// What it reads is what the venue held.
	status, raw, header := ask(t, http.MethodGet, base+"/api/v1/people", "", owner)
	if status != http.StatusOK {
		t.Fatalf("GET people: status %d, body %s", status, raw)
	}
	var people struct {
		Staff []struct {
			Name string `json:"name"`
		} `json:"staff"`
	}
	if err := json.Unmarshal(raw, &people); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range people.Staff {
		got = append(got, p.Name)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(staffNames, ",") {
		t.Errorf("the mirror's staff are %v, the venue's %v", got, staffNames)
	}
	if at := header.Get(api.HeaderMirror); at != generations[0].Last.UTC().Format(time.RFC3339Nano) {
		t.Errorf("%s = %q, want when the backup was last written, %s", api.HeaderMirror, at, generations[0].Last.UTC().Format(time.RFC3339Nano))
	}
	if header.Get(api.HeaderVia) != api.ViaRelay {
		t.Errorf("the mirror's answers do not say they are from away")
	}

	// Every screen's first read answers. A mirror is built with fewer
	// services than a venue, and this is what finds one it needed.
	for _, path := range []string{
		"/api/v1/setup", "/api/v1/venue", "/api/v1/sessions/current", "/api/v1/menu", "/api/v1/stock",
		"/api/v1/audit?from=2026-10-01&to=2026-10-12", "/api/v1/floor", "/api/v1/devices", "/api/v1/printers", "/api/v1/z-reports",
		"/api/v1/settings", "/api/v1/settings/receipt", "/api/v1/settings/tax", "/api/v1/settings/rules",
		"/api/v1/settings/numbering", "/api/v1/settings/payments", "/api/v1/orders",
		"/api/v1/reports/sales?from=2026-10-01&to=2026-10-12",
	} {
		if status, raw, _ := ask(t, http.MethodGet, base+path, "", owner); status != http.StatusOK {
			t.Errorf("GET %s: status %d, body %.200s", path, status, raw)
		}
	}
	// A menu photo comes from the backup's photos.
	if status, raw, _ := ask(t, http.MethodGet, base+"/api/v1/menu/photos/"+photo, "", owner); status != http.StatusOK || string(raw) != "a picture of a filter coffee" {
		t.Errorf("GET the photo: status %d, %d bytes", status, len(raw))
	}

	// Nothing is changed: a new person, a new price list, an order, a
	// printer, each refused in the mirror's own words.
	for _, write := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/people/staff", `{"name":"Intruder","role_id":1,"pin":"135790"}`},
		{http.MethodPut, "/api/v1/settings/venue", `{"name":"Renamed"}`},
		{http.MethodPost, "/api/v1/orders", `{}`},
		{http.MethodPost, "/api/v1/devices/codes", `{"name":"Another","kind":"back_office"}`},
		{http.MethodPost, "/api/v1/printers", `{}`},
	} {
		status, raw, _ := ask(t, write.method, base+write.path, write.body, owner)
		if status != http.StatusConflict || !strings.Contains(string(raw), string(api.CodeMirrorReadOnly)) {
			t.Errorf("%s %s: status %d, body %s; want 409 %s", write.method, write.path, status, raw, api.CodeMirrorReadOnly)
		}
	}
	// And guests do not order from it.
	if status, _, _ := ask(t, http.MethodGet, base+"/order/", "", nil); status != http.StatusNotFound {
		t.Errorf("GET /order/ on a mirror: status %d", status)
	}

	// The copy itself holds none of it: not the browser, not the sign-in,
	// not an attempt. What the mirror let in, it remembers beside the copy.
	copied := opened(t, up.Dir)
	var copyDevices, copySessions, copyAttempts int
	if err := copied.Reader().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM devices), (SELECT COUNT(*) FROM staff_sessions), (SELECT COUNT(*) FROM sign_in_attempts)`).Scan(&copyDevices, &copySessions, &copyAttempts); err != nil {
		t.Fatal(err)
	}
	var sourceAttempts int
	if err := source.Reader().QueryRowContext(ctx, `SELECT COUNT(*) FROM sign_in_attempts`).Scan(&sourceAttempts); err != nil {
		t.Fatal(err)
	}
	if copyDevices != sourceDevices || copySessions != sourceSessions || copyAttempts != sourceAttempts {
		t.Errorf("the copy has %d devices, %d sessions, %d attempts; the venue had %d, %d, %d: the mirror wrote to its copy",
			copyDevices, copySessions, copyAttempts, sourceDevices, sourceSessions, sourceAttempts)
	}
	if err := copied.Close(ctx); err != nil {
		t.Fatal(err)
	}

	if err := stop(); err != nil {
		t.Fatalf("stopping the mirror: %v", err)
	}
	// The copy is gone with it.
	if _, err := os.Stat(up.Dir); !os.IsNotExist(err) {
		t.Errorf("the mirror's copy is still at %s (%v)", up.Dir, err)
	}
	// The backup is as it was found, to the file.
	if after := filesIn(t, to.Folder); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("the backup folder changed under a mirror:\nbefore %v\nafter  %v", before, after)
	}
	// And the venue never heard of the browser or the sign-in.
	var devicesNow, sessionsNow, staffNow int
	if err := source.Reader().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM devices), (SELECT COUNT(*) FROM staff_sessions), (SELECT COUNT(*) FROM staff)`).Scan(&devicesNow, &sessionsNow, &staffNow); err != nil {
		t.Fatal(err)
	}
	if devicesNow != sourceDevices || sessionsNow != sourceSessions || staffNow != len(staffNames) {
		t.Errorf("the venue now has %d devices, %d sessions, %d staff; it had %d, %d, %d", devicesNow, sessionsNow, staffNow, sourceDevices, sourceSessions, len(staffNames))
	}
}

// TestAMirrorStartedAgainRemembers: the browser paired last time needs no
// code, the PIN is asked for again, and a lock set by wrong PINs is still
// there, though the copy it was set over is gone.
func TestAMirrorStartedAgainRemembers(t *testing.T) {
	ctx := context.Background()
	dataDir, to, _ := backedUpVenue(t)
	source := opened(t, dataDir)
	var ownerID string
	if err := source.Reader().QueryRowContext(ctx, `SELECT public_id FROM staff WHERE name = 'Priya'`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	signIn := func(base string, device map[string]string, pin string) (int, string) {
		status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/sessions", `{"staff_id":"`+ownerID+`","pin":"`+pin+`"}`, device)
		var in struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(raw, &in)
		return status, in.Token
	}

	first, _, stop := startMirror(t, to, stateDir)
	base := "http://" + first.Addr
	status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/devices/pair", `{"code":"`+first.Code+`"}`, nil)
	if status != http.StatusCreated {
		t.Fatalf("pairing: status %d, body %s", status, raw)
	}
	var paired struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &paired); err != nil {
		t.Fatal(err)
	}
	device := map[string]string{"Authorization": "Bearer " + paired.Token}
	status, session := signIn(base, device, "246813")
	if status != http.StatusCreated {
		t.Fatalf("signing in: status %d", status)
	}
	// The code is spent: a second browser does not get in on it.
	if status, _, _ := ask(t, http.MethodPost, base+"/api/v1/devices/pair", `{"code":"`+first.Code+`"}`, nil); status != http.StatusBadRequest {
		t.Errorf("the code a second time: status %d", status)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	second, said, stop := startMirror(t, to, stateDir)
	base = "http://" + second.Addr
	if second.Code == first.Code || second.Dir == first.Dir {
		t.Errorf("the second mirror has the first's code or directory")
	}
	if !strings.Contains(said.String(), "1 browsers paired before") {
		t.Errorf("the mirror does not say it remembers a browser:\n%s", said.String())
	}
	if status, raw, _ := ask(t, http.MethodGet, base+"/api/v1/devices/me", "", device); status != http.StatusOK {
		t.Errorf("the browser paired last time: status %d, body %s", status, raw)
	}
	owner := map[string]string{"Authorization": "Bearer " + paired.Token, api.HeaderStaffToken: session}
	if status, _, _ := ask(t, http.MethodGet, base+"/api/v1/sessions/current", "", owner); status != http.StatusUnauthorized {
		t.Errorf("last time's session: status %d, want the PIN asked again", status)
	}
	// Three wrong PINs, and the right one reads as wrong.
	for range 3 {
		if status, _ := signIn(base, device, "000000"); status != http.StatusUnauthorized {
			t.Fatalf("a wrong PIN: status %d", status)
		}
	}
	if status, _ := signIn(base, device, "246813"); status != http.StatusUnauthorized {
		t.Errorf("the right PIN after three wrong: status %d, want locked", status)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	third, _, stop := startMirror(t, to, stateDir)
	if status, _ := signIn("http://"+third.Addr, device, "246813"); status != http.StatusUnauthorized {
		t.Errorf("the right PIN on a mirror started again: status %d, want still locked", status)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	// The venue never heard of any of it.
	var locked, attempts int
	if err := source.Reader().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM staff WHERE locked_at IS NOT NULL), (SELECT COUNT(*) FROM sign_in_attempts)`).Scan(&locked, &attempts); err != nil {
		t.Fatal(err)
	}
	if locked != 0 || attempts != 0 {
		t.Errorf("the venue has %d people locked and %d attempts recorded", locked, attempts)
	}
}

// venueRunsAgain starts the venue's own server on its data directory and
// stops it once its backup holds something written after since: a day's
// trading, as far as the backup is concerned.
func venueRunsAgain(t *testing.T, dataDir string, to backup.Config, since time.Time) time.Time {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	// As fast as a venue may send, so the test does not wait half a minute.
	args := append(serverArgs(t, dataDir), "-backup-sync-interval", "5s")
	go func() { done <- run(ctx, args, runOptions{stdout: io.Discard}) }()

	var last time.Time
	deadline := time.Now().Add(90 * time.Second)
	for {
		if found, err := backup.Generations(context.Background(), to); err == nil && len(found) > 0 && found[0].Last.After(since) {
			last = found[0].Last
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the venue's server stopped by itself: %v\n%s", err, readLog(t, dataDir))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the venue's backup was not written again in ninety seconds:\n%s", readLog(t, dataDir))
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("the venue's server did not stop")
	}
	// What it wrote as it stopped is in the backup too.
	if found, err := backup.Generations(context.Background(), to); err == nil && len(found) > 0 {
		last = found[0].Last
	}
	return last
}

// TestAMirrorFollowsItsBackup: the venue goes on after the mirror was
// started. The mirror shows what is newer, says it is newer, keeps whoever
// was signed in signed in, and puts the older copy away.
func TestAMirrorFollowsItsBackup(t *testing.T) {
	ctx := context.Background()
	before := len(leftCopies(os.TempDir()))
	dataDir, to, _ := backedUpVenue(t)
	source := opened(t, dataDir)
	var ownerID string
	if err := source.Reader().QueryRowContext(ctx, `SELECT public_id FROM staff WHERE name = 'Priya'`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}

	up, said, stop := startMirror(t, to, t.TempDir(), "-follow", "1s")
	base := "http://" + up.Addr
	if !strings.Contains(said.String(), "looks at the backup every 1s") {
		t.Errorf("the mirror does not say it follows:\n%s", said.String())
	}
	status, raw, _ := ask(t, http.MethodPost, base+"/api/v1/devices/pair", `{"code":"`+up.Code+`"}`, nil)
	if status != http.StatusCreated {
		t.Fatalf("pairing: status %d, body %s", status, raw)
	}
	var paired struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &paired); err != nil {
		t.Fatal(err)
	}
	status, raw, _ = ask(t, http.MethodPost, base+"/api/v1/sessions", `{"staff_id":"`+ownerID+`","pin":"246813"}`, map[string]string{"Authorization": "Bearer " + paired.Token})
	if status != http.StatusCreated {
		t.Fatalf("signing in: status %d, body %s", status, raw)
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	owner := map[string]string{"Authorization": "Bearer " + paired.Token, api.HeaderStaffToken: in.Token}

	venue := func() (name string, asOf time.Time) {
		t.Helper()
		status, raw, header := ask(t, http.MethodGet, base+"/api/v1/venue", "", owner)
		if status != http.StatusOK {
			t.Fatalf("GET venue: status %d, body %s", status, raw)
		}
		var v struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		at, err := time.Parse(time.RFC3339, header.Get(api.HeaderMirror))
		if err != nil {
			t.Fatalf("%s = %q: %v", api.HeaderMirror, header.Get(api.HeaderMirror), err)
		}
		return v.Name, at
	}
	name, first := venue()
	if name != "The Corner Cafe" || !first.Equal(up.AsOf) {
		t.Fatalf("to begin with: %q as of %s; the mirror started as of %s", name, first, up.AsOf)
	}

	// The venue changes its name, and trades long enough to back that up.
	if _, err := source.Writer().ExecContext(ctx, `UPDATE venues SET name = 'The Corner Cafe and Bakery'`); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(ctx); err != nil {
		t.Fatal(err)
	}
	written := venueRunsAgain(t, dataDir, to, up.AsOf)

	// Someone is looking: each look at the venue's name is a request, and
	// a mirror follows only while it is asked things.
	deadline := time.Now().Add(60 * time.Second)
	for {
		name, at := venue()
		if name == "The Corner Cafe and Bakery" {
			if !at.After(first) || at.After(written) {
				t.Errorf("the newer copy is as of %s; the first was %s and the backup was last written %s", at, first, written)
			}
			break
		}
		if !at.Equal(first) {
			t.Fatalf("the time moved to %s and the name is still %q", at, name)
		}
		if time.Now().After(deadline) {
			t.Fatalf("the mirror did not take up the newer backup in a minute:\n%s", said.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Whoever was signed in still is: the same browser, the same session.
	if status, raw, _ := ask(t, http.MethodGet, base+"/api/v1/sessions/current", "", owner); status != http.StatusOK {
		t.Errorf("the session after the copy changed: status %d, body %s", status, raw)
	}
	// And nothing can be changed in the newer copy either.
	if status, _, _ := ask(t, http.MethodPut, base+"/api/v1/settings/venue", `{"name":"Mine"}`, owner); status != http.StatusConflict {
		t.Errorf("a change after the copy changed: status %d", status)
	}
	// The copy it began with is put away once nobody is being answered
	// from it.
	deadline = time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(up.Dir); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the first copy is still at %s", up.Dir)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := stop(); err != nil {
		t.Fatalf("stopping the mirror: %v", err)
	}
	if after := len(leftCopies(os.TempDir())); after != before {
		t.Errorf("%d copies in the temporary directory, and there were %d before", after, before)
	}
}

// TestAMirrorNobodyIsLookingAtAsksNothing: the backup moves on and nobody
// asks the mirror anything. It makes no newer copy: a look at a bucket is
// billed, and a mirror left open overnight should cost nothing.
func TestAMirrorNobodyIsLookingAtAsksNothing(t *testing.T) {
	ctx := context.Background()
	dataDir, to, _ := backedUpVenue(t)
	up, _, stop := startMirror(t, to, t.TempDir(), "-follow", "1s")

	source := opened(t, dataDir)
	if _, err := source.Writer().ExecContext(ctx, `UPDATE venues SET name = 'Renamed'`); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(ctx); err != nil {
		t.Fatal(err)
	}
	venueRunsAgain(t, dataDir, to, up.AsOf)

	// Three looks' worth of time, and not one request.
	time.Sleep(3500 * time.Millisecond)
	if found := leftCopies(os.TempDir()); !containsOnlyOnce(found, up.Dir) {
		t.Errorf("the copies now are %v; the mirror began with %s and nobody asked it anything", found, up.Dir)
	}
	// The first request is answered from what it has, as of when it began.
	_, _, header := ask(t, http.MethodGet, "http://"+up.Addr+"/api/v1/health", "", nil)
	if at := header.Get(api.HeaderMirror); at != up.AsOf.UTC().Format(time.RFC3339Nano) {
		t.Errorf("%s = %q, want the time it began with, %s", api.HeaderMirror, at, up.AsOf.UTC().Format(time.RFC3339Nano))
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

// containsOnlyOnce reports whether dir is among dirs and no other mirror's
// copy from this test is: other tests' are not running beside this one.
func containsOnlyOnce(dirs []string, dir string) bool {
	return len(dirs) == 1 && dirs[0] == dir
}

// TestACopyThatCannotBeMadeIsPutAway: a generation that is not there, as
// a line dropped mid-restore would leave it. The error comes back as one,
// and the directory made for the copy is gone. While following, this is
// what leaves the old copy serving and not the mirror dead.
func TestACopyThatCannotBeMadeIsPutAway(t *testing.T) {
	_, to, _ := backedUpVenue(t)
	before := leftCopies(os.TempDir())
	source := &mirrorSource{from: to, listenAddr: "127.0.0.1:0", log: quietLog()}

	c, err := source.open(context.Background(), backup.Generation{Name: "20260101T000000Z-nothere"})
	if err == nil || c != nil {
		t.Fatalf("open: %+v, %v; want an error and no copy", c, err)
	}
	if after := leftCopies(os.TempDir()); len(after) != len(before) {
		t.Errorf("a copy that could not be made left a directory: %v, and there were %v", after, before)
	}
}

// TestAMirrorStopsWhateverAStalledClientDoes: a request that never
// finishes holds neither a retiring copy nor the mirror's stopping for
// longer than its patience, and the copy is removed all the same.
func TestAMirrorStopsWhateverAStalledClientDoes(t *testing.T) {
	_, to, _ := backedUpVenue(t)
	ctx := context.Background()
	source := &mirrorSource{from: to, listenAddr: "127.0.0.1:0", log: quietLog()}
	generations, err := backup.Generations(ctx, to)
	if err != nil || len(generations) == 0 {
		t.Fatal(err)
	}
	first, err := source.open(ctx, generations[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.open(ctx, generations[0])
	if err != nil {
		t.Fatal(err)
	}
	stalled := make(chan struct{})
	hang := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-stalled })
	first.handler, second.handler = hang, hang
	held := &mirrorHolder{current: first, log: quietLog(), patience: 300 * time.Millisecond}

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

func TestMirrorRefuses(t *testing.T) {
	empty := t.TempDir()
	tests := []struct {
		name string
		args []string
		says string
	}{
		{"nowhere to read from", nil, "say where the backup is"},
		{"a folder with no backup in it", []string{"-from", empty, "-listen-addr", "127.0.0.1:0", "-state-dir", empty}, "no backup"},
		{"a stray argument", []string{"-from", empty, "now"}, "unexpected argument"},
		{"an address that is not one", []string{"-from", empty, "-listen-addr", "everywhere"}, "not host:port"},
		{"following faster than a second", []string{"-from", empty, "-follow", "10ms"}, "at the least"},
		{"following backwards", []string{"-from", empty, "-follow", "-1m"}, "at the least"},
		// A mirror writes nowhere but a directory of its own.
		{"a data directory", []string{"-from", empty, "-data-dir", empty}, "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var said bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := serveMirror(ctx, tt.args, &said, func(mirrorUp) { cancel() })
			if err == nil || !strings.Contains(err.Error(), tt.says) {
				t.Errorf("err = %v, want it to say %q\n%s", err, tt.says, said.String())
			}
		})
	}
}
