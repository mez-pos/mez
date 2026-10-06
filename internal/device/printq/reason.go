package printq

import "errors"

// Unreachable is the reason a job is kept with when its printer did not
// take the bytes. It is all the queue screen and the printers list say about
// such a send: what the transport said, such as a dial error naming an
// address and how the network answered it, reads as the result of a scan of
// the venue's network, so it goes to the server's log only (ADR-259).
const Unreachable = "Could not reach the printer"

// shownError is a failed send whose own text is fit for the queue screen:
// one the queue or a transport wrote itself, naming nothing the network said.
type shownError struct{ err error }

func (e *shownError) Error() string { return e.err.Error() }
func (e *shownError) Unwrap() error { return e.err }

// shown marks err as fit for the queue screen as it is.
func shown(err error) error { return &shownError{err} }

// Reason is what a job is kept with, for the screens, after a send failed
// with err: the text of an error marked shown, or Unreachable.
func Reason(err error) string {
	var s *shownError
	if errors.As(err, &s) {
		return s.Error()
	}
	return Unreachable
}
