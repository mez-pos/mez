package api

import (
	"fmt"
	"testing"
	"time"
)

// TestLimitsOnOneAddressCountApart: two limits on the same address must not
// share a window, or a guest reading the menu would spend the pairing
// limit, and a window opened under one length would be judged by another.
func TestLimitsOnOneAddressCountApart(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now })
	small := limit{requests: 1, per: time.Minute}
	large := limit{requests: 100, per: time.Minute}
	if ok, _ := l.allow(small.key("10.0.0.9"), small); !ok {
		t.Fatal("the first request under the small limit was refused")
	}
	if ok, _ := l.allow(small.key("10.0.0.9"), small); ok {
		t.Fatal("the second request under the small limit was allowed")
	}
	if ok, _ := l.allow(large.key("10.0.0.9"), large); !ok {
		t.Error("a request under the large limit was refused because the small one was spent")
	}
}

// TestAFullMapSharesOneOverflowWindow: once the map holds its cap of live
// windows, a newcomer is not given a window of its own, nor is anyone's
// evicted for it (a flood of addresses could then push a till's window out
// and past its limit): the newcomers share one window under the limit,
// strictly, and wait reads it too. Each limit has its own.
func TestAFullMapSharesOneOverflowWindow(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now })
	lim := limit{requests: 2, per: time.Minute}
	for i := 0; i < maxLimiterKeys; i++ {
		if ok, _ := l.allow(lim.key(fmt.Sprintf("10.0.%d.%d", i/256, i%256)), lim); !ok {
			t.Fatalf("address %d refused while the map filled", i)
		}
	}
	// Two newcomers spend the shared window; the third is refused, and told
	// how long, though it never asked before.
	if ok, _ := l.allow(lim.key("172.16.0.1"), lim); !ok {
		t.Fatal("the first newcomer was refused")
	}
	if ok, _ := l.allow(lim.key("172.16.0.2"), lim); !ok {
		t.Fatal("the second newcomer was refused")
	}
	if ok, wait := l.allow(lim.key("172.16.0.3"), lim); ok || wait <= 0 {
		t.Fatalf("the third newcomer: allowed %v, wait %s", ok, wait)
	}
	if w := l.wait(lim.key("172.16.0.4"), lim); w <= 0 {
		t.Errorf("wait for a newcomer to a full map = %s, want the overflow window's", w)
	}
	// The addresses that had windows keep them, untouched.
	if ok, _ := l.allow(lim.key("10.0.0.0"), lim); !ok {
		t.Error("an address with a window of its own was refused")
	}
	// Another limit's newcomers have an overflow of their own.
	other := limit{requests: 1, per: time.Hour}
	if ok, _ := l.allow(other.key("172.16.0.9"), other); !ok {
		t.Error("another limit's newcomer was refused by this limit's overflow")
	}
	if len(l.windows) > maxLimiterKeys+2 {
		t.Errorf("the map grew to %d, want the cap plus the overflow windows", len(l.windows))
	}
}
