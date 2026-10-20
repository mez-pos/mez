package relay

import (
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/relay/wire"
)

const (
	// guestPrefix is where guests live. /v/<venue>/t/<table> is what a table's
	// QR code says; everything below /v/<venue>/ is the venue's to answer.
	guestPrefix = "/v/"

	// maxRequestBody is the largest thing a guest may send. An order is a
	// few hundred bytes of JSON.
	maxRequestBody = 1 << 20

	// guestIOTimeout bounds one read from, or one write to, a guest. It
	// rolls: a slow phone may take as long as it likes while bytes move,
	// and a silent one is let go.
	guestIOTimeout = 30 * time.Second
)

// hopHeaders describe one connection and are never forwarded across the
// socket in either direction.
var hopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// forwardedHeaders are the relay's to set. A guest that sends them is
// describing itself, and the venue must not be shown that as fact.
var forwardedHeaders = []string{
	"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto",
	"X-Forwarded-Prefix", "X-Real-Ip",
}

// The two things a guest is told when the relay cannot serve them. Staff
// have to be able to explain both.
const (
	offlineTitle = "Ordering is offline"
	offlineText  = "This cafe's online ordering is not reachable right now. Please order at the counter."
	busyTitle    = "One moment"
	busyText     = "A lot of people are ordering at once. Please try again in a few seconds."
)

// handleBareVenue sends /v/<venue> to /v/<venue>/, so the venue's relative
// links resolve under its own prefix.
func (s *Server) handleBareVenue(w http.ResponseWriter, r *http.Request) {
	venueID := r.PathValue("venue")
	if !ValidVenueID(venueID) {
		s.refuseNotFound(w, r)
		return
	}
	target := guestPrefix + venueID + "/"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusPermanentRedirect)
}

// handleGuest is a guest request for a venue named in the path.
func (s *Server) handleGuest(w http.ResponseWriter, r *http.Request) {
	venueID := r.PathValue("venue")
	prefix := guestPrefix + venueID
	// The escaped path, so what reaches the venue is what the guest sent and
	// not a decoding of it. A venue id never needs escaping, so a path that
	// does not start with the plain prefix was not written by a QR code.
	path, ok := strings.CutPrefix(r.URL.EscapedPath(), prefix)
	if !ValidVenueID(venueID) || !ok || !strings.HasPrefix(path, "/") {
		s.refuseNotFound(w, r)
		return
	}
	s.forward(w, r, venueID, prefix, path)
}

// byHost serves a venue at its own hostname, <venue>.<domain>, and hands
// every other request to next.
//
// A hostname each is what keeps venues apart in a guest's browser: to it,
// two hostnames are two sites, with their own cookies, storage and service
// workers, where two paths on one hostname are one site (ADR-203).
func (s *Server) byHost(next http.Handler) http.Handler {
	if s.venueDomain == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		venueID, ok := venueFromHost(r.Host, s.venueDomain)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		// The whole path is the venue's: there is no prefix to take off.
		s.forward(w, r, venueID, "", r.URL.EscapedPath())
	})
}

// venueFromHost reads the venue out of a Host header. Only one label under
// the domain names a venue, and never a reserved one.
func venueFromHost(host, domain string) (string, bool) {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	label, ok := strings.CutSuffix(host, "."+domain)
	if !ok || !ValidVenueID(label) || Reserved(label) {
		return "", false
	}
	return label, true
}

// forward sends one guest request to venueID and streams the answer back.
// prefix is what was taken off the front of the guest's path to get path;
// it is empty for a venue served at its own hostname.
func (s *Server) forward(w http.ResponseWriter, r *http.Request, venueID, prefix, path string) {
	noteVenue(r, venueID)

	// The client's own allowance first, before the venue's: a venue's
	// bucket is shared by every guest in the room, and one script spending
	// it would lock all of them out. The address's stream is counted open
	// until this request is over, whichever way it ends.
	release, wait, err := s.ips.admit(s.clientIP(r), s.now())
	switch {
	case errors.Is(err, errRateLimited):
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		s.refuse(w, r, http.StatusTooManyRequests, codeRateLimited, busyTitle, busyText)
		return
	case errors.Is(err, errBusy):
		w.Header().Set("Retry-After", "5")
		s.refuse(w, r, http.StatusServiceUnavailable, codeVenueBusy, busyTitle, busyText)
		return
	}
	defer release()

	a := s.agentFor(venueID)
	if a == nil {
		// Also the answer for a venue that does not exist: the relay has no
		// list of venues to tell the two apart, and should not say if it had.
		s.refuse(w, r, http.StatusServiceUnavailable, codeVenueOffline, offlineTitle, offlineText)
		return
	}
	if staffOnly(path) && !a.remote.Load() {
		// The back office and the staff API cross the relay only for a venue
		// whose key says so (ADR-306). Nothing is sent down the socket: to
		// the venue, and to whoever is asking, the path does not exist.
		s.refuseNotFound(w, r)
		return
	}
	if ok, wait := a.bucket.take(s.now()); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		s.refuse(w, r, http.StatusTooManyRequests, codeRateLimited, busyTitle, busyText)
		return
	}

	st, err := a.open()
	if errors.Is(err, errBusy) {
		w.Header().Set("Retry-After", "5")
		s.refuse(w, r, http.StatusServiceUnavailable, codeVenueBusy, busyTitle, busyText)
		return
	}
	if err != nil {
		s.refuse(w, r, http.StatusServiceUnavailable, codeVenueOffline, offlineTitle, offlineText)
		return
	}
	defer a.forget(st.id)

	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	head := wire.RequestHead{Method: r.Method, Path: path, Header: s.requestHeader(r, prefix)}
	msg, err := wire.EncodeHead(wire.TypeRequest, st.id, head)
	if err != nil {
		// The only way a head fails to encode is by not fitting a frame.
		s.refuse(w, r, http.StatusRequestHeaderFieldsTooLarge, codeHeaderTooLong,
			"Request too large", "This request could not be sent. Please scan the code again.")
		return
	}
	if a.send(msg) != nil {
		s.refuse(w, r, http.StatusServiceUnavailable, codeVenueOffline, offlineTitle, offlineText)
		return
	}

	if err := s.sendBody(a, st, w, r); err != nil {
		a.reset(st.id, "request body not sent")
		switch {
		case errors.Is(err, errTooLarge):
			s.refuse(w, r, http.StatusRequestEntityTooLarge, codeTooLarge,
				"Request too large", "This request could not be sent.")
		case errors.Is(err, errAgentGone):
			s.refuse(w, r, http.StatusServiceUnavailable, codeVenueOffline, offlineTitle, offlineText)
		default:
			// The guest went away mid-upload. Nobody is left to answer.
			noteCut(r, "guest gone")
		}
		return
	}

	s.relayResponse(a, st, prefix, w, r)
}

// staffPrefixes are the paths that are the venue's own people's and not a
// guest's: the back office bundle and the API it speaks to. A guest's app
// lives under /order/ and asks for nothing outside it (ADR-202).
var staffPrefixes = []string{"/backoffice", "/api"}

// StaffPath is staffOnly for whoever serves a venue's hostname beside the
// relay, so that what it gives the back office and what the relay keeps
// from guests are the one judgement (ADR-355).
func StaffPath(rawPath string) bool { return staffOnly(rawPath) }

// staffOnly reports whether rawPath, as the guest sent it (escaped, with the
// venue prefix already taken off), is one of the staff's. The prefix must be
// the whole first segment: "/apiary" is not "/api".
//
// The venue routes on the decoded, cleaned path, so that is what is compared
// here: "/%62ackoffice/" and "/t/../backoffice/" are "/backoffice/" to the
// venue and are refused as it. A path that cannot be decoded is refused too,
// since nothing a guest's app sends is one.
func staffOnly(rawPath string) bool {
	p, _, _ := strings.Cut(rawPath, "?")
	decoded, err := url.PathUnescape(p)
	if err != nil {
		return true
	}
	cleaned := path.Clean("/" + strings.ToLower(decoded))
	for _, prefix := range staffPrefixes {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return true
		}
	}
	return false
}

// requestHeader is the guest's headers as the venue should see them: nothing
// about the connection, nothing the guest claimed about where it came from,
// and the relay's own account of that in its place.
func (s *Server) requestHeader(r *http.Request, prefix string) http.Header {
	h := r.Header.Clone()
	stripHop(h)
	for _, name := range forwardedHeaders {
		h.Del(name)
	}
	h.Set("X-Forwarded-For", s.clientIP(r))
	h.Set("X-Forwarded-Host", r.Host)
	h.Set("X-Forwarded-Proto", s.scheme(r))
	if prefix != "" {
		h.Set("X-Forwarded-Prefix", prefix)
	}
	h.Set(headerRequestID, requestIDFrom(r.Context()))
	return h
}

// stripHop removes the hop-by-hop headers, and whatever Connection names.
func stripHop(h http.Header) {
	for _, value := range h.Values("Connection") {
		for name := range strings.SplitSeq(value, ",") {
			if name = textproto.TrimString(name); name != "" {
				h.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		h.Del(name)
	}
}

// originHeaders act on the whole origin, and the origin is every venue on
// this relay. A venue may not send them.
//
// Service-Worker-Allowed would let one venue's script take requests meant
// for another; Clear-Site-Data would empty every venue's storage on the
// guest's phone.
var originHeaders = []string{"Service-Worker-Allowed", "Clear-Site-Data"}

// copyResponseHeader writes a venue's response headers for a guest.
//
// A venue speaks in its own paths, as if it were at the root, and is in fact
// one directory of an address it shares with strangers. So a cookie is kept
// to the venue's prefix whatever path it asked for, a redirect to "/x" goes
// to "/v/<venue>/x", and nothing that reaches beyond the prefix is passed on.
func copyResponseHeader(dst, venue http.Header, prefix string) {
	// The venue's headers came off the socket as JSON, spelled however the
	// venue liked. Every check below is by canonical name, so they are made
	// canonical first, or "service-worker-allowed" walks past all of them.
	src := make(http.Header, len(venue))
	for name, values := range venue {
		for _, v := range values {
			src.Add(name, v)
		}
	}
	stripHop(src)
	if prefix != "" {
		// With a hostname of its own a venue's origin is its own to manage.
		for _, name := range originHeaders {
			src.Del(name)
		}
	}
	for name, values := range src {
		switch name {
		case "Set-Cookie":
			for _, line := range values {
				if scoped, ok := scopeCookie(line, prefix); ok {
					dst.Add(name, scoped)
				}
			}
		case "Location":
			if len(values) > 0 {
				dst.Set(name, scopeLocation(values[0], prefix))
			}
		default:
			dst[name] = values
		}
	}
}

// scopeCookie confines a venue's cookie to the venue. One that cannot be
// read is dropped: passing it on unread would be passing on its scope.
func scopeCookie(line, prefix string) (string, bool) {
	c, err := http.ParseSetCookie(line)
	if err != nil {
		return "", false
	}
	// No Domain, with a prefix or without: the cookie belongs to this host.
	// A venue at cafe.example may otherwise set one for all of example,
	// which every other venue's guests would then send.
	c.Domain = ""
	if strings.HasPrefix(c.Path, "/") {
		c.Path = prefix + c.Path
	} else {
		c.Path = prefix + "/"
	}
	scoped := c.String()
	return scoped, scoped != ""
}

// scopeLocation puts a redirect to one of the venue's own paths under the
// venue's prefix. Anything else, a full URL or a relative path, is already
// right or already none of the relay's business.
func scopeLocation(location, prefix string) string {
	if strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
		return prefix + location
	}
	return location
}

// errTooLarge is a guest body over maxRequestBody.
var errTooLarge = errors.New("relay: request body too large")

// sendBody forwards the guest's request body and closes the relay's half of
// the stream.
func (s *Server) sendBody(a *agent, st *stream, w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)
	body := http.MaxBytesReader(w, r.Body, maxRequestBody)
	buf := make([]byte, wire.MaxChunk)
	for {
		// Not every writer can set a deadline; one that cannot is a test's.
		_ = rc.SetReadDeadline(time.Now().Add(guestIOTimeout))
		n, err := body.Read(buf)
		if n > 0 {
			msg, encErr := wire.Encode(wire.Frame{Type: wire.TypeBody, Stream: st.id, Payload: buf[:n]})
			if encErr != nil {
				return encErr
			}
			if sendErr := a.send(msg); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			break
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errTooLarge
		}
		if err != nil {
			return err
		}
	}
	_ = rc.SetReadDeadline(time.Time{})

	msg, err := wire.Encode(wire.Frame{Type: wire.TypeEnd, Stream: st.id})
	if err != nil {
		return err
	}
	return a.send(msg)
}

// relayResponse copies the venue's answer to the guest as it arrives.
func (s *Server) relayResponse(a *agent, st *stream, prefix string, w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	started := false

	// fail ends the exchange when it cannot be completed. Before the head
	// there is still an honest status to send. After it the only honest
	// thing is to cut the connection, so the guest's browser sees a broken
	// download and not a short one that looks whole; http.ErrAbortHandler
	// is how net/http spells that, and it is what its own proxy does.
	fail := func(status int, code, why string) {
		if !started {
			s.refuse(w, r, status, code, "Ordering is unavailable",
				"This cafe's ordering did not answer. Please try again, or order at the counter.")
			return
		}
		noteCut(r, why)
		panic(http.ErrAbortHandler)
	}

	waiting := time.NewTimer(s.headTimeout)
	defer waiting.Stop()

	// step handles one frame and reports whether the exchange is over.
	step := func(f wire.Frame) bool {
		switch f.Type {
		case wire.TypeResponse:
			head, err := wire.DecodeResponse(f)
			if err != nil || started {
				a.reset(st.id, "bad response head")
				fail(http.StatusBadGateway, codeBadGateway, "bad response head")
				return true
			}
			waiting.Stop()
			copyResponseHeader(w.Header(), head.Header, prefix)
			w.Header().Set(headerRequestID, requestIDFrom(r.Context()))
			w.WriteHeader(head.Status)
			started = true
			return false

		case wire.TypeBody:
			if !started {
				a.reset(st.id, "body before head")
				fail(http.StatusBadGateway, codeBadGateway, "body before head")
				return true
			}
			_ = rc.SetWriteDeadline(time.Now().Add(guestIOTimeout))
			if _, err := w.Write(f.Payload); err != nil {
				a.reset(st.id, "guest gone")
				noteCut(r, "guest gone")
				return true
			}
			// Every piece goes out as it lands, so a stream of kitchen
			// events reaches the phone when it happens.
			_ = rc.Flush()
			return false

		case wire.TypeEnd:
			if !started {
				fail(http.StatusBadGateway, codeBadGateway, "end before head")
			}
			return true

		default: // wire.TypeReset; the agent lets nothing else through
			fail(http.StatusBadGateway, codeBadGateway, "venue reset the stream")
			return true
		}
	}

	for {
		select {
		case f := <-st.frames:
			if step(f) {
				return
			}

		case <-st.overrun:
			fail(http.StatusBadGateway, codeBadGateway, "guest too slow")
			return

		case <-a.ctx.Done():
			// What the venue sent before it went is still the guest's: a
			// response that was whole on arrival is not thrown away because
			// the socket closed a moment after it.
			select {
			case f := <-st.frames:
				if step(f) {
					return
				}
			default:
				fail(http.StatusServiceUnavailable, codeVenueOffline, "venue disconnected")
				return
			}

		case <-r.Context().Done():
			a.reset(st.id, "guest gone")
			noteCut(r, "guest gone")
			return

		case <-waiting.C:
			a.reset(st.id, "timed out")
			fail(http.StatusGatewayTimeout, codeTimeout, "timed out")
			return
		}
	}
}

func (s *Server) refuseNotFound(w http.ResponseWriter, r *http.Request) {
	s.refuse(w, r, http.StatusNotFound, codeNotFound, "Not found", "There is nothing at this address.")
}

// refuse answers a guest the relay could not serve. A browser gets a page a
// person can read and staff can explain; anything else gets the same error
// envelope the venue's API uses.
func (s *Server) refuse(w http.ResponseWriter, r *http.Request, status int, code, title, message string) {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		s.writeError(w, status, code, message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	page := `<!doctype html><html lang="en"><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>` + html.EscapeString(title) + `</title>` +
		`<body style="font:16px/1.5 system-ui,sans-serif;margin:15vh auto;max-width:22rem;padding:0 1.5rem">` +
		`<h1 style="font-size:1.25rem">` + html.EscapeString(title) + `</h1>` +
		`<p>` + html.EscapeString(message) + `</p></body></html>`
	if _, err := io.WriteString(w, page); err != nil {
		s.log.Debug("relay: writing a response", "error", err.Error())
	}
}
