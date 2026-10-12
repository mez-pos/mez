package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/config"
	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/tunnel"
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

// venueLink is the venue's way to the outside, as it stands now: its
// pairing, the tunnel that pairing or the settings gave it, and the address
// it is reached at. It is one thing because pairing while the server runs
// changes all three together (ADR-320), and everything that reads one of
// them reads it here, so nothing is left holding what was true at start.
type venueLink struct {
	// settingsPublicURL is public_url as the settings file gave it, which
	// stands over a pairing's hostname for as long as the server runs.
	settingsPublicURL string

	mu      sync.Mutex
	pairing *cloud.Pairing
	use     pairingUse
	client  *tunnel.Client
	closed  bool
}

func newVenueLink(settingsPublicURL string, p *cloud.Pairing, use pairingUse) *venueLink {
	return &venueLink{settingsPublicURL: settingsPublicURL, pairing: p, use: use}
}

// inUse is the pairing the venue runs on, or nil.
func (l *venueLink) inUse() *cloud.Pairing {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.use != pairingInUse {
		return nil
	}
	return l.pairing
}

// pairedTo is the pairing as the API shows it: who, where and since when,
// and neither key. False for a venue that is not paired, or is and does not
// use it.
func (l *venueLink) pairedTo() (api.TunnelPairing, bool) {
	p := l.inUse()
	if p == nil {
		return api.TunnelPairing{}, false
	}
	return api.TunnelPairing{Owner: p.Owner.Name, Hostname: p.Venue.Hostname, PairedAt: p.PairedAt}, true
}

// publicURL is where guests reach the venue: what the settings file says,
// or the pairing's hostname, or nowhere.
func (l *venueLink) publicURL() string {
	if l.settingsPublicURL != "" {
		return l.settingsPublicURL
	}
	if p := l.inUse(); p != nil {
		return p.PublicURL()
	}
	return ""
}

// syncFloor is the fastest a backup may send when it goes to the bucket
// the venue's pairing gave it: mez Cloud's bucket is billed to mez Cloud,
// and at the five seconds a venue may set for a bucket of its own the
// requests cost more than the venue pays (ADR-313, ADR-315). Any other
// destination, and a venue with no pairing, has no floor but the setting's.
// A pairing set aside counts: the bucket is still theirs.
func (l *venueLink) syncFloor(dest backup.Config) time.Duration {
	l.mu.Lock()
	p := l.pairing
	l.mu.Unlock()
	if p == nil || dest.Kind != backup.KindS3 {
		return 0
	}
	theirs, ours := dest.S3, p.Backup
	if theirs.Bucket != ours.Bucket || theirs.Endpoint != ours.Endpoint || theirs.Path != ours.Path {
		return 0
	}
	return backup.DefaultSyncInterval
}

// status is the tunnel's, in the API's words. Off with none.
func (l *venueLink) status() api.TunnelStatus {
	l.mu.Lock()
	client := l.client
	l.mu.Unlock()
	return tunnelStatus(client)
}

// connect makes client the venue's tunnel and starts it. It is for the
// start, when there is no other to stop.
func (l *venueLink) connect(client *tunnel.Client) {
	l.mu.Lock()
	l.client = client
	l.mu.Unlock()
	client.Start()
}

// swapTimeout bounds the goodbye to the tunnel a new pairing replaces.
const swapTimeout = 10 * time.Second

// apply puts a pairing that has just been kept into use: from now the
// venue is paired to it, reached at its hostname, and connected to its
// relay with its key. dial builds the tunnel for it. The tunnel there was,
// from an earlier pairing, is said goodbye to first: its key has been
// revoked, and two tunnels for one venue take each other's place for ever.
//
// It reports whether mez must be restarted for the pairing to take hold,
// which it must only if the tunnel could not be built.
func (l *venueLink) apply(ctx context.Context, p cloud.Pairing, dial func(cloud.Pairing) (*tunnel.Client, error)) (restartNeeded bool, err error) {
	client, err := dial(p)
	if err != nil {
		return true, err
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return true, errors.New("mez is stopping")
	}
	old := l.client
	l.pairing, l.use, l.client = &p, pairingInUse, client
	l.mu.Unlock()

	if old != nil {
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), swapTimeout)
		_ = old.Close(closing)
		cancel()
	}
	client.Start()
	return false, nil
}

// Close stops the tunnel, if there is one, and refuses any after it.
func (l *venueLink) Close(ctx context.Context) error {
	l.mu.Lock()
	client := l.client
	l.closed = true
	l.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close(ctx)
}

// ensureCloudOwner gives a paired venue the person mez Cloud signs in as,
// when it has none: a venue paired by a build from before there was such a
// person, or one whose pairing could not make them (ADR-323). One who is
// there is left as found, turned off included. It never stops a start: the
// till does not need the cloud, and what went wrong is in the log.
//
// A pairing does not say which venue it is for, so on a machine with more
// than one there is nobody it can safely be given to, and pairing again
// from the venue that was meant is what makes them.
func ensureCloudOwner(ctx context.Context, database *db.DB, staff *auth.StaffAuth, p *cloud.Pairing, log *slog.Logger) {
	if p == nil {
		return
	}
	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		log.Warn("the person mez Cloud signs in as could not be looked for", "error", err)
		return
	}
	switch len(venues) {
	case 0:
		// No venue yet: the wizard makes one, and pairing in it makes them.
		return
	case 1:
	default:
		log.Warn("this machine has more than one venue, so the person mez Cloud signs in as is not made at start; pair again from the venue that is paired", "venues", len(venues))
		return
	}
	if _, err := staff.EnsureCloudOwner(ctx, venues[0].ID, p.Owner.Name); err != nil {
		log.Warn("the person mez Cloud signs in as could not be made; the owner cannot open the back office from mez Cloud until this venue is paired again",
			"venue_id", venues[0].ID, "error", err)
	}
}
