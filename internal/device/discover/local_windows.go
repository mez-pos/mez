//go:build windows

package discover

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// winspool.drv again, as internal/device/printq calls it to print: no cgo,
// the DLL is loaded at run time.
var (
	winspool          = syscall.NewLazyDLL("winspool.drv")
	procEnumPrintersW = winspool.NewProc("EnumPrintersW")
)

const (
	// printerEnumLocal is PRINTER_ENUM_LOCAL: the printers installed on
	// this computer. Connections to another computer's shared printers
	// are left out, because the server cannot print on one (ADR-259).
	printerEnumLocal = 0x2
	// errInsufficientBuffer is ERROR_INSUFFICIENT_BUFFER: the first call's
	// way of saying how much room the list needs.
	errInsufficientBuffer syscall.Errno = 122
	// maxEnumBytes bounds the buffer a spooler can ask for.
	maxEnumBytes = 1 << 20
)

// printerInfo4 is PRINTER_INFO_4W: a name, and no call to the printer
// itself to get it, so listing is quick even when one is switched off.
type printerInfo4 struct {
	printerName *uint16
	serverName  *uint16
	attributes  uint32
}

// installedPrinters lists the printers installed on this computer.
func installedPrinters() ([]string, error) {
	if err := winspool.Load(); err != nil {
		return nil, fmt.Errorf("loading winspool.drv: %w", err)
	}
	var needed, count uint32
	ok, _, errno := procEnumPrintersW.Call(printerEnumLocal, 0, 4, 0, 0,
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if ok != 0 || needed == 0 {
		return nil, nil // no printers at all
	}
	if !errors.Is(errno, errInsufficientBuffer) {
		return nil, fmt.Errorf("EnumPrinters: %w", errno)
	}
	if needed > maxEnumBytes {
		return nil, fmt.Errorf("EnumPrinters asked for %d bytes", needed)
	}
	// Words, not bytes, so the structures in it are aligned for their pointers.
	buf := make([]uintptr, (uintptr(needed)+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	ok, _, errno = procEnumPrintersW.Call(printerEnumLocal, 0, 4,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		return nil, fmt.Errorf("EnumPrinters: %w", errno)
	}
	infos := unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buf[0])), count)
	names := make([]string, 0, count)
	for _, info := range infos {
		if info.printerName != nil {
			names = append(names, utf16String(info.printerName))
		}
	}
	return names, nil
}

// maxNameUnits bounds the walk along a name the spooler gave: it always
// ends one, and this is for the day it does not.
const maxNameUnits = 1024

// utf16String reads a NUL-terminated UTF-16 string.
func utf16String(p *uint16) string {
	units := unsafe.Slice(p, maxNameUnits)
	for i, u := range units {
		if u == 0 {
			return syscall.UTF16ToString(units[:i])
		}
	}
	return syscall.UTF16ToString(units)
}
