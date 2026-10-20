package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/backup"
)

// Backups is what the settings need from the venue's backup (phase 12,
// Task 4d). *backup.Manager is the implementation.
type Backups interface {
	Config() backup.Config
	Status() backup.Status
	SyncInterval() time.Duration
	RestartNeeded() bool
	Set(ctx context.Context, cfg backup.Config) (restartNeeded bool, err error)
	BackUpNow(ctx context.Context) error
}

// ErrNoBackups means this server was built without a backup to set.
var ErrNoBackups = errors.New("this server has no backup to configure")

// ErrBackupOff means there is no destination to back up to.
var ErrBackupOff = errors.New("no backup destination is set")

// BackupUnusable is a destination that is written correctly and could not
// be written to: a drive not plugged in, a key refused, no internet. The
// reason is the destination's own.
type BackupUnusable struct{ Reason string }

func (e *BackupUnusable) Error() string { return "the backup destination cannot be used: " + e.Reason }

// BackupFailed is "back up now" not getting through, and why.
type BackupFailed struct{ Reason string }

func (e *BackupFailed) Error() string { return "the backup did not go: " + e.Reason }

// BackupInput is where the owner wants the backup to go. Kind is "", for
// none, "folder" or "s3"; the fields of the other kinds are ignored, so a
// screen need not clear a box the owner has moved away from.
//
// SecretAccessKey left empty keeps the secret already saved, when the
// bucket, its region and endpoint and the key id are all as they were:
// the secret is never sent back to a screen, so a screen has none to send
// again. Change any of those and the secret is asked for again.
type BackupInput struct {
	Kind            string
	Folder          string
	Bucket          string
	Path            string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

// Backup is where the backup goes and how it is doing. It never carries
// the secret, only whether one is saved.
type Backup struct {
	Kind        backup.Kind
	Folder      string
	Bucket      string
	Path        string
	Region      string
	Endpoint    string
	AccessKeyID string
	SecretSet   bool

	// State, LastSyncedAt and Error are the backup that is running.
	State        backup.State
	LastSyncedAt time.Time
	Error        string
	// SyncInterval is how often it sends what changed. Zero with none
	// running.
	SyncInterval time.Duration
	// RestartNeeded says the destination above is saved and not yet in
	// use: the one in use is the one from before, until mez is restarted.
	RestartNeeded bool
}

// Backup reads where the backup goes and how it is doing.
func (s *Service) Backup(context.Context) (Backup, error) {
	if s.backups == nil {
		return Backup{}, ErrNoBackups
	}
	return s.backupNow(), nil
}

func (s *Service) backupNow() Backup {
	cfg, st := s.backups.Config(), s.backups.Status()
	return Backup{
		Kind: cfg.Kind, Folder: cfg.Folder,
		Bucket: cfg.S3.Bucket, Path: cfg.S3.Path, Region: cfg.S3.Region, Endpoint: cfg.S3.Endpoint,
		AccessKeyID: cfg.S3.AccessKeyID, SecretSet: cfg.S3.SecretAccessKey != "",
		State: st.State, LastSyncedAt: st.LastSyncedAt, Error: st.Error,
		SyncInterval:  s.backups.SyncInterval(),
		RestartNeeded: s.backups.RestartNeeded(),
	}
}

// UpdateBackup saves a new destination. It is tried before it is kept: one
// that cannot be written to is refused and nothing changes.
func (s *Service) UpdateBackup(ctx context.Context, in BackupInput) (Backup, error) {
	if s.backups == nil {
		return Backup{}, ErrNoBackups
	}
	cfg, err := s.backupConfig(in)
	if err != nil {
		return Backup{}, err
	}
	if _, err := s.backups.Set(ctx, cfg); err != nil {
		return Backup{}, backupError(err)
	}
	return s.backupNow(), nil
}

// backupConfig is the input as a destination, with only its own kind's
// fields and the saved secret where none was sent.
func (s *Service) backupConfig(in BackupInput) (backup.Config, error) {
	trim := strings.TrimSpace
	switch backup.Kind(trim(in.Kind)) {
	case backup.KindOff:
		return backup.Config{}, nil
	case backup.KindFolder:
		return backup.Config{Kind: backup.KindFolder, Folder: trim(in.Folder)}, nil
	case backup.KindS3:
		b := backup.S3{
			Bucket: trim(in.Bucket), Path: trim(in.Path), Region: trim(in.Region),
			// A slash on the end is how an address is pasted, and means nothing.
			Endpoint:    strings.TrimRight(trim(in.Endpoint), "/"),
			AccessKeyID: trim(in.AccessKeyID),
			// Not trimmed: it is not this service's to say what a secret
			// may start or end with.
			SecretAccessKey: in.SecretAccessKey,
		}
		if b.SecretAccessKey == "" {
			// Kept only for the key it belongs to, at the place it was
			// given for. A new key id with the old secret would be a pair
			// nobody chose, and the old key signed for another address is
			// a key lent to wherever the box now says.
			was := s.backups.Config().S3
			if was.AccessKeyID != "" && was.AccessKeyID == b.AccessKeyID &&
				was.Bucket == b.Bucket && was.Endpoint == b.Endpoint && was.Region == b.Region {
				b.SecretAccessKey = was.SecretAccessKey
			}
		}
		return backup.Config{Kind: backup.KindS3, S3: b}, nil
	default:
		return backup.Config{}, &Invalid{Field: "kind", Message: "The kind of backup is none, a folder or a bucket."}
	}
}

// BackUpNow copies everything to the destination now and reports how the
// backup stands after it.
func (s *Service) BackUpNow(ctx context.Context) (Backup, error) {
	if s.backups == nil {
		return Backup{}, ErrNoBackups
	}
	// A bucket on a bad line is retried for minutes, and somebody is
	// watching a button.
	ctx, cancel := context.WithTimeout(ctx, backUpNowTimeout)
	defer cancel()
	if err := s.backups.BackUpNow(ctx); err != nil {
		if errors.Is(err, backup.ErrOff) {
			return Backup{}, ErrBackupOff
		}
		return Backup{}, &BackupFailed{Reason: err.Error()}
	}
	return s.backupNow(), nil
}

// backUpNowTimeout is how long "back up now" is given.
const backUpNowTimeout = 30 * time.Second

// backupError puts the backup's refusals into the settings' words.
func backupError(err error) error {
	var invalid *backup.InvalidConfig
	switch {
	case errors.As(err, &invalid):
		return &Invalid{Field: invalid.Field, Message: sentence(invalid.Reason)}
	case errors.Is(err, backup.ErrUnusable):
		reason := strings.TrimPrefix(err.Error(), backup.ErrUnusable.Error()+": ")
		return &BackupUnusable{Reason: reason}
	default:
		return err
	}
}

// sentence makes a reason written for a log into one a screen can show.
func sentence(reason string) string {
	if reason == "" {
		return reason
	}
	out := strings.ToUpper(reason[:1]) + reason[1:]
	if !strings.HasSuffix(out, ".") {
		out += "."
	}
	return out
}
