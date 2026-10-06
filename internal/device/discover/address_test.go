package discover

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
)

func TestURLs(t *testing.T) {
	// A till with a real card, two virtual ones, and addresses no tablet could use.
	cards := func(t *testing.T) func() ([]net.Addr, error) {
		return func() ([]net.Addr, error) {
			return []net.Addr{
				cidr(t, "127.0.0.1/8"), cidr(t, "172.25.224.1/20"), cidr(t, "192.168.29.211/24"),
				cidr(t, "10.0.75.1/24"), cidr(t, "203.0.113.5/24"), cidr(t, "169.254.3.3/16"), cidr(t, "fe80::1/64"),
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
		{"every address: the card the traffic leaves by is first", ":7777", via("192.168.29.211"),
			[]string{"http://192.168.29.211:7777", "http://10.0.75.1:7777", "http://172.25.224.1:7777"}},
		{"said as 0.0.0.0", "0.0.0.0:7777", via("192.168.29.211"),
			[]string{"http://192.168.29.211:7777", "http://10.0.75.1:7777", "http://172.25.224.1:7777"}},
		{"with the internet down they are in order", ":7777", offline,
			[]string{"http://10.0.75.1:7777", "http://172.25.224.1:7777", "http://192.168.29.211:7777"}},
		{"an outbound address that is not a private card changes nothing", ":7777", via("203.0.113.5"),
			[]string{"http://10.0.75.1:7777", "http://172.25.224.1:7777", "http://192.168.29.211:7777"}},
		{"an outbound address as IPv6 writes it", ":7777", via("::ffff:192.168.29.211"),
			[]string{"http://192.168.29.211:7777", "http://10.0.75.1:7777", "http://172.25.224.1:7777"}},
		{"port 80 is not said", ":80", via("192.168.29.211"),
			[]string{"http://192.168.29.211", "http://10.0.75.1", "http://172.25.224.1"}},
		{"bound to one address, it is at that address", "192.168.29.211:7777", via("10.0.75.1"), []string{"http://192.168.29.211:7777"}},
		{"bound to loopback, nowhere a second device can reach", "127.0.0.1:7791", via("192.168.29.211"), []string{}},
		{"bound to the IPv6 loopback", "[::1]:7777", via("192.168.29.211"), []string{}},
		{"bound to one IPv6 address", "[fd00::5]:7777", offline, []string{"http://[fd00::5]:7777"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Addresser{Addrs: cards(t), Outbound: tt.outbound}
			got, err := a.URLs(tt.listen)
			if err != nil {
				t.Fatalf("URLs: %v", err)
			}
			if got == nil || !slices.Equal(got, tt.want) {
				t.Errorf("URLs(%q) = %v, want %v", tt.listen, got, tt.want)
			}
		})
	}
}

func TestURLsRefusals(t *testing.T) {
	a := &Addresser{Addrs: func() ([]net.Addr, error) { return nil, errors.New("no netlink") }}
	if _, err := a.URLs(":7777"); err == nil {
		t.Error("URLs with addresses that cannot be read: no error")
	}
	if _, err := a.URLs("7777"); err == nil {
		t.Error("URLs with a listen address that is not one: no error")
	}
}

// TestURLsOnThisMachine runs the real thing: whatever it finds, every address
// is a private one on the port asked for, and asking the system for its
// outbound address does not fail the answer.
func TestURLsOnThisMachine(t *testing.T) {
	urls, err := (&Addresser{}).URLs(":7777")
	if err != nil {
		t.Fatalf("URLs: %v", err)
	}
	for _, u := range urls {
		host, port, err := net.SplitHostPort(u[len("http://"):])
		if err != nil || port != "7777" || !netip.MustParseAddr(host).IsPrivate() {
			t.Errorf("%q is not a private address on port 7777", u)
		}
	}
	t.Logf("this machine: %v", urls)
}
