package ts

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

type colour string

const colourRed colour = "red"

type inner struct {
	Deep string `json:"deep"`
}

type sample struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Colour    colour           `json:"colour"`
	Fine      bool             `json:"fine"`
	When      time.Time        `json:"when"`
	Maybe     *inner           `json:"maybe,omitempty"`
	OrNothing *inner           `json:"or_nothing"`
	Many      []inner          `json:"many"`
	ByName    map[string]int64 `json:"by_name"`
	Anything  any              `json:"anything"`
	Skipped   string           `json:"-"`
	//lint:ignore U1000 it exists to prove an unexported field is not emitted
	unexported string
	NoTag      int
}

type embedder struct {
	inner
	Own string `json:"own"`
}

func generated(t *testing.T, build func(*Generator) error) string {
	t.Helper()
	g := New(Options{
		Header: "// generated",
		Inline: map[reflect.Type]string{reflect.TypeOf(time.Time{}): "string"},
	})
	if err := build(g); err != nil {
		t.Fatalf("building: %v", err)
	}
	return g.Source()
}

func TestFieldsAndTypes(t *testing.T) {
	src := generated(t, func(g *Generator) error {
		if err := g.Union("Colour", colourRed, "red", "green"); err != nil {
			return err
		}
		_, err := g.Add(sample{})
		return err
	})

	want := []string{
		"export type Colour =\n  | 'red'\n  | 'green';",
		"id: number;",
		"name: string;",
		"colour: Colour;",
		"fine: boolean;",
		"when: string;",
		"maybe?: Inner;",
		"or_nothing: Inner | null;",
		"many: Inner[];",
		"by_name: Record<string, number>;",
		"anything: unknown;",
		"NoTag: number;",
	}
	for _, w := range want {
		if !strings.Contains(src, w) {
			t.Errorf("the output does not contain %q:\n%s", w, src)
		}
	}
	for _, unwanted := range []string{"Skipped", "unexported"} {
		if strings.Contains(src, unwanted) {
			t.Errorf("%s was emitted", unwanted)
		}
	}
	if !strings.HasPrefix(src, "// generated\n") {
		t.Error("the header is missing")
	}
}

// TestEmbeddedStructsAreFlattened matches what encoding/json does with
// them, which is the only thing a client will ever see.
func TestEmbeddedStructsAreFlattened(t *testing.T) {
	src := generated(t, func(g *Generator) error {
		_, err := g.Add(embedder{})
		return err
	})
	if !strings.Contains(src, "export interface Embedder {\n  deep: string;\n  own: string;\n}") {
		t.Errorf("embedded fields were not flattened:\n%s", src)
	}
	if strings.Contains(src, "inner:") {
		t.Error("the embedded struct was emitted as a field")
	}
}

// TestAUnionNeedsANamedType is the mistake this API exists to prevent: a
// union registered against plain string would make every string in the API
// that union.
func TestAUnionNeedsANamedType(t *testing.T) {
	g := New(Options{})
	if err := g.Union("Thing", "", "a", "b"); err == nil {
		t.Fatal("a union was registered against plain string")
	}
	if err := g.Union("Thing", 1, "a"); err == nil {
		t.Error("a union was registered against an int")
	}
	if err := g.Union("Thing", colourRed); err == nil {
		t.Error("a union with no values was accepted")
	}
}

func TestNamesAndCollisions(t *testing.T) {
	t.Run("an unexported name is capitalised", func(t *testing.T) {
		src := generated(t, func(g *Generator) error {
			_, err := g.Add(inner{})
			return err
		})
		if !strings.Contains(src, "export interface Inner {") {
			t.Errorf("output:\n%s", src)
		}
	})

	t.Run("a name can be overridden", func(t *testing.T) {
		g := New(Options{Names: map[reflect.Type]string{reflect.TypeOf(inner{}): "Deepest"}})
		name, err := g.Add(inner{})
		if err != nil || name != "Deepest" {
			t.Fatalf("Add = %q, %v", name, err)
		}
	})

	t.Run("two types wanting one name is an error, not a silent overwrite", func(t *testing.T) {
		g := New(Options{})
		if err := g.Union("Inner", colourRed, "x"); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Add(inner{}); err == nil {
			t.Error("a name was used twice")
		}
	})

	t.Run("a type with no name needs one", func(t *testing.T) {
		g := New(Options{})
		if _, err := g.Add(struct{ A int }{}); err == nil {
			t.Error("an anonymous struct was emitted")
		}
	})
}

func TestRawAndAddedTwice(t *testing.T) {
	g := New(Options{})
	if err := g.Raw("Custom", "export type Custom = string;\n", inner{}); err != nil {
		t.Fatal(err)
	}
	// A type with a hand-written block is used, not emitted again.
	name, err := g.Add(inner{})
	if err != nil || name != "Custom" {
		t.Fatalf("Add after Raw = %q, %v", name, err)
	}
	if got := strings.Count(g.Source(), "export type Custom"); got != 1 {
		t.Errorf("Custom is declared %d times:\n%s", got, g.Source())
	}
	if strings.Contains(g.Source(), "export interface Inner") {
		t.Errorf("the type was emitted as well as written by hand:\n%s", g.Source())
	}
	if names := g.Names(); len(names) != 1 || names[0] != "Custom" {
		t.Errorf("Names = %v", names)
	}
}

// TestSourceIsStable is what makes the generated file reviewable: the same
// input gives byte-identical output, so a diff is a real change.
func TestSourceIsStable(t *testing.T) {
	build := func(g *Generator) error {
		if err := g.Union("Colour", colourRed, "red"); err != nil {
			return err
		}
		if _, err := g.Add(sample{}); err != nil {
			return err
		}
		_, err := g.Add(embedder{})
		return err
	}
	first, second := generated(t, build), generated(t, build)
	if first != second {
		t.Error("two runs produced different files")
	}
}

func TestAnUnsupportedTypeIsAnError(t *testing.T) {
	g := New(Options{})
	type withChannel struct {
		C chan int `json:"c"`
	}
	if _, err := g.Add(withChannel{}); err == nil {
		t.Error("a channel was given a TypeScript type")
	}
	if _, err := g.Add("not a struct"); err == nil {
		t.Error("a string was added as a struct")
	}
}
