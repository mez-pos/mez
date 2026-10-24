//go:build windows

package printq

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/mez-pos/mez/internal/domain/printjob"
)

// The Windows print spooler, called directly: winspool.drv's RAW printing
// path, the same one "copy /b file \\PC\Printer" takes. RAW means the
// spooler passes the ESC/POS bytes to the printer untouched, which is how a
// USB receipt printer the venue computer drives itself gets a job. No cgo:
// syscall loads the DLL at run time.
var (
	winspool             = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
)

// errInvalidPrinterName is ERROR_INVALID_PRINTER_NAME: no printer by that
// name on this computer. A retry will not find one; a person must fix the
// printer's settings and retry the job.
const errInvalidPrinterName syscall.Errno = 1801

// docInfo1 is DOC_INFO_1W.
type docInfo1 struct {
	docName    *uint16
	outputFile *uint16
	datatype   *uint16
}

// spoolerTimeout is how long the spooler is given to take a job. A spooler
// that is stuck, or a printer set to print directly that is off, holds a
// Win32 call open for ever; the queue must not hang on it, nor the stop.
const spoolerTimeout = 45 * time.Second

// sendSpooler hands data to the Windows spooler for printer, giving up on a
// spooler that does not answer. The call itself cannot be cancelled, so a
// call given up on runs out on its own goroutine, and the job is tried
// again later as any failed send is.
func sendSpooler(ctx context.Context, printer string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("spooler: nothing to print")
	}
	result := make(chan error, 1)
	go func() { result <- spool(printer, data) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(spoolerTimeout):
		return fmt.Errorf("spooler: %q did not take the job within %s", printer, spoolerTimeout)
	}
}

// spool is the Win32 conversation with the spooler, start to end.
func spool(printer string, data []byte) error {
	if err := winspool.Load(); err != nil {
		return fmt.Errorf("spooler: loading winspool.drv: %v: %w", err, printjob.ErrPermanent)
	}
	name, err := syscall.UTF16PtrFromString(printer)
	if err != nil {
		return fmt.Errorf("spooler: printer name %q: %v: %w", printer, err, printjob.ErrPermanent)
	}

	var h syscall.Handle
	if ok, _, errno := procOpenPrinterW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&h)), 0); ok == 0 {
		if errors.Is(errno, errInvalidPrinterName) {
			return shown(fmt.Errorf("no printer named %q on this computer: %w", printer, printjob.ErrPermanent))
		}
		return fmt.Errorf("spooler: opening %q: %w", printer, errno)
	}
	defer procClosePrinter.Call(uintptr(h)) //nolint:errcheck // nothing to do if closing fails

	doc := docInfo1{docName: utf16("mez"), datatype: utf16("RAW")}
	if job, _, errno := procStartDocPrinterW.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&doc))); job == 0 {
		return fmt.Errorf("spooler: starting a document on %q: %w", printer, errno)
	}
	// EndDocPrinter hands the job to the printer; without it the spooler
	// holds the bytes forever.
	defer procEndDocPrinter.Call(uintptr(h)) //nolint:errcheck // reported by the write below if it matters
	if ok, _, errno := procStartPagePrinter.Call(uintptr(h)); ok == 0 {
		return fmt.Errorf("spooler: starting a page on %q: %w", printer, errno)
	}
	var written uint32
	ok, _, errno := procWritePrinter.Call(uintptr(h), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&written)))
	procEndPagePrinter.Call(uintptr(h)) //nolint:errcheck // the write's result is what matters
	if ok == 0 {
		return fmt.Errorf("spooler: writing to %q: %w", printer, errno)
	}
	if int(written) != len(data) {
		return fmt.Errorf("spooler: %q took %d of %d bytes", printer, written, len(data))
	}
	return nil
}

func utf16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s) // constants here, never with a NUL
	return p
}
