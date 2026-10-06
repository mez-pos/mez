//go:build !windows

package printq

import (
	"context"
	"fmt"

	"github.com/byjoellobo/mez/internal/domain/printjob"
)

// sendSpooler has no spooler to send to off Windows. The job goes dead with
// the reason, rather than retrying something that cannot work.
func sendSpooler(_ context.Context, printer string, _ []byte) error {
	return shown(fmt.Errorf("printer %q uses the Windows spooler, and this server is not on Windows: %w", printer, printjob.ErrPermanent))
}
