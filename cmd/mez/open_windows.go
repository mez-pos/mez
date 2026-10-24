//go:build windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/mez-pos/mez/internal/winsvc"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

// ownConsole reports whether this process is the only one on its console:
// what a double-click in Explorer gives it, where a terminal's command line
// shares the terminal's. No console at all (a service, a scheduled task) is
// not a double-click.
func ownConsole() bool {
	proc := kernel32.NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return false
	}
	var ids [2]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return n == 1
}

// freeConsole lets go of the console window this process was given, so
// that a shortcut does not leave a black box on the screen.
func freeConsole() {
	proc := kernel32.NewProc("FreeConsole")
	if proc.Find() == nil {
		_, _, _ = proc.Call()
	}
}

// alertBox shows text in a box the person must dismiss.
func alertBox(text string) {
	caption, _ := windows.UTF16PtrFromString("mez")
	body, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, body, caption, windows.MB_OK|windows.MB_ICONWARNING|windows.MB_SETFOREGROUND|windows.MB_TOPMOST)
}

// browserPaths is where Edge, which every Windows 10 and 11 has, and then
// Chrome are found: the installer's own order, which these replace
// (ADR-366).
func browserPaths() []string {
	pf64, pf32, local := os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")
	if pf64 == "" {
		pf64 = os.Getenv("ProgramFiles")
	}
	var paths []string
	for _, dir := range []string{pf32, pf64} {
		if dir != "" {
			paths = append(paths, filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe"))
		}
	}
	for _, dir := range []string{pf64, pf32, local} {
		if dir != "" {
			paths = append(paths, filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	return paths
}

// openBrowser opens the till at url as an app: a window of its own with no
// address bar or tabs (ADR-272). Full screen is the browser's kiosk mode.
// With neither browser the address is handed to Windows, which opens it in
// whatever the person uses.
func openBrowser(url string, fullscreen bool) error {
	for _, path := range browserPaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		args := []string{"--app=" + url, "--start-maximized"}
		if fullscreen {
			args = []string{"--app=" + url, "--kiosk"}
			if filepath.Base(path) == "msedge.exe" {
				args = append(args, "--edge-kiosk-type=fullscreen")
			}
		}
		cmd := exec.Command(path, args...)
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		if err := cmd.Start(); err != nil {
			continue
		}
		_ = cmd.Process.Release()
		return nil
	}
	verb, _ := windows.UTF16PtrFromString("open")
	file, _ := windows.UTF16PtrFromString(url)
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// elevateServiceStart runs "mez service start" as an administrator, which
// asks the person. Saying no is an error.
func elevateServiceStart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString("service start")
	return windows.ShellExecute(0, verb, file, args, nil, windows.SW_HIDE)
}

// doubleClicked is what "mez" with no arguments does when it was started by
// a double-click rather than from a terminal (ADR-366). With the service
// installed, the till is opened through it, as the shortcut does: a second
// server could not take the port, and the person wanted the till. Without
// it the server runs here, and the person is told that this window is what
// keeps it up. Handled is false when the server is to run as usual.
func doubleClicked(stdout io.Writer) (handled bool, err error) {
	if !ownConsole() {
		return false, nil
	}
	svc, err := winsvc.New(winsvc.Options{})
	if err != nil {
		return false, nil
	}
	status, err := svc.Status()
	if err != nil || status == winsvc.StatusNotInstalled {
		_, _ = io.WriteString(stdout, "\n  Keep this window open: closing it stops mez.\n"+
			"  To have mez start by itself with Windows and stay up, run the mez installer.\n\n")
		return false, nil
	}
	freeConsole()
	return true, runOpen(nil, io.Discard)
}

// hideOwnConsole lets go of a console nobody is reading: the one a
// shortcut gave "mez open".
func hideOwnConsole() {
	if ownConsole() {
		freeConsole()
	}
}
