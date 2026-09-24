package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/device/escpos"
)

// isTestPrint reports whether the command line asks for a test print:
// `nofiltr --test-print 192.168.1.50`, or -test-print, or with "=".
func isTestPrint(args []string) bool {
	if len(args) == 0 {
		return false
	}
	name := strings.TrimLeft(args[0], "-")
	return strings.HasPrefix(args[0], "-") && (name == "test-print" || strings.HasPrefix(name, "test-print="))
}

// testPrintOptions is what --test-print was asked to do.
type testPrintOptions struct {
	Addr   string
	Paper  escpos.Paper
	Drawer bool
	Save   string
}

// printTransport sends a test page; tests swap it for a recorder.
type printTransport func(ctx context.Context, addr string, data []byte) error

// runTestPrint prints the test page (escpos.TestPage) so a cafe owner can
// see before service that the printer, its paper width and its drawer all
// work. It needs no database and no server: a printer is tested before the
// venue is even set up.
func runTestPrint(args []string, stdout io.Writer, now time.Time, send printTransport) error {
	opts, err := parseTestPrintFlags(args, stdout)
	if err != nil {
		return err
	}
	page, err := escpos.TestPage(opts.Paper, now, buildInfo().Version, opts.Drawer)
	if err != nil {
		return err
	}
	if opts.Save != "" {
		if err := os.WriteFile(opts.Save, page, 0o600); err != nil {
			return fmt.Errorf("test-print: saving the page: %w", err)
		}
		fmt.Fprintf(stdout, "Saved the test page to %s (%d bytes). Send it to a printer as raw bytes.\n", opts.Save, len(page))
		return nil
	}
	if err := send(context.Background(), opts.Addr, page); err != nil {
		return fmt.Errorf("test-print: %w. Check the printer is on, on this network, and at that address", err)
	}
	fmt.Fprintf(stdout, "Sent the test page to %s (%d columns).\n"+
		"On the slip: a full-width ruler, bold and large text, a barcode, a QR code, and a cut.\n", opts.Addr, opts.Paper.Columns)
	if opts.Drawer {
		fmt.Fprintln(stdout, "The cash drawer should have opened.")
	}
	return nil
}

func parseTestPrintFlags(args []string, out io.Writer) (testPrintOptions, error) {
	var o testPrintOptions
	set := flag.NewFlagSet("nofiltr --test-print", flag.ContinueOnError)
	set.SetOutput(out)
	set.StringVar(&o.Addr, "test-print", "", "the printer's address, host or host:port (port 9100 if none)")
	width := set.Int("paper", 80, "roll width in mm: 80 or 58")
	set.BoolVar(&o.Drawer, "drawer", false, "also kick the cash drawer wired to the printer")
	set.StringVar(&o.Save, "save", "", "write the page's bytes to this file instead of sending them")
	if err := set.Parse(args); err != nil {
		return o, err
	}
	if set.NArg() > 0 {
		return o, fmt.Errorf("test-print: unexpected argument %q", set.Arg(0))
	}
	switch *width {
	case 80:
		o.Paper = escpos.Paper80
	case 58:
		o.Paper = escpos.Paper58
	default:
		return o, fmt.Errorf("test-print: paper is 80 or 58, not %d", *width)
	}
	if strings.TrimSpace(o.Addr) == "" && o.Save == "" {
		return o, errors.New("test-print: give the printer's address, for example --test-print 192.168.1.50")
	}
	return o, nil
}
