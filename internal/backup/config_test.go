package backup

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	elsewhere := filepath.Join(t.TempDir(), "backups")

	// A venue kept in a folder that happens to be called db.
	dataDirInDB := filepath.Join(t.TempDir(), "mez", databaseDir)

	tests := []struct {
		name    string
		cfg     Config
		dataDir string // "" is dataDir
		ok      bool
	}{
		{name: "off", cfg: Config{}, ok: true},
		{name: "a folder elsewhere", cfg: Config{Kind: KindFolder, Folder: elsewhere}, ok: true},
		{name: "a folder beside the data directory", cfg: Config{Kind: KindFolder, Folder: dataDir + "-backup"}, ok: true},
		{name: "off with a folder left in", cfg: Config{Folder: elsewhere}},
		{name: "a folder with no name", cfg: Config{Kind: KindFolder}},
		{name: "a folder of spaces", cfg: Config{Kind: KindFolder, Folder: "  "}},
		{name: "a relative folder", cfg: Config{Kind: KindFolder, Folder: "backups"}},
		{name: "the data directory itself", cfg: Config{Kind: KindFolder, Folder: dataDir}},
		{name: "inside the data directory", cfg: Config{Kind: KindFolder, Folder: filepath.Join(dataDir, "backups")}},
		{name: "inside it, spelt the long way", cfg: Config{Kind: KindFolder, Folder: filepath.Join(dataDir, "photos", "..", "backups")}},
		{name: "the data directory is where the copies would go", cfg: Config{Kind: KindFolder, Folder: filepath.Dir(dataDirInDB)}, dataDir: dataDirInDB},
		{name: "the data directory is under where the copies would go", cfg: Config{Kind: KindFolder, Folder: filepath.Dir(dataDirInDB)}, dataDir: filepath.Join(dataDirInDB, "venue")},
		{name: "the root of the venue's drive", cfg: Config{Kind: KindFolder, Folder: filepath.VolumeName(dataDir) + string(filepath.Separator)}, ok: true},
		{name: "a trailing separator", cfg: Config{Kind: KindFolder, Folder: dataDir + string(filepath.Separator)}},
		{name: "a kind from a later build", cfg: Config{Kind: "gcs"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			against := dataDir
			if tt.dataDir != "" {
				against = tt.dataDir
			}
			err := tt.cfg.Validate(against)
			if tt.ok && err != nil {
				t.Fatalf("Validate: %v, want it accepted", err)
			}
			if !tt.ok && !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate: %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestValidateABucket(t *testing.T) {
	good := S3{Bucket: "corner-cafe", Region: "ap-south-1", AccessKeyID: "AKID", SecretAccessKey: "secret"}
	with := func(change func(*S3)) Config {
		s := good
		change(&s)
		return Config{Kind: KindS3, S3: s}
	}

	tests := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{name: "a bucket at Amazon", cfg: with(func(*S3) {}), ok: true},
		{name: "a bucket at R2, with no region", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "https://abc123.r2.cloudflarestorage.com" }), ok: true},
		{name: "a MinIO on the LAN, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://192.168.1.20:9000" }), ok: true},
		{name: "a MinIO on this machine, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://127.0.0.1:9000" }), ok: true},
		{name: "a MinIO at a ULA address, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://[fd12:3456::20]:9000" }), ok: true},
		{name: "a NAS by its bare name, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://nas:9000" }), ok: true},
		{name: "localhost, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://localhost:9000" }), ok: true},
		{name: "a public name, over https", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "https://s3.example.com" }), ok: true},
		{name: "a public address, over https", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "https://203.0.113.9:9000" }), ok: true},
		{name: "a path", cfg: with(func(s *S3) { s.Path = "venues/corner" }), ok: true},
		{name: "a dotted bucket", cfg: with(func(s *S3) { s.Bucket = "backups.corner.cafe" }), ok: true},
		{name: "a bucket with hyphens together", cfg: with(func(s *S3) { s.Bucket = "corner--cafe" }), ok: true},

		{name: "no bucket", cfg: with(func(s *S3) { s.Bucket = "" })},
		{name: "a bucket in capitals", cfg: with(func(s *S3) { s.Bucket = "CornerCafe" })},
		{name: "a bucket of two letters", cfg: with(func(s *S3) { s.Bucket = "cc" })},
		{name: "a bucket ending in a hyphen", cfg: with(func(s *S3) { s.Bucket = "corner-" })},
		{name: "a bucket with a slash in it", cfg: with(func(s *S3) { s.Bucket = "corner/cafe" })},
		{name: "no key", cfg: with(func(s *S3) { s.AccessKeyID = "" })},
		{name: "no secret", cfg: with(func(s *S3) { s.SecretAccessKey = "" })},
		{name: "neither a region nor an endpoint", cfg: with(func(s *S3) { s.Region = "" })},
		{name: "an endpoint with no scheme", cfg: with(func(s *S3) { s.Endpoint = "abc123.r2.cloudflarestorage.com" })},
		{name: "an endpoint that is not http", cfg: with(func(s *S3) { s.Endpoint = "ftp://nas.local" })},
		{name: "a public name over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://s3.example.com" })},
		{name: "a dotted local name over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://nas.local:9000" })},
		{name: "a public address over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://203.0.113.9:9000" })},
		{name: "a public IPv6 address over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://[2001:db8::9]:9000" })},
		{name: "a name that only looks like a LAN address, over plain HTTP", cfg: with(func(s *S3) { s.Region = ""; s.Endpoint = "http://192.168.1.20.example.com" })},
		{name: "an endpoint with the bucket on the end", cfg: with(func(s *S3) { s.Endpoint = "https://abc123.r2.cloudflarestorage.com/corner-cafe" })},
		{name: "an endpoint with a key in it", cfg: with(func(s *S3) { s.Endpoint = "https://AKID:secret@nas.local" })},
		{name: "an endpoint with a key in it that does not parse", cfg: with(func(s *S3) { s.Endpoint = "https://AKID:secret@nas.local:port" })},
		{name: "an endpoint with a key and no scheme", cfg: with(func(s *S3) { s.Endpoint = "AKID:secret@nas.local" })},
		{name: "a bucket with two dots together", cfg: with(func(s *S3) { s.Bucket = "corner..cafe" })},
		{name: "a bucket with a dot and a hyphen together", cfg: with(func(s *S3) { s.Bucket = "corner.-cafe" })},
		{name: "a bucket starting with a dot", cfg: with(func(s *S3) { s.Bucket = ".corner" })},
		{name: "a path with a leading slash", cfg: with(func(s *S3) { s.Path = "/venues" })},
		{name: "a path with a trailing slash", cfg: with(func(s *S3) { s.Path = "venues/" })},
		{name: "a path that climbs", cfg: with(func(s *S3) { s.Path = "venues/../other" })},
		{name: "a path with backslashes", cfg: with(func(s *S3) { s.Path = `venues\corner` })},
		{name: "a bucket and a folder", cfg: Config{Kind: KindS3, S3: good, Folder: filepath.Join(t.TempDir(), "b")}},
		{name: "a folder with a bucket left in", cfg: Config{Kind: KindFolder, Folder: filepath.Join(t.TempDir(), "b"), S3: good}},
		{name: "off with a bucket left in", cfg: Config{S3: good}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate(t.TempDir())
			if tt.ok && err != nil {
				t.Fatalf("Validate: %v, want it accepted", err)
			}
			if !tt.ok && !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate: %v, want ErrInvalidConfig", err)
			}
			if err != nil && strings.Contains(err.Error(), good.SecretAccessKey+"@") {
				t.Errorf("the error repeats the secret: %v", err)
			}
		})
	}
}

// TestTheSecretIsNotPrinted: a destination ends up in log lines and error
// messages, and the log is the first thing sent to whoever is helping.
func TestTheSecretIsNotPrinted(t *testing.T) {
	const secret = "wJalrXUtnFEMI-do-not-print"
	cfg := Config{Kind: KindS3, S3: S3{Bucket: "corner-cafe", Endpoint: "https://r2.example", AccessKeyID: "AKID", SecretAccessKey: secret}}

	var logged bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	log.Info("config", "backup", cfg, "s3", cfg.S3, "ptr", &cfg.S3)

	printed := []string{
		fmt.Sprint(cfg), fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), cfg.S3.String(),
		fmt.Sprintf("%+v", &cfg), fmt.Sprintf("%v", cfg.S3), cfg.Destination(), logged.String(),
	}
	for _, p := range printed {
		if strings.Contains(p, secret) {
			t.Errorf("the secret was printed: %s", p)
		}
	}
	if !strings.Contains(fmt.Sprintf("%+v", cfg), "corner-cafe") {
		t.Errorf("the rest of the destination should still print: %+v", cfg)
	}
	if got, want := cfg.Destination(), "s3://corner-cafe/mez at https://r2.example"; got != want {
		t.Errorf("Destination = %q, want %q", got, want)
	}
}

// TestWithinOnWindows: Windows does not tell two spellings of a folder
// apart by their case, and a check that did would let the backup into the
// data directory by a capital letter.
func TestWithinOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows folds case")
	}
	tests := []struct {
		path, dir string
		want      bool
	}{
		{`D:\Mez\Backups`, `d:\mez`, true},
		{`d:\mez`, `D:\MEZ\`, true},
		{`D:\mez-backup`, `D:\mez`, false},
		{`E:\mez\backups`, `D:\mez`, false},
		{`\\nas\share\mez\backups`, `\\nas\share\mez`, true},
		{`\\nas\share\backups`, `\\nas\share\mez`, false},
	}
	for _, tt := range tests {
		if got := within(tt.path, tt.dir); got != tt.want {
			t.Errorf("within(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	elsewhere := filepath.Join(t.TempDir(), "backups")
	quoted := `"` + filepath.ToSlash(elsewhere) + `"`

	tests := []struct {
		name    string
		file    string // "" writes none
		want    Config
		invalid bool
	}{
		{name: "no file is no backup", want: Config{}},
		{name: "off, said out loud", file: `{"kind":""}`, want: Config{}},
		{name: "a folder", file: `{"kind":"folder","folder":` + quoted + `}`, want: Config{Kind: KindFolder, Folder: filepath.ToSlash(elsewhere)}},
		{name: "not JSON", file: `kind = folder`, invalid: true},
		{name: "a misspelt key", file: `{"kind":"folder","foldr":` + quoted + `}`, invalid: true},
		{name: "a folder that fails the checks", file: `{"kind":"folder","folder":"backups"}`, invalid: true},
		{
			name: "a bucket",
			file: `{"kind":"s3","s3":{"bucket":"corner-cafe","endpoint":"https://abc.r2.cloudflarestorage.com","access_key_id":"AKID","secret_access_key":"shh"}}`,
			want: Config{Kind: KindS3, S3: S3{Bucket: "corner-cafe", Endpoint: "https://abc.r2.cloudflarestorage.com", AccessKeyID: "AKID", SecretAccessKey: "shh"}},
		},
		{name: "a bucket with no key", file: `{"kind":"s3","s3":{"bucket":"corner-cafe","region":"ap-south-1"}}`, invalid: true},
		{name: "a bucket with a misspelt key", file: `{"kind":"s3","s3":{"bucket":"corner-cafe","region":"ap-south-1","access_key":"AKID","secret_access_key":"shh"}}`, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if tt.file != "" {
				if err := os.WriteFile(filepath.Join(dataDir, ConfigFile), []byte(tt.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadConfig(dataDir)
			if tt.invalid {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("LoadConfig: %v, want ErrInvalidConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got != tt.want {
				t.Errorf("LoadConfig = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestAPlainHTTPEndpointMustBeOnThisNetwork: the backup is the whole
// database, and over plain HTTP to anywhere past the router it would cross
// the internet in clear. The refusal says what to do instead.
func TestAPlainHTTPEndpointMustBeOnThisNetwork(t *testing.T) {
	tests := []struct {
		endpoint string
		ok       bool
	}{
		{"http://192.168.1.20:9000", true},
		{"http://10.0.0.5", true},
		{"http://172.16.4.4:9000", true},
		{"http://127.0.0.1:9000", true},
		{"http://[::1]:9000", true},
		{"http://[fd12:3456::20]:9000", true},
		{"http://169.254.10.20:9000", true},
		{"http://nas:9000", true},
		{"http://minio", true},
		{"http://localhost:9000", true},
		{"https://s3.example.com", true},
		{"https://203.0.113.9:9000", true},

		{"http://s3.example.com", false},
		{"http://nas.local:9000", false},
		{"http://203.0.113.9:9000", false},
		{"http://8.8.8.8", false},
		{"http://[2001:db8::9]:9000", false},
		{"http://192.168.1.20.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			cfg := Config{Kind: KindS3, S3: S3{Bucket: "corner-cafe", Endpoint: tt.endpoint, AccessKeyID: "AKID", SecretAccessKey: "secret"}}
			err := cfg.Validate(t.TempDir())
			if tt.ok {
				if err != nil {
					t.Fatalf("Validate: %v, want it accepted", err)
				}
				return
			}
			want := fmt.Sprintf("the endpoint %q must be https: a plain http address is only allowed for a server on this network", tt.endpoint)
			var inv *InvalidConfig
			if !errors.As(err, &inv) || inv.Field != "endpoint" || inv.Reason != want {
				t.Fatalf("Validate: %v, want an endpoint error saying %q", err, want)
			}
		})
	}
}
