package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// parseTOML reads the flat subset of TOML this configuration actually uses:
// top-level key = value pairs, # comments, double-quoted strings and bare
// numbers, booleans or words.
//
// It is hand-written rather than a dependency. The whole configuration is six
// scalar settings, adding a TOML library would mean an ADR and a supply-chain
// surface for a cafe owner's antivirus to object to, and the cost of not
// supporting tables and arrays is a clearer error message rather than a missing
// feature. See ADR-018.
//
// Everything it refuses - tables, arrays, inline tables, single-quoted strings,
// duplicate keys - is refused loudly, with a line number. Silence is the one
// behaviour a config parser must not have: a setting that does not apply is
// worse than a file that does not load.
func parseTOML(r io.Reader) (map[string]string, error) {
	values := make(map[string]string)

	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "[") {
			return nil, fmt.Errorf("line %d: tables are not supported, this config is a flat list of key = value", line)
		}

		name, rest, ok := strings.Cut(text, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: want key = value, got %q", line, text)
		}
		name = strings.TrimSpace(name)
		if err := validKeyName(name); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if _, dup := values[name]; dup {
			return nil, fmt.Errorf("line %d: duplicate key %q", line, name)
		}

		value, err := parseValue(strings.TrimSpace(rest))
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", line, name, err)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading lines: %w", err)
	}
	return values, nil
}

func validKeyName(name string) error {
	if name == "" {
		return errors.New("empty key")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
		default:
			return fmt.Errorf("key %q: only letters, digits, underscore and dash are allowed", name)
		}
	}
	return nil
}

// parseValue reads one right-hand side. Every setting is a string by the time
// it reaches the caller, so numbers and booleans need no conversion here: build
// is the one place that decides what a value means.
func parseValue(v string) (string, error) {
	if v == "" {
		return "", errors.New("missing value")
	}
	switch v[0] {
	case '\'':
		return "", errors.New("single-quoted strings are not supported, use double quotes")
	case '[', '{':
		return "", errors.New("arrays and inline tables are not supported")
	case '"':
		return parseQuoted(v)
	}

	// A bare value. A # here starts a comment: no unquoted setting this config
	// holds can legitimately contain one.
	if i := strings.IndexByte(v, '#'); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("missing value")
	}
	if strings.ContainsAny(v, "[]{}") {
		return "", errors.New("arrays and inline tables are not supported")
	}
	return v, nil
}

func parseQuoted(v string) (string, error) {
	end := closingQuote(v)
	if end < 0 {
		return "", errors.New("unterminated string")
	}
	s, err := strconv.Unquote(v[:end+1])
	if err != nil {
		return "", fmt.Errorf("bad string: %w", err)
	}
	if tail := strings.TrimSpace(v[end+1:]); tail != "" && !strings.HasPrefix(tail, "#") {
		return "", fmt.Errorf("unexpected text after value: %q", tail)
	}
	return s, nil
}

// closingQuote returns the index of the quote that ends the string starting at
// v[0], honouring backslash escapes, or -1 if there is none. Finding it before
// looking for a comment is what lets a value contain a # character.
func closingQuote(v string) int {
	for i := 1; i < len(v); i++ {
		switch v[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}
