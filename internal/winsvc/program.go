package winsvc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kardianos/service"
)

// program is the server as the service library drives it: Start must return
// at once, so the server runs on a goroutine that Stop ends and waits for.
type program struct {
	run     func(ctx context.Context, system SystemLog) error
	timeout time.Duration

	// log takes the wrapper's own lines; system is what the server is
	// handed. Either may be nil.
	log    SystemLog
	system SystemLog

	// exit ends the process. It is os.Exit outside the tests.
	exit func(int)

	mu       sync.Mutex
	stopping bool
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
}

// Start starts the server and returns. What the server makes of its
// configuration, its port and its database is found out on the goroutine,
// and a failure there ends the process (see died).
func (p *program) Start(service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	p.mu.Lock()
	p.stopping, p.cancel, p.done, p.err = false, cancel, done, nil
	p.mu.Unlock()

	p.say("mez is starting.")

	go func() {
		err := p.run(ctx, p.system)

		p.mu.Lock()
		stopping := p.stopping
		p.err = err
		p.mu.Unlock()

		if !stopping {
			p.died(err)
		}
		close(done)
	}()

	return nil
}

// Stop ends the server and waits for it to finish what it was doing. Its
// error is the server's own, from the way down.
func (p *program) Stop(service.Service) error {
	p.mu.Lock()
	p.stopping = true
	cancel, done := p.cancel, p.done
	p.mu.Unlock()

	if cancel == nil {
		return nil // never started
	}
	cancel()

	select {
	case <-done:
	case <-time.After(p.timeout):
		return fmt.Errorf("the server did not stop within %s", p.timeout)
	}

	p.mu.Lock()
	err := p.err
	p.mu.Unlock()

	if err != nil {
		p.complain(fmt.Sprintf("mez stopped, and not cleanly: %v", err))
		return err
	}
	p.say("mez stopped.")
	return nil
}

// died is the server stopping when nobody asked it to: it could not start,
// or it lost its listener. The process ends without telling the Service
// Control Manager that it stopped, because that is the one ending Windows
// restarts a service after. A stop reported with an error is left stopped.
func (p *program) died(err error) {
	if err != nil {
		p.complain(fmt.Sprintf("mez stopped on its own and will be started again: %v", err))
	} else {
		p.complain("mez stopped on its own and will be started again.")
	}
	exit := p.exit
	if exit == nil {
		exit = os.Exit
	}
	exit(1)
}

// say and complain write the wrapper's own lines. There is nowhere to
// report a system log that cannot be written to.
func (p *program) say(line string) {
	if p.log != nil {
		_ = p.log.Info(line)
	}
}

func (p *program) complain(line string) {
	if p.log != nil {
		_ = p.log.Error(line)
	}
}
