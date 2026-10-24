//go:build windows

package winsvc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// TestTheManagerHearsCheckpointsWhileTheServerStops (ADR-375): a stop, a
// shutdown or a pre-shutdown ends the server, and while it does the manager
// is told StopPending with a rising checkpoint, so it does not take a stop
// that lasts for a hang and kill the process.
func TestTheManagerHearsCheckpointsWhileTheServerStops(t *testing.T) {
	for _, cmd := range []svc.Cmd{svc.Stop, svc.Shutdown, svc.PreShutdown} {
		t.Run(fmt.Sprintf("command %d", cmd), func(t *testing.T) {
			release := make(chan struct{})
			prog := &program{timeout: 5 * time.Second, run: func(ctx context.Context, _ SystemLog) error {
				<-ctx.Done()
				<-release // a slow shutdown: the checkpoints must cover it
				return nil
			}}
			h := &handler{prog: prog, tick: 10 * time.Millisecond}
			requests := make(chan svc.ChangeRequest)
			changes := make(chan svc.Status, 64)
			result := make(chan [2]any, 1)
			go func() {
				specific, code := h.Execute(nil, requests, changes)
				result <- [2]any{specific, code}
			}()

			if st := <-changes; st.State != svc.StartPending {
				t.Fatalf("first status %v, want StartPending", st.State)
			}
			if st := <-changes; st.State != svc.Running || st.Accepts&svc.AcceptPreShutdown == 0 || st.Accepts&svc.AcceptStop == 0 {
				t.Fatalf("running status %+v, want Running accepting stop and pre-shutdown", st)
			}
			requests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: svc.Status{State: svc.Running}}
			if st := <-changes; st.State != svc.Running {
				t.Fatalf("interrogated: %v", st.State)
			}

			requests <- svc.ChangeRequest{Cmd: cmd}
			first := <-changes
			if first.State != svc.StopPending || first.CheckPoint != 1 || first.WaitHint == 0 {
				t.Fatalf("on %v: %+v, want StopPending, checkpoint 1, a wait hint", cmd, first)
			}
			second := <-changes
			if second.State != svc.StopPending || second.CheckPoint <= first.CheckPoint {
				t.Fatalf("while stopping: %+v after %+v", second, first)
			}
			close(release)
			select {
			case r := <-result:
				if r[0] != false || r[1] != uint32(0) {
					t.Fatalf("Execute returned %v", r)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Execute did not return after the server stopped")
			}
		})
	}
}
