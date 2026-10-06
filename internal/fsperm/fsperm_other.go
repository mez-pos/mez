//go:build !windows

package fsperm

import "os"

func restrict(path string, dir bool) error {
	if dir {
		return os.Chmod(path, 0o700)
	}
	return os.Chmod(path, 0o600)
}
