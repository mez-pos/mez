package invoice

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultTemplate is the format from docs/02-domain.md. With a two-letter
// prefix it produces "NF/2026-27/000001", 17 characters, one over GST's
// limit: a GST series wants "{prefix}/{fy:short}/{seq:06d}" instead.
const DefaultTemplate = "{prefix}/{fy}/{seq:06d}"

// maxSeqWidth bounds {seq:0Nd}. 18 digits is every positive int64 but the
// last few, and no venue will issue a quintillion invoices.
const maxSeqWidth = 18

// Template is a parsed invoice number format. The tokens are:
//
//	{prefix}     the series prefix
//	{fy}         the financial year, "2026-27" or "2026"
//	{fy:short}   the same with two-digit years, "26-27" or "26"
//	{seq}        the number, unpadded
//	{seq:0Nd}    the number, zero-padded to N digits (1 to 18)
//
// Everything else is copied as written. A template must hold exactly one
// {seq}, or two invoices could format to the same string.
type Template struct {
	raw   string
	parts []part
}

type tokenKind int

const (
	literal tokenKind = iota
	tokPrefix
	tokFY
	tokFYShort
	tokSeq
)

type part struct {
	kind  tokenKind
	text  string // for a literal
	width int    // for {seq:0Nd}; 0 means unpadded
}

// ParseTemplate parses and checks a template.
func ParseTemplate(s string) (Template, error) {
	var parts []part
	seqs := 0
	rest := s
	addLiteral := func(text string) error {
		if strings.ContainsRune(text, '}') {
			return fmt.Errorf("%w: stray } in %q", ErrInvalidTemplate, s)
		}
		parts = append(parts, part{kind: literal, text: text})
		return nil
	}
	for rest != "" {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			if err := addLiteral(rest); err != nil {
				return Template{}, err
			}
			break
		}
		if open > 0 {
			if err := addLiteral(rest[:open]); err != nil {
				return Template{}, err
			}
		}
		end := strings.IndexByte(rest[open:], '}')
		if end < 0 {
			return Template{}, fmt.Errorf("%w: unclosed { in %q", ErrInvalidTemplate, s)
		}
		p, err := parseToken(rest[open+1 : open+end])
		if err != nil {
			return Template{}, fmt.Errorf("%w: %v in %q", ErrInvalidTemplate, err, s)
		}
		if p.kind == tokSeq {
			seqs++
		}
		parts = append(parts, p)
		rest = rest[open+end+1:]
	}
	if seqs != 1 {
		return Template{}, fmt.Errorf("%w: %q has %d {seq} tokens, needs exactly one", ErrInvalidTemplate, s, seqs)
	}
	return Template{raw: s, parts: parts}, nil
}

// parseToken reads the inside of one {...}.
func parseToken(tok string) (part, error) {
	switch tok {
	case "prefix":
		return part{kind: tokPrefix}, nil
	case "fy":
		return part{kind: tokFY}, nil
	case "fy:short":
		return part{kind: tokFYShort}, nil
	case "seq":
		return part{kind: tokSeq}, nil
	}
	spec, ok := strings.CutPrefix(tok, "seq:0")
	if !ok {
		return part{}, fmt.Errorf("unknown token {%s}", tok)
	}
	digits, ok := strings.CutSuffix(spec, "d")
	width, err := strconv.Atoi(digits)
	if !ok || err != nil || width < 1 || width > maxSeqWidth || digits != strconv.Itoa(width) {
		return part{}, fmt.Errorf("{%s}: want {seq:0Nd} with N from 1 to %d", tok, maxSeqWidth)
	}
	return part{kind: tokSeq, width: width}, nil
}

// Format writes one invoice number. A number wider than the padding is
// written in full, never truncated: truncating would make two numbers equal.
func (t Template) Format(prefix string, fy FinancialYear, seq int64) (string, error) {
	if len(t.parts) == 0 {
		return "", fmt.Errorf("%w: zero Template, use ParseTemplate", ErrInvalidTemplate)
	}
	if seq < 1 {
		return "", fmt.Errorf("%w: %d", ErrInvalidSeq, seq)
	}
	var b strings.Builder
	for _, p := range t.parts {
		switch p.kind {
		case literal:
			b.WriteString(p.text)
		case tokPrefix:
			b.WriteString(prefix)
		case tokFY:
			b.WriteString(fy.Label())
		case tokFYShort:
			b.WriteString(fy.ShortLabel())
		case tokSeq:
			fmt.Fprintf(&b, "%0*d", p.width, seq)
		}
	}
	return b.String(), nil
}

// String is the template as written.
func (t Template) String() string { return t.raw }
