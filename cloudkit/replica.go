package cloudkit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/backup"
	"github.com/mez-pos/mez/internal/cloud"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/mirror"
	"github.com/mez-pos/mez/internal/service/closeofday"
	"github.com/mez-pos/mez/internal/service/watched"
)

// The events an owner may be told about (ADR-360; Phase 15, Task 3c),
// read off a replica past a cursor in the venue's event stream
// (Replica.Watched): what each was worth as the order's projection says,
// who did it, and who let it through.
type (
	// WatchedEvent is one such event. Its ID is the cursor.
	WatchedEvent = watched.Event
	// WatchedKind is what happened.
	WatchedKind = watched.Kind
	// WatchedPerson is who: their name, and whether their role closes a
	// day (shift.manage), which is what "not a cashier" means.
	WatchedPerson = watched.Person
)

// The kinds of watched event.
const (
	WatchedVoid       = watched.Void
	WatchedComp       = watched.Comp
	WatchedRefund     = watched.Refund
	WatchedCreditNote = watched.CreditNote
	WatchedDiscount   = watched.Discount
)

// WatchedLimit is the most events Replica.Watched reads at one asking.
const WatchedLimit = watched.DefaultLimit

// A venue's replica, read for its figures (ADR-362; Phase 15, Task 3):
// the newest copy in its bucket, brought up to the bucket's newest write
// on each Refresh, with the close-of-day figures ADR-360's message is made
// of read off it. A mirror less its door and its server. The cloud's
// reader keeps one a venue and a cursor beside it.
type (
	// Replica is one venue's copy. Refresh takes a newer write when the
	// bucket has one; Closes reads the Z-reports past a cursor; Close puts
	// the copy away. It is used by one goroutine.
	Replica = mirror.Replica
	// CloseOfDay is one Z-report's figures for the message: every amount
	// in minor units of its Currency, every one the slip's.
	CloseOfDay = closeofday.Close
	// CloseTally is how many of something and what they came to.
	CloseTally = closeofday.Tally
)

// ErrReplicaClosed is a replica asked something after Close.
var ErrReplicaClosed = mirror.ErrReplicaClosed

// ReplicaOptions are what a replica is opened with. Bucket and Venue are
// required.
type ReplicaOptions struct {
	// Bucket is where the venue's backup is. It is only read: the reading
	// key is enough.
	Bucket Bucket
	// Venue is the venue's public id, as it paired. A copy that holds
	// another venue is refused.
	Venue string
	// CopyDir is where each copy is restored. Empty is the system's
	// temporary directory.
	CopyDir string
	// Log is told what goes wrong.
	Log *slog.Logger
	// Patience is how long a copy's database is given to close. Zero is
	// ten seconds.
	Patience time.Duration
}

// OpenReplica restores the newest copy in the bucket, checks it and brings
// it up to this build's schema. It holds exactly one venue or it is
// refused, and nothing is left behind when it fails.
func OpenReplica(ctx context.Context, o ReplicaOptions) (*Replica, error) {
	if !cloud.ValidVenuePublicID(o.Venue) {
		return nil, fmt.Errorf("cloudkit: a replica is opened for a venue by its public id, and %q is not one", o.Venue)
	}
	return mirror.OpenReplica(ctx, mirror.ReplicaOptions{
		From: backup.Config{Kind: backup.KindS3, S3: o.Bucket}, Venue: o.Venue,
		CopyDir: o.CopyDir, Log: o.Log, Patience: o.Patience,
	})
}

// MoneyText renders an amount in minor units as the owner reads it in a
// message: the major unit with the currency's decimal places and a group
// separator, no symbol and no code ("24,310.00"; "1,00,000.00" in rupees,
// grouped as India writes them). A negative amount keeps its sign. It is
// the slip's own rendering (internal/device/escpos), as ADR-360 asks,
// without the symbol a slip's total carries.
func MoneyText(minor int64, currency string) string {
	s := money.Amount{Minor: minor, Currency: currency}.String()
	s = strings.TrimSuffix(s, " "+currency)
	sign := ""
	if rest, found := strings.CutPrefix(s, "-"); found {
		sign, s = "-", rest
	}
	whole, frac, hasFrac := strings.Cut(s, ".")
	grouped := groupDigits(whole, currency == "INR")
	if hasFrac {
		return sign + grouped + "." + frac
	}
	return sign + grouped
}

// groupDigits puts separators into a run of digits: every three from the
// right, or, as India writes, three then every two.
func groupDigits(digits string, indian bool) string {
	if len(digits) <= 3 {
		return digits
	}
	head, tail := digits[:len(digits)-3], digits[len(digits)-3:]
	width := 3
	if indian {
		width = 2
	}
	var groups []string
	for len(head) > width {
		groups = append([]string{head[len(head)-width:]}, groups...)
		head = head[:len(head)-width]
	}
	groups = append([]string{head}, groups...)
	return strings.Join(groups, ",") + "," + tail
}
