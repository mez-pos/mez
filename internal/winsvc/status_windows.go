//go:build windows

package winsvc

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// status asks the Service Control Manager with the one right it takes to
// ask. The service library opens the service with the rights to start and
// stop it as well, which only an administrator holds, so through it `mez
// service status` was refused to everyone else once a service existed.
func (s *Service) status() (Status, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return "", fmt.Errorf("reaching the service manager: %w", err)
	}
	defer windows.CloseServiceHandle(manager) //nolint:errcheck // nothing to do if it fails

	name, err := windows.UTF16PtrFromString(Name)
	if err != nil {
		return "", err
	}
	handle, err := windows.OpenService(manager, name, windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return StatusNotInstalled, nil
	}
	if err != nil {
		return "", err
	}
	defer windows.CloseServiceHandle(handle) //nolint:errcheck // nothing to do if it fails

	var st windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(handle, &st); err != nil {
		return "", err
	}
	switch st.CurrentState {
	case windows.SERVICE_RUNNING, windows.SERVICE_START_PENDING:
		return StatusRunning, nil
	default:
		return StatusStopped, nil
	}
}
