//go:build !windows

package main

import (
	"errors"
	"io"
	"os/exec"
	"runtime"
)

// openBrowser hands the address to the desktop.
func openBrowser(url string, _ bool) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, url).Start()
}

// alertBox has no box to show: the error is printed.
func alertBox(string) {}

// elevateServiceStart is a Windows matter.
func elevateServiceStart() error { return errors.New("there is no Windows service on this system") }

// doubleClicked is a Windows matter: elsewhere the server runs as usual.
func doubleClicked(io.Writer) (bool, error) { return false, nil }

// hideOwnConsole has no console to hide.
func hideOwnConsole() {}
