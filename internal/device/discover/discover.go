// Package discover looks for receipt printers the server could print on
// (phase 12, Task 3c, ADR-268): anything on its own networks that answers
// on the raw-print port, and the printers Windows has installed on this
// computer. It finds candidates, not printers: a port that opens proves
// nothing about what is behind it, which is what the test page is for.
package discover

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/device/escpos"
)

// The search is sized for a cafe's network: one /24 a card, a few hundred
// addresses, each given long enough to answer on a LAN and no longer.
const (
	// dialTimeout is how long one address has to accept. A printer on the
	// same switch answers in a millisecond; an address nobody has takes
	// the whole of it.
	dialTimeout = 350 * time.Millisecond
	// workers is how many addresses are tried at once.
	workers = 64
	// maxNetworks bounds the networks searched, for a machine with many
	// virtual adapters.
	maxNetworks = 4
	// searchBits is the largest network searched, a /24. A card on a /16
	// is searched around its own address only.
	searchBits = 24
)

// Result is what a search found.
type Result struct {
	// Network is each address that accepted a connection on the print
	// port, in order.
	Network []string
	// Local is each printer installed on this computer that could be a
	// receipt printer, by the name Windows gives it.
	Local []string
	// Searched is the networks that were searched, as CIDRs.
	Searched []string
	// LocalErr is why the installed printers could not be listed, or nil.
	// The network's results stand without them.
	LocalErr error
}

// Finder searches. The zero value searches this machine's own networks and
// its own installed printers; the fields are for tests.
type Finder struct {
	// Addrs lists the addresses of the adapters to search from. Nil is this
	// machine's real adapters, or all of them when none looks real: a
	// printer is not on a virtual machine's network (ADR-271).
	Addrs func() ([]net.Addr, error)
	// Dial opens a connection. Nil is a net.Dialer.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// Local lists the installed printers. Nil is the operating system's.
	Local func() ([]string, error)

	// One search at a time: two would halve each other's chances inside
	// the same timeout, and a screen asking twice wants one answer.
	mu sync.Mutex
}

// Find searches, returning when every address has answered or run out of time.
func (f *Finder) Find(ctx context.Context) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	addrs := f.Addrs
	if addrs == nil {
		addrs = systemAddrs
	}
	own, err := addrs()
	if err != nil {
		return Result{}, fmt.Errorf("discover: listing this machine's addresses: %w", err)
	}
	nets, mine := networks(own)

	var res Result
	var targets []netip.Addr
	for _, n := range nets {
		res.Searched = append(res.Searched, n.String())
		targets = append(targets, hosts(n, mine)...)
	}
	res.Network = f.probe(ctx, targets)

	local := f.Local
	if local == nil {
		local = installedPrinters
	}
	names, err := local()
	if err != nil {
		res.LocalErr = fmt.Errorf("discover: listing the installed printers: %w", err)
	}
	for _, name := range names {
		if ReceiptCandidate(name) {
			res.Local = append(res.Local, name)
		}
	}
	slices.Sort(res.Local)
	res.Local = slices.Compact(res.Local)
	return res, ctx.Err()
}

// networks picks the networks to search from this machine's addresses, and
// returns the machine's own addresses on them. Only private IPv4 networks
// are searched: a printer is on the venue's LAN (ADR-259), a public or
// carrier network is somebody else's, and IPv6 has no range small enough
// to walk.
func networks(addrs []net.Addr) ([]netip.Prefix, map[netip.Addr]bool) {
	var out []netip.Prefix
	mine := map[netip.Addr]bool{}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if !ip.Is4() || !ip.IsPrivate() {
			continue
		}
		mine[ip] = true
		ones, _ := ipnet.Mask.Size()
		if ones < searchBits {
			ones = searchBits
		}
		prefix := netip.PrefixFrom(ip, ones).Masked()
		if !slices.Contains(out, prefix) && len(out) < maxNetworks {
			out = append(out, prefix)
		}
	}
	return out, mine
}

// hosts is every address of a network a printer could have: not the
// network's own, not its broadcast, and not this machine's.
func hosts(n netip.Prefix, mine map[netip.Addr]bool) []netip.Addr {
	size := uint32(1) << (32 - n.Bits())
	if size < 4 {
		return nil // a /31 or /32 has no neighbours to ask
	}
	base := binary.BigEndian.Uint32(n.Addr().AsSlice())
	out := make([]netip.Addr, 0, size-2)
	for i := uint32(1); i < size-1; i++ {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], base+i)
		if ip := netip.AddrFrom4(b); !mine[ip] {
			out = append(out, ip)
		}
	}
	return out
}

// probe tries the print port of every target and returns the ones that
// accepted, in address order.
func (f *Finder) probe(ctx context.Context, targets []netip.Addr) []string {
	dial := f.Dial
	if dial == nil {
		var d net.Dialer
		dial = d.DialContext
	}
	queue := make(chan netip.Addr)
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		found []netip.Addr
	)
	for range min(workers, len(targets)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range queue {
				one, cancel := context.WithTimeout(ctx, dialTimeout)
				conn, err := dial(one, "tcp", net.JoinHostPort(ip.String(), escpos.DefaultPort))
				cancel()
				if err != nil {
					continue
				}
				// Nothing is sent: a byte to something that is not a
				// printer is a byte it may act on.
				_ = conn.Close()
				mu.Lock()
				found = append(found, ip)
				mu.Unlock()
			}
		}()
	}
feed:
	for _, ip := range targets {
		select {
		case queue <- ip:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()

	slices.SortFunc(found, func(a, b netip.Addr) int { return a.Compare(b) })
	out := make([]string, len(found))
	for i, ip := range found {
		out[i] = ip.String()
	}
	return out
}

// notPrinters are words in the names of things Windows lists as printers
// that put nothing on paper. A test page sent to one opens a save dialog
// nobody can see, on a server that runs as a service.
var notPrinters = []string{"pdf", "xps", "onenote", "fax", "send to", "document writer", "anydesk", "rustdesk", "teamviewer", "remote desktop"}

// ReceiptCandidate reports whether an installed printer's name could be a
// receipt printer's: not one of Windows' own virtual printers.
func ReceiptCandidate(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	for _, word := range notPrinters {
		if strings.Contains(lower, word) {
			return false
		}
	}
	return true
}
