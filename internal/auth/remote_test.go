package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

// wrongFromAway types n wrong PINs for a staff member through the relay.
func wrongFromAway(t *testing.T, w *staffWorld, staffRef string, n int) {
	t.Helper()
	for i := range n {
		w.clock.advance(time.Second)
		if _, err := w.auth.SignInRemote(context.Background(), w.till, staffRef, "0000"); err != ErrInvalidPIN {
			t.Fatalf("wrong PIN %d from away: err = %v, want exactly ErrInvalidPIN", i+1, err)
		}
	}
}

// clearLock lifts a hard lock as the people service and `mez unlock` do.
func clearLock(t *testing.T, w *staffWorld, staffID int64) {
	t.Helper()
	w.clock.advance(time.Second)
	at := w.clock.now().UTC()
	if _, err := w.db.Writer().ExecContext(context.Background(), `UPDATE staff SET locked_at = NULL, locked_reason = '' WHERE id = ?`, staffID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Writer().ExecContext(context.Background(), `UPDATE staff_lockouts SET cleared_at = ? WHERE staff_id = ? AND cleared_at IS NULL`, at, staffID); err != nil {
		t.Fatal(err)
	}
	w.clock.advance(time.Second)
}

// TestTheThirdWrongPINFromAwayHardLocks: two are forgiven by a right one from
// away; three in a row lock until someone clears it, with no window to wait
// out. Locked, the right PIN is refused as a wrong one from away and at the
// venue, and the sessions open before it are ended (ADR-314).
func TestTheThirdWrongPINFromAwayHardLocks(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()

	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); locked {
		t.Fatal("locked before the third wrong PIN")
	}
	away, err := w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357")
	if err != nil {
		t.Fatalf("the right PIN from away after two wrong: %v", err)
	}
	atVenue, err := w.auth.SignIn(ctx, w.bar, "stf_RAVI0001", "1357")
	if err != nil {
		t.Fatalf("the right PIN at the venue: %v", err)
	}

	// The right one started the count again: two more lock nothing.
	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); locked {
		t.Fatal("a right PIN from away did not start the count again")
	}
	wrongFromAway(t, w, "stf_RAVI0001", 1)
	locked, reason := hardLocked(t, w, "stf_RAVI0001")
	if !locked || reason != RemoteHardLockReason {
		t.Fatalf("after the third wrong PIN from away: locked %v, reason %q", locked, reason)
	}

	if _, err := w.auth.Session(ctx, w.till, away.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the session opened from away before the lock: err = %v, want ended", err)
	}
	if _, err := w.auth.Session(ctx, w.bar, atVenue.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the session opened at the venue before the lock: err = %v, want ended", err)
	}
	// A week on, it has not lifted by itself.
	w.clock.advance(7 * 24 * time.Hour)
	if _, err := w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357"); err != ErrInvalidPIN {
		t.Errorf("the right PIN from away, locked: err = %v, want exactly ErrInvalidPIN", err)
	}
	if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357"); err != ErrInvalidPIN {
		t.Errorf("the right PIN at the venue, locked: err = %v, want exactly ErrInvalidPIN", err)
	}
}

// TestParallelGuessesFromAwayCannotOutrunTheLock fires many PINs at once
// through the relay, one of them right. Each waits its turn, and reads the
// person again when it comes: once the third wrong one has locked them, the
// rest are refused as a locked person's are, the right one among them too.
func TestParallelGuessesFromAwayCannotOutrunTheLock(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)

	const burst = 12
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "0000")
		}()
	}
	close(start)
	wg.Wait()
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); !locked {
		t.Fatal("not locked")
	}
	count := map[string]int{}
	for _, o := range outcomes(t, w.db) {
		count[o]++
	}
	if count[outcomeInvalidPIN] != RemoteMaxFailures || count[outcomeInactiveStaff] != burst-1 {
		t.Errorf("recorded %v: want %d PINs compared as live guesses and %d refused as a locked person's",
			count, RemoteMaxFailures, burst-1)
	}
}

// TestALockThatFellWhileWaitingRefusesTheRightPIN: a right PIN that was
// already on its way when the lock fell opens nothing.
func TestALockThatFellWhileWaitingRefusesTheRightPIN(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)

	// Hold Ravi's turn, as an attempt in progress does, start a right PIN
	// behind it, and let the lock fall before that one is let through.
	release := w.auth.locks.lock(fmt.Sprintf("staff:%d", w.ravi.ID))
	done := make(chan error, 1)
	go func() {
		_, err := w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357")
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	if _, err := w.db.Writer().ExecContext(ctx, `UPDATE staff SET locked_at = ?, locked_reason = ? WHERE id = ?`,
		w.clock.now().UTC(), RemoteHardLockReason, w.ravi.ID); err != nil {
		t.Fatal(err)
	}
	release()
	if err := <-done; err != ErrInvalidPIN {
		t.Errorf("the right PIN, let through after the lock fell: err = %v, want exactly ErrInvalidPIN", err)
	}
}

// TestAttemptsFromAwayAndAtTheVenueAreCountedApart: a right PIN at the till
// does not forgive wrong ones from away, wrong ones from away do not bring
// the till's window lock closer, and the till's lock does not stand in the
// way of a right PIN from away.
func TestAttemptsFromAwayAndAtTheVenueAreCountedApart(t *testing.T) {
	ctx := context.Background()

	t.Run("a right PIN at the venue does not forgive two from away", func(t *testing.T) {
		w := newStaffWorld(t)
		wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)
		if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357"); err != nil {
			t.Fatalf("the right PIN at the venue: %v", err)
		}
		wrongFromAway(t, w, "stf_RAVI0001", 1)
		if locked, _ := hardLocked(t, w, "stf_RAVI0001"); !locked {
			t.Error("the third wrong PIN from away did not lock")
		}
	})

	t.Run("wrong PINs from away are not the venue's", func(t *testing.T) {
		w := newStaffWorld(t)
		for range DefaultStaffLockout.MaxFailures - 1 {
			w.clock.advance(time.Second)
			if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "0000"); err != ErrInvalidPIN {
				t.Fatalf("a wrong PIN at the venue: %v", err)
			}
		}
		wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)
		w.clock.advance(time.Second)
		if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357"); err != nil {
			t.Errorf("four wrong at the venue and two from away locked the venue's window: %v", err)
		}
	})

	t.Run("wrong PINs at the venue are not from away", func(t *testing.T) {
		w := newStaffWorld(t)
		for range DefaultStaffLockout.MaxFailures {
			w.clock.advance(time.Second)
			if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "0000"); err != ErrInvalidPIN {
				t.Fatalf("a wrong PIN at the venue: %v", err)
			}
		}
		w.clock.advance(time.Second)
		if _, err := w.auth.SignIn(ctx, w.till, "stf_RAVI0001", "1357"); !errors.Is(err, ErrLockedOut) {
			t.Fatalf("the venue's window is not locked: %v", err)
		}
		if locked, _ := hardLocked(t, w, "stf_RAVI0001"); locked {
			t.Fatal("five wrong at the venue hard-locked")
		}
		if _, err := w.auth.SignInRemote(ctx, w.bar, "stf_RAVI0001", "1357"); err != nil {
			t.Errorf("the right PIN from away during the venue's window lock: %v", err)
		}
	})
}

// TestAClearedLockWipesTheStrikesFromAway: once someone at the venue clears
// the lock, the right PIN works from away at once and the count starts again.
func TestAClearedLockWipesTheStrikesFromAway(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures)
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); !locked {
		t.Fatal("not locked")
	}
	clearLock(t, w, w.ravi.ID)

	wrongFromAway(t, w, "stf_RAVI0001", RemoteMaxFailures-1)
	if locked, _ := hardLocked(t, w, "stf_RAVI0001"); locked {
		t.Fatal("two wrong PINs after a clear locked: the strikes before it still counted")
	}
	if _, err := w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357"); err != nil {
		t.Errorf("after the clear, the right PIN from away: %v", err)
	}
}

// TestTheDeviceLimitCountsEveryAttemptOnIt: the tablet's own limit is not
// split in two by where its attempts came from.
func TestTheDeviceLimitCountsEveryAttemptOnIt(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	for i := range DefaultDeviceLockout.MaxFailures {
		w.clock.advance(time.Second)
		signIn := w.auth.SignIn
		if i%2 == 0 {
			signIn = w.auth.SignInRemote
		}
		if _, err := signIn(ctx, w.till, "stf_NOBODY01", "0000"); err != ErrInvalidPIN {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	w.clock.advance(time.Second)
	var locked *LockedError
	if _, err := w.auth.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357"); !errors.As(err, &locked) || locked.Scope != "device" {
		t.Errorf("after %d failures on one device, from away: err = %v, want a device lock", DefaultDeviceLockout.MaxFailures, err)
	}
}

// TestASessionKnowsWhereItWasOpened: one opened through the relay says so and
// lasts RemoteSessionTTL whatever the venue's own length is; one opened at
// the venue does neither. Each attempt is recorded with how it arrived.
func TestASessionKnowsWhereItWasOpened(t *testing.T) {
	w := newStaffWorld(t)
	ctx := context.Background()
	long, err := NewStaffAuth(StaffOptions{DB: w.db, Now: w.clock.now, SessionTTL: 48 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	opened := w.clock.now().UTC()

	away, err := long.SignInRemote(ctx, w.till, "stf_RAVI0001", "1357")
	if err != nil {
		t.Fatalf("SignInRemote: %v", err)
	}
	if !away.Session.Remote || !away.Session.ExpiresAt.Equal(opened.Add(RemoteSessionTTL)) {
		t.Errorf("opened from away: remote %v, expires %v; want remote, %v", away.Session.Remote, away.Session.ExpiresAt, opened.Add(RemoteSessionTTL))
	}
	here, err := long.SignIn(ctx, w.bar, "stf_RAVI0001", "1357")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if here.Session.Remote || !here.Session.ExpiresAt.Equal(opened.Add(48*time.Hour)) {
		t.Errorf("opened at the venue: remote %v, expires %v", here.Session.Remote, here.Session.ExpiresAt)
	}

	w.clock.advance(RemoteSessionTTL - time.Second)
	read, err := long.Session(ctx, w.till, away.Token)
	if err != nil || !read.Remote {
		t.Fatalf("a second before twelve hours: %+v, %v", read, err)
	}
	w.clock.advance(time.Second)
	if _, err := long.Session(ctx, w.till, away.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("at twelve hours the session from away: err = %v, want expired", err)
	}
	if read, err := long.Session(ctx, w.bar, here.Token); err != nil || read.Remote {
		t.Errorf("at twelve hours the venue's session: %+v, %v", read, err)
	}

	rows, err := w.db.Reader().QueryContext(ctx, `SELECT via FROM sign_in_attempts ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var vias []string
	for rows.Next() {
		var via string
		if err := rows.Scan(&via); err != nil {
			t.Fatal(err)
		}
		vias = append(vias, via)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(vias, []string{"relay", ""}) {
		t.Errorf("attempts recorded as arriving %q, want relay then the venue", vias)
	}
}
