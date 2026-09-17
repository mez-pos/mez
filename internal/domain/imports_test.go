package domain

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestDomainImportsStayPure guards the one architectural boundary this project
// actually depends on: internal/domain knows nothing about storage or
// transport.
//
// That is what lets the money logic be read and tested on its own, and it is
// the kind of rule that decays in month four unless something fails loudly. So
// it fails loudly here rather than living in a document.
func TestDomainImportsStayPure(t *testing.T) {
	mod := modulePath(t)

	tests := []struct {
		name string
		path string
		why  string
	}{
		{
			name: "persistence",
			path: mod + "/internal/db",
			why:  "domain must not know how it is stored",
		},
		{
			name: "transport",
			path: mod + "/internal/api",
			why:  "domain must not know how it is served",
		},
		{
			name: "net/http",
			path: "net/http",
			why:  "an http type in a money path means a handler leaked into the domain",
		},
		{
			name: "database/sql",
			path: "database/sql",
			why:  "a query in the domain means a projection leaked into the database",
		},
	}

	imports, scanned := domainImports(t)

	// A walk that silently finds nothing would make every case below pass and
	// leave the boundary unguarded. Fail instead.
	if scanned == 0 {
		t.Fatal("parsed no .go files under internal/domain: the walk is broken, not the code")
	}
	t.Logf("checked %d import(s) across %d file(s)", len(imports), scanned)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for imp, files := range imports {
				if imp != tt.path && !strings.HasPrefix(imp, tt.path+"/") {
					continue
				}
				t.Errorf("internal/domain imports %q\n  why this is banned: %s\n  found in: %s",
					imp, tt.why, strings.Join(files, ", "))
			}
		})
	}
}

// domainImports parses every Go file under the current package directory and
// returns each imported path with the files that import it, plus the number of
// files parsed. Test files are included on purpose: a domain test that reaches
// for a database is the same mistake as domain code doing it.
func domainImports(t *testing.T) (map[string][]string, int) {
	t.Helper()

	imports := make(map[string][]string)
	scanned := 0
	fset := token.NewFileSet()

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		scanned++

		for _, spec := range f.Imports {
			p, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			imports[p] = append(imports[p], filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/domain: %v", err)
	}

	return imports, scanned
}

// modulePath reads the module path out of go.mod so the banned prefixes above
// stay correct if the module is ever renamed.
func modulePath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating working directory: %v", err)
	}

	for {
		gomod := filepath.Join(dir, "go.mod")
		b, err := os.ReadFile(gomod)
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					return strings.TrimSpace(rest)
				}
			}
			t.Fatalf("no module line in %s", gomod)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("walked to the filesystem root without finding go.mod")
		}
		dir = parent
	}
}
