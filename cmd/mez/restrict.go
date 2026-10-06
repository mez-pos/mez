package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/fsperm"
)

// restrict leaves path to the people who run the server (ADR-259) and says
// so at Warn when it cannot. It never stops the server: a till that will
// not start mid-service costs the venue more than a directory another
// account on the computer could read.
func restrict(log *slog.Logger, what, path string) {
	if err := fsperm.Restrict(path); err != nil {
		log.Warn("could not restrict the "+what+" to its owner", "path", path, "error", err)
	}
}

// restrictConfig restricts config.toml beside the binary, which can hold
// the relay key and a gateway's secret. Only on Windows: there a folder
// such as C:\mez inherits an ACL that lets every user read it, while on
// Linux the file is the administrator's, in a directory the service cannot
// write (deploy.md), and its mode is theirs to set.
func restrictConfig(log *slog.Logger, goos, exeDir string) {
	if goos != "windows" {
		return
	}
	path := filepath.Join(exeDir, config.FileName)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return // no file is the usual case: the server boots without one
	}
	restrict(log, "config file", path)
}

// binaryDir is the directory the running binary is in, where config.Load
// looks for config.toml, or "" if it cannot be told.
func binaryDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}
