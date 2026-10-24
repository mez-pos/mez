//go:build !windows

package winsvc

import "errors"

// AllowStartByUsers is a Windows matter.
func AllowStartByUsers() error {
	return errors.New("there is no Windows service on this system")
}
