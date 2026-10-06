//go:build !windows

package discover

// installedPrinters has no spooler to ask off Windows: a printer plugged
// into a Linux server is not one the print queue can drive (ADR-080), so
// there is nothing to offer.
func installedPrinters() ([]string, error) { return nil, nil }
