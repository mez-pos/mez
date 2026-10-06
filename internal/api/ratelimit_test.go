package api

import (
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
