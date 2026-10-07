package relay

import (
	"errors"
	"testing"
	"time"
)

func TestIPTableAdmits(t *testing.T) {
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	lim := Limit{PerMinute: 60, Burst: 3}

	// step is one admit, with what it should come to.
	type step struct {
		ip      string
		at      time.Duration
		release bool // release the admission at once
		wantErr error
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"within the burst", []step{
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
		}},
		{"past the burst", []step{
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, errRateLimited},
		}},
		{"another address is not held to the first one's spending", []step{
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, errRateLimited},
			{"2.2.2.2", 0, true, nil},
		}},
		{"a second later there is a token again", []step{
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", 0, true, nil},
			{"1.1.1.1", time.Second, true, nil},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := newIPTable(lim, start)
			for i, st := range tt.steps {
				release, wait, err := table.admit(st.ip, start.Add(st.at))
				if !errors.Is(err, st.wantErr) {
					t.Fatalf("step %d: err = %v, want %v", i+1, err, st.wantErr)
				}
				if errors.Is(err, errRateLimited) && wait <= 0 {
					t.Errorf("step %d: wait = %v, want a positive wait", i+1, wait)
				}
				if release != nil && st.release {
					release()
				}
			}
		})
	}
}

func TestIPTableCapsOpenStreams(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	table := newIPTable(Limit{PerMinute: 6000, Burst: 1000}, now)

	releases := make([]func(), 0, maxStreamsPerIP)
	for i := range maxStreamsPerIP {
		release, _, err := table.admit("1.1.1.1", now)
		if err != nil {
			t.Fatalf("stream %d: %v", i+1, err)
		}
		releases = append(releases, release)
	}
	if _, _, err := table.admit("1.1.1.1", now); !errors.Is(err, errBusy) {
		t.Fatalf("one over the cap: err = %v, want errBusy", err)
	}
	// Someone else is not kept out by the first address being full.
	if _, _, err := table.admit("2.2.2.2", now); err != nil {
		t.Fatalf("another address: %v", err)
	}
	releases[0]()
	if _, _, err := table.admit("1.1.1.1", now); err != nil {
		t.Fatalf("after a release: %v", err)
	}
}

func TestIPTablePrunesIdleAddresses(t *testing.T) {
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	table := newIPTable(DefaultIPLimit, start)

	// Three addresses: one idle, one still in flight, one recently seen.
	if release, _, err := table.admit("idle", start); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
	if _, _, err := table.admit("busy", start); err != nil {
		t.Fatal(err)
	}
	if release, _, err := table.admit("recent", start.Add(ipIdle-time.Second)); err != nil {
		t.Fatal(err)
	} else {
		release()
	}

	// Within the prune interval nothing is walked, idle or not.
	if release, _, err := table.admit("probe", start.Add(ipPruneEvery-time.Millisecond)); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
	if got := table.size(); got != 4 {
		t.Fatalf("before the prune: size = %d, want 4", got)
	}

	// Once the idle time has passed and a prune is due, the idle address
	// and the first probe go, the busy one is kept for its open stream and
	// the recent one for being recent.
	if _, _, err := table.admit("probe", start.Add(ipIdle+ipPruneEvery)); err != nil {
		t.Fatal(err)
	}
	if got := table.size(); got != 3 {
		t.Errorf("after the prune: size = %d, want 3 (busy, recent, probe)", got)
	}
	if _, ok := table.entries["idle"]; ok {
		t.Error("the idle address was kept")
	}
	if _, ok := table.entries["busy"]; !ok {
		t.Error("the address with a stream open was dropped")
	}
}
