//go:build windows

package winsvc

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

// setRecovery tells the Service Control Manager to restart the service
// after each delay in turn, the last one for every death after it, and to
// start the count again once it has stayed up for reset.
//
// The service library sets one delay and no more, so this goes to the
// manager directly.
func setRecovery(name string, delays []time.Duration, reset time.Duration) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("reaching the service manager: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck // nothing to do if it fails

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening the service: %w", err)
	}
	defer s.Close() //nolint:errcheck // nothing to do if it fails

	actions := make([]mgr.RecoveryAction, len(delays))
	for i, d := range delays {
		actions[i] = mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: d}
	}
	if err := s.SetRecoveryActions(actions, uint32(reset.Seconds())); err != nil {
		return fmt.Errorf("setting the restarts: %w", err)
	}
	return nil
}
