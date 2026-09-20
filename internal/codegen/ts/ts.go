// Package ts turns Go request and response structs into TypeScript
// declarations.
//
// It exists so the frontends never hand-write a type that mirrors a Go
// struct, which docs/03-conventions.md bans: a hand-written mirror is a lie
// waiting to happen, and the day it lies is the day a total is wrong on a
// screen. The generated file is committed, and a test regenerates it and
// fails if it differs, so Go and TypeScript cannot drift apart.
//
// It handles what this API actually uses: structs, embedded structs (which
// encoding/json flattens, so this flattens them too), slices, maps,
// pointers, named string types registered as unions, and the handful of
// types whose JSON is not their struct.
package ts

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Options configure a Generator.
type Options struct {
	// Header is written at the top of the file, comment markers included.
	Header string

	// Inline maps a Go type to the TypeScript it is written as, for types
	// whose JSON is not their struct: a time.Time is a string.
	Inline map[reflect.Type]string

	// Names renames a Go type in TypeScript. Without one, an exported name
	// is kept and an unexported one is capitalised.
	Names map[reflect.Type]string
}

// Generator collects declarations and writes them out in the order they
// were discovered, so the same input always produces the same file.
type Generator struct {
	header string
	inline map[reflect.Type]string
	names  map[reflect.Type]string

	emitted map[reflect.Type]string // struct types already declared
	taken   map[string]reflect.Type // TS name to the type that has it
	order   []string
	decls   map[string]string
}

// New returns a Generator.
func New(opts Options) *Generator {
	g := &Generator{
		header:  opts.Header,
		inline:  map[reflect.Type]string{},
		names:   map[reflect.Type]string{},
		emitted: map[reflect.Type]string{},
		taken:   map[string]reflect.Type{},
		decls:   map[string]string{},
	}
	for t, expr := range opts.Inline {
		g.inline[t] = expr
	}
	for t, name := range opts.Names {
		g.names[t] = name
	}
	return g
}

// Union declares a string union and writes every value of sample's type as
// that union from then on.
//
// sample is one value of the Go type, so the mapping cannot be attached to
// the wrong type by a typo in a string. That type must be a named one: a
// union registered against plain string would turn every string in the API
// into it, which is a mistake worth refusing rather than debugging.
func (g *Generator) Union(name string, sample any, values ...string) error {
	if len(values) == 0 {
		return fmt.Errorf("ts: union %s has no values", name)
	}
	t := reflect.TypeOf(sample)
	if t == nil || t.PkgPath() == "" {
		return fmt.Errorf("ts: union %s needs a sample of a named type, not %v: "+
			"a builtin would make every value of it this union", name, t)
	}
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = quote(v)
	}
	if err := g.declare(name, fmt.Sprintf("export type %s =\n  | %s;", name, strings.Join(quoted, "\n  | ")), reflect.TypeOf(sample)); err != nil {
		return err
	}
	g.inline[reflect.TypeOf(sample)] = name
	return nil
}

// Raw declares a block written by hand, for a type whose JSON no amount of
// reflection can work out: an envelope with a custom marshaller.
//
// sample may be nil when the block stands for no Go type.
func (g *Generator) Raw(name, source string, sample any) error {
	var t reflect.Type
	if sample != nil {
		t = reflect.TypeOf(sample)
	}
	if err := g.declare(name, strings.TrimRight(source, "\n"), t); err != nil {
		return err
	}
	if t != nil {
		g.inline[t] = name
	}
	return nil
}

// Add declares an interface for sample's struct type, and for every struct
// it reaches. It returns the TypeScript name.
func (g *Generator) Add(sample any) (string, error) {
	return g.addType(reflect.TypeOf(sample))
}

func (g *Generator) addType(t reflect.Type) (string, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if expr, ok := g.inline[t]; ok {
		return expr, nil
	}
	if name, ok := g.emitted[t]; ok {
		return name, nil
	}
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("ts: %s is not a struct", t)
	}

	name, err := g.nameOf(t)
	if err != nil {
		return "", err
	}
	// Claim the name before walking the fields, so a type that reaches
	// itself does not recurse forever.
	g.emitted[t] = name
	g.taken[name] = t
	g.order = append(g.order, name)

	fields, err := g.fieldsOf(t)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)
	for _, f := range fields {
		b.WriteString("  " + f + "\n")
	}
	b.WriteString("}")
	g.decls[name] = b.String()
	return name, nil
}

// fieldsOf renders a struct's fields, flattening embedded structs the way
// encoding/json does.
func (g *Generator) fieldsOf(t reflect.Type) ([]string, error) {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		// An embedded struct is never "exported" when its own type is
		// unexported, but encoding/json still promotes the exported fields
		// inside it, so it has to be walked rather than skipped.
		embedded := f.Anonymous && deref(f.Type).Kind() == reflect.Struct
		if !f.IsExported() && !embedded {
			continue
		}
		tag := f.Tag.Get("json")
		jsonName, opts, _ := strings.Cut(tag, ",")
		if jsonName == "-" && opts == "" {
			continue
		}

		// An embedded struct with no name of its own is spread into this
		// one, which is what encoding/json does with it.
		if f.Anonymous && jsonName == "" && deref(f.Type).Kind() == reflect.Struct {
			if _, inlined := g.inline[deref(f.Type)]; !inlined {
				nested, err := g.fieldsOf(deref(f.Type))
				if err != nil {
					return nil, err
				}
				out = append(out, nested...)
				continue
			}
		}

		if jsonName == "" {
			jsonName = f.Name
		}
		expr, err := g.exprOf(f.Type)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", t.Name(), f.Name, err)
		}
		optional := ""
		if strings.Contains(opts, "omitempty") {
			optional = "?"
		}
		if f.Type.Kind() == reflect.Pointer && optional == "" {
			expr += " | null"
		}
		out = append(out, fmt.Sprintf("%s%s: %s;", jsonName, optional, expr))
	}
	return out, nil
}

// exprOf is the TypeScript for one Go type.
func (g *Generator) exprOf(t reflect.Type) (string, error) {
	if expr, ok := g.inline[t]; ok {
		return expr, nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		return g.exprOf(t.Elem())
	case reflect.String:
		return "string", nil
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number", nil
	case reflect.Slice, reflect.Array:
		// A byte slice is base64 in JSON, and the only ones here are raw
		// JSON, which has its own inline entry. Anything else is a list.
		if t.Elem().Kind() == reflect.Uint8 {
			return "string", nil
		}
		elem, err := g.exprOf(t.Elem())
		if err != nil {
			return "", err
		}
		return elem + "[]", nil
	case reflect.Map:
		key, err := g.exprOf(t.Key())
		if err != nil {
			return "", err
		}
		value, err := g.exprOf(t.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Record<%s, %s>", key, value), nil
	case reflect.Struct:
		return g.addType(t)
	case reflect.Interface:
		return "unknown", nil
	default:
		return "", fmt.Errorf("ts: no TypeScript for %s", t.Kind())
	}
}

// declare records a block under a name, refusing to use one name twice.
func (g *Generator) declare(name, source string, t reflect.Type) error {
	if owner, taken := g.taken[name]; taken {
		return fmt.Errorf("ts: %s is declared twice (%v and %v)", name, owner, t)
	}
	g.taken[name] = t
	g.order = append(g.order, name)
	g.decls[name] = source
	if t != nil && t.Kind() == reflect.Struct {
		g.emitted[t] = name
	}
	return nil
}

// nameOf is a Go type's TypeScript name: the override, or the Go name with
// its first letter capitalised, since half of these are unexported.
func (g *Generator) nameOf(t reflect.Type) (string, error) {
	if name, ok := g.names[t]; ok {
		return name, nil
	}
	name := t.Name()
	if name == "" {
		return "", fmt.Errorf("ts: %s has no name; give it one with Names", t)
	}
	name = strings.ToUpper(name[:1]) + name[1:]
	if owner, taken := g.taken[name]; taken && owner != t {
		return "", fmt.Errorf("ts: %s and %v both want the name %s; rename one with Names", t, owner, name)
	}
	return name, nil
}

// Source is the file: the header, then every declaration in the order it
// was discovered.
func (g *Generator) Source() string {
	var b strings.Builder
	b.WriteString(g.header)
	if !strings.HasSuffix(g.header, "\n") {
		b.WriteString("\n")
	}
	for _, name := range g.order {
		b.WriteString("\n")
		b.WriteString(g.decls[name])
		b.WriteString("\n")
	}
	return b.String()
}

// Names is every declaration, sorted, for a test that wants to check the
// file covers what it should.
func (g *Generator) Names() []string {
	out := append([]string(nil), g.order...)
	sort.Strings(out)
	return out
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func quote(s string) string { return `'` + strings.ReplaceAll(s, `'`, `\'`) + `'` }
