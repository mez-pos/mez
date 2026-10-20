package printers

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mez-pos/mez/internal/device/discover"
)

// finder answers a search with what it was given.
type finder struct {
	res   discover.Result
	err   error
	calls int
}

func (f *finder) Find(context.Context) (discover.Result, error) {
	f.calls++
	return f.res, f.err
}

func TestDiscoverMarksWhatTheVenueHas(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newService(t)
	spooler := errors.New("the spooler is not running")
	f := &finder{res: discover.Result{
		Network:  []string{"192.168.1.50", "192.168.1.60", "10.0.0.9", "8.8.8.8"},
		Local:    []string{"EPSON TM-T82", "POS-80C", `\\OFFICE\Shared`, "COM1,9600"},
		Searched: []string{"192.168.1.0/24"},
		LocalErr: spooler,
	}}
	s.finder = f

	for _, in := range []Settings{
		{Name: "Counter", Transport: Network, Address: "192.168.1.50:9100", PaperMM: 80},
		{Name: "Bar", Transport: USB, Address: "epson tm-t82", PaperMM: 80},
	} {
		if _, err := s.Create(ctx, 1, in); err != nil {
			t.Fatalf("Create %s: %v", in.Name, err)
		}
	}

	got, err := s.Discover(ctx, 1)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// 10.0.0.9 is venue 2's printer, not this venue's. 8.8.8.8 is not on a LAN.
	wantNet := []Candidate{{"192.168.1.50", true}, {"192.168.1.60", false}, {"10.0.0.9", false}}
	if !slices.Equal(got.Network, wantNet) {
		t.Errorf("Network = %+v, want %+v", got.Network, wantNet)
	}
	// A shared printer and a port are names the settings refuse, so they are not offered.
	wantUSB := []Candidate{{"EPSON TM-T82", true}, {"POS-80C", false}}
	if !slices.Equal(got.USB, wantUSB) {
		t.Errorf("USB = %+v, want %+v", got.USB, wantUSB)
	}
	if !slices.Equal(got.Searched, []string{"192.168.1.0/24"}) || !errors.Is(got.LocalErr, spooler) {
		t.Errorf("Searched = %v, LocalErr = %v", got.Searched, got.LocalErr)
	}

	// Everything offered is something Create takes as it stands.
	for _, c := range got.Network {
		if _, err := (Settings{Name: "x", Transport: Network, Address: c.Address, PaperMM: 80}).clean(); err != nil {
			t.Errorf("offered %q, which Create refuses: %v", c.Address, err)
		}
	}
	for _, c := range got.USB {
		if _, err := (Settings{Name: "x", Transport: USB, Address: c.Address, PaperMM: 80}).clean(); err != nil {
			t.Errorf("offered %q, which Create refuses: %v", c.Address, err)
		}
	}
}

func TestDiscoverWithNothingFound(t *testing.T) {
	s, _, _ := newService(t)
	s.finder = &finder{}
	got, err := s.Discover(context.Background(), 1)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// Empty lists, not null: the screen maps over them.
	if got.Network == nil || got.USB == nil || got.Searched == nil || len(got.Network)+len(got.USB)+len(got.Searched) != 0 {
		t.Errorf("Found = %+v, want three empty lists", got)
	}
}

func TestDiscoverRefusals(t *testing.T) {
	s, _, _ := newService(t)
	if _, err := s.Discover(context.Background(), 1); !errors.Is(err, ErrNoFinder) {
		t.Errorf("with no finder: %v, want ErrNoFinder", err)
	}
	boom := errors.New("no netlink")
	s.finder = &finder{err: boom}
	if _, err := s.Discover(context.Background(), 1); !errors.Is(err, boom) {
		t.Errorf("a failed search: %v, want its error", err)
	}
}

func TestWithFinder(t *testing.T) {
	_, d, q := newService(t)
	f := &finder{}
	s, err := New(d, q, clock{}, "1.2.3", WithFinder(f))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Discover(context.Background(), 1); err != nil || f.calls != 1 {
		t.Errorf("Discover = %v after %d searches", err, f.calls)
	}
}
