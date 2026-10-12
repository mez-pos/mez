package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/service/settings"
)

// The backup settings error codes.
const (
	// CodeBackupUnusable is a destination written correctly that could not
	// be written to. Nothing was saved; the message is the reason.
	CodeBackupUnusable ErrorCode = "backup_unusable"
	// CodeBackupOff is "back up now" with no destination set.
	CodeBackupOff ErrorCode = "backup_off"
	// CodeBackupFailed is "back up now" not getting through. The message
	// is the reason.
	CodeBackupFailed ErrorCode = "backup_failed"
	// CodeBackupUnavailable is a server with no backup to configure.
	CodeBackupUnavailable ErrorCode = "backup_unavailable"
)

// backupBucket is an S3 bucket: Amazon's, R2, or any service that speaks
// it. path is the folder inside it, "" for "mez"; region is for Amazon,
// endpoint for everyone else.
type backupBucket struct {
	Bucket      string `json:"bucket"`
	Path        string `json:"path"`
	Region      string `json:"region"`
	Endpoint    string `json:"endpoint"`
	AccessKeyID string `json:"access_key_id"`
}

// backupRequest is where the owner wants the backup to go. kind is "" for
// none, "folder" or "s3"; the other kind's fields are ignored.
// secret_access_key left out or empty keeps the secret already saved,
// when bucket, region, endpoint and access_key_id are all as they were.
type backupRequest struct {
	Kind   string              `json:"kind"`
	Folder string              `json:"folder"`
	S3     backupBucketRequest `json:"s3"`
}

// backupBucketRequest is a bucket and the secret half of its key.
type backupBucketRequest struct {
	backupBucket
	SecretAccessKey string `json:"secret_access_key"`
}

// backupStatus is how the running backup is doing. last_synced_at is when
// the destination last had everything, absent if it has not since the
// server started; error is what is wrong while something is.
// sync_interval_seconds is how often it sends what changed, and so the most
// a dead machine can lose (ADR-315); absent with no backup running.
type backupStatus struct {
	State               backup.State `json:"state"`
	LastSyncedAt        *time.Time   `json:"last_synced_at,omitempty"`
	Error               string       `json:"error,omitempty"`
	SyncIntervalSeconds int          `json:"sync_interval_seconds,omitempty"`
}

// backupResponse is the backup as saved and as running. The secret is
// never in it: secret_set says whether one is saved. restart_needed means
// the destination shown is saved and not yet in use, and status is still
// the one from before, until mez is restarted.
type backupResponse struct {
	Kind          backup.Kind  `json:"kind"`
	Folder        string       `json:"folder"`
	S3            backupS3     `json:"s3"`
	Status        backupStatus `json:"status"`
	RestartNeeded bool         `json:"restart_needed"`
}

type backupS3 struct {
	backupBucket
	SecretSet bool `json:"secret_set"`
}

func (s *Server) mountBackupSettings(mux *http.ServeMux) {
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermSettingsEdit, h))))
	}
	handle("GET /api/v1/settings/backup", s.readBackup)
	handle("PUT /api/v1/settings/backup", s.updateBackup)
	mux.HandleFunc("/api/v1/settings/backup", s.methodNotAllowed)
	handle("POST /api/v1/settings/backup/now", s.backUpNow)
	mux.HandleFunc("/api/v1/settings/backup/now", s.methodNotAllowed)
}

func (s *Server) readBackup(w http.ResponseWriter, r *http.Request) {
	b, err := s.settings.Backup(r.Context())
	s.writeBackup(w, r, b, err)
}

func (s *Server) updateBackup(w http.ResponseWriter, r *http.Request) {
	var req backupRequest
	if !s.decode(w, r, &req) {
		return
	}
	b, err := s.settings.UpdateBackup(r.Context(), settings.BackupInput{
		Kind: req.Kind, Folder: req.Folder,
		Bucket: req.S3.Bucket, Path: req.S3.Path, Region: req.S3.Region, Endpoint: req.S3.Endpoint,
		AccessKeyID: req.S3.AccessKeyID, SecretAccessKey: req.S3.SecretAccessKey,
	})
	s.writeBackup(w, r, b, err)
}

func (s *Server) backUpNow(w http.ResponseWriter, r *http.Request) {
	b, err := s.settings.BackUpNow(r.Context())
	s.writeBackup(w, r, b, err)
}

// writeBackup answers with the backup as it now stands, or the refusal.
func (s *Server) writeBackup(w http.ResponseWriter, r *http.Request, b settings.Backup, err error) {
	var unusable *settings.BackupUnusable
	var failed *settings.BackupFailed
	switch {
	case err == nil:
	case errors.As(err, &unusable):
		writeError(w, s.log, http.StatusUnprocessableEntity, CodeBackupUnusable, unusable.Reason)
		return
	case errors.As(err, &failed):
		// The server is well; what it was asked to reach is not.
		writeError(w, s.log, http.StatusBadGateway, CodeBackupFailed, failed.Reason)
		return
	case errors.Is(err, settings.ErrBackupOff):
		writeError(w, s.log, http.StatusConflict, CodeBackupOff, "No backup destination is set.")
		return
	case errors.Is(err, settings.ErrNoBackups):
		writeError(w, s.log, http.StatusNotFound, CodeBackupUnavailable, "This server has no backup to configure.")
		return
	default:
		s.writeSettingsError(w, r, err)
		return
	}

	out := backupResponse{
		Kind: b.Kind, Folder: b.Folder,
		S3: backupS3{
			backupBucket: backupBucket{Bucket: b.Bucket, Path: b.Path, Region: b.Region, Endpoint: b.Endpoint, AccessKeyID: b.AccessKeyID},
			SecretSet:    b.SecretSet,
		},
		Status:        backupStatus{State: b.State, Error: b.Error, SyncIntervalSeconds: int(b.SyncInterval / time.Second)},
		RestartNeeded: b.RestartNeeded,
	}
	if !b.LastSyncedAt.IsZero() {
		at := b.LastSyncedAt.UTC()
		out.Status.LastSyncedAt = &at
	}
	writeJSON(w, s.log, http.StatusOK, out)
}
