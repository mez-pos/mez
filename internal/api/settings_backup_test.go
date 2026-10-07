package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// TestBackupSettingsOverHTTP is the owner in the Backups section, against
// a real backup: nothing set, a folder named and tried, backed up now,
// refusals with their code and field, and off again.
func TestBackupSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	dataDir := filepath.Dir(w.db.Path())
	manager, err := backup.NewManager(backup.ManagerOptions{DataDir: dataDir, DBPath: w.db.Path()})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Registered after the till's, so it runs first: the backup is stopped
	// while the database is still open, which a test may do and a server
	// on Linux may not.
	t.Cleanup(func() { manager.Close(context.Background()) })

	svc, err := settings.New(settings.Options{DB: w.db, Backups: manager})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, path, body string) (int, backupResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, path, body, owner)
		if rec.Code >= 400 {
			return rec.Code, backupResponse{}, errorOf(t, rec)
		}
		var out backupResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}
	const at, now = "/api/v1/settings/backup", "/api/v1/settings/backup/now"
	quote := func(s string) string { b, _ := json.Marshal(s); return string(b) }

	status, out, _ := call(http.MethodGet, at, "")
	if status != http.StatusOK || out.Kind != backup.KindOff || out.Status.State != backup.StateOff || out.RestartNeeded {
		t.Fatalf("a new venue: %d %+v", status, out)
	}
	if status, _, refused := call(http.MethodPost, now, ""); status != http.StatusConflict || refused.Code != CodeBackupOff {
		t.Errorf("back up now with nothing set: %d %+v", status, refused)
	}

	folder := filepath.Join(t.TempDir(), "backups")
	status, out, _ = call(http.MethodPut, at, `{"kind":"folder","folder":`+quote(folder)+`}`)
	if status != http.StatusOK || out.Kind != backup.KindFolder || out.Folder != folder {
		t.Fatalf("saving a folder: %d %+v", status, out)
	}
	if _, err := os.Stat(filepath.Join(folder, "mez-backup.txt")); err != nil {
		t.Errorf("the folder was not tried before it was kept: %v", err)
	}

	status, out, _ = call(http.MethodPost, now, "")
	if status != http.StatusOK || out.Status.State != backup.StateOK || out.Status.LastSyncedAt == nil || out.Status.Error != "" {
		t.Fatalf("back up now: %d %+v", status, out)
	}
	if out.Status.LastSyncedAt.Location().String() != "UTC" {
		t.Errorf("last_synced_at is in %s, want UTC", out.Status.LastSyncedAt.Location())
	}

	t.Run("refusals", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "blocked")
		if err := os.WriteFile(blocked, []byte("not a folder"), 0o600); err != nil {
			t.Fatal(err)
		}
		tests := []struct {
			name   string
			body   string
			status int
			code   ErrorCode
			field  string
		}{
			{"a relative folder", `{"kind":"folder","folder":"backups"}`, http.StatusUnprocessableEntity, CodeInvalidSetting, "folder"},
			{"a folder in the data directory", `{"kind":"folder","folder":` + quote(filepath.Join(dataDir, "b")) + `}`, http.StatusUnprocessableEntity, CodeInvalidSetting, "folder"},
			{"a folder that cannot be made", `{"kind":"folder","folder":` + quote(blocked) + `}`, http.StatusUnprocessableEntity, CodeBackupUnusable, ""},
			{"a kind there is not", `{"kind":"tape"}`, http.StatusUnprocessableEntity, CodeInvalidSetting, "kind"},
			{"a bucket with no secret", `{"kind":"s3","s3":{"bucket":"corner-cafe","region":"ap-south-1","access_key_id":"AKID"}}`, http.StatusUnprocessableEntity, CodeInvalidSetting, "secret_access_key"},
			{"a bucket with a bad name", `{"kind":"s3","s3":{"bucket":"Corner Cafe","region":"ap-south-1","access_key_id":"AKID","secret_access_key":"s"}}`, http.StatusUnprocessableEntity, CodeInvalidSetting, "bucket"},
			{"not JSON", `kind=folder`, http.StatusBadRequest, CodeInvalidRequest, ""},
		}
		for _, tt := range tests {
			status, _, refused := call(http.MethodPut, at, tt.body)
			if status != tt.status || refused.Code != tt.code || refused.Field != tt.field {
				t.Errorf("%s: %d %+v, want %d %s on %q", tt.name, status, refused, tt.status, tt.code, tt.field)
			}
			if refused.Message == "" {
				t.Errorf("%s: refused with no reason", tt.name)
			}
		}
		if _, out, _ := call(http.MethodGet, at, ""); out.Kind != backup.KindFolder || out.Folder != folder {
			t.Errorf("a refused save changed the destination: %+v", out)
		}
	})

	t.Run("the other kind's boxes are ignored", func(t *testing.T) {
		// A screen that left the bucket's boxes filled in when the owner
		// chose a folder.
		body := `{"kind":"folder","folder":` + quote(folder) + `,"s3":{"bucket":"left-over","access_key_id":"AKID"}}`
		status, out, refused := call(http.MethodPut, at, body)
		if status != http.StatusOK || out.S3.Bucket != "" {
			t.Errorf("%d %+v %+v", status, out, refused)
		}
	})

	t.Run("a cashier cannot read, change or run it", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, c := range [][2]string{{http.MethodGet, at}, {http.MethodPut, at}, {http.MethodPost, now}} {
			if rec := send(w.handler, c[0], c[1], `{"kind":""}`, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: %d", c[0], c[1], rec.Code)
			}
		}
	})
	for _, c := range [][2]string{{http.MethodDelete, at}, {http.MethodGet, now}} {
		if rec := send(w.handler, c[0], c[1], "", owner); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: %d", c[0], c[1], rec.Code)
		}
	}

	status, out, _ = call(http.MethodPut, at, `{"kind":""}`)
	if status != http.StatusOK || out.Kind != backup.KindOff || out.Status.State != backup.StateOff || out.Folder != "" {
		t.Fatalf("turning it off: %d %+v", status, out)
	}
}

// fakeBackups is a backup that keeps what it is given and does nothing
// with it, for what the API does with a secret.
type fakeBackups struct {
	cfg     backup.Config
	restart bool
}

func (f *fakeBackups) Config() backup.Config { return f.cfg }
func (f *fakeBackups) Status() backup.Status { return backup.Status{State: backup.StateOK} }
func (f *fakeBackups) RestartNeeded() bool   { return f.restart }
func (f *fakeBackups) BackUpNow(context.Context) error {
	return nil
}
func (f *fakeBackups) Set(_ context.Context, cfg backup.Config) (bool, error) {
	if err := cfg.Validate(os.TempDir()); err != nil {
		return false, err
	}
	f.cfg = cfg
	return f.restart, nil
}

// TestABucketsSecretGoesInAndNeverComesOut: the one field that is written
// and not read. A screen that has no secret to send again must still be
// able to change the path.
func TestABucketsSecretGoesInAndNeverComesOut(t *testing.T) {
	const secret = "wJalrXUtnFEMI-never-shown"
	w := newTill(t)
	fake := &fakeBackups{restart: true}
	svc, err := settings.New(settings.Options{DB: w.db, Backups: fake})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	const at = "/api/v1/settings/backup"

	put := func(body string) (int, string) {
		t.Helper()
		rec := send(w.handler, http.MethodPut, at, body, owner)
		return rec.Code, rec.Body.String()
	}

	status, body := put(`{"kind":"s3","s3":{"bucket":"corner-cafe","endpoint":"https://abc.r2.cloudflarestorage.com/","access_key_id":" AKID ","secret_access_key":"` + secret + `"}}`)
	if status != http.StatusOK {
		t.Fatalf("saving a bucket: %d %s", status, body)
	}
	if strings.Contains(body, secret) {
		t.Fatalf("the answer to a save carries the secret: %s", body)
	}
	var out backupResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if !out.S3.SecretSet || out.S3.AccessKeyID != "AKID" || out.S3.Endpoint != "https://abc.r2.cloudflarestorage.com" || !out.RestartNeeded {
		t.Errorf("saved as %+v, want the secret marked set, the key id and endpoint tidied, and the restart said", out)
	}
	if fake.cfg.S3.SecretAccessKey != secret {
		t.Fatalf("the secret that reached the backup: %q", fake.cfg.S3.SecretAccessKey)
	}
	if rec := send(w.handler, http.MethodGet, at, "", owner); strings.Contains(rec.Body.String(), secret) {
		t.Errorf("reading the settings carries the secret: %s", rec.Body)
	}

	// The path changed, and no secret sent: the saved one is kept.
	if status, body = put(`{"kind":"s3","s3":{"bucket":"corner-cafe","path":"venues/corner","endpoint":"https://abc.r2.cloudflarestorage.com","access_key_id":"AKID"}}`); status != http.StatusOK {
		t.Fatalf("changing the path with no secret: %d %s", status, body)
	}
	if fake.cfg.S3.SecretAccessKey != secret || fake.cfg.S3.Path != "venues/corner" {
		t.Errorf("after changing the path: %+v", fake.cfg)
	}

	// Another address, and no secret: the saved key is not signed for
	// wherever the box now says.
	for _, body := range []string{
		`{"kind":"s3","s3":{"bucket":"corner-cafe","endpoint":"https://elsewhere.example","access_key_id":"AKID"}}`,
		`{"kind":"s3","s3":{"bucket":"another-bucket","endpoint":"https://abc.r2.cloudflarestorage.com","access_key_id":"AKID"}}`,
	} {
		status, answer := put(body)
		if status != http.StatusUnprocessableEntity || !strings.Contains(answer, `"field":"secret_access_key"`) {
			t.Errorf("%s: %d %s, want the secret asked for again", body, status, answer)
		}
	}
	if fake.cfg.S3.Endpoint != "https://abc.r2.cloudflarestorage.com" || fake.cfg.S3.Bucket != "corner-cafe" {
		t.Errorf("a refused save moved the bucket: %+v", fake.cfg)
	}

	// A different key id and no secret: the old secret is not lent to it.
	status, body = put(`{"kind":"s3","s3":{"bucket":"corner-cafe","endpoint":"https://abc.r2.cloudflarestorage.com","access_key_id":"OTHER"}}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"secret_access_key"`) {
		t.Errorf("a new key id with no secret: %d %s, want the secret asked for", status, body)
	}
	if strings.Contains(body, secret) {
		t.Errorf("the refusal carries the secret: %s", body)
	}
	if fake.cfg.S3.AccessKeyID != "AKID" {
		t.Errorf("a refused save changed the key: %+v", fake.cfg)
	}
}

// TestBackupSettingsWithNoBackup is a server built without one.
func TestBackupSettingsWithNoBackup(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	rec := send(w.handler, http.MethodGet, "/api/v1/settings/backup", "", owner)
	if rec.Code != http.StatusNotFound || errorOf(t, rec).Code != CodeBackupUnavailable {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
