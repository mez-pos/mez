package discover

import (
	"net"
	"net/netip"
	"slices"
	"strconv"
)

// Addresser works out where on the LAN this server is, for the screen that
// tells a second device what to open (phase 12, Task 3d, ADR-269). The zero
// value asks this machine; the fields are for tests.
type Addresser struct {
	// Addrs lists this machine's interface addresses. Nil is net.InterfaceAddrs.
	Addrs func() ([]net.Addr, error)
	// Outbound is the address this machine would send from to reach the
	// rest of the world, or false when it has no route. Nil asks the
	// operating system. It is how the card a tablet can reach is told from
	// a virtual one that leads nowhere.
	Outbound func() (netip.Addr, bool)
}

// URLs is each address a device on the venue's network could open to reach
// a server listening on listenAddr, most likely first.
//
// A server bound to one address is at that address; one bound to loopback
// is nowhere a second device can reach, and has none. Bound to every
// address, it is at each private IPv4 address the machine has: the one its
// traffic leaves by first, then the rest in order.
func (a *Addresser) URLs(listenAddr string) ([]string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, err
	}
	if host != "" {
		if ip, err := netip.ParseAddr(host); err == nil && !ip.IsUnspecified() {
			if ip.IsLoopback() {
				return []string{}, nil
			}
			return []string{urlOf(ip, port)}, nil
		}
		// A name, or 0.0.0.0: every address.
	}

	addrs := a.Addrs
	if addrs == nil {
		addrs = net.InterfaceAddrs
	}
	own, err := addrs()
	if err != nil {
		return nil, err
	}
	_, mine := networks(own)
	ips := make([]netip.Addr, 0, len(mine))
	for ip := range mine {
		ips = append(ips, ip)
	}
	slices.SortFunc(ips, func(x, y netip.Addr) int { return x.Compare(y) })

	outbound := a.Outbound
	if outbound == nil {
		outbound = outboundAddr
	}
	if first, ok := outbound(); ok {
		if i := slices.Index(ips, first.Unmap()); i > 0 {
			ips = slices.Insert(slices.Delete(ips, i, i+1), 0, first.Unmap())
		}
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = urlOf(ip, port)
	}
	return out, nil
}

// urlOf is the address as typed into a browser. The LAN is plain HTTP
// (ADR-257), and port 80 is the one that need not be said.
func urlOf(ip netip.Addr, port string) string {
	if n, err := strconv.Atoi(port); err == nil && n == 80 {
		return "http://" + ip.String()
	}
	return "http://" + net.JoinHostPort(ip.String(), port)
}

// outboundAddr asks the operating system which address it would send from.
// Connecting a UDP socket sends nothing: it only makes the system choose a
// route, and the address chosen is read back. 192.0.2.1 is a documentation
// address (RFC 5737) nobody has. A machine with no default route has no
// answer, which is a cafe whose internet is down and is not an error.
func outboundAddr() (netip.Addr, bool) {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return netip.Addr{}, false
	}
	defer conn.Close()
	udp, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}
	ip, ok := netip.AddrFromSlice(udp.IP)
	return ip.Unmap(), ok
}
