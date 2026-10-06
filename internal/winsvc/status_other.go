//go:build !windows

package winsvc

import (
	"errors"

	"github.com/kardianos/service"
)

// status is the service library's own answer, off Windows.
func (s *Service) status() (Status, error) {
	st, err := s.svc.Status()
	switch {
	case errors.Is(err, service.ErrNotInstalled):
		return StatusNotInstalled, nil
	case err != nil:
		return "", err
	case st == service.StatusRunning:
		return StatusRunning, nil
	default:
		return StatusStopped, nil
	}
}
