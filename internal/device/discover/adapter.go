package discover

import (
	"net"
	"strings"
)

// Iface is a network adapter: the name the operating system shows for it,
// and its addresses.
type Iface struct {
	Name  string
	Addrs []net.Addr
}

// systemIfaces lists this machine's adapters that are up and are not its
// loopback.
func systemIfaces() ([]Iface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]Iface, 0, len(all))
	for _, i := range all {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			// One adapter that cannot be read does not hide the others.
			continue
		}
		out = append(out, Iface{Name: i.Name, Addrs: addrs})
	}
	return out, nil
}

// virtualWords are what the adapters of virtual machines, containers and
// VPNs are called: "vEthernet (WSL)" and "VirtualBox Host-Only Network" on
// Windows, "docker0" and "virbr0" on Linux. A tablet on the venue's wifi is
// on none of them, and neither is a printer.
var virtualWords = []string{
	"vethernet", "wsl", "hyper-v", "virtualbox", "vmware", "vmnet", "docker", "virbr", "veth",
	"tailscale", "zerotier", "wireguard", "tun", "tap", "utun", "vpn", "bluetooth", "loopback", "npcap",
}

// virtualPrefixes are names that are only virtual at the start: "br-1a2b"
// is a Docker bridge, and "Ethernet (bridged)" is not.
var virtualPrefixes = []string{"br-", "lxc", "cni", "flannel", "cali"}

// Virtual reports whether an adapter's name is one a virtual adapter has.
// It is a judgement by name, and so can be wrong: what it is used for is
// the order things are shown in and which networks are searched first,
// never whether an address is shown at all.
func Virtual(name string) bool {
	lower := strings.ToLower(name)
	for _, word := range virtualWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// searchable is the addresses a search for printers starts from: the real
// adapters', or every adapter's when none looks real, so that a machine
// whose only card has an odd name still searches.
func searchable(ifaces []Iface) []net.Addr {
	var real, all []net.Addr
	for _, i := range ifaces {
		all = append(all, i.Addrs...)
		if !Virtual(i.Name) {
			real = append(real, i.Addrs...)
		}
	}
	if len(real) > 0 {
		return real
	}
	return all
}

// systemAddrs is searchable over this machine's adapters.
func systemAddrs() ([]net.Addr, error) {
	ifaces, err := systemIfaces()
	if err != nil {
		return nil, err
	}
	return searchable(ifaces), nil
}
