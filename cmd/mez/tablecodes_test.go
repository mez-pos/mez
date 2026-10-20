package main

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/service/tablecodes"
)

// codeLine is one table as the command prints it: its name, its code, and
// the path a guest opens.
var codeLine = regexp.MustCompile(`(?m)^  Table (\S+)\s+([a-z0-9]{12})   /order/t/([a-z0-9]{12})$`)

func runTableCodesIn(t *testing.T, args ...string) string {
	t.Helper()
	var out strings.Builder
	if err := runTableCodes(args, &out); err != nil {
		t.Fatalf("table-codes: %v\n%s", err, out.String())
	}
	return out.String()
}

// TestTableCodesGivesEveryTableACodeThatLeadsToIt: the way to a code until
// the back office prints them. Each code printed is one the server accepts.
func TestTableCodesGivesEveryTableACodeThatLeadsToIt(t *testing.T) {
	ctx := context.Background()
	dir, args := setupArgs(t)
	runSetupIn(t, append(args, "-demo-menu"))

	out := runTableCodesIn(t, "-data-dir", dir)
	lines := codeLine.FindAllStringSubmatch(out, -1)
	if len(lines) != 12 {
		t.Fatalf("printed %d tables, want the demo floor's twelve:\n%s", len(lines), out)
	}

	svc, err := tablecodes.New(opened(t, dir), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		name, code, inPath := line[1], line[2], line[3]
		if code != inPath {
			t.Errorf("table %s: the path says %s and the code %s", name, inPath, code)
		}
		table, err := svc.Resolve(ctx, code)
		if err != nil || table.TableName != name {
			t.Errorf("table %s's printed code leads to %+v, %v", name, table, err)
		}
	}

	// Run again, it is the same list: a sticker on a table goes on working.
	if again := runTableCodesIn(t, "-data-dir", dir); again != out {
		t.Errorf("a second run changed the codes:\n%s\nthen\n%s", out, again)
	}
}

func TestTableCodesWithNoFloorSaysSo(t *testing.T) {
	dir, args := setupArgs(t)
	runSetupIn(t, args)
	if out := runTableCodesIn(t, "-data-dir", dir); !strings.Contains(out, "no tables yet") {
		t.Errorf("a venue with no floor:\n%s", out)
	}
}

func TestTableCodesRefusals(t *testing.T) {
	var out strings.Builder
	if err := runTableCodes([]string{"-data-dir", t.TempDir()}, &out); err == nil || !strings.Contains(err.Error(), "mez setup") {
		t.Errorf("no database: %v, want to be sent to setup", err)
	}
	if err := runTableCodes([]string{"extra"}, &out); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("a stray argument: %v", err)
	}

	// A database with no venue in it.
	empty := opened(t, t.TempDir())
	if err := empty.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureTableCodes(context.Background(), empty); !errors.Is(err, ErrNoVenue) {
		t.Errorf("no venue: %v, want ErrNoVenue", err)
	}
}
