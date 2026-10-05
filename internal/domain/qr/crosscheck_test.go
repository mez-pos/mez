package qr

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestAgainstAReferenceEncoder compares codes, module for module, with
// another encoder's: every version, every mask, at lengths that fill a code
// to the byte and that leave it nearly empty.
//
// It runs only when QR_REFERENCE names a program that reads lines of
// "version mask hexdata" and prints each matrix as rows of 0 and 1 joined
// by "/". It was run against python-qrcode when this package was written
// (ADR-217); golden_test.go keeps a few of its answers so the build does
// not need it.
func TestAgainstAReferenceEncoder(t *testing.T) {
	program := os.Getenv("QR_REFERENCE")
	if program == "" {
		t.Skip("QR_REFERENCE is not set")
	}
	type job struct {
		version, mask int
		data          []byte
	}
	var jobs []job
	var input bytes.Buffer
	for version := 1; version <= 40; version++ {
		full := (8*dataCodewords(version) - 4 - countBits(version)) / 8
		for i, n := range []int{full, full - 1, 1, full / 2} {
			if n < 1 {
				continue
			}
			data := make([]byte, n)
			for k := range data {
				data[k] = byte(k*31 + version*7 + i)
			}
			mask := (version + i) % 8
			jobs = append(jobs, job{version, mask, data})
			fmt.Fprintf(&input, "%d %d %s\n", version, mask, hex.EncodeToString(data))
		}
	}
	// Every mask, on one small code.
	for mask := range 8 {
		data := []byte("upi://pay?pa=cafe%40okbank&am=540.00&cu=INR")
		jobs = append(jobs, job{4, mask, data})
		fmt.Fprintf(&input, "4 %d %s\n", mask, hex.EncodeToString(data))
	}

	// And the codes golden_test.go pins, so those fingerprints are of codes
	// the reference agrees with.
	for _, g := range goldens {
		jobs = append(jobs, job{g.version, g.mask, []byte(g.data)})
		fmt.Fprintf(&input, "%d %d %s\n", g.version, g.mask, hex.EncodeToString([]byte(g.data)))
	}

	cmd := exec.Command(strings.Fields(program)[0], strings.Fields(program)[1:]...)
	cmd.Stdin = &input
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running %s: %v", program, err)
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r", "")), "\n")
	if len(lines) != len(jobs) {
		t.Fatalf("the reference answered %d of %d", len(lines), len(jobs))
	}
	for i, j := range jobs {
		got := matrix(encode(j.data, j.version, j.mask))
		if got != lines[i] {
			t.Errorf("version %d, mask %d, %d bytes: not the reference's code", j.version, j.mask, len(j.data))
		}
	}
	t.Logf("%d codes match the reference", len(jobs))
}

// matrix is a code as rows of 0 and 1 joined by "/".
func matrix(c Code) string {
	rows := make([]string, c.Size)
	for y := range c.Size {
		var b strings.Builder
		for x := range c.Size {
			if c.Dark(x, y) {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		}
		rows[y] = b.String()
	}
	return strings.Join(rows, "/")
}
