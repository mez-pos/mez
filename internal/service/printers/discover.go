package printers

import (
	"context"
	"strings"

	"github.com/byjoellobo/mez/internal/device/escpos"
)

// Candidate is something a search found that the venue could add as a
// printer: the address to give it, and whether the venue has it already.
type Candidate struct {
	Address string `json:"address"`
	Known   bool   `json:"known"`
}

// Found is what a search for printers found (phase 12, Task 3c, ADR-268).
type Found struct {
	// Network is each address on the server's own networks that answers on
	// the print port. It may not be a printer: the test page says.
	Network []Candidate
	// USB is each printer Windows has installed on the server's computer.
	USB []Candidate
	// Searched is the networks that were searched, as CIDRs.
	Searched []string
	// LocalErr is why the installed printers could not be listed, or nil.
	LocalErr error
}

// Discover searches for printers and marks the ones the venue already has.
// Every candidate it returns is one Create would accept as it stands: an
// installed printer whose name the settings would refuse is left out, not
// offered and then refused.
func (s *Service) Discover(ctx context.Context, venueID int64) (Found, error) {
	if s.finder == nil {
		return Found{}, ErrNoFinder
	}
	have, err := s.db.Queries().PrintersForVenue(ctx, venueID)
	if err != nil {
		return Found{}, err
	}
	known := map[string]bool{}
	for _, p := range have {
		known[knownKey(Transport(p.Transport), p.Address)] = true
	}

	res, err := s.finder.Find(ctx)
	if err != nil {
		return Found{}, err
	}
	out := Found{Network: []Candidate{}, USB: []Candidate{}, Searched: res.Searched, LocalErr: res.LocalErr}
	if out.Searched == nil {
		out.Searched = []string{}
	}
	for _, addr := range res.Network {
		if validNetworkAddress(addr) {
			out.Network = append(out.Network, Candidate{Address: addr, Known: known[knownKey(Network, addr)]})
		}
	}
	for _, name := range res.Local {
		if windowsNameProblem(name) == "" {
			out.USB = append(out.USB, Candidate{Address: name, Known: known[knownKey(USB, name)]})
		}
	}
	return out, nil
}

// knownKey is a printer's transport and address as two of them are compared:
// a network address with or without the default port is the same printer,
// and Windows does not tell printer names apart by case.
func knownKey(t Transport, address string) string {
	address = strings.TrimSpace(address)
	if t == Network {
		address = strings.TrimSuffix(address, ":"+escpos.DefaultPort)
	} else {
		address = strings.ToLower(address)
	}
	return string(t) + "\x00" + address
}
