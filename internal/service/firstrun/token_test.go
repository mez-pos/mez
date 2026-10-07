package firstrun

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var tokenShape = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestOpenTokenDrawsOneAndKeepsIt(t *testing.T) {
	dir := t.TempDir()
	first, err := OpenToken(dir, true)
	if err != nil {
		t.Fatalf("OpenToken: %v", err)
	}
	value, err := ReadToken(dir)
	if err != nil || !tokenShape.MatchString(value) {
		t.Fatalf("the token on disk = %q, %v", value, err)
	}
	if !first.Valid(value) {
		t.Error("the token on disk does not open setup")
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(dir, TokenFile)); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("token file mode = %v, %v, want 0600", info.Mode().Perm(), err)
		}
	}

	// A restart keeps it: the link the installer opened must still work.
	again, err := OpenToken(dir, true)
	if err != nil {
		t.Fatalf("OpenToken again: %v", err)
	}
	if !again.Valid(value) {
		t.Error("a restart changed the token")
	}

	// Another machine's is another token.
	other, err := OpenToken(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if other.Valid(value) {
		t.Error("two data directories drew the same token")
	}
}

func TestTokenValid(t *testing.T) {
	dir := t.TempDir()
	tok, err := OpenToken(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := ReadToken(dir)
	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"the token", value, true},
		{"nothing", "", false},
		{"its first half", value[:16], false},
		{"with a character more", value + "0", false},
		{"in capitals", strings.ToUpper(value), value == strings.ToUpper(value)},
		{"with a space", " " + value, false},
		{"another token", strings.Repeat("0", 32), false},
	}
	for _, tt := range tests {
		if got := tok.Valid(tt.candidate); got != tt.want {
			t.Errorf("%s: Valid = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTokenSpend(t *testing.T) {
	dir := t.TempDir()
	tok, err := OpenToken(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := ReadToken(dir)
	if err := tok.Spend(); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if tok.Valid(value) || tok.Valid("") {
		t.Error("a spent token still opens setup")
	}
	if _, err := ReadToken(dir); !errors.Is(err, ErrNoToken) {
		t.Errorf("after Spend the file reads %v, want ErrNoToken", err)
	}
	if err := tok.Spend(); err != nil {
		t.Errorf("spending twice: %v", err)
	}
}

func TestOpenTokenWhenNoSetupIsNeeded(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenToken(dir, true); err != nil {
		t.Fatal(err)
	}
	stale, _ := ReadToken(dir)

	// The venue was made some other way (mez setup) while a token lay there.
	tok, err := OpenToken(dir, false)
	if err != nil {
		t.Fatalf("OpenToken: %v", err)
	}
	if tok.Valid(stale) || tok.Valid("") {
		t.Error("a venue that needs no setup has a token that opens it")
	}
	if _, err := ReadToken(dir); !errors.Is(err, ErrNoToken) {
		t.Errorf("the stale file reads %v, want ErrNoToken", err)
	}
	// And with nothing there to remove, it is not an error.
	if _, err := OpenToken(t.TempDir(), false); err != nil {
		t.Errorf("OpenToken on an empty directory: %v", err)
	}
}

// TestOpenTokenReplacesAFileThatIsNotOne: whatever is in the file, only a
// token this wrote is one. An empty file must not make "" open setup.
func TestOpenTokenReplacesAFileThatIsNotOne(t *testing.T) {
	for name, content := range map[string]string{
		"empty": "", "a newline": "\n", "too short": "abc123\n", "not hex": strings.Repeat("z", 32),
		"capitals": strings.Repeat("AB", 16), "two lines": strings.Repeat("a", 32) + "\n" + strings.Repeat("b", 32),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, TokenFile), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadToken(dir); !errors.Is(err, ErrNoToken) {
				t.Errorf("ReadToken = %v, want ErrNoToken", err)
			}
			tok, err := OpenToken(dir, true)
			if err != nil {
				t.Fatalf("OpenToken: %v", err)
			}
			value, err := ReadToken(dir)
			if err != nil || !tokenShape.MatchString(value) || !tok.Valid(value) {
				t.Errorf("after OpenToken the file holds %q (%v)", value, err)
			}
			if tok.Valid(strings.TrimSpace(content)) {
				t.Errorf("what was in the file (%q) opens setup", content)
			}
		})
	}
}

func TestOpenTokenFailsWhereItCannotWrite(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no", "such", "directory")
	if _, err := OpenToken(missing, true); err == nil {
		t.Error("OpenToken wrote a token into a directory that does not exist")
	}
}

func TestSetupLink(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	for base, want := range map[string]string{
		"http://localhost:7777":  "http://localhost:7777/setup#token=" + token,
		"http://localhost:7777/": "http://localhost:7777/setup#token=" + token,
	} {
		if got := SetupLink(base, token); got != want {
			t.Errorf("SetupLink(%q) = %q, want %q", base, got, want)
		}
	}
}
