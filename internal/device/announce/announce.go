// Package announce gives the server a name on the venue's network (ADR-379):
// mez.local, answered over multicast DNS, so that a phone, tablet or laptop
// that resolves .local names (iPhone, iPad, Mac, Windows, newer Android)
// finds the till by name whatever address the router gave the PC today. A
// device that cannot resolve the name is no worse off than before: the
// addresses and the QR codes on the pairing screen stand.
package announce

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Name is the host the server answers for, without the .local.
const Name = "mez"

// Hostname is the name a device types: Name under .local.
const Hostname = Name + ".local"

// URL is where a browser opens the server by name: port 80 is left off.
func URL(port int) string {
	if port == 80 {
		return "http://" + Hostname
	}
	return "http://" + net.JoinHostPort(Hostname, strconv.Itoa(port))
}

// Announcer answers for Hostname on the local network until closed.
type Announcer struct {
	log *slog.Logger

	mu    sync.Mutex
	conn  *mdns.Conn
	conns []net.PacketConn
}

// New returns an Announcer that has not started.
func New(log *slog.Logger) *Announcer {
	if log == nil {
		log = slog.Default()
	}
	return &Announcer{log: log}
}

// Start begins answering. The multicast sockets are bound with the address
// shared, since Windows's own resolver and Bonjour hold port 5353 too; a
// machine that still refuses is reported, and the server serves without a
// name.
func (a *Announcer) Start(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		return errors.New("announce: already started")
	}
	var (
		v4 *ipv4.PacketConn
		v6 *ipv6.PacketConn
	)
	if c, err := listenShared("udp4", mdns.DefaultAddressIPv4); err == nil {
		v4 = ipv4.NewPacketConn(c)
		a.conns = append(a.conns, c)
	} else {
		a.log.Info("announce: no IPv4 multicast socket", "error", err)
	}
	if c, err := listenShared("udp6", mdns.DefaultAddressIPv6); err == nil {
		v6 = ipv6.NewPacketConn(c)
		a.conns = append(a.conns, c)
	} else {
		a.log.Debug("announce: no IPv6 multicast socket", "error", err)
	}
	if v4 == nil && v6 == nil {
		return errors.New("announce: no multicast socket could be opened")
	}
	conn, err := mdns.NewServer(v4, v6, mdns.WithName("mez"), mdns.WithLocalNames(Hostname))
	if err != nil {
		a.closeConns()
		return fmt.Errorf("announce: starting the mDNS responder: %w", err)
	}
	a.conn = conn
	a.log.Info("answering for a name on the network", "hostname", Hostname)
	return nil
}

// Close stops answering.
func (a *Announcer) Close(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == nil {
		return nil
	}
	err := a.conn.Close()
	a.conn = nil
	a.closeConns()
	if err != nil {
		return fmt.Errorf("announce: closing: %w", err)
	}
	return nil
}

func (a *Announcer) closeConns() {
	for _, c := range a.conns {
		_ = c.Close()
	}
	a.conns = nil
}

// listenShared binds a multicast UDP socket with the address shared, so that
// another responder on the machine does not keep this one out.
func listenShared(network, address string) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: shareAddress}
	return lc.ListenPacket(context.Background(), network, address)
}
