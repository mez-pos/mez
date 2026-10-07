package backup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
		{name: "a kind from a later build", cfg: Config{Kind: "s3"}},
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
