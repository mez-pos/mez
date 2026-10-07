package discover

import (
	"net"
	"net/netip"
	"slices"
	"strconv"
)

// Address is one place on the venue's network this server can be opened
// at, and the adapter it is on.
type Address struct {
	// URL is the address as a browser takes it, with no path.
	URL string
	// Adapter is the adapter's name as the operating system shows it:
	// "Wi-Fi", "Ethernet", "vEthernet (WSL)". Empty when it is not known.
	Adapter string
	// Virtual is true for an adapter that looks like a virtual machine's
	// or a VPN's: a tablet is unlikely to be on it.
	Virtual bool
}

// Addresser works out where on the LAN this server is, for the screen that
// tells a second device what to open (phase 12, Tasks 3d and 3f, ADR-269,
// ADR-271). The zero value asks this machine; the fields are for tests.
type Addresser struct {
	// Ifaces lists this machine's adapters. Nil asks the operating system.
	Ifaces func() ([]Iface, error)
	// Outbound is the address this machine would send from to reach the
	// rest of the world, or false when it has no route. Nil asks the
	// operating system. It only orders the real adapters among themselves.
	Outbound func() (netip.Addr, bool)
}

// Addresses is each address a device on the venue's network could open to
// reach a server listening on listenAddr.
//
// A server bound to one address is at that address; one bound to loopback
// is nowhere a second device can reach, and has none. Bound to every
// address, it is at each private IPv4 address the machine has: the real
// adapters' first, the one its traffic leaves by first among those, and
// the virtual adapters' last.
func (a *Addresser) Addresses(listenAddr string) ([]Address, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, err
	}
	ifaces := a.Ifaces
	if ifaces == nil {
		ifaces = systemIfaces
	}

	if host != "" {
		if ip, err := netip.ParseAddr(host); err == nil && !ip.IsUnspecified() {
			if ip.IsLoopback() {
				return []Address{}, nil
			}
			one := Address{URL: urlOf(ip, port)}
			// The adapter's name is a nicety: without it the address stands.
			if all, err := ifaces(); err == nil {
				one.Adapter, one.Virtual = adapterOf(all, ip.Unmap())
			}
			return []Address{one}, nil
		}
		// A name, or 0.0.0.0: every address.
	}

	all, err := ifaces()
	if err != nil {
		return nil, err
	}
	type found struct {
		ip netip.Addr
		Address
	}
	var out []found
	for _, i := range all {
		virtual := Virtual(i.Name)
		for _, ip := range privateV4(i.Addrs) {
			if slices.ContainsFunc(out, func(f found) bool { return f.ip == ip }) {
				continue
			}
			out = append(out, found{ip, Address{URL: urlOf(ip, port), Adapter: i.Name, Virtual: virtual}})
		}
	}

	outbound := a.Outbound
	if outbound == nil {
		outbound = outboundAddr
	}
	first, routed := outbound()
	first = first.Unmap()
	slices.SortStableFunc(out, func(x, y found) int {
		switch {
		case x.Virtual != y.Virtual:
			if x.Virtual {
				return 1
			}
			return -1
		case routed && x.ip == first:
			return -1
		case routed && y.ip == first:
			return 1
		default:
			return x.ip.Compare(y.ip)
		}
	})
	addresses := make([]Address, len(out))
	for i, f := range out {
		addresses[i] = f.Address
	}
	return addresses, nil
}

// privateV4 is the private IPv4 addresses among an adapter's: the ones a
// device on a venue's LAN could reach it at.
func privateV4(addrs []net.Addr) []netip.Addr {
	var out []netip.Addr
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		if ip = ip.Unmap(); ip.Is4() && ip.IsPrivate() {
			out = append(out, ip)
		}
	}
	return out
}

// adapterOf names the adapter an address is on.
func adapterOf(ifaces []Iface, ip netip.Addr) (name string, virtual bool) {
	for _, i := range ifaces {
		for _, a := range i.Addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if have, ok := netip.AddrFromSlice(ipnet.IP); ok && have.Unmap() == ip {
				return i.Name, Virtual(i.Name)
			}
		}
	}
	return "", false
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
