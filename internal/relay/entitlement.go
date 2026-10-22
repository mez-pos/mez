package relay

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mez-pos/mez/internal/relay/wire"
)

// Entitlement is whether the relay carries a venue, until when, and how far
// (ADR-312, ADR-316). A key says who a venue is; this says whether it is
// served. The relay acts on Until and Remote and on nothing else. State,
// PaidUntil and Reason are words for people, passed on to the venue as they
// are given.
type Entitlement struct {
	// Until is when the relay stops carrying the venue. The zero time is
	// never, and NoEnd is for ever.
	Until time.Time
	// Remote says the back office may cross, if the venue's key says so too.
	Remote bool
	// State is what whoever keeps the entitlements calls where the venue
	// stands: "trial", "active", "grace" (ADR-313). Empty from a file.
	State string
	// PaidUntil is the end of what has been paid for, which is earlier than
	// Until wherever there is a grace. Zero when nobody is counting.
	PaidUntil time.Time
	// Reason is a short code for why the venue stands where it does. It is
	// what a venue that is turned away is told.
	Reason string
	// LastReport is when the venue's last close-of-day message went, and
	// MessagesURL where its owner changes what is sent (ADR-364). Words
	// for the venue's Devices screen; zero and empty from a file.
	LastReport  time.Time
	MessagesURL string
}

// NoEnd is the Until of a venue that is carried with no date set.
var NoEnd = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// The reasons the relay gives when whoever keeps the entitlements gave none.
const (
	// ReasonExpired is a venue whose date has passed.
	ReasonExpired = "expired"
	// ReasonNotListed is a venue the entitlements file does not name.
	ReasonNotListed = "not_listed"
	// ReasonNever is a venue the entitlements file names to turn away.
	ReasonNever = "never"
)

// covers reports whether the venue is carried at now.
func (e Entitlement) covers(now time.Time) bool { return now.Before(e.Until) }

// reason is why a venue that is not covered is turned away.
func (e Entitlement) reason() string {
	if e.Reason != "" {
		return e.Reason
	}
	return ReasonExpired
}

// same reports whether two entitlements would be told to a venue alike.
func (e Entitlement) same(o Entitlement) bool {
	return e.Until.Equal(o.Until) && e.PaidUntil.Equal(o.PaidUntil) &&
		e.Remote == o.Remote && e.State == o.State && e.Reason == o.Reason &&
		e.LastReport.Equal(o.LastReport) && e.MessagesURL == o.MessagesURL
}

// told is the entitlement as the venue is told it. remote is what the relay
// will in fact carry: the entitlement's word and the key's together.
func (e Entitlement) told(remote bool) wire.Entitlement {
	out := wire.Entitlement{State: e.State, Remote: remote, Reason: e.Reason, MessagesURL: e.MessagesURL}
	if !e.LastReport.IsZero() {
		last := e.LastReport.UTC()
		out.LastReport = &last
	}
	if !e.Until.IsZero() && e.Until.Before(NoEnd) {
		until := e.Until.UTC()
		out.Until = &until
	}
	if !e.PaidUntil.IsZero() {
		paid := e.PaidUntil.UTC()
		out.PaidUntil = &paid
	}
	return out
}

// Entitlements says whether a venue is carried. It is asked when a venue
// connects and about once an hour for as long as it stays, never for a
// guest's request, so it may read a file or a database.
//
// An error is not a refusal. The relay carries on as it was: a venue that is
// connected stays, and one that is connecting is let in on what its key
// says. A store that is down must not close a cafe's ordering.
type Entitlements interface {
	Entitled(ctx context.Context, venueID string) (Entitlement, error)
}

// everyone carries every venue with a valid key for ever: a relay with no
// entitlements set, which is what a relay was before there were any.
type everyone struct{}

func (everyone) Entitled(context.Context, string) (Entitlement, error) {
	return Entitlement{Until: NoEnd, Remote: true}, nil
}

// StaticEntitlements reads entitlements from a file, one venue to a line:
//
//	# venue      until        back office
//	my-cafe      2026-11-12   remote
//	corner-bar   forever      guests
//	old-place    never
//	*            forever
//
// until is a day (carried through the end of it, UTC), an instant as
// 2026-11-12T18:30:00Z, "forever" or "never". The third word is "remote" or
// "guests", and "remote" when left out: the back office crosses if the
// venue's key allows it as well. "*" is every venue not named, and without
// it a venue not named is not carried.
//
// The file is read again when it has changed, the next time a venue is
// asked about, so an edit takes hold within the hour and at once for a venue
// that connects. A file that can no longer be read keeps what was last read.
type StaticEntitlements struct {
	path string
	log  *slog.Logger

	mu     sync.Mutex
	venues map[string]Entitlement
	rest   *Entitlement
	mod    time.Time
	size   int64
}

// LoadStaticEntitlements reads the file at path. A file that cannot be read
// as entitlements is an error here, where an operator is watching.
func LoadStaticEntitlements(path string, log *slog.Logger) (*StaticEntitlements, error) {
	if log == nil {
		return nil, errors.New("relay: a log is required")
	}
	s := &StaticEntitlements{path: path, log: log}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Len is how many venues the file names, "*" not counted.
func (s *StaticEntitlements) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.venues)
}

// Entitled answers from the file, reading it again first if it has changed.
func (s *StaticEntitlements) Entitled(_ context.Context, venueID string) (Entitlement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fi, err := os.Stat(s.path); err != nil {
		s.log.Error("relay: the entitlements file cannot be read; what was last read stands", "path", s.path, "error", err.Error())
	} else if !fi.ModTime().Equal(s.mod) || fi.Size() != s.size {
		if err := s.loadLocked(); err != nil {
			s.log.Error("relay: the entitlements file changed and cannot be read; what was last read stands", "error", err.Error())
			// Not again until it changes again.
			s.mod, s.size = fi.ModTime(), fi.Size()
		} else {
			s.log.Info("relay: entitlements read again", "path", s.path, "venues", len(s.venues))
		}
	}
	if e, ok := s.venues[venueID]; ok {
		return e, nil
	}
	if s.rest != nil {
		return *s.rest, nil
	}
	return Entitlement{Reason: ReasonNotListed}, nil
}

func (s *StaticEntitlements) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *StaticEntitlements) loadLocked() error {
	f, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("relay: entitlements: %w", err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("relay: entitlements: %w", err)
	}
	venues, rest, err := parseEntitlements(f)
	if err != nil {
		return fmt.Errorf("relay: entitlements: %s: %w", s.path, err)
	}
	s.venues, s.rest, s.mod, s.size = venues, rest, fi.ModTime(), fi.Size()
	return nil
}

// maxEntitlementLine bounds one line of the file. A venue id is at most 63
// characters and a date thirty.
const maxEntitlementLine = 512

// parseEntitlements reads the file's lines. Everything after a # is a note.
func parseEntitlements(r io.Reader) (venues map[string]Entitlement, rest *Entitlement, err error) {
	venues = map[string]Entitlement{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, maxEntitlementLine), maxEntitlementLine)
	for n := 1; sc.Scan(); n++ {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 2 || len(fields) > 3 {
			return nil, nil, fmt.Errorf("line %d: want a venue, until when, and remote or guests", n)
		}
		e, err := parseEntitlement(fields[1:])
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", n, err)
		}
		switch venue := fields[0]; {
		case venue == "*":
			if rest != nil {
				return nil, nil, fmt.Errorf("line %d: * is given twice", n)
			}
			rest = &e
		case !ValidVenueID(venue):
			return nil, nil, fmt.Errorf("line %d: %q is not a venue id", n, venue)
		default:
			if _, twice := venues[venue]; twice {
				return nil, nil, fmt.Errorf("line %d: %s is given twice", n, venue)
			}
			venues[venue] = e
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	return venues, rest, nil
}

// parseEntitlement reads "until" and, if it is there, "remote" or "guests".
func parseEntitlement(fields []string) (Entitlement, error) {
	e := Entitlement{Remote: true}
	switch until := fields[0]; until {
	case "forever":
		e.Until = NoEnd
	case "never":
		e.Reason = ReasonNever
	default:
		if day, err := time.Parse(time.DateOnly, until); err == nil {
			// Through the end of the day written, as a person means it.
			e.Until = day.AddDate(0, 0, 1)
		} else if at, err := time.Parse(time.RFC3339, until); err == nil {
			e.Until = at.UTC()
		} else {
			return Entitlement{}, fmt.Errorf("%q is not a day like 2026-11-12, an instant like 2026-11-12T18:30:00Z, forever or never", until)
		}
	}
	if len(fields) == 2 {
		switch fields[1] {
		case "remote":
		case "guests":
			e.Remote = false
		default:
			return Entitlement{}, fmt.Errorf("%q is not remote or guests", fields[1])
		}
	}
	return e, nil
}
