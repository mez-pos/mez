package relay

import (
	"errors"
	"sync"
	"time"
)

// DefaultIPLimit is what one address may ask of the relay, across every
// venue. It is well under a venue's own allowance: a phone opening the menu
// is a few dozen requests, and one that asks for more than a hundred a
// minute is a script. A venue's bucket (Limit) is shared by all its guests,
// so without this one anonymous client could drain it and lock every real
// guest out of ordering.
var DefaultIPLimit = Limit{PerMinute: 120, Burst: 60}

const (
	// maxStreamsPerIP is how many guest requests one address may have in
	// flight at once. A guest watching an order holds one open; a browser
	// holds six. Sixteen leaves room for a shared office address and no
	// room for one client to take a venue's maxStreams for itself.
	maxStreamsPerIP = 16

	// ipIdle is how long an address is remembered after its last request.
	ipIdle = 10 * time.Minute

	// ipPruneEvery bounds how often the table of addresses is walked.
	ipPruneEvery = time.Minute
)

// errRateLimited is an address that has spent its allowance.
var errRateLimited = errors.New("relay: too many requests from this address")

// ipEntry is one address: its bucket, its open streams and when it last
// asked for anything.
type ipEntry struct {
	bucket *bucket
	open   int
	seen   time.Time
}

// ipTable is every address the relay has heard from lately. Unlike a
// venue's bucket, which lives and dies with its socket, an address has no
// connection to end, so idle entries are pruned as new requests arrive.
type ipTable struct {
	mu      sync.Mutex
	lim     Limit
	entries map[string]*ipEntry
	pruned  time.Time
}

func newIPTable(lim Limit, now time.Time) *ipTable {
	return &ipTable{lim: lim, entries: map[string]*ipEntry{}, pruned: now}
}

// admit lets one request from ip through, counting it open until release is
// called. It refuses with errBusy when the address has maxStreamsPerIP in
// flight, and with errRateLimited, and how long until a token is back, when
// its bucket is empty.
func (t *ipTable) admit(ip string, now time.Time) (release func(), wait time.Duration, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.prune(now)
	e := t.entries[ip]
	if e == nil {
		e = &ipEntry{bucket: newBucket(t.lim, now)}
		t.entries[ip] = e
	}
	e.seen = now
	if e.open >= maxStreamsPerIP {
		return nil, 0, errBusy
	}
	if ok, wait := e.bucket.take(now); !ok {
		return nil, wait, errRateLimited
	}
	e.open++
	return func() {
		t.mu.Lock()
		e.open--
		t.mu.Unlock()
	}, 0, nil
}

// prune forgets addresses with nothing open that have been quiet for ipIdle.
// It runs at most once in ipPruneEvery; the caller holds the lock.
func (t *ipTable) prune(now time.Time) {
	if now.Sub(t.pruned) < ipPruneEvery {
		return
	}
	t.pruned = now
	for ip, e := range t.entries {
		if e.open == 0 && now.Sub(e.seen) >= ipIdle {
			delete(t.entries, ip)
		}
	}
}

// size is how many addresses are remembered. Tests read it.
func (t *ipTable) size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.entries)
}
