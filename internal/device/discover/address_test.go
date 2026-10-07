package discover

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
)

// short writes addresses as "url adapter" with a "*" for a virtual one.
func short(addresses []Address) []string {
	out := make([]string, len(addresses))
	for i, a := range addresses {
		out[i] = a.URL[len("http://"):] + " " + a.Adapter
		if a.Virtual {
			out[i] += "*"
		}
	}
	return out
}

func TestAddresses(t *testing.T) {
	// A till on wifi with a cable plugged in too, two virtual adapters, and
	// addresses no tablet could use.
	cards := func(t *testing.T) func() ([]Iface, error) {
		return func() ([]Iface, error) {
			return []Iface{
				{"vEthernet (WSL)", []net.Addr{cidr(t, "172.25.224.1/20"), cidr(t, "fe80::1/64")}},
				{"Wi-Fi", []net.Addr{cidr(t, "192.168.29.211/24"), cidr(t, "fe80::2/64")}},
				{"VirtualBox Host-Only Network", []net.Addr{cidr(t, "10.0.75.1/24")}},
				{"Ethernet", []net.Addr{cidr(t, "192.168.1.40/24"), cidr(t, "169.254.3.3/16")}},
				{"Mobile", []net.Addr{cidr(t, "203.0.113.5/24")}},
				{"Odd", []net.Addr{&net.IPAddr{IP: net.ParseIP("192.168.9.9")}}},
			}, nil
		}
	}
	via := func(ip string) func() (netip.Addr, bool) {
		return func() (netip.Addr, bool) { return netip.MustParseAddr(ip), true }
	}
	offline := func() (netip.Addr, bool) { return netip.Addr{}, false }

	tests := []struct {
		name     string
		listen   string
		outbound func() (netip.Addr, bool)
		want     []string
	}{
		{"every address: real adapters first, the one the traffic leaves by first of those, virtual last", ":7777", via("192.168.29.211"),
			[]string{"192.168.29.211:7777 Wi-Fi", "192.168.1.40:7777 Ethernet", "10.0.75.1:7777 VirtualBox Host-Only Network*", "172.25.224.1:7777 vEthernet (WSL)*"}},
		{"said as 0.0.0.0, leaving by the cable", "0.0.0.0:7777", via("192.168.1.40"),
			[]string{"192.168.1.40:7777 Ethernet", "192.168.29.211:7777 Wi-Fi", "10.0.75.1:7777 VirtualBox Host-Only Network*", "172.25.224.1:7777 vEthernet (WSL)*"}},
		{"with the internet down the real ones are in order", ":7777", offline,
			[]string{"192.168.1.40:7777 Ethernet", "192.168.29.211:7777 Wi-Fi", "10.0.75.1:7777 VirtualBox Host-Only Network*", "172.25.224.1:7777 vEthernet (WSL)*"}},
		{"traffic leaving by a virtual adapter does not put it first", ":7777", via("172.25.224.1"),
			[]string{"192.168.1.40:7777 Ethernet", "192.168.29.211:7777 Wi-Fi", "172.25.224.1:7777 vEthernet (WSL)*", "10.0.75.1:7777 VirtualBox Host-Only Network*"}},
		{"an outbound address as IPv6 writes it", ":7777", via("::ffff:192.168.29.211"),
			[]string{"192.168.29.211:7777 Wi-Fi", "192.168.1.40:7777 Ethernet", "10.0.75.1:7777 VirtualBox Host-Only Network*", "172.25.224.1:7777 vEthernet (WSL)*"}},
		{"port 80 is not said", ":80", via("192.168.29.211"),
			[]string{"192.168.29.211 Wi-Fi", "192.168.1.40 Ethernet", "10.0.75.1 VirtualBox Host-Only Network*", "172.25.224.1 vEthernet (WSL)*"}},
		{"bound to one address, it is at that address, by its adapter's name", "192.168.29.211:7777", via("10.0.75.1"), []string{"192.168.29.211:7777 Wi-Fi"}},
		{"bound to a virtual adapter's address", "10.0.75.1:7777", offline, []string{"10.0.75.1:7777 VirtualBox Host-Only Network*"}},
		{"bound to an address no adapter has", "192.168.50.5:7777", offline, []string{"192.168.50.5:7777 "}},
		{"bound to loopback, nowhere a second device can reach", "127.0.0.1:7791", via("192.168.29.211"), []string{}},
		{"bound to the IPv6 loopback", "[::1]:7777", via("192.168.29.211"), []string{}},
		{"bound to one IPv6 address", "[fd00::5]:7777", offline, []string{"[fd00::5]:7777 "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Addresser{Ifaces: cards(t), Outbound: tt.outbound}
			got, err := a.Addresses(tt.listen)
			if err != nil {
				t.Fatalf("Addresses: %v", err)
			}
			if got == nil || !slices.Equal(short(got), tt.want) {
				t.Errorf("Addresses(%q) = %q, want %q", tt.listen, short(got), tt.want)
			}
		})
	}
}

func TestAddressesListsAnAddressOnce(t *testing.T) {
	a := &Addresser{
		Ifaces: func() ([]Iface, error) {
			return []Iface{
				{"Ethernet", []net.Addr{cidr(t, "192.168.1.40/24")}},
				{"Network Bridge", []net.Addr{cidr(t, "192.168.1.40/24")}},
			}, nil
		},
		Outbound: func() (netip.Addr, bool) { return netip.Addr{}, false },
	}
	got, err := a.Addresses(":7777")
	if err != nil || len(got) != 1 || got[0].Adapter != "Ethernet" {
		t.Errorf("Addresses = %q, %v, want the one address once", short(got), err)
	}
}

func TestAddressesRefusals(t *testing.T) {
	broken := &Addresser{Ifaces: func() ([]Iface, error) { return nil, errors.New("no netlink") }}
	if _, err := broken.Addresses(":7777"); err == nil {
		t.Error("Addresses with adapters that cannot be read: no error")
	}
	if _, err := broken.Addresses("7777"); err == nil {
		t.Error("Addresses with a listen address that is not one: no error")
	}
	// Bound to one address, the adapters are only for its name.
	got, err := broken.Addresses("192.168.1.40:7777")
	if err != nil || len(got) != 1 || got[0].URL != "http://192.168.1.40:7777" || got[0].Adapter != "" {
		t.Errorf("bound to one address with no adapters to read: %q, %v", short(got), err)
	}
}

func TestVirtual(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Wi-Fi", false}, {"Ethernet", false}, {"Ethernet 2", false}, {"Local Area Connection", false},
		{"eth0", false}, {"enp3s0", false}, {"wlan0", false}, {"wlp2s0", false}, {"en0", false}, {"Ethernet (bridged)", false}, {"", false},
		{"vEthernet (WSL)", true}, {"vEthernet (Default Switch)", true}, {"VirtualBox Host-Only Network", true},
		{"VMware Network Adapter VMnet8", true}, {"docker0", true}, {"br-1a2b3c4d5e6f", true}, {"veth9f1c2d3", true},
		{"virbr0", true}, {"Tailscale", true}, {"ZeroTier One [abc]", true}, {"utun3", true}, {"tun0", true},
		{"Bluetooth Network Connection", true}, {"Loopback Pseudo-Interface 1", true}, {"cni0", true},
	}
	for _, tt := range tests {
		if got := Virtual(tt.name); got != tt.want {
			t.Errorf("Virtual(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSearchable(t *testing.T) {
	wifi := cidr(t, "192.168.29.211/24")
	wsl := cidr(t, "172.25.224.1/20")
	vbox := cidr(t, "10.0.75.1/24")
	tests := []struct {
		name   string
		ifaces []Iface
		want   []net.Addr
	}{
		{"real adapters only, when there is one", []Iface{{"vEthernet (WSL)", []net.Addr{wsl}}, {"Wi-Fi", []net.Addr{wifi}}, {"VirtualBox", []net.Addr{vbox}}}, []net.Addr{wifi}},
		{"every adapter, when none looks real", []Iface{{"vEthernet (WSL)", []net.Addr{wsl}}, {"VirtualBox", []net.Addr{vbox}}}, []net.Addr{wsl, vbox}},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		if got := searchable(tt.ifaces); !slices.Equal(got, tt.want) {
			t.Errorf("%s: searchable = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestAddressesOnThisMachine runs the real thing: whatever it finds, every
// address is a private one on the port asked for, no virtual adapter is
// listed before a real one, and asking the system does not fail.
func TestAddressesOnThisMachine(t *testing.T) {
	addresses, err := (&Addresser{}).Addresses(":7777")
	if err != nil {
		t.Fatalf("Addresses: %v", err)
	}
	seenVirtual := false
	for _, a := range addresses {
		host, port, err := net.SplitHostPort(a.URL[len("http://"):])
		if err != nil || port != "7777" || !netip.MustParseAddr(host).IsPrivate() {
			t.Errorf("%q is not a private address on port 7777", a.URL)
		}
		if seenVirtual && !a.Virtual {
			t.Errorf("%q (%s) is listed after a virtual adapter", a.URL, a.Adapter)
		}
		seenVirtual = seenVirtual || a.Virtual
	}
	if _, err := systemAddrs(); err != nil {
		t.Errorf("systemAddrs: %v", err)
	}
	t.Logf("this machine: %q", short(addresses))
}
