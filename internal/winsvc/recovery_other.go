//go:build !windows

package winsvc

import "time"

// setRecovery is Windows's. Elsewhere the unit file says when to restart.
func setRecovery(string, []time.Duration, time.Duration) error { return nil }
