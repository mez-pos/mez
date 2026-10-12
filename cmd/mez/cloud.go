package main

import (
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/config"
)

// pairingUse is what the server makes of the venue's pairing (ADR-319).
type pairingUse int

const (
	// notPaired is a venue with no pairing, which is nearly every venue.
	notPaired pairingUse = iota
	// pairingInUse is a venue reached through the relay its pairing names.
	pairingInUse
	// pairingSetAside is a venue that is paired and whose settings file
	// names a relay of its own: the file is what whoever runs the machine
	// wrote, and it is the one obeyed.
	pairingSetAside
)

// loadPairing reads the venue's pairing from its data directory. None is
// nil and no error. One that cannot be read stops the start, as a backup's
// destination that cannot be read does: the owner believes the venue is
// connected, and the way out is in the message.
func loadPairing(dataDir string) (*cloud.Pairing, error) {
	p, err := cloud.Load(dataDir)
	if err != nil {
		return nil, fmt.Errorf("%w; pair the venue again from the back office, or delete that file to start with no pairing", err)
	}
	return p, nil
}

// withPairing is the configuration the server runs with, given the venue's
// pairing: where its relay is, the key to it, where it is reached, and that
// its back office crosses.
//
// A relay named in the settings file wins whole. It is not mixed with the
// pairing's key by key: a relay's address from one place and its key from
// another is a venue that connects to nothing and says why to nobody.
func withPairing(cfg config.Config, p *cloud.Pairing) (config.Config, pairingUse) {
	if p == nil {
		return cfg, notPaired
	}
	if cfg.RelayEnabled() {
		return cfg, pairingSetAside
	}
	cfg.RelayURL = p.Relay.URL
	cfg.RelayKey = p.Relay.Key
	// A venue is paired to have its back office reached, and the key it was
	// given is one the relay issued for that.
	cfg.RemoteBackoffice = true
	// Where the venue said it is reached still stands if it said: a venue
	// with a domain of its own in front of the cloud's hostname.
	if cfg.PublicURL == "" {
		cfg.PublicURL = p.PublicURL()
	}
	return cfg, pairingInUse
}

// pairedTo is the pairing as the API shows it: who, where and since when,
// and neither key.
func pairedTo(p *cloud.Pairing, use pairingUse) func() (api.TunnelPairing, bool) {
	return func() (api.TunnelPairing, bool) {
		if p == nil || use != pairingInUse {
			return api.TunnelPairing{}, false
		}
		return api.TunnelPairing{Owner: p.Owner.Name, Hostname: p.Venue.Hostname, PairedAt: p.PairedAt}, true
	}
}

// cloudSyncFloor is the fastest a backup may send when it goes to the bucket
// the venue's pairing gave it: mez Cloud's bucket is billed to mez Cloud,
// and at the five seconds a venue may set for a bucket of its own the
// requests cost more than the venue pays (ADR-313, ADR-315). Any other
// destination, and a venue with no pairing, has no floor but the setting's.
func cloudSyncFloor(p *cloud.Pairing) func(backup.Config) time.Duration {
	return func(dest backup.Config) time.Duration {
		if p == nil || dest.Kind != backup.KindS3 {
			return 0
		}
		theirs, ours := dest.S3, p.Backup
		if theirs.Bucket != ours.Bucket || theirs.Endpoint != ours.Endpoint || theirs.Path != ours.Path {
			return 0
		}
		return backup.DefaultSyncInterval
	}
}
