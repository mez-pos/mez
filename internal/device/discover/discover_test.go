package discover

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// cidr is an interface address as net.InterfaceAddrs gives one.
func cidr(t *testing.T, s string) net.Addr {
	t.Helper()
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return &net.IPNet{IP: ip, Mask: ipnet.Mask}
}

// lan is a network where only some addresses have a printer.
type lan struct {
	mu      sync.Mutex
	open    map[string]bool
	dialled []string
}

func (l *lan) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	l.mu.Lock()
	l.dialled = append(l.dialled, addr)
	open := l.open[addr]
	l.mu.Unlock()
	if network != "tcp" {
		return nil, errors.New("not tcp")
	}
	if !open {
		return nil, errors.New("connection refused")
	}
	a, b := net.Pipe()
	_ = b.Close()
	return a, nil
}

func TestFindSearchesItsOwnNetworks(t *testing.T) {
	l := &lan{open: map[string]bool{"192.168.1.50:9100": true, "192.168.1.7:9100": true, "10.4.4.200:9100": true}}
	f := &Finder{
		Addrs: func() ([]net.Addr, error) {
			return []net.Addr{
				cidr(t, "127.0.0.1/8"), cidr(t, "192.168.1.10/24"), cidr(t, "10.4.4.9/16"),
				cidr(t, "203.0.113.5/24"), cidr(t, "169.254.3.3/16"), cidr(t, "fe80::1/64"), cidr(t, "192.168.1.11/24"),
				&net.IPAddr{IP: net.ParseIP("192.168.9.9")},
			}, nil
		},
		Dial:  l.dial,
		Local: func() ([]string, error) { return nil, nil },
	}
	res, err := f.Find(context.Background())
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	// Private IPv4 only, and a /16 is searched as the /24 around the card.
	if want := []string{"192.168.1.0/24", "10.4.4.0/24"}; !slices.Equal(res.Searched, want) {
		t.Errorf("Searched = %v, want %v", res.Searched, want)
	}
	if want := []string{"10.4.4.200", "192.168.1.7", "192.168.1.50"}; !slices.Equal(res.Network, want) {
		t.Errorf("Network = %v, want %v", res.Network, want)
	}
	// 254 hosts a network, less this machine's own two on the first and one on the second.
	if got, want := len(l.dialled), 252+253; got != want {
		t.Errorf("%d addresses tried, want %d", got, want)
	}
	for _, addr := range l.dialled {
		host, port, _ := net.SplitHostPort(addr)
		ip := netip.MustParseAddr(host)
		if port != "9100" || !ip.IsPrivate() {
			t.Fatalf("tried %s: not the print port of a private address", addr)
		}
		if last := ip.As4()[3]; last == 0 || last == 255 {
			t.Fatalf("tried %s: a network's own address or its broadcast", addr)
		}
		if slices.Contains([]string{"192.168.1.10", "192.168.1.11", "10.4.4.9"}, host) {
			t.Fatalf("tried %s: this machine's own address", addr)
		}
	}
}

func TestFindWithNoNetworkToSearch(t *testing.T) {
	l := &lan{}
	f := &Finder{
		Addrs: func() ([]net.Addr, error) {
			return []net.Addr{cidr(t, "127.0.0.1/8"), cidr(t, "203.0.113.5/24"), cidr(t, "192.168.1.1/32")}, nil
		},
		Dial:  l.dial,
		Local: func() ([]string, error) { return []string{"EPSON TM-T82"}, nil },
	}
	res, err := f.Find(context.Background())
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(res.Network) != 0 || len(l.dialled) != 0 {
		t.Errorf("found %v after trying %d addresses, want nothing tried", res.Network, len(l.dialled))
	}
	if !slices.Equal(res.Local, []string{"EPSON TM-T82"}) {
		t.Errorf("Local = %v", res.Local)
	}
}

func TestFindBoundsTheNetworksItSearches(t *testing.T) {
	l := &lan{}
	f := &Finder{
		Addrs: func() ([]net.Addr, error) {
			var out []net.Addr
			for _, c := range []string{"10.0.1.2/24", "10.0.2.2/24", "10.0.3.2/24", "10.0.4.2/24", "10.0.5.2/24", "10.0.6.2/24"} {
				out = append(out, cidr(t, c))
			}
			return out, nil
		},
		Dial:  l.dial,
		Local: func() ([]string, error) { return nil, nil },
	}
	res, err := f.Find(context.Background())
	if err != nil || len(res.Searched) != maxNetworks {
		t.Errorf("searched %v (%v), want %d networks", res.Searched, err, maxNetworks)
	}
}

func TestFindLocalPrinters(t *testing.T) {
	boom := errors.New("the spooler is not running")
	tests := []struct {
		name    string
		local   func() ([]string, error)
		want    []string
		wantErr bool
	}{
		{"the virtual ones are left out, the rest sorted once each",
			func() ([]string, error) {
				return []string{"POS-80C", "Microsoft Print to PDF", "EPSON TM-T82", "Fax", "Microsoft XPS Document Writer", "OneNote (Desktop)", "POS-80C", "  "}, nil
			},
			[]string{"EPSON TM-T82", "POS-80C"}, false},
		{"none installed", func() ([]string, error) { return nil, nil }, nil, false},
		{"a spooler that cannot be asked is said, and does not fail the search", func() ([]string, error) { return nil, boom }, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Finder{Addrs: func() ([]net.Addr, error) { return nil, nil }, Local: tt.local}
			res, err := f.Find(context.Background())
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if !slices.Equal(res.Local, tt.want) {
				t.Errorf("Local = %v, want %v", res.Local, tt.want)
			}
			if (res.LocalErr != nil) != tt.wantErr || (tt.wantErr && !errors.Is(res.LocalErr, boom)) {
				t.Errorf("LocalErr = %v", res.LocalErr)
			}
		})
	}
}

func TestFindFailsWhenTheMachinesAddressesCannotBeRead(t *testing.T) {
	f := &Finder{Addrs: func() ([]net.Addr, error) { return nil, errors.New("no netlink") }}
	if _, err := f.Find(context.Background()); err == nil || !strings.Contains(err.Error(), "addresses") {
		t.Errorf("Find = %v, want a failure naming the addresses", err)
	}
}

// TestFindStopsWhenAskedTo: a search abandoned by its caller does not go on
// dialling a network for nobody.
func TestFindStopsWhenAskedTo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	l := &lan{}
	f := &Finder{
		Addrs: func() ([]net.Addr, error) { return []net.Addr{cidr(t, "192.168.1.10/24")}, nil },
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			once.Do(cancel)
			return l.dial(ctx, network, addr)
		},
		Local: func() ([]string, error) { return nil, nil },
	}
	if _, err := f.Find(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Find = %v, want context.Canceled", err)
	}
	if n := len(l.dialled); n >= 253 {
		t.Errorf("%d addresses tried after the search was abandoned", n)
	}
}

// TestFindGivesEachAddressItsOwnShortWait: the dial is handed a deadline, so
// one address nobody has cannot hold the search.
func TestFindGivesEachAddressItsOwnShortWait(t *testing.T) {
	var mu sync.Mutex
	var longest time.Duration
	f := &Finder{
		Addrs: func() ([]net.Addr, error) { return []net.Addr{cidr(t, "192.168.1.10/29")}, nil },
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("an address was tried with no deadline")
			}
			mu.Lock()
			longest = max(longest, time.Until(deadline))
			mu.Unlock()
			return nil, errors.New("timeout")
		},
		Local: func() ([]string, error) { return nil, nil },
	}
	if _, err := f.Find(context.Background()); err != nil {
		t.Fatal(err)
	}
	if longest <= 0 || longest > dialTimeout {
		t.Errorf("the longest wait was %v, want at most %v", longest, dialTimeout)
	}
}

func TestReceiptCandidate(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"EPSON TM-T82", true}, {"POS-80C", true}, {"XP-80", true}, {"Star TSP100 Cutter", true},
		{"Microsoft Print to PDF", false}, {"Microsoft XPS Document Writer", false}, {"Fax", false},
		{"OneNote for Windows 10", false}, {"Send To OneNote 2016", false}, {"AnyDesk Printer", false}, {"RustDesk Printer", false}, {"", false}, {"   ", false},
	}
	for _, tt := range tests {
		if got := ReceiptCandidate(tt.name); got != tt.want {
			t.Errorf("ReceiptCandidate(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestInstalledPrintersCanBeAsked runs the real listing: on Windows it calls
// the spooler, elsewhere it has nothing to ask. Either way it must not fail
// or return a name that is empty.
func TestInstalledPrintersCanBeAsked(t *testing.T) {
	names, err := installedPrinters()
	if err != nil {
		t.Skipf("the spooler could not be asked on this machine: %v", err)
	}
	for _, name := range names {
		if strings.TrimSpace(name) == "" || strings.ContainsRune(name, 0) {
			t.Errorf("an installed printer is named %q", name)
		}
	}
	t.Logf("%d installed printers: %q", len(names), names)
}
