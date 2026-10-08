package tunnel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

// errNoSuchPath is a guest path that is not one.
var errNoSuchPath = errors.New("tunnel: not a path a guest can ask for")

// HeaderVia marks a request that came through the relay, and ViaRelay is
// what it says. The tunnel sets it on every request it builds and removes
// whatever arrived under that name, so on the server it is the tunnel's
// word and nobody else's: a request from the venue's own network never
// carries it, and one from the internet always does. The API reads it to
// hold a relayed request to tighter rules (ADR-306).
const (
	HeaderVia = "X-Mez-Via"
	ViaRelay  = "relay"
)

// The back office's two homes on the server: its bundle, and the API it
// speaks to. With Options.BackOffice on, a relayed request for one of them
// is passed as it is. They are written out here, and are the whole list: the
// terminal at "/", the kitchen display and everything else the server has
// are reached by no path the tunnel will carry (ADR-306).
const (
	backOfficeBundle = "/backoffice"
	backOfficeAPI    = "/api/v1/"
)

// forBackOffice reports whether a decoded, already checked path is one of
// the back office's. The bundle's own directory with or without its slash,
// and anything beneath either prefix; "/backofficer" and "/api/v10/x" are
// neither. Case matters, as it does to the server's routes.
func forBackOffice(p string) bool {
	return p == backOfficeBundle || strings.HasPrefix(p, backOfficeBundle+"/") || strings.HasPrefix(p, backOfficeAPI)
}

// guestRequest builds the request the server sees for a guest's frames.
//
// Every guest path is put under mount, unless it is there already: "/t/abc"
// becomes "/order/t/abc", "/order/assets/app.js" stays as it is, and
// "/api/v1/orders" becomes "/order/api/v1/orders", which is not the staff
// API and never will be. There is no list of what a guest may reach, to be
// kept in step with the routes: the guest app's mount is the whole of it,
// and what a guest needs from the server is served from under there.
//
// backOffice is the venue's remote_backoffice setting. Off, the paragraph
// above is the whole rule. On, the two prefixes forBackOffice names are left
// where they are, and nothing else is: the checks before it are the same,
// so a path cannot climb or hide a separator on its way to either.
func guestRequest(ctx context.Context, head wire.RequestHead, body []byte, mount string, backOffice bool) (*http.Request, error) {
	u, err := url.ParseRequestURI(head.Path)
	if err != nil || u.Host != "" || u.Scheme != "" || !climbsNowhere(u.Path) || encodesSeparator(u.RawPath) {
		return nil, errNoSuchPath
	}
	// A path already under the mount is left there. The guest app names its
	// own files and its API by the paths the server has them at, "/order/...",
	// so that the same bundle works on the venue's wifi and through the
	// relay; putting those under the mount again would look for
	// "/order/order/...". Either way the request lands under the mount.
	if !strings.HasPrefix(u.Path, mount) && !(backOffice && forBackOffice(u.Path)) {
		base := strings.TrimSuffix(mount, "/")
		if u.RawPath != "" {
			u.RawPath = base + u.RawPath
		}
		u.Path = base + u.Path
	}

	header := head.Header
	if header == nil {
		header = http.Header{}
	}
	markRelayed(header)
	req := &http.Request{
		Method:        head.Method,
		URL:           u,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Host:          header.Get("X-Forwarded-Host"),
		RequestURI:    u.RequestURI(),
		RemoteAddr:    remoteAddr(header.Get("X-Forwarded-For")),
	}
	return req.WithContext(ctx), nil
}

// markRelayed says, in the request's headers, that it came through the
// relay. The headers came off the socket as JSON, spelled however the far
// end liked, so a copy of the mark that arrived is removed whatever its
// case before the tunnel writes its own.
func markRelayed(header http.Header) {
	for name := range header {
		if strings.EqualFold(name, HeaderVia) {
			delete(header, name)
		}
	}
	header.Set(HeaderVia, ViaRelay)
}

// climbsNowhere reports whether a path stays where it is put. One with a
// ".." in it, or an empty or backslashed segment, could be read by something
// further in as a way out from under the mount, so it is refused whole and
// not cleaned up: a guest's phone never sends one.
func climbsNowhere(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	segments := strings.Split(p[1:], "/")
	for i, seg := range segments {
		if seg == ".." || seg == "." {
			return false
		}
		// A trailing slash is a directory; an empty segment anywhere else
		// is "//".
		if seg == "" && i != len(segments)-1 {
			return false
		}
	}
	return true
}

// encodesSeparator reports whether an escaped path hides a slash or a
// backslash as %2F or %5C. The mount check below reads the decoded path,
// but net/http routes on the escaped one, segment by segment, so
// "/order%2Fx" would pass the check as "/order/x" and then miss every
// pattern under the mount and fall to whatever is at "/". No guest URL
// needs an encoded separator, so one is refused whole.
func encodesSeparator(rawPath string) bool {
	if rawPath == "" {
		return false
	}
	upper := strings.ToUpper(rawPath)
	return strings.Contains(upper, "%2F") || strings.Contains(upper, "%5C")
}

// remoteAddr is the guest's address as the relay reported it, in the form
// net/http gives one. The server logs it; it is not evidence of anything.
func remoteAddr(forwardedFor string) string {
	if ip := net.ParseIP(strings.TrimSpace(forwardedFor)); ip != nil {
		return net.JoinHostPort(ip.String(), "0")
	}
	return "0.0.0.0:0"
}
