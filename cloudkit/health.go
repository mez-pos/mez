package cloudkit

import (
	"github.com/mez-pos/mez/internal/relay"
	"github.com/mez-pos/mez/internal/relay/wire"
)

// What a venue says of its own health over the relay connection it already
// holds (ADR-361; Phase 15, Task 4). The cloud's Entitlements may also be a
// HealthSink: the relay then tells each venue on its dial that it reads
// health, and hands the sink each summary, newest first when it is slow.
// The cloud's clock alerts (Phase 15, Task 5) are drawn from these.
type (
	// HealthSink receives a venue's summary.
	HealthSink = relay.HealthSink
	// VenueHealth is the summary: printers, backup, the open shift and the
	// opening hours, as the venue sent it.
	VenueHealth = wire.Health
	// PrinterHealth is one printer's last success and last failure.
	PrinterHealth = wire.PrinterHealth
	// BackupHealth is the venue's backup as it sees it.
	BackupHealth = wire.BackupHealth
	// ShiftHealth is the shift now open.
	ShiftHealth = wire.ShiftHealth
	// DayHours is one weekday's opening and closing, in the venue's zone.
	DayHours = wire.DayHours
)
